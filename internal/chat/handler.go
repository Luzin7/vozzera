package chat

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

type CreateRoomRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type UpdateRoomRequest struct {
	Name string `json:"name"`
}

type UpdateMessageRequest struct {
	Content string `json:"content"`
}

type ChatDeps struct {
	Repo           Repository
	Publisher      realtime.Publisher
	Registerer     realtime.Registerer
	Handler        realtime.InboundHandler
	AuthMW         func(http.Handler) http.Handler
	AllowedOrigins []string
}

type Handler struct {
	listRooms      *ListRoomsService
	createRoom     *CreateRoomService
	updateRoom     *UpdateRoomService
	deleteRoom     *DeleteRoomService
	getMessages    *GetMessagesService
	updateMessage  *UpdateMessageService
	deleteMessage  *DeleteMessageService
	registerer     realtime.Registerer
	handler        realtime.InboundHandler
	allowedOrigins []string
}

func RegisterHandlers(mux *http.ServeMux, deps ChatDeps) {
	h := &Handler{
		listRooms:      NewListRoomsService(deps.Repo),
		createRoom:     NewCreateRoomService(deps.Repo, deps.Publisher),
		updateRoom:     NewUpdateRoomService(deps.Repo, deps.Publisher),
		deleteRoom:     NewDeleteRoomService(deps.Repo, deps.Publisher),
		getMessages:    NewGetMessagesService(deps.Repo),
		updateMessage:  NewUpdateMessageService(deps.Repo, deps.Publisher),
		deleteMessage:  NewDeleteMessageService(deps.Repo, deps.Publisher),
		registerer:     deps.Registerer,
		handler:        deps.Handler,
		allowedOrigins: deps.AllowedOrigins,
	}

	mux.Handle("GET /api/rooms", deps.AuthMW(http.HandlerFunc(h.handleListRooms)))
	mux.Handle("POST /api/rooms", deps.AuthMW(http.HandlerFunc(h.handleCreateRoom)))
	mux.Handle("PATCH /api/rooms/{id}", deps.AuthMW(http.HandlerFunc(h.handleUpdateRoom)))
	mux.Handle("DELETE /api/rooms/{id}", deps.AuthMW(http.HandlerFunc(h.handleDeleteRoom)))
	mux.Handle("GET /api/rooms/{id}/messages", deps.AuthMW(http.HandlerFunc(h.handleGetMessages)))
	mux.Handle("PATCH /api/rooms/{id}/messages/{content_id}", deps.AuthMW(http.HandlerFunc(h.handleUpdateMessage)))
	mux.Handle("DELETE /api/rooms/{id}/messages/{content_id}", deps.AuthMW(http.HandlerFunc(h.handleDeleteMessage)))
	mux.Handle("GET /api/ws", deps.AuthMW(http.HandlerFunc(h.handleWebSocket)))
}

func (h *Handler) handleListRooms(w http.ResponseWriter, r *http.Request) {
	out, err := h.listRooms.Execute(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, RoomsPresenter(out.Rooms))
}

func (h *Handler) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	out, err := h.createRoom.Execute(r.Context(), CreateRoomInput{
		Name:  req.Name,
		Type:  req.Type,
		IsMod: claims.CanModerate(),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, RoomPresenter(out.Room))
}

func (h *Handler) handleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req UpdateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	out, err := h.updateRoom.Execute(r.Context(), UpdateRoomInput{
		ID:    roomID,
		Name:  req.Name,
		IsMod: claims.CanModerate(),
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, RoomPresenter(out.Room))
}

func (h *Handler) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	if err := h.deleteRoom.Execute(r.Context(), DeleteRoomInput{
		ID:    roomID,
		IsMod: claims.CanModerate(),
	}); err != nil {
		httpx.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
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

func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			for _, allowed := range h.allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := realtime.NewClient(h.registerer, conn, claims.UserID, claims.Username, claims.SessionID, h.handler)

	h.registerer.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
