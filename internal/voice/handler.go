package voice

import (
	"encoding/json"
	"net/http"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type tokenRequest struct {
	RoomID uuid.UUID `json:"room_id"`
}

type VoiceDeps struct {
	Access     VoiceRoomAccess
	Issuer     *TokenIssuer
	LiveKitURL string
	AuthMW     func(http.Handler) http.Handler
	ApiKey     string
	ApiSecret  string
	Presence   *VoiceRoomPresence
	Publisher  realtime.Publisher
}

type Handler struct {
	token   *TokenService
	webhook *WebhookHandler
}

func RegisterHandlers(mux *http.ServeMux, deps VoiceDeps) {
	h := &Handler{
		token:   NewTokenService(deps.Access, deps.Issuer, deps.LiveKitURL),
		webhook: NewWebhookHandler(deps.ApiKey, deps.ApiSecret, deps.Presence, deps.Publisher),
	}

	mux.Handle("POST /api/voice/token", deps.AuthMW(http.HandlerFunc(h.handleToken)))
	mux.Handle("POST /api/voice/webhook", http.HandlerFunc(h.webhook.Handle))
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
		Claims: claims,
		RoomID: req.RoomID,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, TokenPresenter(out))
}
