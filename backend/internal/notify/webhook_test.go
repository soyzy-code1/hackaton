package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"invoicekeeper/internal/config"
)

func TestPostWebhookSendsEnvelope(t *testing.T) {
	var mu sync.Mutex
	var got Envelope
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Webhook-Secret") != "secret" {
			t.Errorf("missing secret header")
		}
		var env Envelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		mu.Lock()
		got = env
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Config{
		WebhookEnabled: true,
		WebhookURL:     srv.URL,
		WebhookSecret:  "secret",
	}
	PostWebhook(cfg, EventPaymentOccurrencePaid, map[string]string{"ok": "1"})
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if got.Event != EventPaymentOccurrencePaid {
		t.Fatalf("event %q", got.Event)
	}
}
