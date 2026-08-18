package auth

import (
	"context"
	"net/http"
	"strings"
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
	repo.getUserByIdentifier = func(ctx context.Context, arg GetUserByIdentifierParams) (User, error) {
		if arg.Username != "luand" && arg.Email != "luand@example.com" {
			return User{}, pgx.ErrNoRows
		}
		return User{ID: uuid.New(), Username: "luand", Email: "luand@example.com", PasswordHash: hash}, nil
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

	t.Run("identifier muito longo", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), LoginInput{Username: strings.Repeat("a", 255), Password: "secret123"})
		assertStatus(t, err, http.StatusBadRequest)
	})

	t.Run("login por email", func(t *testing.T) {
		var got GetUserByIdentifierParams
		repo.getUserByIdentifier = func(ctx context.Context, arg GetUserByIdentifierParams) (User, error) {
			got = arg
			return User{ID: uuid.New(), Username: "luand", Email: "luand@example.com", PasswordHash: hash}, nil
		}
		repo.insertSession = func(ctx context.Context, arg InsertSessionParams) (InsertSessionRow, error) {
			return InsertSessionRow{ID: uuid.New(), ExpiresAt: arg.ExpiresAt}, nil
		}

		_, err := svc.Execute(context.Background(), LoginInput{Username: " LUAND@Example.COM ", Password: "secret123"})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if got.Email != "luand@example.com" {
			t.Errorf("Email = %q, want luand@example.com", got.Email)
		}
	})

	t.Run("sucesso cria sessão com TTL", func(t *testing.T) {
		repo.getUserByIdentifier = func(ctx context.Context, arg GetUserByIdentifierParams) (User, error) {
			return User{ID: uuid.New(), Username: "luand", Email: "luand@example.com", PasswordHash: hash}, nil
		}
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
