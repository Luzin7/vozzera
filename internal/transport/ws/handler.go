package ws

import (
	"encoding/json"
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/gorilla/websocket"
)

type SnapshotProvider interface {
	OnConnect(client *realtime.Client) []realtime.Envelope
}

type HandlerDeps struct {
	Registerer        realtime.Registerer
	Router            *Router
	SnapshotProviders []SnapshotProvider
	AllowedOrigins    []string
}

type Handler struct {
	registerer        realtime.Registerer
	router            *Router
	snapshotProviders []SnapshotProvider
	allowedOrigins    []string
	upgrader          websocket.Upgrader
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{
		registerer:        deps.Registerer,
		router:            deps.Router,
		snapshotProviders: deps.SnapshotProviders,
		allowedOrigins:    deps.AllowedOrigins,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				for _, allowed := range deps.AllowedOrigins {
					if origin == allowed {
						return true
					}
				}
				return false
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := realtime.NewClient(
		h.registerer, conn,
		claims.UserID, claims.Username,
		claims.SessionID, h.router,
	)

	h.registerer.Register(client)

	go client.WritePump()
	go client.ReadPump()

	for _, provider := range h.snapshotProviders {
		envelopes := provider.OnConnect(client)

		for _, env := range envelopes {
			data, err := json.Marshal(env)
			if err != nil {
				continue
			}

			client.Send(data)
		}
	}

}
