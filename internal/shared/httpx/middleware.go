package httpx

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey int

const userKey ctxKey = 0

type UserClaims struct {
	UserID   uuid.UUID
	Username string
}

type TokenParser func(token string) (UserClaims, error)

func Auth(parse TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("auth_token")
			if err != nil {
				http.Error(w, "Não autenticado", http.StatusUnauthorized)
				return
			}

			claims, err := parse(cookie.Value)
			if err != nil {
				http.Error(w, "Token inválido", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) (UserClaims, bool) {
	claims, ok := ctx.Value(userKey).(UserClaims)
	return claims, ok
}
