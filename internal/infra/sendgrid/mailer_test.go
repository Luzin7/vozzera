package sendgrid

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSend(t *testing.T) {
	var gotReq *http.Request
	var gotBody sendRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ler body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("desserializar body: %v", err)
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	m := &SendGridMailer{
		cfg:    Config{APIKey: "test-key", FromAddress: "from@example.com", FromName: "Vozzera"},
		client: server.Client(),
		apiURL: server.URL,
	}

	if err := m.Send("to@example.com", "Assunto", "<p>oi</p>"); err != nil {
		t.Fatalf("Send() erro inesperado: %v", err)
	}

	if gotReq.Method != http.MethodPost {
		t.Errorf("method = %q, esperava POST", gotReq.Method)
	}
	if gotReq.Header.Get("Authorization") != "Bearer test-key" {
		t.Errorf("Authorization = %q", gotReq.Header.Get("Authorization"))
	}
	if gotReq.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", gotReq.Header.Get("Content-Type"))
	}
	if gotBody.Subject != "Assunto" {
		t.Errorf("subject = %q", gotBody.Subject)
	}
	if len(gotBody.Personalizations) != 1 || len(gotBody.Personalizations[0].To) != 1 || gotBody.Personalizations[0].To[0].Email != "to@example.com" {
		t.Errorf("destinatário = %+v", gotBody.Personalizations)
	}
	if gotBody.From.Email != "from@example.com" || gotBody.From.Name != "Vozzera" {
		t.Errorf("remetente = %+v", gotBody.From)
	}
	if len(gotBody.Content) != 1 || gotBody.Content[0].Type != "text/html" || !strings.Contains(gotBody.Content[0].Value, "<p>oi</p>") {
		t.Errorf("conteúdo = %+v", gotBody.Content)
	}
}

func TestSendServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"errors":[{"message":"forbidden"}]}`)
	}))
	defer server.Close()

	m := &SendGridMailer{
		cfg:    Config{APIKey: "test-key", FromAddress: "from@example.com"},
		client: server.Client(),
		apiURL: server.URL,
	}

	err := m.Send("to@example.com", "Assunto", "<p>oi</p>")
	if err == nil {
		t.Fatal("Send() esperava erro")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("erro = %q, esperava menção ao status 403", err)
	}
}

func TestNewSendGridMailerInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "api key vazia", cfg: Config{FromAddress: "from@example.com"}},
		{name: "remetente vazio", cfg: Config{APIKey: "key"}},
		{name: "config vazia", cfg: Config{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewSendGridMailer(tt.cfg); err == nil {
				t.Error("NewSendGridMailer() esperava erro")
			}
		})
	}
}
