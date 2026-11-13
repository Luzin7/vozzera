package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/infra/mailjet"
	"github.com/Luzin7/vozzera-backend/internal/shared/config"
)

func TestHealthHandlerReportsEmailState(t *testing.T) {
	tests := []struct {
		name         string
		emailEnabled bool
		wantEmail    string
	}{
		{name: "mailer habilitado", emailEnabled: true, wantEmail: "enabled"},
		{name: "mailer desabilitado", emailEnabled: false, wantEmail: "disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			healthHandler(tt.emailEnabled)(w, httptest.NewRequest(http.MethodGet, "/api/health", nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}

			var body healthResponse
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decodificar body: %v", err)
			}
			if body.Status != "ok" {
				t.Errorf("status = %q, want ok", body.Status)
			}
			if body.Email != tt.wantEmail {
				t.Errorf("email = %q, want %q", body.Email, tt.wantEmail)
			}
		})
	}
}

func TestBuildMailerFallsBackToNoopWithoutConfig(t *testing.T) {
	mailer, emailEnabled, err := buildMailer(&config.Config{})
	if err != nil {
		t.Fatalf("buildMailer() erro inesperado: %v", err)
	}
	if emailEnabled {
		t.Error("envio de email habilitado sem configuração do Mailjet")
	}
	if _, ok := mailer.(*mailjet.NoopMailer); !ok {
		t.Errorf("mailer = %T, want *mailjet.NoopMailer", mailer)
	}
}
