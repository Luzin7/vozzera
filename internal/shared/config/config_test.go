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

func TestConfig_KeyValueKeyPrefix(t *testing.T) {
	cfg := &Config{KeyValuePrefix: "prod"}

	if got := cfg.KeyValueKeyPrefix(); got != "vz:prod" {
		t.Errorf("KeyValueKeyPrefix() = %q, want vz:prod", got)
	}
}
