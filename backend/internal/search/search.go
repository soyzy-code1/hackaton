// Package search provides a dependency-free web search client that is used by
// the subscription analysis feature.
//
// The client talks to public HTML search engines (no API keys required),
// downloads the most relevant result pages and extracts concrete prices from
// the page text. The extracted prices are the evidence that the AI assistant
// is allowed to rely on when it compares a stored subscription with a cheaper
// alternative.
package search

import (
	"context"
	"encoding/base64"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	// DefaultUserAgent is sent with every request so that search engines
	// answer with the regular HTML layout instead of an anti-bot page.
	DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

	// DefaultResultLimit caps how many hits a single provider returns.
	DefaultResultLimit = 8

	// DefaultFetchPages is the number of result pages downloaded per query.
	DefaultFetchPages = 5

	// DefaultFetchTimeout bounds the download of a single result page.
	DefaultFetchTimeout = 12 * time.Second

	// DefaultMaxExcerpt limits how much page text is handed over to the AI.
	DefaultMaxExcerpt = 900
)

// Hit is a single raw search engine result.
type Hit struct {
	Title   string
	URL     string
	Snippet string
}

// Provider is an adapter for a single search engine.
type Provider interface {
	// Name is used in logs and in the AI payload so that the source of the
	// data stays transparent.
	Name() string
	// Search returns up to limit hits for the given query.
	Search(ctx context.Context, query string, limit int) ([]Hit, error)
}

// Result is a search hit enriched with the prices found on the page.
type Result struct {
	Title   string          `json:"title"`
	URL     string          `json:"url"`
	Snippet string          `json:"snippet,omitempty"`
	Prices  []PriceEvidence `json:"prices"`
	Excerpt string          `json:"excerpt,omitempty"`
}

// Report is the result of one search query including the pages that were
// downloaded and inspected.
type Report struct {
	Query   string   `json:"query"`
	Engine  string   `json:"engine"`
	Results []Result `json:"results"`
}

// Options configures the Service.
type Options struct {
	Providers    []Provider
	Client       *http.Client
	FetchPages   int
	FetchTimeout time.Duration
	MaxExcerpt   int
	// Subject is the subscription the search is about. When it is set, only the
	// hits that mention the subject are kept and the prices are filtered by the
	// words that describe a price in the context.
	Subject string
}

// Service performs a search and enriches the hits with page prices.
type Service struct {
	providers    []Provider
	client       *http.Client
	fetchPages   int
	fetchTimeout time.Duration
	maxExcerpt   int
	subject      string
}

// NewService builds a Service with sane defaults.
func NewService(opts Options) *Service {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	fetchPages := opts.FetchPages
	if fetchPages <= 0 {
		fetchPages = DefaultFetchPages
	}
	fetchTimeout := opts.FetchTimeout
	if fetchTimeout <= 0 {
		fetchTimeout = DefaultFetchTimeout
	}
	maxExcerpt := opts.MaxExcerpt
	if maxExcerpt <= 0 {
		maxExcerpt = DefaultMaxExcerpt
	}
	providers := opts.Providers
	if len(providers) == 0 {
		providers = []Provider{NewSeznamProvider(client), NewBingProvider(client)}
	}
	return &Service{
		providers:    providers,
		client:       client,
		fetchPages:   fetchPages,
		fetchTimeout: fetchTimeout,
		maxExcerpt:   maxExcerpt,
		subject:      strings.TrimSpace(opts.Subject),
	}
}

// Subject returns the subscription the service searches for.
func (s *Service) Subject() string { return s.subject }

// Providers exposes the configured engines (used by tests).
func (s *Service) Providers() []Provider { return s.providers }

// Search runs the query through the configured providers (the first one that
// answers wins) and downloads the top pages to look for concrete prices.
func (s *Service) Search(ctx context.Context, query string) (*Report, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}
	report := &Report{Query: query, Results: []Result{}}
	var lastErr error
	var fallback []Hit
	for _, provider := range s.providers {
		hits, err := provider.Search(ctx, query, DefaultResultLimit)
		if err != nil {
			lastErr = err
			continue
		}
		if len(hits) == 0 {
			continue
		}
		if len(fallback) == 0 {
			fallback = hits
		}
		if s.subject != "" {
			relevant := FilterRelevant(hits, s.subject)
			// Keep the raw hits when the engine returned only a few results:
			// the model is able to judge them better than a hard filter.
			if len(relevant) >= 3 {
				hits = relevant
			} else if len(relevant) > 0 {
				hits = append(relevant, hits...)
			}
		}
		report.Engine = provider.Name()
		report.Results = s.enrich(ctx, hits)
		return report, nil
	}
	if len(fallback) > 0 && s.subject != "" {
		// Nothing relevant was found, but the engine did answer: keep the raw
		// hits so that the model can still judge them.
		report.Engine = s.providers[0].Name()
		report.Results = s.enrich(ctx, fallback)
		return report, nil
	}
	if lastErr != nil {
		return report, lastErr
	}
	return report, nil
}

// enrich downloads the top pages of the hit list and extracts prices.
func (s *Service) enrich(ctx context.Context, hits []Hit) []Result {
	limit := s.fetchPages
	if limit > len(hits) {
		limit = len(hits)
	}
	if limit <= 0 {
		return []Result{}
	}
	results := make([]Result, len(hits))
	for i, hit := range hits {
		results[i] = Result{Title: hit.Title, URL: hit.URL, Snippet: hit.Snippet, Prices: []PriceEvidence{}}
	}
	type fetched struct {
		idx  int
		body string
		err  error
	}
	resultsCh := make(chan fetched, limit)
	for i := 0; i < limit; i++ {
		go func(idx int, target string) {
			body, err := s.fetchPage(ctx, target)
			resultsCh <- fetched{idx: idx, body: body, err: err}
		}(i, hits[i].URL)
	}
	for i := 0; i < limit; i++ {
		res := <-resultsCh
		if res.err != nil {
			continue
		}
		text := StripTags(res.body)
		prices := ExtractRelevantPrices(text)
		evidence := make([]PriceEvidence, 0, len(prices))
		for _, p := range prices {
			evidence = append(evidence, p.Evidence())
		}
		results[res.idx].Prices = evidence
		results[res.idx].Excerpt = excerptAround(text, evidence, s.maxExcerpt)
	}
	return results
}

// fetchPage downloads a single page and returns its raw HTML.
func (s *Service) fetchPage(ctx context.Context, target string) (string, error) {
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		return "", fmt.Errorf("unsupported url %q", target)
	}
	fetchCtx, cancel := context.WithTimeout(ctx, s.fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	for len(buf) < 512*1024 {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf), nil
}

// excerptAround builds a short excerpt of the page text, preferring the part
// of the text where the prices were found.
func excerptAround(text string, evidence []PriceEvidence, limit int) string {
	if limit <= 0 {
		limit = DefaultMaxExcerpt
	}
	if len(text) <= limit {
		return text
	}
	start := 0
	if len(evidence) > 0 {
		if idx := strings.Index(text, evidence[0].Raw); idx > 0 {
			start = idx - limit/3
			if start < 0 {
				start = 0
			}
		}
	}
	end := start + limit
	if end > len(text) {
		end = len(text)
	}
	return strings.TrimSpace(text[start:end])
}

var (
	scriptRe = regexp.MustCompile(`(?is)<script[\s\S]*?</script>`)
	styleRe  = regexp.MustCompile(`(?is)<style[\s\S]*?</style>`)
	tagRe    = regexp.MustCompile(`(?s)<[^>]+>`)
)

// StripTags removes scripts, styles and markup and collapses whitespace.
func StripTags(source string) string {
	cleaned := scriptRe.ReplaceAllString(source, " ")
	cleaned = styleRe.ReplaceAllString(cleaned, " ")
	cleaned = tagRe.ReplaceAllString(cleaned, " ")
	cleaned = html.UnescapeString(cleaned)
	return strings.Join(strings.Fields(cleaned), " ")
}

// Normalize lowers a string and drops everything but letters and digits so
// that search queries and subscription titles can be compared.
func Normalize(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r >= 0x0400 {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// --- Seznam ---------------------------------------------------------------

// SeznamProvider scrapes search.seznam.cz. It understands multi-word Cyrillic
// queries, which makes it the primary engine for Russian subscriptions.
type SeznamProvider struct {
	Client  *http.Client
	BaseURL string
}

// NewSeznamProvider builds a Seznam provider.
func NewSeznamProvider(client *http.Client) *SeznamProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &SeznamProvider{Client: client, BaseURL: "https://search.seznam.cz/"}
}

var seznamHitRe = regexp.MustCompile(`<h3[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)

// Name implements Provider.
func (p *SeznamProvider) Name() string { return "seznam" }

// Search implements Provider.
func (p *SeznamProvider) Search(ctx context.Context, query string, limit int) ([]Hit, error) {
	endpoint := p.BaseURL
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	endpoint += "?q=" + url.QueryEscape(query)
	body, err := fetchHTML(ctx, p.Client, endpoint)
	if err != nil {
		return nil, err
	}
	hits := []Hit{}
	seen := map[string]bool{}
	for _, match := range seznamHitRe.FindAllStringSubmatch(body, -1) {
		link := strings.TrimSpace(match[1])
		title := StripTags(match[2])
		if link == "" || title == "" {
			continue
		}
		if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
			continue
		}
		if seen[link] {
			continue
		}
		seen[link] = true
		hits = append(hits, Hit{Title: title, URL: link})
		if limit > 0 && len(hits) >= limit {
			break
		}
	}
	return hits, nil
}

// --- Bing -----------------------------------------------------------------

// BingProvider scrapes the classic Bing HTML result list. It is used as a
// fallback when the primary engine does not return anything useful.
type BingProvider struct {
	Client  *http.Client
	BaseURL string
}

// NewBingProvider builds a Bing provider.
func NewBingProvider(client *http.Client) *BingProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &BingProvider{Client: client, BaseURL: "https://www.bing.com/search"}
}

var (
	bingBlockRe   = regexp.MustCompile(`(?is)<li class="b_algo"[\s\S]*?</li>`)
	bingTitleRe   = regexp.MustCompile(`(?is)<h2[^>]*>\s*<a[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)
	bingSnippetRe = regexp.MustCompile(`(?is)<p[^>]*>([\s\S]*?)</p>`)
)

// Name implements Provider.
func (p *BingProvider) Name() string { return "bing" }

// Search implements Provider.
func (p *BingProvider) Search(ctx context.Context, query string, limit int) ([]Hit, error) {
	endpoint := p.BaseURL + "?q=" + url.QueryEscape(query) + "&count=20&setlang=ru"
	body, err := fetchHTML(ctx, p.Client, endpoint)
	if err != nil {
		return nil, err
	}
	hits := []Hit{}
	seen := map[string]bool{}
	appendHit := func(rawURL, title, snippet string) {
		link := decodeBingURL(rawURL)
		if link == "" || title == "" {
			return
		}
		if seen[link] {
			return
		}
		seen[link] = true
		hits = append(hits, Hit{Title: title, URL: link, Snippet: snippet})
	}
	for _, block := range bingBlockRe.FindAllString(body, -1) {
		titleMatch := bingTitleRe.FindStringSubmatch(block)
		if titleMatch == nil {
			continue
		}
		snippet := ""
		if snippetMatch := bingSnippetRe.FindStringSubmatch(block); snippetMatch != nil {
			snippet = StripTags(snippetMatch[1])
		}
		appendHit(titleMatch[1], StripTags(titleMatch[2]), snippet)
		if limit > 0 && len(hits) >= limit {
			return hits, nil
		}
	}
	if len(hits) == 0 {
		// Fallback for layouts where the b_algo list is not present.
		for _, titleMatch := range bingTitleRe.FindAllStringSubmatch(body, -1) {
			appendHit(titleMatch[1], StripTags(titleMatch[2]), "")
			if limit > 0 && len(hits) >= limit {
				break
			}
		}
	}
	return hits, nil
}

// decodeBingURL unwraps the /ck/a redirect used by Bing.
func decodeBingURL(raw string) string {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if target := parsed.Query().Get("u"); strings.HasPrefix(target, "a1") {
		payload := target[2:]
		if decoded, err := base64.RawURLEncoding.DecodeString(payload); err == nil {
			return string(decoded)
		}
		if decoded, err := base64.URLEncoding.DecodeString(payload); err == nil {
			return string(decoded)
		}
	}
	if parsed.Host == "www.bing.com" && strings.HasPrefix(parsed.Path, "/ck/") {
		return ""
	}
	if !strings.HasPrefix(parsed.Scheme, "http") {
		return ""
	}
	return parsed.String()
}

// --- shared HTTP ----------------------------------------------------------

// fetchHTML performs a plain GET request and returns the response body.
func fetchHTML(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ru-RU,ru;q=0.9,en;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d for %s", resp.StatusCode, endpoint)
	}
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	for len(buf) < 512*1024 {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf), nil
}
