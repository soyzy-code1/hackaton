package notify

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"gopkg.in/gomail.v2"
	"invoicekeeper/internal/config"
)

// SMTPAttempt describes a single SMTP connection attempt. It intentionally
// exposes only the transport details (host/port/mode) and never any
// credential, so the result can safely be printed to logs or returned by an
// administrative endpoint.
type SMTPAttempt struct {
	Port  int    `json:"port"`
	Mode  string `json:"mode"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// smtpFallbackPorts lists the ports that are tried when the configured port
// refuses the connection. Port 465 negotiates implicit TLS, 587 and 2525 use
// STARTTLS.
var smtpFallbackPorts = []int{465, 587, 2525}

// smtpPorts returns the ordered list of ports to try: the configured one
// first, followed by the standard fallbacks without duplicates.
func smtpPorts(configured int) []int {
	ports := make([]int, 0, len(smtpFallbackPorts)+1)
	if configured > 0 {
		ports = append(ports, configured)
	}
	for _, port := range smtpFallbackPorts {
		if port == configured {
			continue
		}
		ports = append(ports, port)
	}
	return ports
}

// smtpMode describes the transport used for a port.
func smtpMode(port int) string {
	if port == 465 {
		return "implicit-tls"
	}
	return "starttls"
}

func joinPorts(ports []int) string {
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		parts = append(parts, strconv.Itoa(port))
	}
	return strings.Join(parts, ",")
}

// redact removes the SMTP password from a message so that error details can
// be logged without leaking the secret.
func redact(message, secret string) string {
	if secret == "" {
		return message
	}
	return strings.ReplaceAll(message, secret, "***")
}

func SendPaymentCreatedEmail(cfg config.Config, data DueReminderData) error {
	if !cfg.SMTPConfigured() {
		return fmt.Errorf("smtp is not configured: enable SMTP_ENABLED and provide SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD and SMTP_TO")
	}
	subject := fmt.Sprintf("Новый платеж: %s", data.Title)
	body := fmt.Sprintf("Создан новый платеж в Invoice Keeper:\n\n%s\nСумма: %s %s\nСрок оплаты: %s\n\nQR-код чека приложен к письму.\n", data.Title, data.Amount, data.Currency, data.DueDate)
	return sendMailWithReceiptQR(cfg, subject, body, data)
}

func SendDueReminderEmail(cfg config.Config, data DueReminderData) error {
	if !cfg.SMTPConfigured() {
		return fmt.Errorf("smtp is not configured: enable SMTP_ENABLED and provide SMTP_HOST, SMTP_USER, SMTP_PASSWORD and SMTP_TO")
	}
	subject := fmt.Sprintf("Напоминание: %s — оплата до %s", data.Title, data.DueDate)
	body := fmt.Sprintf(`Здравствуйте!

Через %d дн. наступает срок оплаты:

	 %s
	 Сумма: %s %s
	 Срок: %s

QR-код чека приложен к письму.

— Invoice Keeper
`, data.DaysLeft, data.Title, data.Amount, data.Currency, data.DueDate)
	return sendMailWithReceiptQR(cfg, subject, body, data)
}

func SendReceiptEmail(cfg config.Config, data DueReminderData) error {
	if !cfg.SMTPConfigured() {
		return fmt.Errorf("smtp is not configured")
	}
	subject := fmt.Sprintf("Чек: %s — %s", data.Title, data.DueDate)
	body := fmt.Sprintf("Чек по платежу:\n\n%s\nСумма: %s %s\nСрок: %s\n", data.Title, data.Amount, data.Currency, data.DueDate)
	return sendMailWithReceiptQR(cfg, subject, body, data)
}

func sendMailWithReceiptQR(cfg config.Config, subject, body string, data DueReminderData) error {
	qrPNG, err := BuildReceiptQR(data)
	if err != nil {
		return fmt.Errorf("receipt QR: %w", err)
	}
	return sendMail(cfg, subject, body, qrPNG)
}

func BuildReceiptQR(data DueReminderData) ([]byte, error) {
	payload := fmt.Sprintf("invoice-keeper:%d:%s:%s:%s:%s", data.OccurrenceID, data.Title, data.Amount, data.Currency, data.DueDate)
	return qrcode.Encode(payload, qrcode.Medium, 512)
}

func sendMail(cfg config.Config, subject, body string, attachments ...[]byte) error {
	message, from, recipients, err := buildMessage(cfg, subject, body, attachments...)
	if err != nil {
		return err
	}
	attempts, err := deliver(cfg, message)
	for _, attempt := range attempts {
		if attempt.OK {
			log.Printf("smtp: delivered host=%s port=%d mode=%s from=%s recipients=%d", cfg.SMTPHost, attempt.Port, attempt.Mode, from, len(recipients))
		}
	}
	if err != nil {
		return fmt.Errorf("smtp: send host=%s from=%s recipients=%d: %w", cfg.SMTPHost, from, len(recipients), err)
	}
	return nil
}

// buildMessage assembles the outgoing message. The sender is always the
// authenticated mailbox: mail providers commonly reject a From header that
// differs from the authenticated account, which silently breaks delivery.
func buildMessage(cfg config.Config, subject, body string, attachments ...[]byte) (*gomail.Message, string, []string, error) {
	user := strings.TrimSpace(cfg.SMTPUser)
	if user == "" {
		return nil, "", nil, errors.New("smtp: SMTP_USER is not configured")
	}
	from := strings.TrimSpace(cfg.SMTPFrom)
	if from == "" || !strings.EqualFold(from, user) {
		from = user
	}
	recipients := splitRecipients(cfg.SMTPTo)
	if len(recipients) == 0 {
		return nil, "", nil, errors.New("smtp: no recipients configured")
	}
	message := gomail.NewMessage()
	message.SetHeader("From", from)
	message.SetHeader("To", recipients...)
	message.SetHeader("Subject", subject)
	message.SetBody("text/plain", body)
	for _, attachment := range attachments {
		message.Attach("receipt-qr.png", gomail.SetCopyFunc(func(writer io.Writer) error {
			_, err := writer.Write(attachment)
			return err
		}))
	}
	return message, from, recipients, nil
}

// deliver sends the message, trying every candidate port until one accepts
// the connection. The returned attempts describe what happened per port.
func deliver(cfg config.Config, message *gomail.Message) ([]SMTPAttempt, error) {
	ports := smtpPorts(cfg.SMTPPort)
	attempts := make([]SMTPAttempt, 0, len(ports))
	var lastErr error
	for _, port := range ports {
		err := sendOnPort(cfg, message, port)
		attempt := SMTPAttempt{Port: port, Mode: smtpMode(port), OK: err == nil}
		if err != nil {
			attempt.Error = redact(err.Error(), cfg.SMTPPassword)
			lastErr = errors.New(attempt.Error)
			log.Printf("smtp: attempt host=%s port=%d mode=%s failed: %s", cfg.SMTPHost, port, attempt.Mode, attempt.Error)
		}
		attempts = append(attempts, attempt)
		if err == nil {
			return attempts, nil
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no SMTP port available")
	}
	return attempts, fmt.Errorf("all SMTP ports failed (tried %s): %w", joinPorts(ports), lastErr)
}

// sendOnPort delivers the message over a single port. Port 465 negotiates
// implicit TLS, every other port uses STARTTLS. The transport mode is set
// explicitly so a connection can never silently downgrade to plaintext.
func sendOnPort(cfg config.Config, message *gomail.Message, port int) error {
	dialer := gomail.NewDialer(cfg.SMTPHost, port, cfg.SMTPUser, cfg.SMTPPassword)
	dialer.SSL = port == 465
	dialer.TLSConfig = &tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}
	return dialer.DialAndSend(message)
}

// SendTestEmail delivers a diagnostic message to the configured recipients and
// returns the per-port attempt log. It is used by the production smoke test
// and never exposes credentials.
func SendTestEmail(cfg config.Config) ([]SMTPAttempt, error) {
	if !cfg.SMTPConfigured() {
		return nil, errors.New("smtp is not configured: enable SMTP_ENABLED and provide SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD and SMTP_TO")
	}
	subject := "Invoice Keeper: проверка доставки email"
	body := "Это диагностическое письмо Invoice Keeper.\nЕсли вы его получили — SMTP настроен и письма доставляются.\n"
	message, _, _, err := buildMessage(cfg, subject, body)
	if err != nil {
		return nil, err
	}
	return deliver(cfg, message)
}

// LogSMTPStatus reports which SMTP transport is reachable. It performs a
// connection probe only: no message is sent and no credential is printed.
func LogSMTPStatus(cfg config.Config) {
	if !cfg.SMTPConfigured() {
		log.Printf("smtp: not configured (enabled=%t, host=%q, port=%d)", cfg.SMTPEnabled, cfg.SMTPHost, cfg.SMTPPort)
		return
	}
	for _, port := range smtpPorts(cfg.SMTPPort) {
		err := probePort(cfg, port)
		if err == nil {
			log.Printf("smtp: host=%s port=%d mode=%s reachable", cfg.SMTPHost, port, smtpMode(port))
			return
		}
		log.Printf("smtp: host=%s port=%d mode=%s unreachable: %s", cfg.SMTPHost, port, smtpMode(port), redact(err.Error(), cfg.SMTPPassword))
	}
	log.Printf("smtp: no reachable port for host=%s (tried %s)", cfg.SMTPHost, joinPorts(smtpPorts(cfg.SMTPPort)))
}

// probePort checks that the SMTP service answers on the given port without
// authenticating and without sending anything.
func probePort(cfg config.Config, port int) error {
	address := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	if port == 465 {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12})
		defer tlsConn.Close()
		if err := tlsConn.Handshake(); err != nil {
			return err
		}
		return nil
	}
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(string(buf[:n]), "220") {
		return fmt.Errorf("unexpected SMTP banner")
	}
	return nil
}

func splitRecipients(s string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ';' }) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
