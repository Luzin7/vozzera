package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

const sessionCookieName = "auth_token"

func setSessionCookie(w http.ResponseWriter, sessionID uuid.UUID, expiresAt time.Time) {
	http.SetCookie(w, sessionCookie("", -1, false))
	http.SetCookie(w, sessionCookie(sessionID.String(), int(time.Until(expiresAt).Seconds()), true))
}

func clearSessionCookies(w http.ResponseWriter) {
	http.SetCookie(w, sessionCookie("", -1, false))
	http.SetCookie(w, sessionCookie("", -1, true))
}

func sessionCookie(value string, maxAge int, partitioned bool) *http.Cookie {
	return &http.Cookie{
		Name:        sessionCookieName,
		Value:       value,
		Path:        "/",
		HttpOnly:    true,
		Secure:      true,
		SameSite:    http.SameSiteNoneMode,
		MaxAge:      maxAge,
		Partitioned: partitioned,
	}
}
