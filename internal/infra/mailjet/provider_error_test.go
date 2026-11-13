package mailjet

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewProviderErrorMasksBearerToken(t *testing.T) {
	const token = "abcdef1234567890"

	err := NewProviderError(http.StatusUnauthorized, `{"ErrorMessage":"invalid Authorization: Bearer `+token+`"}`)

	if strings.Contains(err.ResponseBody(), token) {
		t.Errorf("ResponseBody() não mascarou Bearer: %q", err.ResponseBody())
	}
	if !strings.Contains(err.ResponseBody(), redactedValue) {
		t.Errorf("ResponseBody() = %q, want conter %q", err.ResponseBody(), redactedValue)
	}
}

func TestNewProviderErrorTruncatesBody(t *testing.T) {
	err := NewProviderError(http.StatusInternalServerError, strings.Repeat("a", maxProviderErrorBodyBytes+100))

	got := err.ResponseBody()
	if len(got) != maxProviderErrorBodyBytes+len("…") {
		t.Errorf("len(ResponseBody()) = %d, want %d", len(got), maxProviderErrorBodyBytes+len("…"))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("ResponseBody() sem marca de truncamento: %q", got)
	}
}

func TestSendProviderErrorCarriesStatusAndRedactedBody(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			name:   "credenciais inválidas",
			status: http.StatusUnauthorized,
			body:   `{"ErrorMessage":"Invalid API key test-key"}`,
			want:   "Invalid API key",
		},
		{
			name:   "remetente não verificado",
			status: http.StatusBadRequest,
			body:   `{"ErrorMessage":"Missing or invalid sender address"}`,
			want:   "invalid sender address",
		},
		{
			name:   "conta bloqueada",
			status: http.StatusForbidden,
			body:   `{"ErrorMessage":"access forbidden for secret test-secret"}`,
			want:   "access forbidden",
		},
		{
			name:   "indisponível",
			status: http.StatusServiceUnavailable,
			body:   `{"ErrorMessage":"service unavailable"}`,
			want:   "service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			}))
			defer server.Close()

			err := newTestMailer(t, server).Send("to@example.com", "Assunto", "<p>oi</p>")
			if err == nil {
				t.Fatal("Send() esperava erro")
			}

			var providerErr *ProviderError
			if !errors.As(err, &providerErr) {
				t.Fatalf("erro não carrega status/corpo do provedor: %v", err)
			}
			if providerErr.HTTPStatus() != tt.status {
				t.Errorf("HTTPStatus() = %d, want %d", providerErr.HTTPStatus(), tt.status)
			}
			if !strings.Contains(providerErr.ResponseBody(), tt.want) {
				t.Errorf("ResponseBody() = %q, want conter %q", providerErr.ResponseBody(), tt.want)
			}
			if strings.Contains(providerErr.ResponseBody(), testConfig().APIKey) {
				t.Errorf("ResponseBody() não redigiu a API key: %q", providerErr.ResponseBody())
			}
			if strings.Contains(providerErr.ResponseBody(), testConfig().APISecret) {
				t.Errorf("ResponseBody() não redigiu o API secret: %q", providerErr.ResponseBody())
			}
		})
	}
}

func TestSendRejectedMessageCarriesProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"Messages":[{"Status":"error","Errors":[{"ErrorCode":"send-0003","ErrorMessage":"recipient rejected"}]}]}`)
	}))
	defer server.Close()

	err := newTestMailer(t, server).Send("to@example.com", "Assunto", "<p>oi</p>")

	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("erro não carrega status do provedor: %v", err)
	}
	if providerErr.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus() = %d, want %d", providerErr.HTTPStatus(), http.StatusBadRequest)
	}
	if !strings.Contains(providerErr.ResponseBody(), "recipient rejected") {
		t.Errorf("ResponseBody() = %q, want conter detalhe do Mailjet", providerErr.ResponseBody())
	}
}
