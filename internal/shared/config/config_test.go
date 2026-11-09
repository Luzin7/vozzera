package config

import (
	"errors"
	"testing"
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
