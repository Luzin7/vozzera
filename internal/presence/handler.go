package presence

import (
	"context"
	"net/http"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	allUsers, err := h.svc.stats.ListUsers(ctx)
	if err != nil {
		httpx.WriteJSON(
			w,
			http.StatusInternalServerError,
			map[string]string{"error": "Erro ao carregar lista de usuários"},
		)
		return
	}

	h.svc.totalMu.RLock()
	total := h.svc.total
	h.svc.totalMu.RUnlock()

	h.svc.mu.RLock()
	online := int64(len(h.svc.onlineUsers))
	onlineMap := make(map[uuid.UUID]bool, len(h.svc.onlineUsers))
	for id := range h.svc.onlineUsers {
		onlineMap[id] = true
	}

	onlineList := make([]realtime.UserPresence, 0, len(h.svc.onlineUsers))
	for id, u := range h.svc.onlineUsers {
		onlineList = append(onlineList, realtime.UserPresence{UserID: id, Username: u.Username})
	}
	h.svc.mu.RUnlock()

	offlineUsers := make([]realtime.UserPresence, 0, len(allUsers))
	for _, u := range allUsers {
		if !onlineMap[u.UserID] {
			offlineUsers = append(offlineUsers, u)
		}
	}

	httpx.WriteJSON(
		w,
		http.StatusOK,
		PresenceSnapshotPayload{
			Online:       online,
			Offline:      total - online,
			Total:        total,
			OnlineUsers:  onlineList,
			OfflineUsers: offlineUsers,
		},
	)
}
