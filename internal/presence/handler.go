package presence

import (
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

type HandlerDeps struct {
	Service *Service
	AuthMW  func(http.Handler) http.Handler
}

type Handler struct {
	svc *Service
}

func RegisterHandlers(
	mux *http.ServeMux,
	deps HandlerDeps,
) {
	h := &Handler{
		svc: deps.Service,
	}

	mux.Handle(
		"GET /api/presence",
		deps.AuthMW(
			http.HandlerFunc(h.handlePresence),
		),
	)
}

func (h *Handler) handlePresence(
	w http.ResponseWriter,
	r *http.Request,
) {
	online := int64(len(h.svc.onlineUsers))

	h.svc.totalMu.RLock()
	total := h.svc.total
	h.svc.totalMu.RUnlock()

	httpx.WriteJSON(
		w,
		http.StatusOK,
		PresenceSnapshotPayload{
			Online:      online,
			Offline:     total - online,
			Total:       total,
			OnlineUsers: h.svc.OnlineUsers(),
		},
	)
}
