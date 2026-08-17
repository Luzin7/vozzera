package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestCreateRoomService_Execute(t *testing.T) {
	repo := newFakeRepo()
	repo.createRoom = func(ctx context.Context, arg CreateRoomParams) (Room, error) {
		return Room{ID: uuid.New(), Name: arg.Name, Type: arg.Type}, nil
	}

	events := &fakeBroadcaster{}
	svc := NewCreateRoomService(repo, events)

	t.Run("sem permissão", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), CreateRoomInput{Name: "sala", Type: "text", IsMod: false})
		if !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("erro = %v, want ErrNotAuthorized", err)
		}
	})

	t.Run("tipo inválido", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), CreateRoomInput{Name: "sala", Type: "video", IsMod: true})
		if !errors.Is(err, ErrInvalidRoomType) {
			t.Errorf("erro = %v, want ErrInvalidRoomType", err)
		}
	})

	t.Run("sucesso emite broadcast", func(t *testing.T) {
		out, err := svc.Execute(context.Background(), CreateRoomInput{Name: "sala", Type: "voice", IsMod: true})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Room.Name != "sala" {
			t.Errorf("name = %q, want %q", out.Room.Name, "sala")
		}

		if len(events.events) != 1 {
			t.Fatalf("events = %d, want 1", len(events.events))
		}
		if events.events[0].Action != RoomCreated {
			t.Errorf("action = %q, want %q", events.events[0].Action, RoomCreated)
		}
	})
}
