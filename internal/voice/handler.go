package voice

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

type tokenRequest struct {
	RoomID uuid.UUID `json:"room_id"`
}

type VoiceDeps struct {
	Repo       Repository
	Issuer     *TokenIssuer
	LiveKitURL string
	AuthMW     func(http.Handler) http.Handler
}

type Handler struct {
	token          *TokenService
	listVoiceRooms *ListVoiceRoomsService
}

func RegisterHandlers(mux *http.ServeMux, deps VoiceDeps) {
	h := &Handler{
		token:          NewTokenService(deps.Repo, deps.Issuer, deps.LiveKitURL),
		listVoiceRooms: NewListVoiceRoomsService(deps.Repo),
	}

	mux.Handle("POST /api/voice/token", deps.AuthMW(http.HandlerFunc(h.handleToken)))
	mux.Handle("GET /api/voice/rooms", deps.AuthMW(http.HandlerFunc(h.handleListVoiceRooms)))
}

func (h *Handler) handleToken(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
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

	out, err := h.token.Execute(r.Context(), TokenInput{
		UserID:   claims.UserID,
		Username: claims.Username,
		RoomID:   req.RoomID,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, TokenPresenter(out))
}

func (h *Handler) handleListVoiceRooms(w http.ResponseWriter, r *http.Request) {
	out, err := h.listVoiceRooms.Execute(r.Context())
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, RoomsPresenter(out.Rooms))
}
