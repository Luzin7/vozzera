package config

import (
	"errors"
	"testing"
)

func validMailConfig() *Config {
	return &Config{
		AppURL: "https://app.vozzera.com",
		MailjetConfig: MailjetConfig{
			APIKey:      "key",
			APISecret:   "secret",
			FromAddress: "no-reply@vozzera.com",
		},
	}
}

func TestConfig_ValidateMail(t *testing.T) {
	cases := map[string]struct {
		mutate  func(*Config)
		wantErr error
	}{
		"completo":               {},
		"sem APP_URL":            {mutate: func(c *Config) { c.AppURL = "" }, wantErr: ErrAppURLRequired},
		"sem MAILJET_API_KEY":    {mutate: func(c *Config) { c.MailjetConfig.APIKey = "" }, wantErr: ErrMailjetAPIKeyRequired},
		"sem MAILJET_API_SECRET": {mutate: func(c *Config) { c.MailjetConfig.APISecret = "" }, wantErr: ErrMailjetAPISecretRequired},
		"sem MAIL_FROM_ADDRESS":  {mutate: func(c *Config) { c.MailjetConfig.FromAddress = "" }, wantErr: ErrMailFromAddressRequired},
		"sem nada":               {mutate: func(c *Config) { *c = Config{} }, wantErr: ErrAppURLRequired},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := validMailConfig()
			if tc.mutate != nil {
				tc.mutate(cfg)
			}

			err := cfg.ValidateMail()

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("ValidateMail() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestConfig_ValidateEnvironment(t *testing.T) {
	cases := map[string]struct {
		env     string
		wantErr error
	}{
		"development": {env: environmentDevelopment},
		"production":  {env: environmentProduction},
		"vazio":       {wantErr: ErrEnvironmentInvalid},
		"prod":        {env: "prod", wantErr: ErrEnvironmentInvalid},
		"maiúsculo":   {env: "Production", wantErr: ErrEnvironmentInvalid},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{Environment: tc.env}

			err := cfg.ValidateEnvironment()

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("ValidateEnvironment() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestResolveEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("RENDER", "")
	if got := resolveEnvironment(); got != environmentDevelopment {
		t.Errorf("resolveEnvironment() = %q, want %q", got, environmentDevelopment)
	}

	t.Setenv("APP_ENV", "production")
	if got := resolveEnvironment(); got != environmentProduction {
		t.Errorf("resolveEnvironment() = %q, want %q", got, environmentProduction)
	}

	t.Setenv("APP_ENV", "")
	if got := resolveEnvironment(); got != environmentDevelopment {
		t.Errorf("resolveEnvironment() = %q, want %q", got, environmentDevelopment)
	}

	t.Setenv("APP_ENV", "prod")
	t.Setenv("RENDER", "true")
	if got := resolveEnvironment(); got != environmentProduction {
		t.Errorf("resolveEnvironment() = %q, want %q com RENDER=true", got, environmentProduction)
	}

	t.Setenv("RENDER", "1")
	if got := resolveEnvironment(); got != environmentProduction {
		t.Errorf("resolveEnvironment() = %q, want %q com RENDER=1", got, environmentProduction)
	}
}
