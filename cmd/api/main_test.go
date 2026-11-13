package main

import (
	"errors"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/config"
)

func productionConfig() *config.Config {
	return &config.Config{
		Environment:               "production",
		KeyValueURL:               "redis://localhost:6379/0",
		KeyValuePrefix:            "test",
		SessionRevalidateInterval: 5 * time.Minute,
		AppURL:                    "https://app.vozzera.com",
		MailjetConfig: config.MailjetConfig{
			APIKey:      "key",
			APISecret:   "secret",
			FromAddress: "no-reply@vozzera.com",
		},
	}
}

func TestValidateBootConfigRefusesProductionWithoutRequiredMailVars(t *testing.T) {
	cases := map[string]struct {
		mutate  func(*config.Config)
		wantErr error
	}{
		"completo":               {},
		"sem APP_URL":            {mutate: func(c *config.Config) { c.AppURL = "" }, wantErr: config.ErrAppURLRequired},
		"sem MAILJET_API_KEY":    {mutate: func(c *config.Config) { c.MailjetConfig.APIKey = "" }, wantErr: config.ErrMailjetAPIKeyRequired},
		"sem MAILJET_API_SECRET": {mutate: func(c *config.Config) { c.MailjetConfig.APISecret = "" }, wantErr: config.ErrMailjetAPISecretRequired},
		"sem MAIL_FROM_ADDRESS":  {mutate: func(c *config.Config) { c.MailjetConfig.FromAddress = "" }, wantErr: config.ErrMailFromAddressRequired},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := productionConfig()
			if tc.mutate != nil {
				tc.mutate(cfg)
			}

			err := validateBootConfig(cfg)

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("validateBootConfig() = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateBootConfigAllowsDevelopmentWithoutMailjet(t *testing.T) {
	cfg := &config.Config{
		Environment:               "development",
		KeyValueURL:               "redis://localhost:6379/0",
		KeyValuePrefix:            "test",
		SessionRevalidateInterval: 5 * time.Minute,
	}

	if err := validateBootConfig(cfg); err != nil {
		t.Errorf("validateBootConfig() = %v, want nil", err)
	}
}
