package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type SMTPConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	FromAddress string
	FromName    string
}

type Config struct {
	DatabaseURL        string
	InviteCode         string
	Port               string
	LiveKitURL         string
	LiveKitAPIKey      string
	LiveKitAPISecret   string
	CORSOrigins        []string
	SessionTTL         time.Duration
	SessionTouchWindow time.Duration
	PasswordResetTTL   time.Duration
	AppURL             string
	SMTPConfig         SMTPConfig
}

func Load() *Config {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		InviteCode:         os.Getenv("INVITE_CODE"),
		Port:               port,
		LiveKitURL:         os.Getenv("LIVEKIT_URL"),
		LiveKitAPIKey:      os.Getenv("LIVEKIT_API_KEY"),
		LiveKitAPISecret:   os.Getenv("LIVEKIT_API_SECRET"),
		CORSOrigins:        splitList(os.Getenv("CORS_ORIGINS")),
		SessionTTL:         durationEnv("SESSION_TTL", 7*24*time.Hour),
		SessionTouchWindow: durationEnv("SESSION_TOUCH_WINDOW", 24*time.Hour),
		PasswordResetTTL:   durationEnv("PASSWORD_RESET_TTL", 30*time.Minute),
		AppURL:             os.Getenv("APP_URL"),
		SMTPConfig: SMTPConfig{
			Host:        os.Getenv("SMTP_HOST"),
			Port:        intEnv("SMTP_PORT", 587),
			User:        os.Getenv("SMTP_USER"),
			Password:    os.Getenv("SMTP_PASS"),
			FromAddress: os.Getenv("MAIL_FROM_ADDRESS"),
			FromName:    os.Getenv("MAIL_FROM_NAME"),
		},
	}
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func intEnv(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

func splitList(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
