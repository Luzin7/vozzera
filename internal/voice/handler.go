package voice

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

type Handler struct {
	queries    *Queries
	issuer     *TokenIssuer
	liveKitURL string
}

type tokenRequest struct {
	RoomID uuid.UUID `json:"room_id"`
}

type tokenResponse struct {
	Token    string `json:"token"`
	URL      string `json:"url"`
	RoomName string `json:"room_name"`
}

func RegisterHandlers(mux *http.ServeMux, queries *Queries, issuer *TokenIssuer, liveKitURL string, authMw func(http.Handler) http.Handler) {
	h := &Handler{queries: queries, issuer: issuer, liveKitURL: liveKitURL}

	mux.Handle("POST /api/voice/token", authMw(http.HandlerFunc(h.handleToken)))

	mux.Handle("GET /api/voice/rooms", authMw(http.HandlerFunc(h.handleListVoiceRooms)))
}

func (h *Handler) handleToken(w http.ResponseWriter, r *http.Request) {
	user, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	room, err := h.queries.GetRoomByID(r.Context(), req.RoomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "Sala não encontrada", http.StatusNotFound)
			return
		}
		log.Printf("erro ao buscar sala: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}

	if room.Type != "voice" {
		http.Error(w, "Esta sala não é de voz", http.StatusBadRequest)
		return
	}

	token, err := h.issuer.IssueToken(user.UserID.String(), room.ID.String(), user.Username)
	if err != nil {
		log.Printf("erro ao gerar token do livekit: %v", err)
		http.Error(w, "Erro ao gerar token", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{
		Token:    token,
		URL:      h.liveKitURL,
		RoomName: room.Name,
	})
}

func (h *Handler) handleListVoiceRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.queries.ListVoiceRooms(r.Context())
	if err != nil {
		log.Printf("erro ao listar salas de voz: %v", err)
		http.Error(w, "Erro ao listar salas", http.StatusInternalServerError)
		return
	}

	if rooms == nil {
		rooms = []Room{}
	}

	writeJSON(w, http.StatusOK, rooms)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("erro ao serializar resposta: %v", err)
	}
}
