package auth

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestMeService_Execute(t *testing.T) {
	repo := newFakeRepo()
	repo.getUserByID = func(ctx context.Context, id uuid.UUID) (User, error) {
		return User{ID: id, Username: "luand", Role: "user", Email: "luand@example.com"}, nil
	}

	svc := NewMeService(repo)

	out, err := svc.Execute(context.Background(), MeInput{UserID: uuid.New()})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}
	if out.Username != "luand" || out.Role != "user" || out.Email != "luand@example.com" {
		t.Errorf("out inesperado: %+v", out)
	}

	t.Run("usuário inexistente", func(t *testing.T) {
		repo.getUserByID = func(context.Context, uuid.UUID) (User, error) { return User{}, pgx.ErrNoRows }
		_, err := svc.Execute(context.Background(), MeInput{UserID: uuid.New()})
		assertStatus(t, err, http.StatusUnauthorized)
	})
}
