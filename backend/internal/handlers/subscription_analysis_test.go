package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/search"
)

func TestBuildSearchQueriesUsesTitle(t *testing.T) {
	queries := buildSearchQueries("Яндекс Плюс", "")
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}
	if !strings.Contains(queries[0], "Яндекс Плюс") {
		t.Errorf("queries must mention the subscription title: %v", queries)
	}
}

func TestBuildSearchQueriesAddsCategoryQuery(t *testing.T) {
	queries := buildSearchQueries("Яндекс Плюс", "стриминг музыки")
	if len(queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(queries))
	}
	if !strings.Contains(queries[1], "стриминг музыки") {
		t.Errorf("the second query must use the category: %v", queries)
	}
}

func TestBuildSearchQueriesIgnoresEmptyTitle(t *testing.T) {
	if queries := buildSearchQueries("   ", "стриминг музыки"); len(queries) != 0 {
		t.Fatalf("expected no queries, got %v", queries)
	}
}

func TestDetectCategory(t *testing.T) {
	cases := map[string]string{
		"Spotify Premium":   "стриминг музыки",
		"Яндекс Плюс":       "стриминг музыки",
		"Netflix":           "стриминг видео",
		"Яндекс Диск":       "облачное хранилище",
		"GitHub Copilot":     "сервисы для разработки",
		"ChatGPT Plus":      "ИИ-сервисы",
		"Просто подписка":   "",
	}
	for title, expected := range cases {
		if got := detectCategory(title); got != expected {
			t.Errorf("detectCategory(%q) = %q, want %q", title, got, expected)
		}
	}
}

func TestDecodeFunctionArgumentsAcceptsObject(t *testing.T) {
	args := decodeFunctionArguments(json.RawMessage(`{"query":"Яндекс Плюс цена"}`))
	if args["query"] != "Яндекс Плюс цена" {
		t.Fatalf("unexpected args: %+v", args)
	}
}

func TestDecodeFunctionArgumentsAcceptsEncodedString(t *testing.T) {
	args := decodeFunctionArguments(json.RawMessage(`"{\"query\":\"VK Combo цена\"}"`))
	if args["query"] != "VK Combo цена" {
		t.Fatalf("unexpected args: %+v", args)
	}
}

func TestDecodeFunctionArgumentsHandlesGarbage(t *testing.T) {
	args := decodeFunctionArguments(json.RawMessage(`not json`))
	if len(args) != 0 {
		t.Fatalf("expected empty args, got %+v", args)
	}
}

func TestParseAIAnalysisReadsAlternatives(t *testing.T) {
	content := "```json\n{\"alternatives\":[{\"payment_id\":7,\"payment_title\":\"Spotify Premium\",\"service_name\":\"Яндекс Плюс\",\"alternative_price\":199,\"alternative_currency\":\"RUB\",\"similarity\":\"high\",\"reason\":\"дешевле\",\"source_url\":\"https://example.com\",\"evidence\":\"199 рублей\"}],\"comment\":\"Нашли\"}\n```"
	analysis, err := parseAIAnalysis(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(analysis.Alternatives) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(analysis.Alternatives))
	}
	if analysis.Alternatives[0].ServiceName != "Яндекс Плюс" {
		t.Errorf("unexpected service name: %s", analysis.Alternatives[0].ServiceName)
	}
	if analysis.Comment != "Нашли" {
		t.Errorf("unexpected comment: %s", analysis.Comment)
	}
}

func TestParseAIAnalysisRejectsPlainText(t *testing.T) {
	if _, err := parseAIAnalysis("просто текст без json"); err == nil {
		t.Fatal("expected an error for a non-JSON answer")
	}
}

func TestBuildAlternativesKeepsCheaperSimilarService(t *testing.T) {
	subs := []analysisSubscription{
		{ID: 1, Title: "Spotify Premium", Amount: decimal.NewFromInt(299), Currency: "RUB", Periodicity: "monthly"},
	}
	ai := []aiAlternative{
		{
			PaymentID:           1,
			ServiceName:         "Яндекс Плюс",
			AlternativePrice:    json.RawMessage("199"),
			AlternativeCurrency: "RUB",
			Similarity:          "high",
			Reason:              "Стриминг музыки",
			SourceURL:           "https://example.com",
			Evidence:            "199 рублей в месяц",
		},
	}
	alternatives := buildAlternatives(subs, ai)
	if len(alternatives) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(alternatives))
	}
	alt := alternatives[0]
	if !alt.MonthlySaving.Equal(decimal.NewFromInt(100)) {
		t.Errorf("expected a saving of 100, got %s", alt.MonthlySaving)
	}
	if !alt.AnnualSaving.Equal(decimal.NewFromInt(1200)) {
		t.Errorf("expected an annual saving of 1200, got %s", alt.AnnualSaving)
	}
	if alt.ServiceName != "Яндекс Плюс" {
		t.Errorf("unexpected service name: %s", alt.ServiceName)
	}
	if alt.PaymentTitle != "Spotify Premium" {
		t.Errorf("unexpected payment title: %s", alt.PaymentTitle)
	}
}

func TestBuildAlternativesDropsExpensiveAlternative(t *testing.T) {
	subs := []analysisSubscription{
		{ID: 1, Title: "Spotify Premium", Amount: decimal.NewFromInt(199), Currency: "RUB", Periodicity: "monthly"},
	}
	ai := []aiAlternative{
		{
			PaymentID:           1,
			ServiceName:         "Дорогой сервис",
			AlternativePrice:    json.RawMessage("299"),
			AlternativeCurrency: "RUB",
			Similarity:          "high",
		},
	}
	if alternatives := buildAlternatives(subs, ai); len(alternatives) != 0 {
		t.Fatalf("expected no alternatives, got %+v", alternatives)
	}
}

func TestBuildAlternativesDropsSameService(t *testing.T) {
	subs := []analysisSubscription{
		{ID: 1, Title: "Spotify Premium", Amount: decimal.NewFromInt(299), Currency: "RUB", Periodicity: "monthly"},
	}
	ai := []aiAlternative{
		{
			PaymentID:           1,
			ServiceName:         "spotify premium",
			AlternativePrice:    json.RawMessage("199"),
			AlternativeCurrency: "RUB",
			Similarity:          "high",
		},
	}
	if alternatives := buildAlternatives(subs, ai); len(alternatives) != 0 {
		t.Fatalf("expected no alternatives, got %+v", alternatives)
	}
}

func TestBuildAlternativesDropsWeakSimilarity(t *testing.T) {
	subs := []analysisSubscription{
		{ID: 1, Title: "Spotify Premium", Amount: decimal.NewFromInt(299), Currency: "RUB", Periodicity: "monthly"},
	}
	ai := []aiAlternative{
		{
			PaymentID:           1,
			ServiceName:         "Фитнес-клуб",
			AlternativePrice:    json.RawMessage("99"),
			AlternativeCurrency: "RUB",
			Similarity:          "low",
		},
	}
	if alternatives := buildAlternatives(subs, ai); len(alternatives) != 0 {
		t.Fatalf("expected no alternatives, got %+v", alternatives)
	}
}

func TestBuildAlternativesDropsUnknownPayment(t *testing.T) {
	subs := []analysisSubscription{
		{ID: 1, Title: "Spotify Premium", Amount: decimal.NewFromInt(299), Currency: "RUB", Periodicity: "monthly"},
	}
	ai := []aiAlternative{
		{
			PaymentID:           99,
			ServiceName:         "Яндекс Плюс",
			AlternativePrice:    json.RawMessage("199"),
			AlternativeCurrency: "RUB",
			Similarity:          "high",
		},
	}
	if alternatives := buildAlternatives(subs, ai); len(alternatives) != 0 {
		t.Fatalf("expected no alternatives, got %+v", alternatives)
	}
}

func TestBuildAlternativesSortsBySaving(t *testing.T) {
	subs := []analysisSubscription{
		{ID: 1, Title: "Сервис А", Amount: decimal.NewFromInt(300), Currency: "RUB", Periodicity: "monthly"},
		{ID: 2, Title: "Сервис Б", Amount: decimal.NewFromInt(500), Currency: "RUB", Periodicity: "monthly"},
	}
	ai := []aiAlternative{
		{PaymentID: 1, ServiceName: "Альт А", AlternativePrice: json.RawMessage("250"), AlternativeCurrency: "RUB", Similarity: "high"},
		{PaymentID: 2, ServiceName: "Альт Б", AlternativePrice: json.RawMessage("100"), AlternativeCurrency: "RUB", Similarity: "high"},
	}
	alternatives := buildAlternatives(subs, ai)
	if len(alternatives) != 2 {
		t.Fatalf("expected 2 alternatives, got %d", len(alternatives))
	}
	if alternatives[0].PaymentID != 2 {
		t.Errorf("expected the biggest saving first, got %+v", alternatives[0])
	}
}

func TestParseAIPriceAcceptsNumberAndString(t *testing.T) {
	if amount, ok := parseAIPrice(json.RawMessage("199")); !ok || !amount.Equal(decimal.NewFromInt(199)) {
		t.Errorf("unexpected price: %v %v", amount, ok)
	}
	if amount, ok := parseAIPrice(json.RawMessage(`"199 рублей"`)); !ok || !amount.Equal(decimal.NewFromInt(199)) {
		t.Errorf("unexpected price: %v %v", amount, ok)
	}
	if _, ok := parseAIPrice(json.RawMessage("null")); ok {
		t.Error("null must not be a price")
	}
}

func TestNormalizeSimilarity(t *testing.T) {
	if normalizeSimilarity("High") != "high" {
		t.Error("expected high")
	}
	if normalizeSimilarity("средняя") != "medium" {
		t.Error("expected medium")
	}
	if normalizeSimilarity("low") != "low" {
		t.Error("expected low")
	}
	if normalizeSimilarity("что-то") != "" {
		t.Error("expected an empty value")
	}
}

func TestBuildSummaryMentionsServiceAndSaving(t *testing.T) {
	alt := alternativeOut{
		ServiceName:         "Яндекс Плюс",
		PaymentTitle:        "Spotify Premium",
		MonthlySaving:       decimal.NewFromInt(100),
		AnnualSaving:        decimal.NewFromInt(1200),
		SavingPercent:       decimal.NewFromInt(33),
		AlternativeCurrency: "RUB",
	}
	summary := buildSummary(alt, "Проверено")
	if !strings.Contains(summary, "Яндекс Плюс") {
		t.Errorf("summary must name the service: %s", summary)
	}
	if !strings.Contains(summary, "100") {
		t.Errorf("summary must contain the saving: %s", summary)
	}
	if !strings.Contains(summary, "Spotify Premium") {
		t.Errorf("summary must name the current subscription: %s", summary)
	}
}

func TestTrimSearchResultsKeepsPricesOnly(t *testing.T) {
	results := []search.Result{
		{Title: "a", URL: "https://a", Excerpt: "x"},
		{Title: "b", URL: "https://b", Excerpt: "y", Prices: []search.PriceEvidence{{Raw: "199 рублей"}}},
	}
	trimmed := trimSearchResults(results)
	if len(trimmed) != 1 {
		t.Fatalf("expected 1 result, got %d", len(trimmed))
	}
	if trimmed[0]["title"] != "b" {
		t.Errorf("expected the result with prices to be kept, got %+v", trimmed[0])
	}
}

func TestAmountInRUB(t *testing.T) {
	value, ok := amountInRUB(decimal.NewFromInt(10), "USD")
	if !ok || !value.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("unexpected conversion: %v %v", value, ok)
	}
	if _, ok := amountInRUB(decimal.NewFromInt(10), "JPY"); ok {
		t.Error("unknown currency must not convert")
	}
}
