package notify

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"

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
	recipients := splitRecipients(cfg.SMTPTo)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	}
	for _, to := range recipients {
		msg := buildMessage(from, to, subject, body)
		if err := sendWithStartTLS(addr, cfg.SMTPHost, auth, from, []string{to}, msg); err != nil {
			log.Printf("smtp: send to %s: %v", to, err)
			return err
		}
	}
	return nil
}

func buildMessage(from, to, subject, body string) []byte {
	return []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body))
}

func sendWithStartTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	if cfgPort(addr) == 465 {
		return sendWithImplicitTLS(addr, host, auth, from, to, msg)
	}
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return err
		}
	}
	if auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func sendWithImplicitTLS(addr, host string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func cfgPort(addr string) int {
	_, port, ok := strings.Cut(addr, ":")
	if !ok {
		return 587
	}
	var p int
	for _, ch := range port {
		if ch < '0' || ch > '9' {
			return 587
		}
		p = p*10 + int(ch-'0')
	}
	return p
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
