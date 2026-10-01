package room

import (
	"context"
	"errors"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

func TestRoomAuthorizer_CanSubscribe(t *testing.T) {
	userID := uuid.New()

	t.Run("sala pública", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id}, nil
		}

		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("room:"+roomID.String()))
		if err != nil {
			t.Errorf("erro = %v, want nil", err)
		}
	})

	t.Run("staff_only com moderador", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, StaffOnly: true}, nil
		}
		repo.getUserRole = func(context.Context, uuid.UUID) (string, error) {
			return httpx.RoleMod, nil
		}

		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("room:"+roomID.String()))
		if err != nil {
			t.Errorf("erro = %v, want nil", err)
		}
	})

	t.Run("staff_only com usuário comum", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, StaffOnly: true}, nil
		}
		repo.getUserRole = func(context.Context, uuid.UUID) (string, error) {
			return httpx.RoleUser, nil
		}

		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("room:"+roomID.String()))
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("sala inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(context.Context, uuid.UUID) (Room, error) {
			return Room{}, errors.New("not found")
		}

		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("room:"+uuid.New().String()))
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("tópico inválido", func(t *testing.T) {
		auth := NewRoomAuthorizer(newFakeRepo())
		err := auth.CanSubscribe(context.Background(), userID.String(), realtime.Topic("invalid"))
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})
}
