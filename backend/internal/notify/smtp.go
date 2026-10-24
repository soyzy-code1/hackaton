package notify

import (
	"fmt"
	"strings"

	"gopkg.in/gomail.v2"
	"invoicekeeper/internal/config"
)

func SendDueReminderEmail(cfg config.Config, data DueReminderData) error {
	if !cfg.SMTPConfigured() {
		return nil
	}
	subject := fmt.Sprintf("Напоминание: %s — оплата до %s", data.Title, data.DueDate)
	body := fmt.Sprintf(`Здравствуйте!

Через %d дн. наступает срок оплаты:

	 %s
	 Сумма: %s %s
	 Срок: %s

— Invoice Keeper
`, data.DaysLeft, data.Title, data.Amount, data.Currency, data.DueDate)
	return sendMail(cfg, subject, body)
}

func sendMail(cfg config.Config, subject, body string) error {
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
