package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv             string
	DatabaseURL        string
	SecretKey          string
	SessionTTLHours    int
	AdminLogin         string
	AdminPassword      string
	AdminEmail         string
	Timezone           string
	ReminderDaysBefore int
	TelegramChatID     string
	InternalAPIToken   string

	SMTPEnabled    bool
	SMTPHost       string
	SMTPPort       int
	SMTPUser       string
	SMTPPassword   string
	SMTPFrom       string
	SMTPTo         string
	WebhookEnabled bool
	WebhookURL     string
	WebhookSecret  string
	NotifyPollSec  int
}

func Load() Config {
	smtpTo := getenv("SMTP_TO", "")
	if smtpTo == "" {
		smtpTo = getenv("ADMIN_EMAIL", "admin@example.com")
	}
	return Config{
		AppEnv:             getenv("APP_ENV", "dev"),
		DatabaseURL:        getenv("DATABASE_URL", "sqlite:///./invoice_keeper.db"),
		SecretKey:          getenv("SECRET_KEY", "change-me"),
		SessionTTLHours:    getenvInt("SESSION_TTL_HOURS", 24),
		AdminLogin:         getenv("ADMIN_LOGIN", "admin"),
		AdminPassword:      getenv("ADMIN_PASSWORD", "admin123"),
		AdminEmail:         getenv("ADMIN_EMAIL", "admin@example.com"),
		Timezone:           getenv("TIMEZONE", "Europe/Moscow"),
		ReminderDaysBefore: getenvInt("REMINDER_DAYS_BEFORE", 3),
		TelegramChatID:     getenv("TELEGRAM_CHAT_ID", ""),
		InternalAPIToken:   getenv("INTERNAL_API_TOKEN", "change-me-internal"),

		SMTPEnabled:    getenvBool("SMTP_ENABLED", false),
		SMTPHost:       getenv("SMTP_HOST", ""),
		SMTPPort:       getenvInt("SMTP_PORT", 587),
		SMTPUser:       getenv("SMTP_USER", ""),
		SMTPPassword:   getenv("SMTP_PASSWORD", ""),
		SMTPFrom:       getenv("SMTP_FROM", ""),
		SMTPTo:         smtpTo,
		WebhookEnabled: getenvBool("WEBHOOK_ENABLED", false),
		WebhookURL:     getenv("WEBHOOK_URL", ""),
		WebhookSecret:  getenv("WEBHOOK_SECRET", ""),
		NotifyPollSec:  getenvInt("NOTIFY_POLL_SECONDS", 60),
	}
}

func (c Config) SMTPConfigured() bool {
	return c.SMTPEnabled && c.SMTPHost != "" && c.SMTPFrom != "" && c.SMTPTo != ""
}

func (c Config) WebhookConfigured() bool {
	return c.WebhookEnabled && c.WebhookURL != ""
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
