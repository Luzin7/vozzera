package chat

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

type UpdateMessageRequest struct {
	Content string `json:"content"`
}

type ChatDeps struct {
	Repo       Repository
	RoomAccess RoomAccess
	Publisher  realtime.Publisher
	AuthMW     func(http.Handler) http.Handler
}

type Handler struct {
	getMessages   *GetMessagesService
	updateMessage *UpdateMessageService
	deleteMessage *DeleteMessageService
	roomAccess    RoomAccess
}

func RegisterHandlers(mux *http.ServeMux, deps ChatDeps) {
	h := &Handler{
		getMessages:   NewGetMessagesService(deps.Repo),
		updateMessage: NewUpdateMessageService(deps.Repo, deps.Publisher),
		deleteMessage: NewDeleteMessageService(deps.Repo, deps.Publisher),
		roomAccess:    deps.RoomAccess,
	}

	mux.Handle("GET /api/rooms/{id}/messages", deps.AuthMW(http.HandlerFunc(h.handleGetMessages)))
	mux.Handle("PATCH /api/rooms/{id}/messages/{content_id}", deps.AuthMW(http.HandlerFunc(h.handleUpdateMessage)))
	mux.Handle("DELETE /api/rooms/{id}/messages/{content_id}", deps.AuthMW(http.HandlerFunc(h.handleDeleteMessage)))
}

func (h *Handler) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	if err := h.ensureRoomAccess(r, roomID, claims); err != nil {
		httpx.WriteError(w, err)
		return
	}

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}

	out, err := h.getMessages.Execute(r.Context(), GetMessagesInput{
		RoomID: roomID,
		Limit:  limit,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, MessagesPresenter(out.Messages))
}

func (h *Handler) handleUpdateMessage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	contentID, err := uuid.Parse(r.PathValue("content_id"))
	if err != nil {
		http.Error(w, "ID de conteúdo inválido", http.StatusBadRequest)
		return
	}

	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	var req UpdateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	out, err := h.updateMessage.Execute(r.Context(), UpdateMessageInput{
		RoomID:    roomID,
		ContentID: contentID,
		UserID:    claims.UserID,
		Content:   req.Content,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, UpdateMessagePresenter(out.Message))
}

func (h *Handler) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	contentID, err := uuid.Parse(r.PathValue("content_id"))
	if err != nil {
		http.Error(w, "ID de conteúdo inválido", http.StatusBadRequest)
		return
	}

	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	out, err := h.deleteMessage.Execute(r.Context(), DeleteMessageInput{
		RoomID:    roomID,
		ContentID: contentID,
		UserID:    claims.UserID,
		IsMod:     claims.CanModerate(),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, DeleteMessagePresenter(out.Message))
}

func (h *Handler) ensureRoomAccess(r *http.Request, roomID uuid.UUID, claims httpx.UserClaims) error {
	if h.roomAccess == nil {
		return nil
	}

	allowed, err := h.roomAccess.CanAccess(r.Context(), roomID, claims)
	if err != nil {
		return ErrGetMessages(err)
	}
	if !allowed {
		return ErrRoomNotFound
	}
	return nil
}
