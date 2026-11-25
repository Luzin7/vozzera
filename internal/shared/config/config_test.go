package config

import (
	"errors"
	"testing"
	"time"
)

func TestConfig_ValidateKeyValue(t *testing.T) {
	cases := map[string]struct {
		url    string
		prefix string
		want   error
	}{
		"válido":                      {url: "redis://localhost:6379/0", prefix: "prod", want: nil},
		"prefixo com hífen":           {url: "rediss://kv:6379", prefix: "staging-2", want: nil},
		"url ausente":                 {url: "", prefix: "prod", want: ErrKeyValueURLRequired},
		"prefixo ausente":             {url: "redis://localhost", prefix: "", want: ErrKeyValuePrefixInvalid},
		"prefixo maiúsculo":           {url: "redis://localhost", prefix: "Prod", want: ErrKeyValuePrefixInvalid},
		"prefixo com dois pontos":     {url: "redis://localhost", prefix: "a:b", want: ErrKeyValuePrefixInvalid},
		"prefixo com espaço":          {url: "redis://localhost", prefix: "a b", want: ErrKeyValuePrefixInvalid},
		"prefixo com quebra de linha": {url: "redis://localhost", prefix: "prod\n", want: ErrKeyValuePrefixInvalid},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{KeyValueURL: tc.url, KeyValuePrefix: tc.prefix}

			err := cfg.ValidateKeyValue()

			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestConfig_ValidateSwagger(t *testing.T) {
	cases := map[string]struct {
		enabled  bool
		user     string
		password string
		want     error
	}{
		"desabilitado sem credenciais": {enabled: false, want: nil},
		"habilitado com credenciais":   {enabled: true, user: "user", password: "pass", want: nil},
		"habilitado sem usuário":       {enabled: true, password: "pass", want: ErrSwaggerCredentialsRequired},
		"habilitado sem senha":         {enabled: true, user: "user", want: ErrSwaggerCredentialsRequired},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{
				SwaggerEnabled:       tc.enabled,
				SwaggerBasicUser:     tc.user,
				SwaggerBasicPassword: tc.password,
			}

			err := cfg.ValidateSwagger()

			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestConfig_KeyValueKeyPrefix(t *testing.T) {
	cfg := &Config{KeyValuePrefix: "prod"}

	if got := cfg.KeyValueKeyPrefix(); got != "vz:prod" {
		t.Errorf("KeyValueKeyPrefix() = %q, want vz:prod", got)
	}
}

func TestConfig_ValidateSessionTTL(t *testing.T) {
	cases := map[string]struct {
		ttl, window time.Duration
		want        error
	}{
		"padrão": {ttl: 7 * 24 * time.Hour, window: 24 * time.Hour, want: nil},
		"igual":  {ttl: 24 * time.Hour, window: 24 * time.Hour, want: ErrSessionTTLNotAboveTouchWindow},
		"abaixo": {ttl: time.Hour, window: 24 * time.Hour, want: ErrSessionTTLNotAboveTouchWindow},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{SessionTTL: tc.ttl, SessionTouchWindow: tc.window}

			if err := cfg.ValidateSessionTTL(); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestConfig_ValidateSessionRevalidateInterval(t *testing.T) {
	cases := map[string]struct {
		interval time.Duration
		want     error
	}{
		"padrão":           {interval: 5 * time.Minute, want: nil},
		"mínimo":           {interval: time.Minute, want: nil},
		"abaixo do mínimo": {interval: time.Minute - time.Second, want: ErrSessionRevalidateIntervalTooShort},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{SessionRevalidateInterval: tc.interval}

			if err := cfg.ValidateSessionRevalidateInterval(); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestConfig_LoadSessionRevalidateInterval(t *testing.T) {
	t.Setenv("WS_SESSION_REVALIDATE_INTERVAL", "7m")

	cfg := Load()

	if cfg.SessionRevalidateInterval != 7*time.Minute {
		t.Errorf("SessionRevalidateInterval = %s, want 7m", cfg.SessionRevalidateInterval)
	}
}

func TestConfig_LoadSessionRevalidateIntervalDefault(t *testing.T) {
	t.Setenv("WS_SESSION_REVALIDATE_INTERVAL", "")

	cfg := Load()

	if cfg.SessionRevalidateInterval != 5*time.Minute {
		t.Errorf("SessionRevalidateInterval = %s, want 5m", cfg.SessionRevalidateInterval)
	}
}

func TestConfig_ValidateDurations(t *testing.T) {
	cases := map[string]struct {
		ttl, age time.Duration
		want     error
	}{
		"padrão":         {ttl: 24 * time.Hour, age: 6 * time.Hour, want: nil},
		"idade zero":     {ttl: 24 * time.Hour, age: 0, want: nil},
		"ttl zero":       {ttl: 0, age: time.Hour, want: ErrUsersTotalCacheTTLNotPositive},
		"ttl negativo":   {ttl: -time.Hour, age: time.Hour, want: ErrUsersTotalCacheTTLNotPositive},
		"idade negativa": {ttl: time.Hour, age: -time.Second, want: ErrWSMaxConnectionAgeNegative},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{UsersTotalCacheTTL: tc.ttl, WSMaxConnectionAge: tc.age}

			if err := cfg.ValidateDurations(); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestConfig_LoadUnparseableDurationFailsValidation(t *testing.T) {
	t.Setenv("USERS_TOTAL_CACHE_TTL", "24horas")
	t.Setenv("WS_MAX_CONNECTION_AGE", "abc")

	err := Load().ValidateDurations()

	if !errors.Is(err, ErrDurationInvalid) {
		t.Errorf("err = %v, want ErrDurationInvalid", err)
	}
}

func TestConfig_LoadValidDurations(t *testing.T) {
	t.Setenv("USERS_TOTAL_CACHE_TTL", "2h")
	t.Setenv("WS_MAX_CONNECTION_AGE", "3h")

	cfg := Load()

	if err := cfg.ValidateDurations(); err != nil {
		t.Fatalf("err = %v", err)
	}
	if cfg.UsersTotalCacheTTL != 2*time.Hour || cfg.WSMaxConnectionAge != 3*time.Hour {
		t.Errorf("ttl = %v age = %v", cfg.UsersTotalCacheTTL, cfg.WSMaxConnectionAge)
	}
}
