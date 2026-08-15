package config

import (
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

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
