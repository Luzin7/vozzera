package mail

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeSender struct {
	send func(to, subject, html string) error
}

func (f *fakeSender) Send(to, subject, html string) error {
	return f.send(to, subject, html)
}

type fakeProviderError struct {
	status int
	body   string
}

func (e *fakeProviderError) Error() string {
	return "mailjet respondeu erro"
}

func (e *fakeProviderError) HTTPStatus() int {
	return e.status
}

func (e *fakeProviderError) ResponseBody() string {
	return e.body
}

func waitForStatus(t *testing.T, health *Health, want string) {
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

func TestDispatcherRecordsProviderFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "remetente não verificado", status: http.StatusBadRequest},
		{name: "credenciais", status: http.StatusUnauthorized},
		{name: "conta", status: http.StatusForbidden},
		{name: "limite", status: http.StatusTooManyRequests},
		{name: "interno", status: http.StatusInternalServerError},
		{name: "indisponível", status: http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := NewHealth()
			dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error {
				return &fakeProviderError{status: tt.status, body: "provider failure"}
			}}, health)
			defer dispatcher.Close(context.Background())

			_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")

			waitForStatus(t, health, MailStatusFailing)
		})
	}
}

func TestDispatcherRecordsTransportFailure(t *testing.T) {
	health := NewHealth()
	dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error {
		return errors.New("connection refused")
	}}, health)
	defer dispatcher.Close(context.Background())

	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")

	waitForStatus(t, health, MailStatusFailing)
}

func TestDispatcherSuccessClearsFailure(t *testing.T) {
	var calls atomic.Int32
	health := NewHealth()
	dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error {
		if calls.Add(1) == 1 {
			return &fakeProviderError{status: http.StatusServiceUnavailable, body: "provider failure"}
		}
		return nil
	}}, health)
	defer dispatcher.Close(context.Background())

	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	waitForStatus(t, health, MailStatusFailing)

	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	waitForStatus(t, health, MailStatusOK)
}

func TestDispatcherSendAfterCloseRecordsFailure(t *testing.T) {
	health := NewHealth()
	dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error { return nil }}, health)
	dispatcher.Close(context.Background())

	logs := captureLog(t)
	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")

	if got := health.Status(); got != MailStatusFailing {
		t.Errorf("Status() = %q, want %q", got, MailStatusFailing)
	}
	if !strings.Contains(logs.String(), "dispatcher_closed") {
		t.Errorf("log = %q, want conter dispatcher_closed", logs.String())
	}
}

func TestDispatcherDoesNotBlockWhenQueueFull(t *testing.T) {
	release := make(chan struct{})
	health := NewHealth()
	dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error {
		<-release
		return nil
	}}, health)

	for range dispatcherQueueSize + 1 {
		_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	}

	done := make(chan error, 1)
	go func() {
		done <- dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Send() erro inesperado: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Send() bloqueou com a fila cheia")
	}

	if got := health.Status(); got != MailStatusFailing {
		t.Errorf("Status() = %q, want %q", got, MailStatusFailing)
	}

	close(release)
	dispatcher.Close(context.Background())
}

func TestDispatcherCloseDrainsPendingSends(t *testing.T) {
	var delivered atomic.Int32
	dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error {
		delivered.Add(1)
		return nil
	}}, NewHealth())

	const total = 10
	for range total {
		_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	}

	dispatcher.Close(context.Background())

	if got := delivered.Load(); got != total {
		t.Errorf("entregues = %d, want %d", got, total)
	}
}

func TestDispatcherCloseWaitsForInFlightSend(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var delivered atomic.Int32

	dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error {
		started <- struct{}{}
		<-release
		delivered.Add(1)
		return nil
	}}, NewHealth())

	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	<-started

	closed := make(chan struct{})
	go func() {
		dispatcher.Close(context.Background())
		close(closed)
	}()

	select {
	case <-closed:
		t.Error("Close() retornou antes do envio em curso terminar")
	case <-time.After(20 * time.Millisecond):
	}

	close(release)

	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close() não retornou depois do envio em curso")
	}

	if got := delivered.Load(); got != 1 {
		t.Errorf("entregues = %d, want 1", got)
	}
}

func TestDispatcherDrainTimeoutLogsUndelivered(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)

	dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error {
		started <- struct{}{}
		<-release
		return nil
	}}, NewHealth())
	dispatcher.drainTimeout = 20 * time.Millisecond
	defer func() {
		close(release)
		<-dispatcher.done
	}()

	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	<-started
	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")

	logs := captureLog(t)
	dispatcher.Close(context.Background())

	if !strings.Contains(logs.String(), "undelivered=2") {
		t.Errorf("log = %q, want conter undelivered=2", logs.String())
	}
}

func TestDispatcherLogsProviderErrorAtErrorLevel(t *testing.T) {
	dispatcher := NewMailDispatcher(&fakeSender{send: func(string, string, string) error {
		return &fakeProviderError{status: http.StatusUnauthorized, body: "credenciais inválidas"}
	}}, NewHealth())

	logs := captureLog(t)
	_ = dispatcher.Send("to@example.com", "Assunto", "<p>oi</p>")
	dispatcher.Close(context.Background())

	got := logs.String()
	for _, want := range []string{"email.dispatch.failed", "level=error", "provider_status=401", "credenciais inválidas"} {
		if !strings.Contains(got, want) {
			t.Errorf("log = %q, want conter %q", got, want)
		}
	}
}

func captureLog(t *testing.T) *strings.Builder {
	t.Helper()

	var buf strings.Builder
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buf
}
