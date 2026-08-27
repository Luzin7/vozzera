package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

func TestRoomAuthorizer_CanSubscribe(t *testing.T) {
	t.Run("sala de texto", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(ctx context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, Type: "text"}, nil
		}
		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), "user1", realtime.Topic("room:"+roomID.String()))
		if err != nil {
			t.Errorf("erro = %v, want nil", err)
		}
	})

	t.Run("sala de voz", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(ctx context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, Type: "voice"}, nil
		}
		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), "user1", realtime.Topic("room:"+roomID.String()))
		if err != nil {
			t.Errorf("erro = %v, want nil", err)
		}
	})

	t.Run("sala inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getRoomByID = func(ctx context.Context, id uuid.UUID) (Room, error) {
			return Room{}, errors.New("not found")
		}
		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), "user1", realtime.Topic("room:"+uuid.New().String()))
		if err == nil {
			t.Error("esperava erro, got nil")
		}
	})

	t.Run("tópico inválido", func(t *testing.T) {
		repo := newFakeRepo()
		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), "user1", realtime.Topic("invalid"))
		if err == nil {
			t.Error("esperava erro, got nil")
		}
	})

	t.Run("tipo de sala inválido", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getRoomByID = func(ctx context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, Type: "video"}, nil
		}
		auth := NewRoomAuthorizer(repo)
		err := auth.CanSubscribe(context.Background(), "user1", realtime.Topic("room:"+roomID.String()))
		if !errors.Is(err, ErrNotTextRoom) {
			t.Errorf("erro = %v, want ErrNotTextRoom", err)
		}
	})
}
