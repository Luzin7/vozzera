package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/infra/mail"
	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const resetAppURL = "https://app.vozzera.com"

func newForgotPasswordMux(t *testing.T, repo *fakeRepo, mailer MailSender) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	RegisterHandlers(mux, AuthDeps{
		Repo:             repo,
		SessionTTL:       time.Hour,
		PasswordResetTTL: time.Minute,
		AppURL:           resetAppURL,
		Mailer:           mailer,
		Revoker:          &revokeRecorder{},
		AuthMW: httpx.Auth(func(context.Context, string) (httpx.UserClaims, error) {
			return httpx.UserClaims{UserID: uuid.New(), SessionID: uuid.New()}, nil
		}),
	})
	return mux
}

func forgotPasswordRepo(known bool) *fakeRepo {
	repo := newFakeRepo()
	repo.getUserByEmail = func(_ context.Context, email string) (GetUserByEmailRow, error) {
		if !known {
			return GetUserByEmailRow{}, pgx.ErrNoRows
		}
		return GetUserByEmailRow{ID: uuid.New(), Username: "luand", Email: email}, nil
	}
	repo.insertPasswordResetToken = func(context.Context, InsertPasswordResetTokenParams) (PasswordResetToken, error) {
		return PasswordResetToken{ID: uuid.New()}, nil
	}
	return repo
}

func postForgotPassword(t *testing.T, mux *http.ServeMux, email string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/forgot-password", strings.NewReader(fmt.Sprintf(`{"email":%q}`, email)))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w.Code
}

func TestHandleRequestPasswordReset_UnknownEmailDoesNotSend(t *testing.T) {
	sent := false
	mailer := &fakeMailer{send: func(string, string, string) error {
		sent = true
		return nil
	}}

	got := postForgotPassword(t, newForgotPasswordMux(t, forgotPasswordRepo(false), mailer), "naoexiste@example.com")

	if got != http.StatusOK {
		t.Errorf("status = %d, want 200", got)
	}
	if sent {
		t.Error("email enviado para usuário inexistente")
	}
}

func TestHandleRequestPasswordReset_SendsMailWithResetLink(t *testing.T) {
	var sentBody string
	mailer := &fakeMailer{send: func(to, subject, html string) error {
		sentBody = html
		return nil
	}}

	got := postForgotPassword(t, newForgotPasswordMux(t, forgotPasswordRepo(true), mailer), "luand@example.com")

	if got != http.StatusOK {
		t.Errorf("status = %d, want 200", got)
	}
	if !strings.Contains(sentBody, resetAppURL+"/reset?token=") {
		t.Errorf("email sem link de redefinição: %q", sentBody)
	}
}

func TestHandleRequestPasswordReset_ProviderFailureStillReturnsOK(t *testing.T) {
	mailer := &fakeMailer{send: func(string, string, string) error {
		return errors.New("mailjet respondeu 503")
	}}

	got := postForgotPassword(t, newForgotPasswordMux(t, forgotPasswordRepo(true), mailer), "luand@example.com")

	if got != http.StatusOK {
		t.Errorf("status = %d, want 200", got)
	}
}

func TestHandleRequestPasswordReset_ReturnsBeforeMailerSend(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseWorker := func() {
		releaseOnce.Do(func() { close(release) })
	}

	mailer := &fakeMailer{send: func(to, subject, html string) error {
		started <- subject
		<-release
		return nil
	}}
	dispatcher := mail.NewMailDispatcher(mailer, mail.NewHealth())
	defer dispatcher.Close(context.Background())
	defer releaseWorker()

	_ = dispatcher.Send("warm@example.com", "aquecimento", "")
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker não iniciou")
	}

	response := make(chan int, 1)
	go func() {
		response <- postForgotPassword(t, newForgotPasswordMux(t, forgotPasswordRepo(true), dispatcher), "luand@example.com")
	}()

	select {
	case got := <-response:
		if got != http.StatusOK {
			t.Errorf("status = %d, want 200", got)
		}
	case <-time.After(time.Second):
		t.Fatal("handler bloqueou esperando o mailer")
	}

	select {
	case subject := <-started:
		t.Fatalf("mailer chamado antes do handler retornar: %q", subject)
	default:
	}

	releaseWorker()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("mailer não foi chamado depois do handler retornar")
	}
}
