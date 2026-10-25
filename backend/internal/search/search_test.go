package search

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestExtractPricesFindsRubleAmounts(t *testing.T) {
	text := "Стоимость подписки снизили до 99 рублей. Итак, приобретая подписку за 169 рублей, вы получите доступ."
	prices := ExtractPrices(text)
	if len(prices) < 2 {
		t.Fatalf("expected at least 2 prices, got %d: %+v", len(prices), prices)
	}
	if !prices[0].Amount.Equal(decimal.NewFromInt(99)) {
		t.Errorf("expected 99, got %s", prices[0].Amount)
	}
	if prices[0].Currency != "RUB" {
		t.Errorf("expected RUB, got %s", prices[0].Currency)
	}
	if !prices[1].Amount.Equal(decimal.NewFromInt(169)) {
		t.Errorf("expected 169, got %s", prices[1].Amount)
	}
	if !strings.Contains(prices[0].Context, "Стоимость подписки") {
		t.Errorf("context lost: %q", prices[0].Context)
	}
}

func TestExtractPricesHandlesRubleSymbol(t *testing.T) {
	prices := ExtractPrices("309 ₽ за пользователя в месяц")
	if len(prices) != 1 {
		t.Fatalf("expected 1 price, got %d", len(prices))
	}
	if !prices[0].Amount.Equal(decimal.NewFromInt(309)) {
		t.Errorf("expected 309, got %s", prices[0].Amount)
	}
	if prices[0].Currency != "RUB" {
		t.Errorf("expected RUB, got %s", prices[0].Currency)
	}
}

func TestExtractPricesParsesThousandsAndDecimals(t *testing.T) {
	prices := ExtractPrices("Цена 1 299 ₽, скидка 9.99 USD, пробная версия 1 $")
	if len(prices) != 3 {
		t.Fatalf("expected 3 prices, got %d: %+v", len(prices), prices)
	}
	if !prices[0].Amount.Equal(decimal.NewFromInt(1299)) {
		t.Errorf("expected 1299, got %s", prices[0].Amount)
	}
	expectedNines, _ := decimal.NewFromString("9.99")
	if !prices[1].Amount.Equal(expectedNines) {
		t.Errorf("expected 9.99, got %s", prices[1].Amount)
	}
	if prices[1].Currency != "USD" {
		t.Errorf("expected USD, got %s", prices[1].Currency)
	}
}

func TestExtractPricesIgnoresImplausibleValues(t *testing.T) {
	if prices := ExtractPrices("всего 0 ₽ и 99999999 ₽"); len(prices) != 0 {
		t.Fatalf("expected no plausible prices, got %+v", prices)
	}
}

func TestStripTagsRemovesScriptAndMarkup(t *testing.T) {
	source := `<html><head><style>.a{color:red}</style><script>var x = "999 ₽";</script></head>` +
		`<body><p>Подписка <b>стоит</b> 199 рублей</p></body></html>`
	cleaned := StripTags(source)
	if strings.Contains(cleaned, "999") {
		t.Errorf("script content was not removed: %q", cleaned)
	}
	if !strings.Contains(cleaned, "199 рублей") {
		t.Errorf("text was lost: %q", cleaned)
	}
	if strings.Contains(cleaned, "<") {
		t.Errorf("markup was not removed: %q", cleaned)
	}
}

func TestNormalizeComparesTitles(t *testing.T) {
	if Normalize("Яндекс Плюс") != Normalize("яндекс  плюс") {
		t.Errorf("titles should normalize equally: %q vs %q", Normalize("Яндекс Плюс"), Normalize("яндекс  плюс"))
	}
	if Normalize("Spotify Premium") == Normalize("Яндекс Плюс") {
		t.Error("different services must not normalize equally")
	}
}

func TestSeznamProviderParsesResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body>
			<h3><a href="https://example.com/one">Яндекс плюс : обзор</a></h3>
			<h3><a href="https://example.com/two">Условия использования</a></h3>
			<h3><a href="javascript:void(0)">Пропустить</a></h3>
		</body></html>`))
	}))
	defer server.Close()

	provider := NewSeznamProvider(server.Client())
	provider.BaseURL = server.URL + "/"
	hits, err := provider.Search(context.Background(), "Яндекс Плюс цена", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d: %+v", len(hits), hits)
	}
	if hits[0].URL != "https://example.com/one" {
		t.Errorf("unexpected url: %s", hits[0].URL)
	}
	if hits[0].Title != "Яндекс плюс : обзор" {
		t.Errorf("unexpected title: %q", hits[0].Title)
	}
}

func TestBingProviderDecodesRedirectURL(t *testing.T) {
	// "https://example.com/x" encoded the way Bing wraps it.
	raw := "https://www.bing.com/ck/a?!&&p=1&u=a1aHR0cHM6Ly9leGFtcGxlLmNvbS94"
	decoded := decodeBingURL(raw)
	if decoded != "https://example.com/x" {
		t.Errorf("unexpected decoded url: %q", decoded)
	}
}

func TestBingProviderParsesResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><ol>
			<li class="b_algo"><h2><a href="https://example.com/spotify">Spotify - Web Player</a></h2>
			<p>Слушайте музыку, подкасты и аудиокниги.</p></li>
		</ol></body></html>`))
	}))
	defer server.Close()

	provider := NewBingProvider(server.Client())
	provider.BaseURL = server.URL + "/search"
	hits, err := provider.Search(context.Background(), "spotify premium price", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d: %+v", len(hits), hits)
	}
	if hits[0].URL != "https://example.com/spotify" {
		t.Errorf("unexpected url: %s", hits[0].URL)
	}
	if hits[0].Snippet == "" {
		t.Error("expected a snippet")
	}
}

func TestServiceEnrichesResultsWithPrices(t *testing.T) {
	var searchHits string
	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(searchHits))
	}))
	defer searchServer.Close()

	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><p>Тарифы сервиса: 149 рублей в месяц за подписку.</p></body></html>`))
	}))
	defer pageServer.Close()

	searchHits = `<html><body><h3><a href="` + pageServer.URL + `/pricing">Тарифы сервиса</a></h3></body></html>`

	svc := NewService(Options{
		Providers: []Provider{NewSeznamProvider(searchServer.Client())},
		Client:    searchServer.Client(),
	})
	provider := svc.Providers()[0].(*SeznamProvider)
	provider.BaseURL = searchServer.URL + "/"

	report, err := svc.Search(context.Background(), "тарифы сервиса")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	if len(report.Results[0].Prices) != 1 {
		t.Fatalf("expected 1 price, got %+v", report.Results[0].Prices)
	}
	if !report.Results[0].Prices[0].Amount.Equal(decimal.NewFromInt(149)) {
		t.Errorf("expected 149, got %s", report.Results[0].Prices[0].Amount)
	}
}

func TestPriceToRUB(t *testing.T) {
	price := Price{Amount: decimal.NewFromInt(10), Currency: "USD"}
	value, ok := price.ToRUB()
	if !ok {
		t.Fatal("expected a conversion")
	}
	if !value.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("expected 1000, got %s", value)
	}
	if _, ok := (Price{Currency: "JPY"}).ToRUB(); ok {
		t.Error("unknown currency must not convert")
	}
}
