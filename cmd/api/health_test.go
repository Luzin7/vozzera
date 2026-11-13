package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/infra/mail"
)

type fakeDispatcherSender struct {
	failing atomic.Bool
}

func (s *fakeDispatcherSender) Send(to, subject, html string) error {
	if s.failing.Load() {
		return errors.New("send failed")
	}
	return nil
}

func waitForStatus(t *testing.T, health *mail.Health, want string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if health.Status() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("Status() = %q, want %q", health.Status(), want)
}

func getHealth(t *testing.T, path string, health *mail.Health) int {
	t.Helper()

	mux := http.NewServeMux()
	registerHealthHandlers(mux, health)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Code
}

func TestHealthEndpoints(t *testing.T) {
	healthy := &mail.Health{}
	healthy.RecordSuccess()

	failing := &mail.Health{}
	failing.RecordFailure(http.StatusBadRequest)

	cleared := &mail.Health{}
	cleared.RecordFailure(http.StatusBadRequest)
	cleared.RecordSuccess()

	tests := []struct {
		name       string
		mailHealth *mail.Health
		path       string
		wantCode   int
		wantStatus string
		wantMail   string
	}{
		{name: "shallow com email desligado", path: "/api/health", wantCode: http.StatusOK, wantStatus: "ok", wantMail: "disabled"},
		{name: "deep com email desligado", path: "/api/health/deep", wantCode: http.StatusServiceUnavailable, wantStatus: "degraded", wantMail: "disabled"},
		{name: "shallow sem nenhum envio", mailHealth: &mail.Health{}, path: "/api/health", wantCode: http.StatusOK, wantStatus: "ok", wantMail: "unknown"},
		{name: "deep sem nenhum envio", mailHealth: &mail.Health{}, path: "/api/health/deep", wantCode: http.StatusOK, wantStatus: "ok", wantMail: "unknown"},
		{name: "shallow com email saudável", mailHealth: healthy, path: "/api/health", wantCode: http.StatusOK, wantStatus: "ok", wantMail: "ok"},
		{name: "deep com email saudável", mailHealth: healthy, path: "/api/health/deep", wantCode: http.StatusOK, wantStatus: "ok", wantMail: "ok"},
		{name: "shallow com email falhando", mailHealth: failing, path: "/api/health", wantCode: http.StatusOK, wantStatus: "ok", wantMail: "failing"},
		{name: "deep com email falhando", mailHealth: failing, path: "/api/health/deep", wantCode: http.StatusServiceUnavailable, wantStatus: "degraded", wantMail: "failing"},
		{name: "deep depois de envio bem-sucedido", mailHealth: cleared, path: "/api/health/deep", wantCode: http.StatusOK, wantStatus: "ok", wantMail: "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registerHealthHandlers(mux, tt.mailHealth)

			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCode)
			}

			var body struct {
				Status string `json:"status"`
				Mail   string `json:"mail"`
			}
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("corpo inválido: %v", err)
			}
			if body.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", body.Status, tt.wantStatus)
			}
			if body.Mail != tt.wantMail {
				t.Errorf("mail = %q, want %q", body.Mail, tt.wantMail)
			}
		})
	}
}

func TestDeepHealthTracksAsyncSendOutcome(t *testing.T) {
	health := mail.NewHealth()
	sender := &fakeDispatcherSender{}
	sender.failing.Store(true)

	dispatcher := mail.NewMailDispatcher(sender, health)
	defer dispatcher.Close(context.Background())

	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	waitForStatus(t, health, mail.MailStatusFailing)

	if got := getHealth(t, "/api/health/deep", health); got != http.StatusServiceUnavailable {
		t.Errorf("deep com falha assíncrona = %d, want %d", got, http.StatusServiceUnavailable)
	}

	sender.failing.Store(false)
	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	waitForStatus(t, health, mail.MailStatusOK)

	if got := getHealth(t, "/api/health/deep", health); got != http.StatusOK {
		t.Errorf("deep depois do sucesso = %d, want %d", got, http.StatusOK)
	}
}
