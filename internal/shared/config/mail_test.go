package config

import (
	"errors"
	"testing"
)

func TestConfig_ValidateMail(t *testing.T) {
	cases := map[string]struct {
		appURL      string
		apiKey      string
		fromAddress string
		wantErr     error
	}{
		"completo":              {appURL: "https://app.vozzera.com", apiKey: "SG.key", fromAddress: "no-reply@vozzera.com"},
		"sem APP_URL":           {apiKey: "SG.key", fromAddress: "no-reply@vozzera.com", wantErr: ErrAppURLRequired},
		"sem SENDGRID_API_KEY":  {appURL: "https://app.vozzera.com", fromAddress: "no-reply@vozzera.com", wantErr: ErrSendGridAPIKeyRequired},
		"sem MAIL_FROM_ADDRESS": {appURL: "https://app.vozzera.com", apiKey: "SG.key", wantErr: ErrMailFromAddressRequired},
		"sem nada":              {wantErr: ErrSendGridAPIKeyRequired},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{
				AppURL: tc.appURL,
				SendGridConfig: SendGridConfig{
					APIKey:      tc.apiKey,
					FromAddress: tc.fromAddress,
				},
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
