package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestLoginService_Execute(t *testing.T) {
	hash, err := HashPassword("secret123")
	if err != nil {
		t.Fatalf("HashPassword() erro inesperado: %v", err)
	}

	repo := newFakeRepo()
	repo.getUserByUsername = func(ctx context.Context, username string) (GetUserByUsernameRow, error) {
		if username != "luand" {
			return GetUserByUsernameRow{}, pgx.ErrNoRows
		}
		return GetUserByUsernameRow{ID: uuid.New(), Username: "luand", PasswordHash: hash}, nil
	}

	svc := NewLoginService(repo, time.Hour)

	t.Run("senha errada", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), LoginInput{Username: "luand", Password: "wrongpass"})
		assertStatus(t, err, http.StatusUnauthorized)
	})

	t.Run("usuário inexistente", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), LoginInput{Username: "nobody", Password: "secret123"})
		assertStatus(t, err, http.StatusUnauthorized)
	})

	t.Run("sucesso cria sessão com TTL", func(t *testing.T) {
		var got InsertSessionParams
		repo.insertSession = func(ctx context.Context, arg InsertSessionParams) (InsertSessionRow, error) {
			got = arg
			return InsertSessionRow{ID: uuid.New(), ExpiresAt: arg.ExpiresAt}, nil
		}

		before := time.Now()
		out, err := svc.Execute(context.Background(), LoginInput{Username: "luand", Password: "secret123"})
		after := time.Now()
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}

		if got.ExpiresAt.Before(before.Add(time.Hour-time.Second)) || got.ExpiresAt.After(after.Add(time.Hour+time.Second)) {
			t.Errorf("ExpiresAt = %v fora da janela de 1h", got.ExpiresAt)
		}
		if out.SessionID == uuid.Nil {
			t.Error("SessionID não gerado")
		}
	})
}
