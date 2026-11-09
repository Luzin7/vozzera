package ws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type recordingRevalidator struct {
	calls atomic.Int32
	ticks chan struct{}
	err   func() error
}

func (r *recordingRevalidator) RevalidateSession(context.Context, uuid.UUID) error {
	r.calls.Add(1)
	if r.ticks != nil {
		select {
		case r.ticks <- struct{}{}:
		default:
		}
	}
	if r.err == nil {
		return nil
	}
	return r.err()
}

func TestNewHandler_RequiresRevokerWithRevalidation(t *testing.T) {
	_, err := NewHandler(HandlerDeps{
		SessionRevalidation: realtime.SessionRevalidation{
			Revalidator: &recordingRevalidator{},
			Interval:    time.Second,
		},
	})
	if !errors.Is(err, realtime.ErrMissingSessionRevoker) {
		t.Fatalf("erro = %v, want ErrMissingSessionRevoker", err)
	}
}

func TestClient_RevalidationTickerCallsRevalidator(t *testing.T) {
	hub := realtime.NewHub()
	go hub.Run()
	t.Cleanup(func() { hub.Shutdown(context.Background()) })

	revalidator := &recordingRevalidator{ticks: make(chan struct{}, 16)}
	conn := connectRevalidatingSocket(t, hub, revalidator, 20*time.Millisecond)
	defer conn.Close()

	waitForRevalidations(t, revalidator, 2)
}

func TestClient_ValidSessionKeepsConnectionOpen(t *testing.T) {
	hub := realtime.NewHub()
	go hub.Run()
	t.Cleanup(func() { hub.Shutdown(context.Background()) })

	revalidator := &recordingRevalidator{ticks: make(chan struct{}, 16)}
	conn := connectRevalidatingSocket(t, hub, revalidator, 10*time.Millisecond)
	defer conn.Close()

	waitForRevalidations(t, revalidator, 3)

	if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline erro: %v", err)
	}
	_, _, err := conn.ReadMessage()
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		t.Fatalf("conexão fechada com sessão válida: %v", err)
	}
}

func TestClient_ExpiredSessionClosesWith4001(t *testing.T) {
	hub := realtime.NewHub()
	go hub.Run()
	t.Cleanup(func() { hub.Shutdown(context.Background()) })

	revalidator := &recordingRevalidator{
		err: func() error { return realtime.ErrSessionInvalid(errors.New("sessão expirada")) },
	}
	conn := connectRevalidatingSocket(t, hub, revalidator, 20*time.Millisecond)

	assertCloseCode(t, conn, realtime.CloseSessionRevoked)
}

func TestClient_InfrastructureErrorKeepsConnectionOpen(t *testing.T) {
	hub := realtime.NewHub()
	go hub.Run()
	t.Cleanup(func() { hub.Shutdown(context.Background()) })

	revalidator := &recordingRevalidator{
		ticks: make(chan struct{}, 16),
		err:   func() error { return errors.New("banco indisponível") },
	}
	conn := connectRevalidatingSocket(t, hub, revalidator, 10*time.Millisecond)
	defer conn.Close()

	waitForRevalidations(t, revalidator, 3)

	if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline erro: %v", err)
	}
	_, _, err := conn.ReadMessage()
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		t.Fatalf("conexão fechada após erro de infraestrutura: %v", err)
	}
}

func connectRevalidatingSocket(t *testing.T, hub *realtime.Hub, revalidator realtime.SessionRevalidator, interval time.Duration) *websocket.Conn {
	t.Helper()
	subscribed := make(chan struct{})
	handler, err := NewHandler(HandlerDeps{
		Registerer:        hub,
		Router:            NewRouter(),
		SnapshotProviders: []SnapshotProvider{subscribeBarrier(subscribed)},
		AllowedOrigins:    []string{testOrigin},
		SessionRevalidation: realtime.SessionRevalidation{
			Revalidator: revalidator,
			Revoker:     hub,
			Interval:    interval,
		},
	})
	if err != nil {
		t.Fatalf("NewHandler erro: %v", err)
	}
	sessionID := uuid.New()
	authenticated := httpx.Auth(func(context.Context, string) (httpx.UserClaims, error) {
		return httpx.UserClaims{UserID: uuid.New(), Username: "alice", Role: httpx.RoleUser, SessionID: sessionID}, nil
	})
	server := httptest.NewServer(authenticated(handler))
	t.Cleanup(server.Close)

	header := http.Header{}
	header.Set("Origin", testOrigin)
	header.Set("Cookie", httpx.SessionCookieName+"=sessao")
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), header)
	if err != nil {
		t.Fatalf("Dial erro: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	select {
	case <-subscribed:
	case <-time.After(2 * time.Second):
		t.Fatal("socket não foi registrado no hub")
	}

	return conn
}

func waitForRevalidations(t *testing.T, revalidator *recordingRevalidator, want int32) {
	t.Helper()
	for i := int32(0); i < want; i++ {
		select {
		case <-revalidator.ticks:
		case <-time.After(2 * time.Second):
			t.Fatalf("revalidações = %d, want pelo menos %d", revalidator.calls.Load(), want)
		}
	}
}
