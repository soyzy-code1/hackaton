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

	GigaChatEnabled       bool
	GigaChatCredentials    string
	GigaChatScope         string
	GigaChatAuthURL       string
	GigaChatAPIURL        string
	GigaChatModel         string
	GigaChatVerifyTLS     bool

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
		smtpTo = getenv("ADMIN_EMAIL", "antbkkahack@mail.ru")
	}
	return Config{
		AppEnv:             getenv("APP_ENV", "dev"),
		DatabaseURL:        getenv("DATABASE_URL", "sqlite:///./invoice_keeper.db"),
		SecretKey:          getenv("SECRET_KEY", "change-me"),
		SessionTTLHours:    getenvInt("SESSION_TTL_HOURS", 24),
		AdminLogin:         getenv("ADMIN_LOGIN", "admin"),
		AdminPassword:      getenv("ADMIN_PASSWORD", "admin123"),
		AdminEmail:         getenv("ADMIN_EMAIL", "antbkkahack@mail.ru"),
		Timezone:           getenv("TIMEZONE", "Europe/Moscow"),
		ReminderDaysBefore: getenvInt("REMINDER_DAYS_BEFORE", 3),
		TelegramChatID:     getenv("TELEGRAM_CHAT_ID", ""),
		InternalAPIToken:   getenv("INTERNAL_API_TOKEN", "change-me-internal"),

		GigaChatEnabled:    getenvBool("GIGACHAT_ENABLED", false),
		GigaChatCredentials: getenv("GIGACHAT_CREDENTIALS", ""),
		GigaChatScope:      getenv("GIGACHAT_SCOPE", "GIGACHAT_API_PERS"),
		GigaChatAuthURL:    getenv("GIGACHAT_AUTH_URL", "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"),
		GigaChatAPIURL:     getenv("GIGACHAT_API_URL", "https://gigachat.devices.sberbank.ru/api/v1"),
		GigaChatModel:      getenv("GIGACHAT_MODEL", "GigaChat"),
		GigaChatVerifyTLS:  getenvBool("GIGACHAT_VERIFY_TLS", true),

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
	return c.SMTPEnabled &&
		c.SMTPHost != "" &&
		c.SMTPPort > 0 &&
		c.SMTPUser != "" &&
		c.SMTPPassword != "" &&
		c.SMTPFrom != "" &&
		c.SMTPTo != ""
}

func (c Config) GigaChatConfigured() bool {
	return c.GigaChatEnabled && strings.TrimSpace(c.GigaChatCredentials) != "" &&
		strings.TrimSpace(c.GigaChatAuthURL) != "" && strings.TrimSpace(c.GigaChatAPIURL) != ""
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
