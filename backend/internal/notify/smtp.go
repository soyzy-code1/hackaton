package notify

import (
	"crypto/tls"
	"fmt"
	"io"
	"strings"

	"github.com/skip2/go-qrcode"
	"gopkg.in/gomail.v2"
	"invoicekeeper/internal/config"
)

func SendPaymentCreatedEmail(cfg config.Config, data DueReminderData) error {
	if !cfg.SMTPConfigured() {
		return fmt.Errorf("smtp is not configured: enable SMTP_ENABLED and provide SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD, SMTP_FROM and SMTP_TO")
	}
	subject := fmt.Sprintf("Новый платеж: %s", data.Title)
	body := fmt.Sprintf("Создан новый платеж в Invoice Keeper:\n\n%s\nСумма: %s %s\nСрок оплаты: %s\n\nQR-код чека приложен к письму.\n", data.Title, data.Amount, data.Currency, data.DueDate)
	return sendMailWithReceiptQR(cfg, subject, body, data)
}

func SendDueReminderEmail(cfg config.Config, data DueReminderData) error {
	if !cfg.SMTPConfigured() {
		return fmt.Errorf("smtp is not configured: enable SMTP_ENABLED and provide SMTP_HOST, SMTP_FROM, SMTP_TO")
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
	from := strings.TrimSpace(cfg.SMTPFrom)
	user := strings.TrimSpace(cfg.SMTPUser)
	if from == "" {
		from = user
	}
	recipients := splitRecipients(cfg.SMTPTo)
	if len(recipients) == 0 {
		return fmt.Errorf("smtp: no recipients configured")
	}
	if from == "" || user == "" {
		return fmt.Errorf("smtp: sender and SMTP user must be configured")
	}
	port := cfg.SMTPPort
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

	dialer := gomail.NewDialer(cfg.SMTPHost, port, user, cfg.SMTPPassword)
	// Mail.ru uses implicit TLS on 465 and STARTTLS on 587. Explicitly set the
	// mode so a non-default port cannot accidentally downgrade the connection.
	dialer.SSL = port == 465
	dialer.TLSConfig = &tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}
	if err := dialer.DialAndSend(message); err != nil {
		return fmt.Errorf("smtp: send host=%s port=%d from=%s recipients=%d: %w", cfg.SMTPHost, port, from, len(recipients), err)
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
