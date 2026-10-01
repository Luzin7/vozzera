package room

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestDeleteRoomService_Execute(t *testing.T) {
	mod := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleMod}
	user := httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleUser}

	t.Run("sem permissão", func(t *testing.T) {
		svc := NewDeleteRoomService(newFakeRepo(), &fakePublisher{})

		err := svc.Execute(context.Background(), DeleteRoomInput{ID: uuid.New(), Claims: user})
		if !errors.Is(err, ErrNotAuthorized) {
			t.Errorf("erro = %v, want ErrNotAuthorized", err)
		}
	})

	t.Run("sala inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.deleteRoom = func(context.Context, uuid.UUID) (Room, error) {
			return Room{}, pgx.ErrNoRows
		}
		svc := NewDeleteRoomService(repo, &fakePublisher{})

		err := svc.Execute(context.Background(), DeleteRoomInput{ID: uuid.New(), Claims: mod})
		if !errors.Is(err, ErrRoomNotFound) {
			t.Errorf("erro = %v, want ErrRoomNotFound", err)
		}
	})

	t.Run("sucesso", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.deleteRoom = func(_ context.Context, id uuid.UUID) (Room, error) {
			return Room{ID: id}, nil
		}
		events := &fakePublisher{}
		svc := NewDeleteRoomService(repo, events)

		if err := svc.Execute(context.Background(), DeleteRoomInput{ID: roomID, Claims: mod}); err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}

		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}
		if events.envelopes[0].Type != EventRoomDeleted {
			t.Errorf("type = %q, want %q", events.envelopes[0].Type, EventRoomDeleted)
		}

		var payload RoomDeletedPayload
		if err := json.Unmarshal(events.envelopes[0].Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.ID != roomID {
			t.Errorf("id = %v, want %v", payload.ID, roomID)
		}
	})
}
