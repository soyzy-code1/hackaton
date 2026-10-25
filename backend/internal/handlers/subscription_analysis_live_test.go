package handlers

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/search"
)

// TestSubscriptionAnalysisLive runs the real analysis against GigaChat and the
// live web search. It is skipped unless GIGACHAT_E2E=1 and the credentials
// are present in the environment.
func TestSubscriptionAnalysisLive(t *testing.T) {
	if os.Getenv("GIGACHAT_E2E") != "1" {
		t.Skip("set GIGACHAT_E2E=1 to run the live analysis")
	}
	credentials := os.Getenv("GIGACHAT_CREDENTIALS")
	if credentials == "" {
		t.Skip("GIGACHAT_CREDENTIALS is not set")
	}

	cfg := config.Config{
		GigaChatEnabled:    true,
		GigaChatCredentials: credentials,
		GigaChatScope:      getenvOr("GIGACHAT_SCOPE", "GIGACHAT_API_PERS"),
		GigaChatAuthURL:    getenvOr("GIGACHAT_AUTH_URL", "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"),
		GigaChatAPIURL:     getenvOr("GIGACHAT_API_URL", "https://gigachat.devices.sberbank.ru/api/v1"),
		GigaChatModel:      getenvOr("GIGACHAT_MODEL", "GigaChat"),
		GigaChatVerifyTLS:  false,
		SearchEnabled:      true,
		SearchEngine:       "seznam",
		SearchFetchPages:   3,
		SearchTimeoutSec:   15,
		SearchMaxPayments:  12,
	}
	s := &Server{Cfg: cfg}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	subs := []analysisSubscription{
		{ID: 1, Title: "Яндекс Плюс", Amount: decimal.NewFromInt(299), Currency: "RUB", Periodicity: "monthly", Category: "стриминг музыки"},
		{ID: 2, Title: "Spotify Premium", Amount: decimal.NewFromInt(299), Currency: "RUB", Periodicity: "monthly", Category: "стриминг музыки"},
	}

	reports, queries, warnings := s.gatherSubscriptionEvidence(ctx, subs)
	t.Logf("search queries=%d warnings=%v", queries, warnings)
	for id, list := range reports {
		for _, report := range list {
			t.Logf("sub %d query=%q engine=%s results=%d", id, report.Query, report.Engine, len(report.Results))
			for _, result := range report.Results {
				t.Logf("   [%d prices] %s | %s", len(result.Prices), result.Title, result.URL)
				for _, price := range result.Prices {
					t.Logf("      %s %s :: %s", price.Amount, price.Currency, price.Raw)
				}
			}
		}
	}
	if len(reports) == 0 {
		t.Fatal("the live search returned nothing")
	}

	token, err := s.gigaChatToken()
	if err != nil {
		t.Fatalf("token error: %v", err)
	}
	analysis, err := s.runSubscriptionAnalysis(ctx, token, subs, reports)
	if err != nil {
		t.Fatalf("analysis error: %v", err)
	}
	encoded, _ := json.MarshalIndent(analysis, "", "  ")
	t.Logf("AI ANSWER:\n%s", encoded)

	alternatives := buildAlternatives(subs, analysis.Alternatives)
	for _, alt := range alternatives {
		t.Logf("ALT %s -> %s : saves %s %s / month", alt.PaymentTitle, alt.ServiceName, alt.MonthlySaving, alt.AlternativeCurrency)
	}
}

// TestSearchLiveOnly checks the search client against the real engines.
func TestSearchLiveOnly(t *testing.T) {
	if os.Getenv("GIGACHAT_E2E") != "1" {
		t.Skip("set GIGACHAT_E2E=1 to run the live search")
	}
	svc := search.NewService(search.Options{})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	report, err := svc.Search(ctx, "Яндекс Плюс стоимость подписки в месяц рублей")
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	t.Logf("engine=%s results=%d", report.Engine, len(report.Results))
	for _, result := range report.Results {
		t.Logf("  [%d prices] %s | %s", len(result.Prices), result.Title, result.URL)
		for _, price := range result.Prices {
			t.Logf("     %s %s :: %s", price.Amount, price.Currency, price.Raw)
		}
	}
	if len(report.Results) == 0 {
		t.Fatal("no search results")
	}
}

func getenvOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
