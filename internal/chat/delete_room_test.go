package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestDeleteRoomService_Execute(t *testing.T) {
	t.Run("sem permissao", func(t *testing.T) {
		repo := newFakeRepo()
		events := &fakePublisher{}
		svc := NewDeleteRoomService(repo, events)

		err := svc.Execute(context.Background(), DeleteRoomInput{ID: uuid.New(), IsMod: false})
		if !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("erro = %v, want ErrNotAuthorized", err)
		}
	})

	t.Run("sala inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.deleteRoom = func(ctx context.Context, id uuid.UUID) (Room, error) {
			return Room{}, pgx.ErrNoRows
		}
		events := &fakePublisher{}
		svc := NewDeleteRoomService(repo, events)

		err := svc.Execute(context.Background(), DeleteRoomInput{ID: uuid.New(), IsMod: true})
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("sucesso", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.deleteRoom = func(ctx context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id}, nil
		}
		events := &fakePublisher{}
		svc := NewDeleteRoomService(repo, events)

		err := svc.Execute(context.Background(), DeleteRoomInput{ID: roomID, IsMod: true})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}

		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}
		env := events.envelopes[0]
		if env.Type != EventRoomDeleted {
			t.Errorf("type = %q, want %q", env.Type, EventRoomDeleted)
		}
		var payload RoomDeletedPayload
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.ID != roomID {
			t.Errorf("id = %v, want %v", payload.ID, roomID)
		}
	})
}
