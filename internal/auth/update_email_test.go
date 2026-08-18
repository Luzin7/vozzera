package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestUpdateEmailService_Execute(t *testing.T) {
	repo := newFakeRepo()
	userID := uuid.New()
	repo.getUserByEmail = func(context.Context, string) (GetUserByEmailRow, error) {
		return GetUserByEmailRow{}, pgx.ErrNoRows
	}

	svc := NewUpdateEmailService(repo)

	t.Run("sucesso normaliza e atualiza", func(t *testing.T) {
		var got UpdateUserEmailParams
		repo.updateUserEmail = func(ctx context.Context, arg UpdateUserEmailParams) error {
			got = arg
			return nil
		}

		out, err := svc.Execute(context.Background(), UpdateEmailInput{UserID: userID, Email: " LUAN@Example.com "})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Email != "luan@example.com" {
			t.Errorf("Email = %q, want luan@example.com", out.Email)
		}
		if got.Email != "luan@example.com" || got.ID != userID {
			t.Errorf("UpdateUserEmailParams = %+v", got)
		}
	})

	t.Run("email inválido", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), UpdateEmailInput{UserID: userID, Email: "sem-arroba"})
		assertStatus(t, err, http.StatusBadRequest)
	})

	t.Run("email em uso por outro usuário", func(t *testing.T) {
		repo.getUserByEmail = func(context.Context, string) (GetUserByEmailRow, error) {
			return GetUserByEmailRow{ID: uuid.New(), Username: "outro"}, nil
		}
		_, err := svc.Execute(context.Background(), UpdateEmailInput{UserID: userID, Email: "luan@example.com"})
		assertStatus(t, err, http.StatusConflict)
	})

	t.Run("email atual do próprio usuário é aceito", func(t *testing.T) {
		repo.getUserByEmail = func(context.Context, string) (GetUserByEmailRow, error) {
			return GetUserByEmailRow{ID: userID, Username: "luan"}, nil
		}
		repo.updateUserEmail = func(context.Context, UpdateUserEmailParams) error { return nil }
		if _, err := svc.Execute(context.Background(), UpdateEmailInput{UserID: userID, Email: "luan@example.com"}); err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
	})

	t.Run("violação de unicidade na atualização vira 409", func(t *testing.T) {
		repo.getUserByEmail = func(context.Context, string) (GetUserByEmailRow, error) {
			return GetUserByEmailRow{}, pgx.ErrNoRows
		}
		repo.updateUserEmail = func(context.Context, UpdateUserEmailParams) error {
			return &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint \"users_email_key\""}
		}
		_, err := svc.Execute(context.Background(), UpdateEmailInput{UserID: userID, Email: "luan@example.com"})
		assertStatus(t, err, http.StatusConflict)
	})
}
