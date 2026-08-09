package chat

import (
	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Handler struct {
	queries *Queries
}

type CreateRoomRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func RegisterHandlers(mux *http.ServeMux, queries *Queries, authMw func(http.Handler) http.Handler) {
	h := &Handler{queries: queries}

	mux.Handle("GET /api/rooms", authMw(http.HandlerFunc(h.handleListRooms)))
	mux.Handle("POST /api/rooms", authMw(http.HandlerFunc(h.handleCreateRoom)))
	mux.Handle("GET /api/rooms/{id}/messages", authMw(http.HandlerFunc(h.handleGetMessages)))
	mux.Handle("DELETE /api/rooms/{id}", authMw(http.HandlerFunc(h.handleDeleteRoom)))
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
	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Nome é obrigatório", http.StatusBadRequest)
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

func (h *Handler) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	_, err = h.queries.DeleteRoom(r.Context(), roomID)
	if err != nil {
		log.Printf("erro ao excluir sala: %v", err)
		http.Error(w, "Erro ao excluir sala", http.StatusInternalServerError)
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
