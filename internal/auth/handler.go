package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

type Handler struct {
	queries       *Queries
	inviteCode    string
	sessionTTL    time.Duration
	onSessionKill func(uuid.UUID)
}

type RegisterRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func RegisterHandlers(mux *http.ServeMux, queries *Queries, inviteCode string, sessionTTL time.Duration, onSessionKill func(uuid.UUID), authMw func(http.Handler) http.Handler) {
	h := &Handler{
		queries:       queries,
		inviteCode:    inviteCode,
		sessionTTL:    sessionTTL,
		onSessionKill: onSessionKill,
	}
	mux.HandleFunc("POST /api/register", h.handleRegister)
	mux.HandleFunc("POST /api/login", h.handleLogin)
	mux.Handle("POST /api/logout", authMw(http.HandlerFunc(h.handleLogout)))
	mux.Handle("GET /api/me", authMw(http.HandlerFunc(h.handleMe)))
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	if req.InviteCode != h.inviteCode || h.inviteCode == "" {
		http.Error(w, "Código de convite inválido", http.StatusForbidden)
		return
	}

	username := strings.TrimSpace(req.Username)
	if len(username) < 3 || len(username) > 50 {
		http.Error(w, "Username deve ter entre 3 e 50 caracteres", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 || len(req.Password) > 72 {
		http.Error(w, "Senha deve ter entre 8 e 72 caracteres", http.StatusBadRequest)
		return
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Erro ao processar senha", http.StatusInternalServerError)
		return
	}

	user, err := h.queries.CreateUser(r.Context(), CreateUserParams{
		Username:     username,
		PasswordHash: hashedPassword,
	})
	if err != nil {
		http.Error(w, "Erro ao criar usuário ou username já em uso", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Usuário criado", "id": user.ID.String()})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if len(req.Username) > 50 {
		http.Error(w, "Username inválido", http.StatusBadRequest)
		return
	}

	if len(req.Password) > 72 {
		http.Error(w, "Senha inválida", http.StatusBadRequest)
		return
	}

	user, err := h.queries.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		http.Error(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	if err := CheckPassword(user.PasswordHash, req.Password); err != nil {
		http.Error(w, "Credenciais inválidas", http.StatusUnauthorized)
		return
	}

	session, err := h.queries.InsertSession(r.Context(), InsertSessionParams{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(h.sessionTTL),
	})
	if err != nil {
		log.Printf("erro ao criar sessão: %v", err)
		http.Error(w, "Erro ao criar sessão", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    session.ID.String(),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   int(h.sessionTTL.Seconds()),
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Login realizado com sucesso",
		"id":       user.ID.String(),
		"username": user.Username,
	})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	user, err := h.queries.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "Usuário não encontrado", http.StatusUnauthorized)
			return
		}
		log.Printf("erro ao buscar usuário: %v", err)
		http.Error(w, "Erro ao buscar usuário", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"id":       user.ID.String(),
		"username": user.Username,
		"role":     user.Role,
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	if err := h.queries.DeleteSessionByID(r.Context(), claims.SessionID); err != nil {
		log.Printf("erro ao revogar sessão: %v", err)
		http.Error(w, "Erro ao revogar sessão", http.StatusInternalServerError)
		return
	}

	h.onSessionKill(claims.SessionID)

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
