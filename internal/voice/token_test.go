package voice

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestTokenService_Execute(t *testing.T) {
	repo := newFakeRepo()
	repo.getRoomByID = func(ctx context.Context, id uuid.UUID) (Room, error) {
		return Room{ID: id, Name: "voz", Type: "voice"}, nil
	}

	svc := NewTokenService(repo, NewTokenIssuer("key", "secret"), "wss://livekit")

	t.Run("sala não é de voz", func(t *testing.T) {
		repo.getRoomByID = func(ctx context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, Name: "texto", Type: "text"}, nil
		}
		_, err := svc.Execute(context.Background(), TokenInput{UserID: uuid.New(), Username: "luand", RoomID: uuid.New()})
		if !errors.Is(err, ErrNotVoiceRoom) {
			t.Errorf("erro = %v, want ErrNotVoiceRoom", err)
		}
	})

	t.Run("sala inexistente", func(t *testing.T) {
		repo.getRoomByID = func(context.Context, uuid.UUID) (Room, error) { return Room{}, pgx.ErrNoRows }
		_, err := svc.Execute(context.Background(), TokenInput{UserID: uuid.New(), Username: "luand", RoomID: uuid.New()})
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("sucesso", func(t *testing.T) {
		repo.getRoomByID = func(ctx context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id, Name: "voz", Type: "voice"}, nil
		}
		out, err := svc.Execute(context.Background(), TokenInput{UserID: uuid.New(), Username: "luand", RoomID: uuid.New()})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Token == "" {
			t.Error("token vazio")
		}
		if out.URL != "wss://livekit" {
			t.Errorf("url = %q, want %q", out.URL, "wss://livekit")
		}
		if out.RoomName != "voz" {
			t.Errorf("room name = %q, want %q", out.RoomName, "voz")
		}
	})
}
