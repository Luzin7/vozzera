package ws

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const testOrigin = "https://vozzera.test"

func TestConnectTopics_Member(t *testing.T) {
	got := connectTopics(httpx.UserClaims{Role: httpx.RoleUser})

	want := []realtime.Topic{realtime.GlobalPresenceTopic, realtime.MemberRoomListTopic}
	if !slices.Equal(got, want) {
		t.Errorf("connectTopics = %v, want %v", got, want)
	}
}

func TestConnectTopics_Staff(t *testing.T) {
	got := connectTopics(httpx.UserClaims{Role: httpx.RoleMod})

	want := []realtime.Topic{realtime.GlobalPresenceTopic, realtime.StaffRoomListTopic}
	if !slices.Equal(got, want) {
		t.Errorf("connectTopics = %v, want %v", got, want)
	}
}

func TestHandler_RevokedSessionClosesSocketWith4001(t *testing.T) {
	hub := realtime.NewHub()
	go hub.Run()
	sessionID := uuid.New()
	conn := connectedSocket(t, hub, sessionID)

	if err := hub.Revoke(context.Background(), sessionID); err != nil {
		t.Fatalf("Revoke erro: %v", err)
	}

	assertCloseCode(t, conn, realtime.CloseSessionRevoked)
}

func TestHandler_ServerShutdownClosesSocketWithGoingAway(t *testing.T) {
	hub := realtime.NewHub()
	go hub.Run()
	conn := connectedSocket(t, hub, uuid.New())

	if err := hub.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown erro: %v", err)
	}

	assertCloseCode(t, conn, websocket.CloseGoingAway)
}

func connectedSocket(t *testing.T, hub *realtime.Hub, sessionID uuid.UUID) *websocket.Conn {
	t.Helper()
	handler := NewHandler(HandlerDeps{Registerer: hub, Router: NewRouter(), AllowedOrigins: []string{testOrigin}})
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

	waitUntilSubscribed(t, hub, conn)
	return conn
}

func waitUntilSubscribed(t *testing.T, hub *realtime.Hub, conn *websocket.Conn) {
	t.Helper()
	ping := realtime.Envelope{V: 1, Type: "test.ping", Topic: realtime.GlobalPresenceTopic, TS: time.Now()}
	for range 50 {
		hub.Publish(context.Background(), realtime.GlobalPresenceTopic, ping)
		conn.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
		if _, _, err := conn.ReadMessage(); err == nil {
			conn.SetReadDeadline(time.Time{})
			return
		}
	}
	t.Fatal("socket não foi registrado no hub")
}

func assertCloseCode(t *testing.T, conn *websocket.Conn, want int) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue
		}
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) {
			t.Fatalf("leitura terminou sem frame de fechamento: %v", err)
		}
		if closeErr.Code != want {
			t.Errorf("código de fechamento = %d, want %d", closeErr.Code, want)
		}
		return
	}
}
