package sendgrid

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type providerError interface {
	error
	HTTPStatus() int
	ResponseBody() string
}

func TestNewProviderErrorMasksBearerToken(t *testing.T) {
	const token = "abcdef1234567890"

	err := NewProviderError(http.StatusUnauthorized, `{"errors":[{"message":"invalid Authorization: Bearer `+token+`"}]}`)

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
	const apiKey = "SG.secret-key"

	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			name:   "credenciais inválidas",
			status: http.StatusUnauthorized,
			body:   `{"errors":[{"message":"The provided authorization grant is invalid, expired, or revoked"}]}`,
			want:   "authorization grant is invalid",
		},
		{
			name:   "remetente não verificado",
			status: http.StatusBadRequest,
			body:   `{"errors":[{"message":"The from address does not match a verified Sender Identity"}]}`,
			want:   "verified Sender Identity",
		},
		{
			name:   "conta bloqueada",
			status: http.StatusForbidden,
			body:   `{"errors":[{"message":"access forbidden for key ` + apiKey + `"}]}`,
			want:   "access forbidden",
		},
		{
			name:   "indisponível",
			status: http.StatusServiceUnavailable,
			body:   `{"errors":[{"message":"service unavailable"}]}`,
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

			m := &SendGridMailer{
				cfg:    Config{APIKey: apiKey, FromAddress: "from@example.com"},
				client: server.Client(),
				apiURL: server.URL,
			}

			err := m.Send("to@example.com", "Assunto", "<p>oi</p>")
			if err == nil {
				t.Fatal("Send() esperava erro")
			}

			var pe providerError
			if !errors.As(err, &pe) {
				t.Fatalf("erro não carrega status/corpo do provedor: %v", err)
			}
			if pe.HTTPStatus() != tt.status {
				t.Errorf("HTTPStatus() = %d, want %d", pe.HTTPStatus(), tt.status)
			}
			if !strings.Contains(pe.ResponseBody(), tt.want) {
				t.Errorf("ResponseBody() = %q, want conter %q", pe.ResponseBody(), tt.want)
			}
			if strings.Contains(pe.ResponseBody(), apiKey) {
				t.Errorf("ResponseBody() não redigido: %q", pe.ResponseBody())
			}
		})
	}
}
