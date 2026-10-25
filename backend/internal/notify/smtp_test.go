package notify

import (
	"strings"
	"testing"

	"invoicekeeper/internal/config"
)

func TestSMTPPortsFallBackToStandardOnes(t *testing.T) {
	cases := []struct {
		name       string
		configured int
		want       []int
	}{
		{"configured 587 first", 587, []int{587, 465, 2525}},
		{"configured 465 first", 465, []int{465, 587, 2525}},
		{"unknown port falls back", 25, []int{25, 465, 587, 2525}},
		{"unset port", 0, []int{465, 587, 2525}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := smtpPorts(tc.configured)
			if len(got) != len(tc.want) {
				t.Fatalf("ports %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ports %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestSMTPPortsHaveNoDuplicates(t *testing.T) {
	for _, configured := range []int{0, 25, 465, 587, 2525} {
		seen := map[int]bool{}
		for _, port := range smtpPorts(configured) {
			if seen[port] {
				t.Fatalf("duplicate port %d for configured=%d", port, configured)
			}
			seen[port] = true
		}
	}
}

func TestSMTPModeUsesImplicitTLSOnlyFor465(t *testing.T) {
	if mode := smtpMode(465); mode != "implicit-tls" {
		t.Fatalf("mode for 465 = %q", mode)
	}
	for _, port := range []int{587, 2525, 25} {
		if mode := smtpMode(port); mode != "starttls" {
			t.Fatalf("mode for %d = %q", port, mode)
		}
	}
}

// TestBuildMessageUsesAuthenticatedUserAsSender protects the delivery of the
// mail: providers reject a From header that differs from the authenticated
// mailbox, which breaks every outgoing message.
func TestBuildMessageUsesAuthenticatedUserAsSender(t *testing.T) {
	cfg := config.Config{
		SMTPUser:     "billing@example.com",
		SMTPPassword: "secret",
		SMTPFrom:     "other@example.com",
		SMTPTo:       "owner@example.com",
	}

	message, from, recipients, err := buildMessage(cfg, "Subject", "Body")
	if err != nil {
		t.Fatal(err)
	}
	if from != "billing@example.com" {
		t.Fatalf("from = %q", from)
	}
	if len(recipients) != 1 || recipients[0] != "owner@example.com" {
		t.Fatalf("recipients = %v", recipients)
	}
	header := strings.Join(message.GetHeader("From"), " ")
	if !strings.Contains(header, "billing@example.com") {
		t.Fatalf("From header = %q", header)
	}
	if strings.Contains(header, "other@example.com") {
		t.Fatalf("From header must not use SMTP_FROM: %q", header)
	}
}

func TestBuildMessageKeepsMatchingFrom(t *testing.T) {
	cfg := config.Config{
		SMTPUser: "billing@example.com",
		SMTPPassword: "secret",
		SMTPFrom:  "billing@example.com",
		SMTPTo:    "owner@example.com",
	}
	_, from, _, err := buildMessage(cfg, "Subject", "Body")
	if err != nil {
		t.Fatal(err)
	}
	if from != "billing@example.com" {
		t.Fatalf("from = %q", from)
	}
}

func TestBuildMessageRequiresUserAndRecipients(t *testing.T) {
	if _, _, _, err := buildMessage(config.Config{SMTPTo: "owner@example.com"}, "s", "b"); err == nil {
		t.Fatal("expected an error when SMTP_USER is missing")
	}
	if _, _, _, err := buildMessage(config.Config{SMTPUser: "billing@example.com"}, "s", "b"); err == nil {
		t.Fatal("expected an error when SMTP_TO is missing")
	}
}

func TestRedactHidesPassword(t *testing.T) {
	got := redact("535 5.7.8 auth failed for secret-value", "secret-value")
	if strings.Contains(got, "secret-value") {
		t.Fatalf("password leaked: %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("expected redaction marker: %q", got)
	}
}

func TestRedactWithoutSecretKeepsMessage(t *testing.T) {
	if got := redact("plain error", ""); got != "plain error" {
		t.Fatalf("redact = %q", got)
	}
}

func TestSplitRecipients(t *testing.T) {
	got := splitRecipients("a@example.com, b@example.com; c@example.com")
	want := []string{"a@example.com", "b@example.com", "c@example.com"}
	if len(got) != len(want) {
		t.Fatalf("recipients = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("recipients = %v, want %v", got, want)
		}
	}
	if len(splitRecipients("  ,  ; ")) != 0 {
		t.Fatal("expected no recipients")
	}
}
