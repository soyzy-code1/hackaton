// Command smoketest verifies outbound notification channels in production.
//
// It reports transport-level results only (host, port, TLS mode). Credentials
// are never printed: error strings are redacted by the notify package.
//
// Exit codes:
//	0 - the channel is configured and the delivery succeeded
//	1 - the channel is configured but the delivery failed
//	2 - the command was called with an unknown channel
//	3 - the channel is not configured, nothing was checked
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"invoicekeeper/internal/config"
	"invoicekeeper/internal/notify"
)

const (
	exitOK      = 0
	exitFailure = 1
	exitUnknown = 2
	exitSkipped = 3
)

func main() {
	channel := flag.String("channel", "email", "notification channel to verify: email")
	flag.Parse()

	cfg := config.Load()
	switch *channel {
	case "email":
		os.Exit(runEmail(cfg))
	default:
		fmt.Fprintf(os.Stderr, "unknown channel %q\n", *channel)
		os.Exit(exitUnknown)
	}
}

func runEmail(cfg config.Config) int {
	if !cfg.SMTPConfigured() {
		emit(map[string]interface{}{
			"channel": "email",
			"status":  "skipped",
			"ok":      false,
			"reason":  "smtp is not configured",
		})
		return exitSkipped
	}

	attempts, err := notify.SendTestEmail(cfg)
	payload := map[string]interface{}{
		"channel":  "email",
		"ok":       err == nil,
		"attempts": attempts,
	}
	if err != nil {
		payload["status"] = "failed"
		payload["error"] = err.Error()
		emit(payload)
		return exitFailure
	}
	payload["status"] = "ok"
	emit(payload)
	return exitOK
}

func emit(payload map[string]interface{}) {
	out, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoketest: %v\n", err)
		return
	}
	fmt.Println(string(out))
}
