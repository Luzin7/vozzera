package auth

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestRequestPasswordResetService_Execute_InvalidEmail(t *testing.T) {
	svc := NewRequestPasswordResetService(newFakeRepo(), &fakeMailer{}, "https://app.vozzera.com", 0)

	_, err := svc.Execute(context.Background(), RequestPasswordResetInput{Email: "sem-arroba"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestRequestPasswordResetService_Execute_EmailNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.getUserByEmail = func(ctx context.Context, email string) (GetUserByEmailRow, error) {
		return GetUserByEmailRow{}, pgx.ErrNoRows
	}

	sent := false
	mailer := &fakeMailer{
		send: func(to, subject, html string) error {
			sent = true
			return nil
		},
	}

	svc := NewRequestPasswordResetService(repo, mailer, "https://app.vozzera.com", 0)

	_, err := svc.Execute(context.Background(), RequestPasswordResetInput{Email: "naoexiste@example.com"})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}
	if sent {
		t.Error("email enviado para usuário inexistente")
	}
}

func TestRequestPasswordResetService_Execute_Success(t *testing.T) {
	userID := uuid.New()
	repo := newFakeRepo()
	repo.getUserByEmail = func(ctx context.Context, email string) (GetUserByEmailRow, error) {
		return GetUserByEmailRow{ID: userID, Username: "luand", Email: "luan@example.com"}, nil
	}

	var stored InsertPasswordResetTokenParams
	repo.insertPasswordResetToken = func(ctx context.Context, arg InsertPasswordResetTokenParams) (PasswordResetToken, error) {
		stored = arg
		return PasswordResetToken{ID: uuid.New(), UserID: arg.UserID, TokenHash: arg.TokenHash, ExpiresAt: arg.ExpiresAt}, nil
	}

	var sentTo, sentSubject, sentBody string
	mailer := &fakeMailer{
		send: func(to, subject, html string) error {
			sentTo, sentSubject, sentBody = to, subject, html
			return nil
		},
	}

	svc := NewRequestPasswordResetService(repo, mailer, "https://app.vozzera.com", 0)

	_, err := svc.Execute(context.Background(), RequestPasswordResetInput{Email: "luan@example.com"})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}

	if stored.UserID != userID {
		t.Errorf("token criado para %v, want %v", stored.UserID, userID)
	}
	if stored.TokenHash == "" {
		t.Error("token hash vazio")
	}
	if sentTo != "luan@example.com" {
		t.Errorf("destinatário = %q, want luan@example.com", sentTo)
	}
	if !strings.Contains(sentSubject, "Redefinição de senha") {
		t.Errorf("subject = %q", sentSubject)
	}
	if !strings.Contains(sentBody, "reset?token=") {
		t.Errorf("body sem link de redefinição: %q", sentBody)
	}
}

func TestRequestPasswordResetService_Execute_WithoutAppURL(t *testing.T) {
	repo := newFakeRepo()
	sent := false
	mailer := &fakeMailer{
		send: func(to, subject, html string) error {
			sent = true
			return nil
		},
	}

	svc := NewRequestPasswordResetService(repo, mailer, "", 0)

	_, err := svc.Execute(context.Background(), RequestPasswordResetInput{Email: "luan@example.com"})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}
	if sent {
		t.Error("email enviado sem APP_URL")
	}
}
