package notify

import (
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
	from := cfg.SMTPFrom
	if from == "" {
		from = cfg.SMTPUser
	}
	port := cfg.SMTPPort
	message := gomail.NewMessage()
	message.SetHeader("From", from)
	message.SetHeader("To", splitRecipients(cfg.SMTPTo)...)
	message.SetHeader("Subject", subject)
	message.SetBody("text/plain", body)
	for _, attachment := range attachments {
		message.Attach("receipt-qr.png", gomail.SetCopyFunc(func(writer io.Writer) error {
			_, err := writer.Write(attachment)
			return err
		}))
	}

	dialer := gomail.NewDialer(cfg.SMTPHost, port, cfg.SMTPUser, cfg.SMTPPassword)
	if err := dialer.DialAndSend(message); err != nil {
		return fmt.Errorf("smtp: send to %s: %w", cfg.SMTPTo, err)
	}
	return nil
}

func splitRecipients(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
