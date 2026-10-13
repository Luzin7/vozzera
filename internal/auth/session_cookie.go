package auth

import (
	"net/http"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

const expireImmediately = -1

func setSessionCookie(w http.ResponseWriter, sessionID uuid.UUID, expiresAt time.Time) {
	expireLegacyUnpartitionedCookie(w)

	cookie := baseSessionCookie()
	cookie.Value = sessionID.String()
	cookie.MaxAge = int(time.Until(expiresAt).Seconds())
	cookie.Partitioned = true
	http.SetCookie(w, cookie)
}

func clearSessionCookies(w http.ResponseWriter) {
	expireLegacyUnpartitionedCookie(w)

	cookie := baseSessionCookie()
	cookie.MaxAge = expireImmediately
	cookie.Partitioned = true
	http.SetCookie(w, cookie)
}

func expireLegacyUnpartitionedCookie(w http.ResponseWriter) {
	cookie := baseSessionCookie()
	cookie.MaxAge = expireImmediately
	http.SetCookie(w, cookie)
}

func baseSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     httpx.SessionCookieName,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}
}
