package room

import (
	"context"
	"errors"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestAccess_CanAccess(t *testing.T) {
	user := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleUser}
	mod := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleMod}

	t.Run("sala inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(context.Context, uuid.UUID) (Room, error) {
			return Room{}, pgx.ErrNoRows
		}
		access := NewAccess(repo)

		allowed, err := access.CanAccess(context.Background(), uuid.New(), user)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if allowed {
			t.Error("allowed = true, want false")
		}
	})

	t.Run("staff_only bloqueia usuário comum", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, StaffOnly: true}, nil
		}
		access := NewAccess(repo)

		allowed, err := access.CanAccess(context.Background(), uuid.New(), user)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if allowed {
			t.Error("allowed = true, want false")
		}
	})

	t.Run("moderador ignora staff_only", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, StaffOnly: true}, nil
		}
		access := NewAccess(repo)

		allowed, err := access.CanAccess(context.Background(), uuid.New(), mod)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if !allowed {
			t.Error("allowed = false, want true")
		}
	})
}

func TestAccess_AuthorizeVoice(t *testing.T) {
	user := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleUser}
	mod := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleAdmin}

	t.Run("sala sem voz", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, HasVoice: false}, nil
		}
		access := NewAccess(repo)

		_, err := access.AuthorizeVoice(context.Background(), uuid.New(), user)
		if !errors.Is(err, ErrNotVoiceRoom) {
			t.Errorf("erro = %v, want ErrNotVoiceRoom", err)
		}
	})

	t.Run("staff_only responde como inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, HasVoice: true, StaffOnly: true}, nil
		}
		access := NewAccess(repo)

		_, err := access.AuthorizeVoice(context.Background(), uuid.New(), user)
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("sucesso retorna nome", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, Name: "voz", HasVoice: true}, nil
		}
		access := NewAccess(repo)

		name, err := access.AuthorizeVoice(context.Background(), uuid.New(), mod)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if name != "voz" {
			t.Errorf("name = %q, want %q", name, "voz")
		}
	})
}
