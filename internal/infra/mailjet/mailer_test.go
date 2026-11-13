package mailjet

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type sentPayload struct {
	Messages []struct {
		From struct {
			Email string `json:"Email"`
			Name  string `json:"Name"`
		} `json:"From"`
		To []struct {
			Email string `json:"Email"`
		} `json:"To"`
		Subject  string `json:"Subject"`
		HTMLPart string `json:"HTMLPart"`
	} `json:"Messages"`
}

type observedRequest struct {
	user         string
	pass         string
	method       string
	contentType  string
	body         sentPayload
	readErr      error
	unmarshalErr error
}

func testConfig() Config {
	return Config{
		APIKey:      "test-key",
		APISecret:   "test-secret",
		FromAddress: "from@example.com",
		FromName:    "Vozzera",
	}
}

func newTestMailer(t *testing.T, server *httptest.Server) *Mailer {
	t.Helper()

	m, err := newMailer(testConfig(), server.Client(), server.URL)
	if err != nil {
		t.Fatalf("newMailer() erro inesperado: %v", err)
	}

	return m
}

func TestSendDeliveryPayload(t *testing.T) {
	requests := make(chan observedRequest, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		body, readErr := io.ReadAll(r.Body)

		var payload sentPayload
		unmarshalErr := json.Unmarshal(body, &payload)

		requests <- observedRequest{
			user:         user,
			pass:         pass,
			method:       r.Method,
			contentType:  r.Header.Get("Content-Type"),
			body:         payload,
			readErr:      readErr,
			unmarshalErr: unmarshalErr,
		}

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"Messages":[{"Status":"success"}]}`)
	}))
	defer server.Close()

	if err := newTestMailer(t, server).Send("to@example.com", "Assunto", "<p>oi</p>"); err != nil {
		t.Fatalf("Send() erro inesperado: %v", err)
	}

	got := <-requests
	if got.readErr != nil {
		t.Fatalf("ler body: %v", got.readErr)
	}
	if got.unmarshalErr != nil {
		t.Fatalf("desserializar body: %v", got.unmarshalErr)
	}
	if got.method != http.MethodPost {
		t.Errorf("method = %q, esperava POST", got.method)
	}
	if got.contentType != "application/json" {
		t.Errorf("Content-Type = %q", got.contentType)
	}
	if got.user != "test-key" || got.pass != "test-secret" {
		t.Errorf("basic auth = %q/%q", got.user, got.pass)
	}
	if len(got.body.Messages) != 1 {
		t.Fatalf("messages = %d, esperava 1", len(got.body.Messages))
	}

	msg := got.body.Messages[0]
	if msg.From.Email != "from@example.com" || msg.From.Name != "Vozzera" {
		t.Errorf("remetente = %+v", msg.From)
	}
	if len(msg.To) != 1 || msg.To[0].Email != "to@example.com" {
		t.Errorf("destinatário = %+v", msg.To)
	}
	if msg.Subject != "Assunto" {
		t.Errorf("subject = %q", msg.Subject)
	}
	if msg.HTMLPart != "<p>oi</p>" {
		t.Errorf("html = %q", msg.HTMLPart)
	}
}

func TestSendMessageRejectedWithHTTP200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"Messages":[{"Status":"error","Errors":[{"ErrorCode":"send-0003","StatusCode":400,"ErrorMessage":"recipient rejected"}]}]}`)
	}))
	defer server.Close()

	err := newTestMailer(t, server).Send("to@example.com", "Assunto", "<p>oi</p>")
	if !errors.Is(err, ErrMessageRejected) {
		t.Fatalf("erro = %v, esperava ErrMessageRejected", err)
	}
	if !strings.Contains(err.Error(), "recipient rejected") {
		t.Errorf("erro = %q, esperava detalhe do Mailjet", err)
	}
}

func TestSendMissingMessagesIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{}`)
	}))
	defer server.Close()

	err := newTestMailer(t, server).Send("to@example.com", "Assunto", "<p>oi</p>")
	if !errors.Is(err, ErrUnexpectedResponse) {
		t.Fatalf("erro = %v, esperava ErrUnexpectedResponse", err)
	}
}

func TestSendMapsStatusCodes(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected error
	}{
		{name: "400", status: http.StatusBadRequest, expected: ErrBadRequest},
		{name: "401", status: http.StatusUnauthorized, expected: ErrUnauthorized},
		{name: "403", status: http.StatusForbidden, expected: ErrForbidden},
		{name: "429", status: http.StatusTooManyRequests, expected: ErrRateLimited},
		{name: "500", status: http.StatusInternalServerError, expected: ErrServerFailure},
		{name: "503", status: http.StatusServiceUnavailable, expected: ErrServerFailure},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, `{"ErrorMessage":"falhou"}`)
			}))
			defer server.Close()

			err := newTestMailer(t, server).Send("to@example.com", "Assunto", "<p>oi</p>")
			if !errors.Is(err, tt.expected) {
				t.Fatalf("erro = %v, esperava %v", err, tt.expected)
			}
		})
	}
}

func TestSendTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"Messages":[{"Status":"success"}]}`)
	}))
	defer server.Close()

	m, err := newMailer(testConfig(), &http.Client{Timeout: 10 * time.Millisecond}, server.URL)
	if err != nil {
		t.Fatalf("newMailer() erro inesperado: %v", err)
	}

	err = m.Send("to@example.com", "Assunto", "<p>oi</p>")
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("erro = %v, esperava ErrRequestFailed", err)
	}

	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Errorf("erro = %v, esperava timeout de rede", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Errorf("timeout do cliente confundido com cancelamento: %v", err)
	}
}

func TestNewMailerRejectsIncompleteConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "api key vazia", cfg: Config{APISecret: "secret", FromAddress: "from@example.com"}},
		{name: "api secret vazio", cfg: Config{APIKey: "key", FromAddress: "from@example.com"}},
		{name: "remetente vazio", cfg: Config{APIKey: "key", APISecret: "secret"}},
		{name: "config vazia", cfg: Config{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewMailer(tt.cfg); !errors.Is(err, ErrInvalidConfig) {
				t.Fatalf("erro = %v, esperava ErrInvalidConfig", err)
			}
		})
	}
}

func TestSendReadFailurePreservesCause(t *testing.T) {
	readErr := errors.New("falha de leitura")

	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(errorReader{err: readErr}),
		}, nil
	})}

	m, err := newMailer(testConfig(), client, "https://api.mailjet.com/v3.1/send")
	if err != nil {
		t.Fatalf("newMailer() erro inesperado: %v", err)
	}

	err = m.Send("to@example.com", "Assunto", "<p>oi</p>")
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("erro = %v, esperava ErrRequestFailed", err)
	}
	if !errors.Is(err, readErr) {
		t.Errorf("erro = %v, esperava causa de leitura", err)
	}
}

func TestSendBuildRequestFailurePreservesCause(t *testing.T) {
	m, err := newMailer(testConfig(), http.DefaultClient, "://inválida")
	if err != nil {
		t.Fatalf("newMailer() erro inesperado: %v", err)
	}

	err = m.Send("to@example.com", "Assunto", "<p>oi</p>")
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("erro = %v, esperava ErrRequestFailed", err)
	}

	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		t.Errorf("erro = %v, esperava *url.Error", err)
	}
}

func TestSendInvalidResponsePreservesCause(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{`)
	}))
	defer server.Close()

	err := newTestMailer(t, server).Send("to@example.com", "Assunto", "<p>oi</p>")
	if !errors.Is(err, ErrUnexpectedResponse) {
		t.Fatalf("erro = %v, esperava ErrUnexpectedResponse", err)
	}

	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("erro = %v, esperava *json.SyntaxError", err)
	}
}

func TestDefaultEndpointUsesHTTPS(t *testing.T) {
	if !strings.HasPrefix(mailjetAPIURL, "https://api.mailjet.com/") {
		t.Errorf("endpoint = %q, esperava HTTPS em api.mailjet.com", mailjetAPIURL)
	}
}

func TestNoopMailerReportsNotConfigured(t *testing.T) {
	err := NewNoopMailer().Send("to@example.com", "Assunto", "<p>oi</p>")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("erro = %v, esperava ErrNotConfigured", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
