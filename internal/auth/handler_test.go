package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

func newAuthMux(t *testing.T, repo *fakeRepo) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	RegisterHandlers(mux, AuthDeps{
		Repo:       repo,
		SessionTTL: time.Hour,
		Revoker:    &revokeRecorder{},
		AuthMW: httpx.Auth(func(context.Context, string) (httpx.UserClaims, error) {
			return httpx.UserClaims{UserID: uuid.New(), SessionID: uuid.New()}, nil
		}),
	})
	return mux
}

func TestHandleLogin_SetsPartitionedCookieAndExpiresLegacyOne(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword() erro inesperado: %v", err)
	}
	sessionID := uuid.New()
	repo := newFakeRepo()
	repo.getUserByIdentifier = func(context.Context, GetUserByIdentifierParams) (User, error) {
		return User{ID: uuid.New(), Username: "luand", PasswordHash: hash}, nil
	}
	repo.insertSession = func(context.Context, InsertSessionParams) (InsertSessionRow, error) {
		return InsertSessionRow{ID: sessionID, ExpiresAt: time.Now().Add(time.Hour)}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"luand","password":"secret123"}`))
	w := httptest.NewRecorder()
	newAuthMux(t, repo).ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %d, want 2 (%v)", len(cookies), w.Header().Values("Set-Cookie"))
	}
	if cookies[0].Partitioned || cookies[0].MaxAge >= 0 {
		t.Errorf("primeiro cookie deveria apagar a versão sem partição: %v", cookies[0])
	}
	if !cookies[1].Partitioned || cookies[1].Value != sessionID.String() {
		t.Errorf("segundo cookie deveria ser a sessão particionada: %v", cookies[1])
	}
	if !cookies[1].HttpOnly || !cookies[1].Secure || cookies[1].SameSite != http.SameSiteNoneMode {
		t.Errorf("sessão sem HttpOnly/Secure/SameSite=None: %v", cookies[1])
	}
}

func TestHandleLogout_ExpiresBothCookieVariants(t *testing.T) {
	repo := newFakeRepo()
	repo.deleteSessionByID = func(context.Context, uuid.UUID) error { return nil }

	req := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	req.AddCookie(&http.Cookie{Name: httpx.SessionCookieName, Value: uuid.New().String()})
	w := httptest.NewRecorder()
	newAuthMux(t, repo).ServeHTTP(w, req)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %d, want 2 (%v)", len(cookies), w.Header().Values("Set-Cookie"))
	}
	if cookies[0].Partitioned || cookies[0].MaxAge >= 0 {
		t.Errorf("primeiro cookie deveria apagar a versão sem partição: %v", cookies[0])
	}
	if !cookies[1].Partitioned || cookies[1].MaxAge >= 0 {
		t.Errorf("segundo cookie deveria apagar a versão particionada: %v", cookies[1])
	}
}
