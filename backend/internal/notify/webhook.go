package notify

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"invoicekeeper/internal/config"
)

func PostWebhook(cfg config.Config, event string, data interface{}) bool {
	if !cfg.WebhookConfigured() {
		return false
	}
	env := Envelope{Event: event, Timestamp: time.Now().UTC(), Data: data}
	body, err := json.Marshal(env)
	if err != nil {
		log.Printf("webhook: marshal: %v", err)
		return false
	}
	req, err := http.NewRequest(http.MethodPost, cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook: request: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "InvoiceKeeper/1.0")
	if cfg.WebhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", cfg.WebhookSecret)
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("webhook: post %s: %v", event, err)
		return false
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("webhook: post %s: status %d", event, resp.StatusCode)
		return false
	}
	return true
}

func PostWebhookAsync(cfg config.Config, event string, data interface{}) {
	go func() { PostWebhook(cfg, event, data) }()
}
