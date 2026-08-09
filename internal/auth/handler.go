package auth

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	queries    *Queries
	jwtSecret  string
	inviteCode string
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

func RegisterHandlers(mux *http.ServeMux, queries *Queries, jwtSecret, inviteCode string) {
	h := &Handler{queries: queries, jwtSecret: jwtSecret, inviteCode: inviteCode}
	mux.HandleFunc("POST /api/register", h.handleRegister)
	mux.HandleFunc("POST /api/login", h.handleLogin)
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	if req.InviteCode != h.inviteCode || h.inviteCode == "" {
		http.Error(w, "Código de convite inválido", http.StatusForbidden)
		return
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Erro ao processar senha", http.StatusInternalServerError)
		return
	}

	user, err := h.queries.CreateUser(r.Context(), CreateUserParams{
		Username:     req.Username,
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
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
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

	tokenString, err := GenerateToken(h.jwtSecret, user.ID, user.Username)
	if err != nil {
		http.Error(w, "Erro ao gerar token", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   2592000,
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Login realizado com sucesso",
		"id":       user.ID.String(),
		"username": user.Username,
	})
}
