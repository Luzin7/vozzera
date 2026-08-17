package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestResetPasswordService_Execute_InvalidToken(t *testing.T) {
	svc := NewResetPasswordService(newFakeRepo())

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "", Password: "secret123"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestResetPasswordService_Execute_InvalidPassword(t *testing.T) {
	svc := NewResetPasswordService(newFakeRepo())

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc", Password: "short"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestResetPasswordService_Execute_TokenNotFound(t *testing.T) {
	repo := newFakeRepo()
	repo.getPasswordResetTokenByHash = func(ctx context.Context, tokenHash string) (GetPasswordResetTokenByHashRow, error) {
		return GetPasswordResetTokenByHashRow{}, pgx.ErrNoRows
	}

	svc := NewResetPasswordService(repo)

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc", Password: "secret123"})
	assertStatus(t, err, http.StatusBadRequest)
}

func TestResetPasswordService_Execute_Success(t *testing.T) {
	userID := uuid.New()
	tokenID := uuid.New()
	repo := newFakeRepo()

	repo.getPasswordResetTokenByHash = func(ctx context.Context, tokenHash string) (GetPasswordResetTokenByHashRow, error) {
		return GetPasswordResetTokenByHashRow{ID: tokenID, UserID: userID}, nil
	}

	var updated UpdateUserPasswordParams
	repo.updateUserPassword = func(ctx context.Context, arg UpdateUserPasswordParams) error {
		updated = arg
		return nil
	}

	var deletedToken uuid.UUID
	repo.deletePasswordResetToken = func(ctx context.Context, id uuid.UUID) error {
		deletedToken = id
		return nil
	}

	var revokedUser uuid.UUID
	repo.deleteSessionsByUser = func(ctx context.Context, userID uuid.UUID) error {
		revokedUser = userID
		return nil
	}

	svc := NewResetPasswordService(repo)

	_, err := svc.Execute(context.Background(), ResetPasswordInput{Token: "abc123", Password: "newsecret123"})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}

	if updated.ID != userID {
		t.Errorf("senha atualizada para %v, want %v", updated.ID, userID)
	}
	if err := CheckPassword(updated.PasswordHash, "newsecret123"); err != nil {
		t.Error("senha não foi hashada corretamente")
	}
	if deletedToken != tokenID {
		t.Errorf("token apagado = %v, want %v", deletedToken, tokenID)
	}
	if revokedUser != userID {
		t.Errorf("sessões revogadas de %v, want %v", revokedUser, userID)
	}
}
