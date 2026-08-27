package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestDeleteMessageService_Execute(t *testing.T) {
	t.Run("mensagem alheia ou inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		repo.deleteMessage = func(ctx context.Context, arg DeleteMessageParams) (DeleteMessageRow, error) {
			return DeleteMessageRow{}, pgx.ErrNoRows
		}
		events := &fakePublisher{}
		svc := NewDeleteMessageService(repo, events)

		_, err := svc.Execute(context.Background(), DeleteMessageInput{
			RoomID:    uuid.New(),
			ContentID: uuid.New(),
			UserID:    uuid.New(),
		})
		if !errors.Is(err, ErrMessageNotDeletable) {
			t.Errorf("erro = %v, want ErrMessageNotDeletable", err)
		}
	})

	t.Run("sucesso como autor", func(t *testing.T) {
		roomID := uuid.New()
		contentID := uuid.New()
		userID := uuid.New()

		repo := newFakeRepo()
		repo.deleteMessage = func(ctx context.Context, arg DeleteMessageParams) (DeleteMessageRow, error) {
			return DeleteMessageRow{
				ID:     contentID,
				RoomID: roomID,
				UserID: userID,
			}, nil
		}
		events := &fakePublisher{}
		svc := NewDeleteMessageService(repo, events)

		_, err := svc.Execute(context.Background(), DeleteMessageInput{
			RoomID:    roomID,
			ContentID: contentID,
			UserID:    userID,
			IsMod:     false,
		})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}

		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}
		env := events.envelopes[0]
		if env.Type != EventMessageDeleted {
			t.Errorf("type = %q, want %q", env.Type, EventMessageDeleted)
		}
		var payload MessageDeletedPayload
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.ContentID != contentID {
			t.Errorf("content_id = %v, want %v", payload.ContentID, contentID)
		}
		if payload.RoomID != roomID {
			t.Errorf("room_id = %v, want %v", payload.RoomID, roomID)
		}
		if payload.UserID != userID {
			t.Errorf("user_id = %v, want %v", payload.UserID, userID)
		}
	})
}
