package chat

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	queries *Queries
	hub     *Hub
}

type CreateRoomRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
type UpdateMessageRequest struct {
	Content string `json:"content"`
}

func RegisterHandlers(mux *http.ServeMux, queries *Queries, hub *Hub, authMw func(http.Handler) http.Handler) {
	h := &Handler{queries: queries, hub: hub}

	mux.Handle("GET /api/rooms", authMw(http.HandlerFunc(h.handleListRooms)))
	mux.Handle("POST /api/rooms", authMw(http.HandlerFunc(h.handleCreateRoom)))
	mux.Handle("GET /api/rooms/{id}/messages", authMw(http.HandlerFunc(h.handleGetMessages)))
	mux.Handle("PATCH /api/rooms/{id}/messages/{content_id}", authMw(http.HandlerFunc(h.handleUpdateMessage)))
	mux.Handle("DELETE /api/rooms/{id}/messages/{content_id}", authMw(http.HandlerFunc(h.handleDeleteMessage)))
}

func (h *Handler) handleListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.queries.ListRooms(r.Context())
	if err != nil {
		log.Printf("erro ao listar salas: %v", err)
		http.Error(w, "Erro ao listar salas", http.StatusInternalServerError)
		return
	}

	if rooms == nil {
		rooms = []Room{}
	}

	writeJSON(w, http.StatusOK, rooms)
}

func (h *Handler) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Nome é obrigatório", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 100 {
		http.Error(w, "Nome deve ter no máximo 100 caracteres", http.StatusBadRequest)
		return
	}
	if req.Type != "text" && req.Type != "voice" {
		http.Error(w, "Tipo deve ser 'text' ou 'voice'", http.StatusBadRequest)
		return
	}

	room, err := h.queries.CreateRoom(r.Context(), CreateRoomParams{
		Name: req.Name,
		Type: req.Type,
	})
	if err != nil {
		log.Printf("erro ao criar sala: %v", err)
		http.Error(w, "Erro ao criar sala", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, room)
}

func (h *Handler) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	messages, err := h.queries.GetMessagesByRoom(r.Context(), GetMessagesByRoomParams{
		RoomID: roomID,
		Limit:  int32(limit),
	})
	if err != nil {
		log.Printf("erro ao buscar mensagens: %v", err)
		http.Error(w, "Erro ao buscar mensagens", http.StatusInternalServerError)
		return
	}

	slices.Reverse(messages)

	if messages == nil {
		messages = []GetMessagesByRoomRow{}
	}

	writeJSON(w, http.StatusOK, messages)
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
		http.Error(w, "Não autorizado", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID

	var req UpdateMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "O conteúdo não pode ser vazio", http.StatusBadRequest)
		return
	}
	if len(req.Content) > 4000 {
		http.Error(w, "O conteúdo deve ter no máximo 4000 caracteres", http.StatusBadRequest)
		return
	}

	msg, err := h.queries.UpdateMessage(r.Context(), UpdateMessageParams{
		Content: pgtype.Text{String: req.Content, Valid: true},
		ID:      contentID,
		UserID:  userID,
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(
				w,
				"Mensagem não encontrada ou você não tem permissão para editá-la",
				http.StatusForbidden,
			)
			return
		}

		log.Printf("erro ao atualizar mensagem: %v", err)
		http.Error(w, "Erro interno ao atualizar mensagem", http.StatusInternalServerError)
		return
	}

	h.hub.broadcast <- OutboundEvent{
		Type:      EventMessage,
		Action:    MessageUpdated,
		ID:        msg.ID,
		RoomID:    roomID,
		UserID:    userID,
		Content:   msg.Content.String,
		UpdatedAt: msg.UpdatedAt.Time,
	}

	writeJSON(w, http.StatusOK, msg)
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
		http.Error(w, "Não autorizado", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID

	msg, err := h.queries.DeleteMessage(r.Context(), DeleteMessageParams{
		ID:     contentID,
		UserID: userID,
	})

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Mensagem não encontrada ou você não tem permissão para deletá-la", http.StatusForbidden)
			return
		}

		log.Printf("erro ao deletar mensagem: %v", err)
		http.Error(w, "Erro interno ao deletar mensagem", http.StatusInternalServerError)
		return
	}

	h.hub.broadcast <- OutboundEvent{
		Type:   EventMessage,
		Action: MessageDeleted,
		ID:     msg.ID,
		RoomID: roomID,
		UserID: userID,
	}

	writeJSON(w, http.StatusOK, msg)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("erro ao serializar resposta: %v", err)
	}
}

func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request, userID uuid.UUID, username string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Erro no upgrade HTTP->WS:", err)
		return
	}

	client := &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 256),
		UserID:   userID,
		Username: username,
		Rooms:    make(map[uuid.UUID]bool),
	}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
