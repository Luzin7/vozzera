package config

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var (
	ErrKeyValueURLRequired               = errors.New("KEY_VALUE_URL é obrigatória")
	ErrKeyValuePrefixInvalid             = errors.New("KEY_VALUE_PREFIX é obrigatória e deve casar com ^[a-z0-9-]+$")
	ErrSessionRevalidateIntervalTooShort = errors.New("WS_SESSION_REVALIDATE_INTERVAL deve ser de pelo menos 1m")
	ErrSwaggerCredentialsRequired        = errors.New("SWAGGER_BASIC_USER e SWAGGER_BASIC_PASSWORD são obrigatórias quando SWAGGER_ENABLED=true")
	ErrSendGridAPIKeyRequired            = errors.New("SENDGRID_API_KEY é obrigatória em produção")
	ErrMailFromAddressRequired           = errors.New("MAIL_FROM_ADDRESS é obrigatória em produção")
	ErrAppURLRequired                    = errors.New("APP_URL é obrigatória em produção")
	ErrEnvironmentInvalid                = errors.New("APP_ENV deve ser development ou production")

	keyValuePrefixPattern = regexp.MustCompile(`^[a-z0-9-]+$`)
)

const (
	keyValueNamespace      = "vz:"
	environmentProduction  = "production"
	environmentDevelopment = "development"
)

type SendGridConfig struct {
	APIKey      string
	FromAddress string
	FromName    string
}

type Config struct {
	Environment               string
	DatabaseURL               string
	InviteCode                string
	Port                      string
	LiveKitURL                string
	LiveKitAPIKey             string
	LiveKitAPISecret          string
	KeyValueURL               string
	KeyValuePrefix            string
	CORSOrigins               []string
	SessionTTL                time.Duration
	SessionTouchWindow        time.Duration
	SessionRevalidateInterval time.Duration
	PasswordResetTTL          time.Duration
	AppURL                    string
	ClientIPHeader            string
	SwaggerEnabled            bool
	SwaggerBasicUser          string
	SwaggerBasicPassword      string
	SendGridConfig            SendGridConfig
}

func Load() *Config {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		Environment:               resolveEnvironment(),
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		InviteCode:                os.Getenv("INVITE_CODE"),
		Port:                      port,
		LiveKitURL:                os.Getenv("LIVEKIT_URL"),
		LiveKitAPIKey:             os.Getenv("LIVEKIT_API_KEY"),
		LiveKitAPISecret:          os.Getenv("LIVEKIT_API_SECRET"),
		KeyValueURL:               os.Getenv("KEY_VALUE_URL"),
		KeyValuePrefix:            os.Getenv("KEY_VALUE_PREFIX"),
		CORSOrigins:               splitList(os.Getenv("CORS_ORIGINS")),
		SessionTTL:                durationEnv("SESSION_TTL", 7*24*time.Hour),
		SessionTouchWindow:        durationEnv("SESSION_TOUCH_WINDOW", 24*time.Hour),
		SessionRevalidateInterval: durationEnv("WS_SESSION_REVALIDATE_INTERVAL", 5*time.Minute),
		PasswordResetTTL:          durationEnv("PASSWORD_RESET_TTL", 30*time.Minute),
		AppURL:                    os.Getenv("APP_URL"),
		ClientIPHeader:            strings.TrimSpace(os.Getenv("CLIENT_IP_HEADER")),
		SwaggerEnabled:            boolEnv("SWAGGER_ENABLED", false),
		SwaggerBasicUser:          os.Getenv("SWAGGER_BASIC_USER"),
		SwaggerBasicPassword:      os.Getenv("SWAGGER_BASIC_PASSWORD"),
		SendGridConfig: SendGridConfig{
			APIKey:      os.Getenv("SENDGRID_API_KEY"),
			FromAddress: os.Getenv("MAIL_FROM_ADDRESS"),
			FromName:    os.Getenv("MAIL_FROM_NAME"),
		},
	}
}

func (c *Config) ValidateKeyValue() error {
	if c.KeyValueURL == "" {
		return ErrKeyValueURLRequired
	}
	if !keyValuePrefixPattern.MatchString(c.KeyValuePrefix) {
		return ErrKeyValuePrefixInvalid
	}
	return nil
}

func (c *Config) ValidateSessionRevalidateInterval() error {
	if c.SessionRevalidateInterval < time.Minute {
		return ErrSessionRevalidateIntervalTooShort
	}
	return nil
}

func (c *Config) ValidateSwagger() error {
	if !c.SwaggerEnabled {
		return nil
	}
	if c.SwaggerBasicUser == "" || c.SwaggerBasicPassword == "" {
		return ErrSwaggerCredentialsRequired
	}
	return nil
}

func (c *Config) ValidateMail() error {
	if c.SendGridConfig.APIKey == "" {
		return ErrSendGridAPIKeyRequired
	}
	if c.SendGridConfig.FromAddress == "" {
		return ErrMailFromAddressRequired
	}
	if c.AppURL == "" {
		return ErrAppURLRequired
	}
	return nil
}

func (c *Config) ValidateEnvironment() error {
	if c.Environment == environmentProduction || c.Environment == environmentDevelopment {
		return nil
	}
	return ErrEnvironmentInvalid
}

func (c *Config) IsProduction() bool {
	return c.Environment == environmentProduction
}

func (c *Config) KeyValueKeyPrefix() string {
	return keyValueNamespace + c.KeyValuePrefix
}

func resolveEnvironment() string {
	if renderSignalsProduction() {
		return environmentProduction
	}
	if env := strings.TrimSpace(os.Getenv("APP_ENV")); env != "" {
		return env
	}
	return environmentDevelopment
}

func renderSignalsProduction() bool {
	render := strings.TrimSpace(os.Getenv("RENDER"))
	return render == "true" || render == "1"
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

func boolEnv(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
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
