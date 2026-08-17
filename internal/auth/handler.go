package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

type RegisterRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthDeps struct {
	Repo       Repository
	InviteCode string
	SessionTTL time.Duration
	Revoker    SessionRevoker
	AuthMW     func(http.Handler) http.Handler
}

type Handler struct {
	register *RegisterService
	login    *LoginService
	me       *MeService
	logout   *LogoutService
}

func RegisterHandlers(mux *http.ServeMux, deps AuthDeps) {
	h := &Handler{
		register: NewRegisterService(deps.Repo, deps.InviteCode),
		login:    NewLoginService(deps.Repo, deps.SessionTTL),
		me:       NewMeService(deps.Repo),
		logout:   NewLogoutService(deps.Repo, deps.Revoker),
	}

	mux.HandleFunc("POST /api/register", h.handleRegister)
	mux.HandleFunc("POST /api/login", h.handleLogin)
	mux.Handle("POST /api/logout", deps.AuthMW(http.HandlerFunc(h.handleLogout)))
	mux.Handle("GET /api/me", deps.AuthMW(http.HandlerFunc(h.handleMe)))
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	out, err := h.register.Execute(r.Context(), RegisterInput{
		Username:   req.Username,
		Password:   req.Password,
		InviteCode: req.InviteCode,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, RegisterPresenter(out))
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	out, err := h.login.Execute(r.Context(), LoginInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    out.SessionID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   int(time.Until(out.ExpiresAt).Seconds()),
	})

	httpx.WriteJSON(w, http.StatusOK, LoginPresenter(out))
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	out, err := h.me.Execute(r.Context(), MeInput{UserID: claims.UserID})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, MePresenter(out))
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	if err := h.logout.Execute(r.Context(), LogoutInput{SessionID: claims.SessionID}); err != nil {
		httpx.WriteError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
}
