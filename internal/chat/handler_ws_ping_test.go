package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/Luzin7/vozzera-backend/internal/transport/ws"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const pingTestOrigin = "https://vozzera.test"

var pingTestClaims = httpx.UserClaims{
	UserID:    uuid.New(),
	Username:  "luand",
	Role:      httpx.RoleUser,
	SessionID: uuid.New(),
}

type registeringHub struct {
	*realtime.Hub
	registered chan struct{}
}

func (h *registeringHub) Register(c *realtime.Client) {
	h.Hub.Register(c)
	h.registered <- struct{}{}
}

type chatSocketServer struct {
	server     *httptest.Server
	registered chan struct{}
	pub        *fakePublisher
}

func newChatSocketServer(t *testing.T) chatSocketServer {
	t.Helper()
	hub := realtime.NewHub()
	go hub.Run()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		hub.Shutdown(ctx)
	})

	registerer := &registeringHub{Hub: hub, registered: make(chan struct{}, 4)}
	pub := &fakePublisher{}
	router := ws.NewRouter()
	RegisterChatHandlers(router, ChatHandlerDeps{Registerer: registerer, Publisher: pub})
	handler, err := ws.NewHandler(ws.HandlerDeps{
		Registerer:     registerer,
		Router:         router,
		AllowedOrigins: []string{pingTestOrigin},
	})
	if err != nil {
		t.Fatalf("NewHandler erro: %v", err)
	}
	authenticated := httpx.Auth(func(context.Context, string) (httpx.UserClaims, error) {
		return pingTestClaims, nil
	})
	server := httptest.NewServer(authenticated(handler))
	t.Cleanup(server.Close)

	return chatSocketServer{server: server, registered: registerer.registered, pub: pub}
}

func (s chatSocketServer) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	header.Set("Origin", pingTestOrigin)
	header.Set("Cookie", httpx.SessionCookieName+"=sessao")
	conn, _, err := websocket.DefaultDialer.Dial(socketURL(s.server), header)
	if err != nil {
		t.Fatalf("Dial erro: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	select {
	case <-s.registered:
	case <-time.After(time.Second):
		t.Fatal("socket não foi registrado")
	}
	return conn
}

func socketURL(server *httptest.Server) string {
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func TestChatSocket_AuthenticatedPingGetsSinglePong(t *testing.T) {
	s := newChatSocketServer(t)
	first := s.dial(t)
	second := s.dial(t)

	if err := first.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatalf("WriteMessage erro: %v", err)
	}

	first.SetReadDeadline(time.Now().Add(time.Second))
	_, pong, err := first.ReadMessage()
	if err != nil {
		t.Fatalf("pong não recebido: %v", err)
	}
	if string(pong) != `{"type":"pong"}` {
		t.Fatalf("pong = %s, want {\"type\":\"pong\"}", pong)
	}

	first.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if _, extra, err := first.ReadMessage(); err == nil {
		t.Fatalf("frame extra recebido: %s", extra)
	}

	second.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if _, broadcast, err := second.ReadMessage(); err == nil {
		t.Fatalf("outra conexão recebeu: %s", broadcast)
	}

	if len(s.pub.envelopes) != 0 {
		t.Fatalf("publish chamado %d vez(es), want 0", len(s.pub.envelopes))
	}
}

func TestChatSocket_UnauthenticatedPingRejected(t *testing.T) {
	s := newChatSocketServer(t)

	header := http.Header{}
	header.Set("Origin", pingTestOrigin)
	conn, resp, err := websocket.DefaultDialer.Dial(socketURL(s.server), header)
	if err == nil {
		conn.Close()
		t.Fatal("handshake sem sessão foi aceito")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %v, want 401", resp)
	}
}

func TestChatRouter_UnknownTypeIgnored(t *testing.T) {
	reg := newFakeRegisterer()
	client := realtime.NewClient(reg, nil, uuid.New(), "luand", uuid.New(), nil)
	pub := &fakePublisher{}
	r := ws.NewRouter()
	RegisterChatHandlers(r, ChatHandlerDeps{Registerer: reg, Publisher: pub})

	if err := r.HandleMessage(client, realtime.Envelope{V: 1, Type: "tipo.desconhecido"}); err != nil {
		t.Fatalf("HandleMessage() erro inesperado: %v", err)
	}
	if len(pub.envelopes) != 0 {
		t.Fatalf("publish chamado %d vez(es), want 0", len(pub.envelopes))
	}
}
