package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"invoicekeeper/internal/apperr"
	"invoicekeeper/internal/models"
	"invoicekeeper/internal/search"
)

// subscriptionAnalysisSystemPrompt instructs the model to compare the
// subscriptions stored in the system with what the web search found.
const subscriptionAnalysisSystemPrompt = `Ты — аналитик подписок приложения Invoice Keeper. Твоя задача — найти более выгодные альтернативы подпискам, которые уже заведены в системе, и показать, на сколько дешевле выйдет переход.

Ты работаешь с интернет-поиском: сначала получаешь подписки пользователя, потом вызываешь функцию web_search с поисковым запросом, получаешь найденные страницы с ценами и предлагаешь альтернативы.

Правила:
1. Для КАЖДОЙ подписки сначала вызови web_search с запросом вида «<название> цена подписки в месяц рублей» и с запросом по её категории «<категория> подписка цена в месяц дешевле аналог». Не задавай вопросы вслепую.
2. Используй ТОЛЬКО факты из результатов поиска. Не выдумывай цены, сервисы и ссылки. У каждой альтернативы должна быть ссылка на источник (source_url) и короткая цитата (evidence), подтверждающая цену.
3. Альтернатива обязана быть тематически и функционально похожа: стриминг музыки — для стриминга музыки, облачное хранилище — для облачного хранилища, стриминг видео — для стриминга видео, облачный диск — для облачного диска. Ориентируйся на поле category у подписки. Если функционал сервисов разный, такая пара не подходит.
4. Сравнивай цены за один и тот же период. Если в источнике цена указана за год или за месяц, приведи её к месяцу и обязательно опиши это в поле reason.
5. Для каждой альтернативы заполни: payment_id, payment_title, service_name, alternative_price, alternative_currency, similarity, reason, source_url, evidence.
6. В alternative_currency пиши ISO-код валюты: RUB, USD, EUR.
7. В similarity пиши high, если функционал совпадает полностью, medium, если совпадает основная функция, и не предлагай варианты с low.
8. Альтернатива выгодна, только если она дешевле текущей подписки за тот же период. Цена должна быть положительной.
9. Если подходящих альтернатив нет — верни пустой массив alternatives.
10. В поле comment дай краткое резюме на русском языке: что нашлось, на сколько дешевле в месяц и в год.
11. Отвечай ТОЛЬКО валидным JSON без markdown-блоков и без текста вне JSON.

Формат ответа:
{"alternatives":[{"payment_id":12,"payment_title":"Spotify Premium","service_name":"Яндекс Плюс","alternative_price":199,"alternative_currency":"RUB","similarity":"high","reason":"Стриминг музыки без рекламы, дешевле на 100 ₽ в месяц","source_url":"https://example.com/pricing","evidence":"199 рублей :: Стоимость подписки — 199 рублей в месяц"}],"comment":"Найдена 1 более выгодная альтернатива."}`

// analysisSubscription is a single recurring payment considered by the analysis.
type analysisSubscription struct {
	ID          int
	Title       string
	Amount      decimal.Decimal
	Currency    string
	Periodicity string
	// Category is the thematic group of the service ("стриминг музыки",
	// "облачное хранилище", ...). It is used to find alternatives with a
	// similar purpose.
	Category string
}

// alternativeOut describes a cheaper alternative found on the web.
type alternativeOut struct {
	PaymentID           int             `json:"payment_id"`
	PaymentTitle        string          `json:"payment_title"`
	CurrentPrice        decimal.Decimal `json:"current_price"`
	CurrentCurrency     string          `json:"current_currency"`
	ServiceName         string          `json:"service_name"`
	AlternativePrice    decimal.Decimal `json:"alternative_price"`
	AlternativeCurrency string          `json:"alternative_currency"`
	MonthlySaving       decimal.Decimal `json:"monthly_saving"`
	AnnualSaving        decimal.Decimal `json:"annual_saving"`
	SavingPercent       decimal.Decimal `json:"saving_percent"`
	Similarity          string          `json:"similarity"`
	Reason              string          `json:"reason"`
	SourceURL           string          `json:"source_url"`
	Evidence            string          `json:"evidence"`
}

// subscriptionAnalysisOut describes one analysed subscription.
type subscriptionAnalysisOut struct {
	ID          int              `json:"id"`
	Title       string           `json:"title"`
	Amount      decimal.Decimal  `json:"amount"`
	Currency    string           `json:"currency"`
	Periodicity string           `json:"periodicity"`
	Searched    bool             `json:"searched"`
	QueryCount  int              `json:"query_count"`
	PriceHits   int              `json:"price_hits"`
	BestSaving  *decimal.Decimal `json:"best_saving"`
}

// subscriptionAnalysisResponse is the payload of the analysis endpoint.
type subscriptionAnalysisResponse struct {
	AnalyzedAt      time.Time                 `json:"analyzed_at"`
	Summary         string                    `json:"summary"`
	BestAlternative *alternativeOut           `json:"best_alternative"`
	Alternatives    []alternativeOut          `json:"alternatives"`
	Subscriptions   []subscriptionAnalysisOut `json:"subscriptions"`
	SearchQueries   int                       `json:"search_queries"`
	Warnings        []string                  `json:"warnings"`
}

// aiAlternative is one alternative as returned by the model.
type aiAlternative struct {
	PaymentID           int             `json:"payment_id"`
	PaymentTitle        string          `json:"payment_title"`
	ServiceName         string          `json:"service_name"`
	AlternativePrice    json.RawMessage `json:"alternative_price"`
	AlternativeCurrency string          `json:"alternative_currency"`
	Similarity          string          `json:"similarity"`
	Reason              string          `json:"reason"`
	SourceURL           string          `json:"source_url"`
	Evidence            string          `json:"evidence"`
}

// aiAnalysis is the structured answer of the model.
type aiAnalysis struct {
	Alternatives []aiAlternative `json:"alternatives"`
	Comment      string          `json:"comment"`
}

// analyzeSubscriptions compares the subscriptions stored in the system with
// live web prices and returns the most advantageous alternatives.
func (s *Server) analyzeSubscriptions(c *gin.Context) {
	if !s.Cfg.GigaChatConfigured() {
		writeAppError(c, apperr.New(http.StatusServiceUnavailable, "GigaChat не настроен", "GIGACHAT_NOT_CONFIGURED"))
		return
	}
	if !s.Cfg.SearchConfigured() {
		writeAppError(c, apperr.New(http.StatusServiceUnavailable, "Поиск в интернете отключён", "SEARCH_DISABLED"))
		return
	}
	user, ok := currentUser(c)
	if !ok {
		writeAppError(c, apperr.AuthRequired)
		return
	}

	isActive := true
	payments, err := s.St.ListRecurringPayments(nil, &isActive)
	if err != nil {
		writeAppError(c, apperr.New(http.StatusInternalServerError, "Не удалось загрузить подписки", "PAYMENTS_LOAD_FAILED"))
		return
	}
	if user.Role != "admin" {
		filtered := payments[:0]
		for _, item := range payments {
			for _, assignedID := range item.AssignedUserIDs {
				if assignedID == user.ID {
					filtered = append(filtered, item)
					break
				}
			}
		}
		payments = filtered
	}
	if len(payments) == 0 {
		writeAppError(c, apperr.New(http.StatusUnprocessableEntity, "Нет активных подписок для анализа", "NO_SUBSCRIPTIONS"))
		return
	}

	subscriptions := make([]analysisSubscription, 0, len(payments))
	for _, p := range payments {
		subscriptions = append(subscriptions, analysisSubscription{
			ID:          p.ID,
			Title:       p.Title,
			Amount:      p.Amount,
			Currency:    p.Currency,
			Periodicity: p.Periodicity,
			Category:    detectCategory(p.Title),
		})
	}
	// The most expensive subscriptions have the largest potential saving, so
	// they are analysed first when the list has to be truncated.
	sort.SliceStable(subscriptions, func(i, j int) bool {
		left, _ := amountInRUB(subscriptions[i].Amount, subscriptions[i].Currency)
		right, _ := amountInRUB(subscriptions[j].Amount, subscriptions[j].Currency)
		return left.GreaterThan(right)
	})
	if limit := s.Cfg.SearchMaxPayments; limit > 0 && len(subscriptions) > limit {
		subscriptions = subscriptions[:limit]
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 150*time.Second)
	defer cancel()

	reports, queries, warnings := s.gatherSubscriptionEvidence(ctx, subscriptions)

	token, err := s.gigaChatToken()
	if err != nil {
		writeAppError(c, apperr.New(http.StatusBadGateway, "Не удалось получить токен GigaChat", "GIGACHAT_AUTH_FAILED"))
		return
	}

	analysis, err := s.runSubscriptionAnalysis(ctx, token, subscriptions, reports)
	if err != nil {
		writeAppError(c, apperr.New(http.StatusBadGateway, err.Error(), "ANALYSIS_FAILED"))
		return
	}

	alternatives := buildAlternatives(subscriptions, analysis.Alternatives)
	response := subscriptionAnalysisResponse{
		AnalyzedAt:    time.Now().UTC(),
		Alternatives:  alternatives,
		Subscriptions: make([]subscriptionAnalysisOut, 0, len(subscriptions)),
		SearchQueries: queries,
		Warnings:      warnings,
	}
	if response.Warnings == nil {
		response.Warnings = []string{}
	}
	best := map[int]decimal.Decimal{}
	for _, alt := range alternatives {
		if current, ok := best[alt.PaymentID]; !ok || alt.MonthlySaving.GreaterThan(current) {
			best[alt.PaymentID] = alt.MonthlySaving
		}
	}
	for _, sub := range subscriptions {
		out := subscriptionAnalysisOut{
			ID: sub.ID, Title: sub.Title, Amount: sub.Amount, Currency: sub.Currency,
			Periodicity: sub.Periodicity, Searched: len(reports[sub.ID]) > 0,
		}
		for _, report := range reports[sub.ID] {
			out.QueryCount++
			for _, result := range report.Results {
				out.PriceHits += len(result.Prices)
			}
		}
		if saving, ok := best[sub.ID]; ok {
			value := saving
			out.BestSaving = &value
		}
		response.Subscriptions = append(response.Subscriptions, out)
	}
	if len(alternatives) > 0 {
		response.BestAlternative = &alternatives[0]
		response.Summary = buildSummary(alternatives[0], analysis.Comment)
	} else {
		response.Summary = firstNonEmpty(analysis.Comment, "Более выгодных альтернатив не найдено.")
	}
	if len(alternatives) == 0 && len(warnings) == 0 {
		response.Warnings = append(response.Warnings, "Поиск не нашёл подтверждённых цен по подпискам.")
	}
	c.JSON(http.StatusOK, response)
}

// searchService builds the web search client from the configuration.
func (s *Server) searchService() *search.Service {
	return s.searchServiceFor("")
}

// searchServiceFor builds the web search client bound to a single
// subscription. The subject is used to drop the results that talk about
// something else.
func (s *Server) searchServiceFor(subject string) *search.Service {
	timeout := time.Duration(s.Cfg.SearchTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	seznam := search.NewSeznamProvider(client)
	bing := search.NewBingProvider(client)
	providers := []search.Provider{seznam, bing}
	if strings.EqualFold(strings.TrimSpace(s.Cfg.SearchEngine), "bing") {
		providers = []search.Provider{bing, seznam}
	}
	return search.NewService(search.Options{
		Providers:    providers,
		Client:       client,
		FetchPages:   s.Cfg.SearchFetchPages,
		FetchTimeout: timeout,
		Subject:      subject,
	})
}

// buildSearchQueries creates the search queries used for one subscription.
// The exact name in quotes gives the best relevance, the second query looks
// for a cheaper alternative of the same kind.
func buildSearchQueries(title, category string) []string {
	cleaned := strings.TrimSpace(title)
	if cleaned == "" {
		return nil
	}
	queries := []string{
		fmt.Sprintf("\"%s\" цена подписки рублей в месяц", cleaned),
	}
	category = strings.TrimSpace(category)
	if category != "" {
		queries = append(queries, fmt.Sprintf("%s подписка цена в месяц дешевле аналог", category))
	}
	return queries
}

// categoryHints maps the words of a subscription title to the thematic group
// of the service. The group is used to look for services with the same
// purpose, so that the comparison stays meaningful.
var categoryHints = []struct {
	keywords []string
	category string
}{
	{[]string{"spotify", "apple music", "music", "музык", "soundcloud", "deezer", "tidal", "yandex music", "яндекс плюс", "плюс"}, "стриминг музыки"},
	{[]string{"netflix", "кинопоиск", "окко", "ivi", "ivi.ru", "video", "видео", "кино", "premiere", "плюс"}, "стриминг видео"},
	{[]string{"playstation", "xbox", "nintendo", "игр", "game", "steam", "игров"}, "игровые сервисы"},
	{[]string{"google one", "icloud", "dropbox", "yandex disk", "яндекс диск", "диск", "cloud", "облак", "хранилищ"}, "облачное хранилище"},
	{[]string{"github", "gitlab", "bitbucket", "code", "разработк", "программир", "ide", "jetbrains"}, "сервисы для разработки"},
	{[]string{"chatgpt", "openai", "gpt", "нейросет", "ии ", "gemini", "claude"}, "ИИ-сервисы"},
	{[]string{"zoom", "meet", "teams", "skype", "видеоконференц", "конференц"}, "видеоконференции"},
	{[]string{"notion", "evernote", "заметк", "дневник", "obsidian"}, "заметки и базы знаний"},
	{[]string{"fitness", "фитнес", "трениров", "sport", "спорт", "йога"}, "фитнес и здоровье"},
	{[]string{"adobe", "figma", "photoshop", "canva", "дизайн", "графическ"}, "дизайн и графика"},
	{[]string{"mail", "почт", "email", "почтов"}, "почта"},
	{[]string{"avito", "объявлен", "marketplace", "market", "маркет"}, "маркетплейсы"},
	{[]string{"antivirus", "антивирус", "kaspersky", " Kaspersky", " Dr web", "безопасност"}, "антивирусы"},
	{[]string{"vpn", " впн"}, "VPN"},
}

// detectCategory guesses the thematic group of a subscription by its title.
func detectCategory(title string) string {
	lowered := strings.ToLower(strings.TrimSpace(title))
	if lowered == "" {
		return ""
	}
	for _, hint := range categoryHints {
		for _, keyword := range hint.keywords {
			if strings.Contains(lowered, keyword) {
				return hint.category
			}
		}
	}
	return ""
}

// gatherSubscriptionEvidence runs the web search for every subscription and
// returns the reports grouped by subscription id.
func (s *Server) gatherSubscriptionEvidence(ctx context.Context, subs []analysisSubscription) (map[int][]search.Report, int, []string) {
	reports := make(map[int][]search.Report, len(subs))
	warnings := []string{}
	queries := 0

	type job struct {
		sub  analysisSubscription
		term string
	}
	jobs := make([]job, 0, len(subs)*2)
	for _, sub := range subs {
		for _, term := range buildSearchQueries(sub.Title, sub.Category) {
			jobs = append(jobs, job{sub: sub, term: term})
		}
	}

	const workers = 5
	queue := make(chan job)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range queue {
				svc := s.searchServiceFor(item.sub.Title)
				report, err := svc.Search(ctx, item.term)
				mu.Lock()
				if err != nil || report == nil || len(report.Results) == 0 {
					mu.Unlock()
					continue
				}
				queries++
				reports[item.sub.ID] = append(reports[item.sub.ID], *report)
				mu.Unlock()
			}
		}()
	}
	for _, item := range jobs {
		select {
		case queue <- item:
		case <-ctx.Done():
			warnings = append(warnings, "Поиск прерван по таймауту, часть подписок не проверена.")
		}
	}
	close(queue)
	wg.Wait()

	if len(reports) == 0 {
		warnings = append(warnings, "Поисковые системы не вернули результатов.")
	}
	return reports, queries, warnings
}

// runSubscriptionAnalysis calls GigaChat with a web_search function and
// returns the structured comparison.
func (s *Server) runSubscriptionAnalysis(ctx context.Context, token string, subs []analysisSubscription, reports map[int][]search.Report) (*aiAnalysis, error) {
	payload := buildAnalysisPayload(subs, reports)
	messages := []map[string]interface{}{
		{"role": "system", "content": subscriptionAnalysisSystemPrompt},
		{"role": "user", "content": payload},
	}
	functions := []map[string]interface{}{
		{
			"name":        "web_search",
			"description": "Поиск актуальной информации в интернете: цены подписок, условия и альтернативы сервисов. Возвращает найденные страницы и цены, найденные на них.",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Поисковый запрос на русском языке, например: \"Яндекс Плюс цена подписки в месяц\"",
					},
				},
				"required": []string{"query"},
			},
		},
	}

	content, err := s.gigaChatFunctionLoop(ctx, token, messages, functions, func(name string, args map[string]interface{}) (string, error) {
		if name != "web_search" {
			return "", fmt.Errorf("неизвестная функция %q", name)
		}
		query, _ := args["query"].(string)
		query = strings.TrimSpace(query)
		if query == "" {
			return "", fmt.Errorf("пустой поисковый запрос")
		}
		// The subject of the search is taken from the subscriptions of the
		// user, so that the results about other services are dropped.
		svc := s.searchServiceFor(subjectForQuery(query, subs))
		report, err := svc.Search(ctx, query)
		if err != nil {
			return "", err
		}
		encoded, err := json.Marshal(map[string]interface{}{
			"query":   report.Query,
			"engine":  report.Engine,
			"results": trimSearchResults(report.Results),
		})
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	})
	if err != nil {
		return nil, err
	}

	analysis, parseErr := parseAIAnalysis(content)
	if parseErr == nil {
		return analysis, nil
	}
	// The model did not answer with valid JSON: ask it once more, but with
	// the strict JSON mode enabled.
	return s.gigaChatStructuredRetry(ctx, token, messages, content)
}

// subjectFromQuery guesses the subscription the search query is about. The
// query starts either with the quoted name of the service or with the word
// "альтернатива", so the first meaningful words are the subject.
func subjectFromQuery(query string) string {
	cleaned := strings.TrimSpace(query)
	cleaned = strings.Trim(cleaned, "\"")
	words := strings.Fields(cleaned)
	out := []string{}
	for _, word := range words {
		lowered := strings.ToLower(word)
		switch lowered {
		case "цена", "цены", "цену", "стоимость", "подписки", "подписка", "подписку",
			"рублей", "рубля", "в", "месяц", "месяца", "и", "аналог", "альтернатива",
			"дешевле", "тариф", "тарифы", "в месяц", "за":
			continue
		}
		out = append(out, word)
		if len(out) >= 2 {
			break
		}
	}
	return strings.Join(out, " ")
}

// subjectForQuery picks the subscription of the user whose name is mentioned
// in the search query. The longest matching name wins.
func subjectForQuery(query string, subs []analysisSubscription) string {
	lowered := strings.ToLower(query)
	best := ""
	bestLen := 0
	for _, sub := range subs {
		title := strings.TrimSpace(sub.Title)
		if title == "" {
			continue
		}
		if strings.Contains(lowered, strings.ToLower(title)) && len(title) > bestLen {
			best = title
			bestLen = len(title)
		}
	}
	if best != "" {
		return best
	}
	return subjectFromQuery(query)
}

// gigaChatFunctionLoop runs the chat completion loop and executes the
// function calls requested by the model.
func (s *Server) gigaChatFunctionLoop(ctx context.Context, token string, messages []map[string]interface{}, functions []map[string]interface{}, call func(name string, args map[string]interface{}) (string, error)) (string, error) {
	const maxSteps = 6
	for step := 0; step < maxSteps; step++ {
		payload := map[string]interface{}{
			"model":         s.Cfg.GigaChatModel,
			"messages":      messages,
			"temperature":   0.1,
			"functions":     functions,
			"function_call": "auto",
		}
		result, err := s.gigaChatComplete(ctx, token, payload)
		if err != nil {
			return "", err
		}
		message := result.Choices[0].Message
		if message.FunctionCall == nil {
			return message.Content, nil
		}
		args := decodeFunctionArguments(message.FunctionCall.Arguments)
		content, err := call(message.FunctionCall.Name, args)
		if err != nil {
			content = jsonErrorPayload(err)
		}
		assistant := map[string]interface{}{
			"role":         "assistant",
			"content":      message.Content,
			"function_call": map[string]interface{}{
				"name":      message.FunctionCall.Name,
				"arguments": args,
			},
		}
		messages = append(messages, assistant)
		messages = append(messages, map[string]interface{}{
			"role":    "function",
			"name":    message.FunctionCall.Name,
			"content": content,
		})
	}
	return "", fmt.Errorf("GigaChat не завершил анализ за %d шагов", maxSteps)
}

// gigaChatStructuredRetry repeats the last exchange in strict JSON mode.
func (s *Server) gigaChatStructuredRetry(ctx context.Context, token string, messages []map[string]interface{}, previous string) (*aiAnalysis, error) {
	history := append([]map[string]interface{}{}, messages...)
	history = append(history, map[string]interface{}{
		"role":    "assistant",
		"content": previous,
	})
	history = append(history, map[string]interface{}{
		"role":    "user",
		"content": "Повтори ответ строго в формате JSON без markdown и без текста вне JSON.",
	})
	payload := map[string]interface{}{
		"model":           s.Cfg.GigaChatModel,
		"messages":        history,
		"temperature":     0.1,
		"response_format": map[string]interface{}{"type": "json_object"},
	}
	result, err := s.gigaChatComplete(ctx, token, payload)
	if err != nil {
		return nil, err
	}
	analysis, err := parseAIAnalysis(result.Choices[0].Message.Content)
	if err != nil {
		return nil, fmt.Errorf("GigaChat вернул ответ, который не удалось разобрать")
	}
	return analysis, nil
}

// gigaChatComplete performs a single chat completion request.
func (s *Server) gigaChatComplete(ctx context.Context, token string, payload map[string]interface{}) (*gigaCompletionResponse, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimRight(s.Cfg.GigaChatAPIURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(encoded)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.gigaHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("GigaChat недоступен")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GigaChat вернул HTTP %d", resp.StatusCode)
	}
	var result gigaCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("GigaChat вернул некорректный ответ")
	}
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("GigaChat вернул пустой ответ")
	}
	return &result, nil
}

// decodeFunctionArguments decodes the arguments of a function call. GigaChat
// returns them as a JSON object, but a JSON encoded string is accepted too.
func decodeFunctionArguments(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return map[string]interface{}{}
	}
	if strings.HasPrefix(trimmed, `"`) {
		var inner string
		if err := json.Unmarshal(raw, &inner); err == nil {
			inner = strings.TrimSpace(inner)
			if strings.HasPrefix(inner, "{") {
				trimmed = inner
				raw = json.RawMessage(inner)
			}
		}
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]interface{}{}
	}
	return out
}

// jsonErrorPayload renders an error as a valid JSON string so that GigaChat
// accepts it as a function result.
func jsonErrorPayload(err error) string {
	encoded, marshalErr := json.Marshal(map[string]interface{}{
		"error":   true,
		"message": err.Error(),
		"results": []interface{}{},
	})
	if marshalErr != nil {
		return `{"error":true,"results":[]}`
	}
	return string(encoded)
}

// parseAIAnalysis extracts the JSON answer of the model.
func parseAIAnalysis(content string) (*aiAnalysis, error) {
	cleaned := strings.TrimSpace(content)
	if cleaned == "" {
		return nil, fmt.Errorf("пустой ответ")
	}
	if strings.HasPrefix(cleaned, "```") {
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(strings.TrimSpace(cleaned), "```")
	}
	start := strings.Index(cleaned, "{")
	end := strings.LastIndex(cleaned, "}")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("в ответе нет JSON")
	}
	var analysis aiAnalysis
	if err := json.Unmarshal([]byte(cleaned[start:end+1]), &analysis); err != nil {
		return nil, err
	}
	return &analysis, nil
}

// buildAnalysisPayload renders the subscriptions and the search evidence as
// the user message for the model.
func buildAnalysisPayload(subs []analysisSubscription, reports map[int][]search.Report) string {
	type reportPayload struct {
		Query   string                   `json:"query"`
		Engine  string                   `json:"engine"`
		Results []map[string]interface{} `json:"results"`
	}
	type subPayload struct {
		ID           int             `json:"id"`
		Title        string          `json:"title"`
		Category     string          `json:"category,omitempty"`
		CurrentPrice decimal.Decimal `json:"current_price"`
		Currency     string          `json:"currency"`
		Periodicity  string          `json:"periodicity"`
		Searches     []reportPayload `json:"searches"`
	}
	payload := make([]subPayload, 0, len(subs))
	for _, sub := range subs {
		item := subPayload{
			ID: sub.ID, Title: sub.Title, CurrentPrice: sub.Amount,
			Currency: sub.Currency, Periodicity: sub.Periodicity,
			Category: sub.Category, Searches: []reportPayload{},
		}
		for _, report := range reports[sub.ID] {
			item.Searches = append(item.Searches, reportPayload{
				Query:   report.Query,
				Engine:  report.Engine,
				Results: trimSearchResults(report.Results),
			})
		}
		payload = append(payload, item)
	}
	encoded, err := json.Marshal(map[string]interface{}{
		"subscriptions": payload,
		"note":          "Сравни текущую цену подписки с ценами из результатов поиска и предложи только более дешёвые и функционально похожие сервисы.",
	})
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// trimSearchResults keeps only the results that carry usable price evidence
// and limits the size of the payload.
func trimSearchResults(results []search.Result) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(results))
	for _, result := range results {
		if len(result.Prices) == 0 && result.Snippet == "" {
			continue
		}
		prices := result.Prices
		if len(prices) > 5 {
			prices = prices[:5]
		}
		excerpt := result.Excerpt
		if len([]rune(excerpt)) > 500 {
			excerpt = string([]rune(excerpt)[:500])
		}
		entry := map[string]interface{}{
			"title":   result.Title,
			"url":     result.URL,
			"excerpt": excerpt,
		}
		if result.Snippet != "" {
			entry["snippet"] = result.Snippet
		}
		if len(prices) > 0 {
			entry["prices"] = prices
		}
		out = append(out, entry)
		if len(out) >= 6 {
			break
		}
	}
	return out
}

// buildAlternatives converts the model answer into the response items and
// recalculates the savings with the prices from the database.
func buildAlternatives(subs []analysisSubscription, ai []aiAlternative) []alternativeOut {
	byID := make(map[int]analysisSubscription, len(subs))
	for _, sub := range subs {
		byID[sub.ID] = sub
	}
	out := []alternativeOut{}
	for _, item := range ai {
		sub, ok := byID[item.PaymentID]
		if !ok {
			continue
		}
		serviceName := strings.TrimSpace(item.ServiceName)
		if serviceName == "" {
			continue
		}
		// The alternative must be a different service.
		if search.Normalize(serviceName) == search.Normalize(sub.Title) {
			continue
		}
		price, ok := parseAIPrice(item.AlternativePrice)
		if !ok || price.LessThanOrEqual(decimal.Zero) {
			continue
		}
		currency := strings.ToUpper(strings.TrimSpace(item.AlternativeCurrency))
		if currency == "" {
			currency = sub.Currency
		}
		currentRUB, ok := amountInRUB(sub.Amount, sub.Currency)
		if !ok {
			continue
		}
		alternativeRUB, ok := amountInRUB(price, currency)
		if !ok {
			continue
		}
		saving := currentRUB.Sub(alternativeRUB)
		if !saving.GreaterThan(decimal.Zero) {
			continue
		}
		similarity := normalizeSimilarity(item.Similarity)
		if similarity == "" || similarity == "low" {
			continue
		}
		percent := decimal.Zero
		if currentRUB.GreaterThan(decimal.Zero) {
			percent = saving.Mul(decimal.NewFromInt(100)).Div(currentRUB).Round(1)
		}
		out = append(out, alternativeOut{
			PaymentID:           sub.ID,
			PaymentTitle:        sub.Title,
			CurrentPrice:        sub.Amount,
			CurrentCurrency:     sub.Currency,
			ServiceName:         serviceName,
			AlternativePrice:    price,
			AlternativeCurrency: currency,
			MonthlySaving:       saving.Round(2),
			AnnualSaving:        saving.Mul(decimal.NewFromInt(12)).Round(2),
			SavingPercent:       percent,
			Similarity:          similarity,
			Reason:              strings.TrimSpace(item.Reason),
			SourceURL:           strings.TrimSpace(item.SourceURL),
			Evidence:            strings.TrimSpace(item.Evidence),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].MonthlySaving.GreaterThan(out[j].MonthlySaving)
	})
	return out
}

// parseAIPrice reads the price returned by the model. It accepts a number, a
// numeric string and a string with a currency suffix.
func parseAIPrice(raw json.RawMessage) (decimal.Decimal, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return decimal.Zero, false
	}
	if strings.HasPrefix(trimmed, `"`) {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return decimal.Zero, false
		}
		return parsePriceText(text)
	}
	amount, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Zero, false
	}
	return amount, true
}

// parsePriceText extracts a number from a free-form price string.
func parsePriceText(text string) (decimal.Decimal, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return decimal.Zero, false
	}
	if amount, err := decimal.NewFromString(text); err == nil {
		return amount, true
	}
	prices := search.ExtractPrices(text)
	if len(prices) == 0 {
		return decimal.Zero, false
	}
	return prices[0].Amount, true
}

// normalizeSimilarity maps the answer of the model to a known value.
func normalizeSimilarity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high", "высокая", "высокое", "полное", "полное совпадение":
		return "high"
	case "medium", "средняя", "среднее", "частичное":
		return "medium"
	case "low", "низкая", "низкое", "слабое":
		return "low"
	default:
		return ""
	}
}

// amountInRUB converts an amount into roubles.
func amountInRUB(amount decimal.Decimal, currency string) (decimal.Decimal, bool) {
	rate, ok := search.CurrencyToRUB[strings.ToUpper(strings.TrimSpace(currency))]
	if !ok {
		return decimal.Zero, false
	}
	return amount.Mul(rate), true
}

// buildSummary renders the headline message shown on top of the analysis.
func buildSummary(best alternativeOut, comment string) string {
	symbol := currencySymbol(best.AlternativeCurrency)
	summary := fmt.Sprintf("%s дешевле, чем %s: на %s%s в месяц (на %s%s в год)",
		best.ServiceName, best.PaymentTitle,
		best.MonthlySaving.StringFixed(2), symbol,
		best.AnnualSaving.StringFixed(2), symbol)
	if !best.SavingPercent.IsZero() {
		summary += fmt.Sprintf(" — экономия %.0f%%", best.SavingPercent.InexactFloat64())
	}
	if strings.TrimSpace(comment) != "" {
		summary += ". " + strings.TrimSpace(comment)
	}
	return summary
}

// currencySymbol returns the display symbol of a currency.
func currencySymbol(currency string) string {
	switch strings.ToUpper(strings.TrimSpace(currency)) {
	case "RUB":
		return "₽"
	case "USD":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	default:
		return " "
	}
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// modelsSubscription is a compile time guard that the store model stays
// compatible with the analysis payload.
var _ = models.RecurringPayment{}
