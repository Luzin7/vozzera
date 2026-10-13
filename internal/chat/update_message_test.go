package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestUpdateMessageService_Execute(t *testing.T) {
	repo := newFakeRepo()
	repo.updateMessage = func(ctx context.Context, arg UpdateMessageParams) (UpdateMessageRow, error) {
		return UpdateMessageRow{}, pgx.ErrNoRows
	}

	events := &fakePublisher{}
	svc := NewUpdateMessageService(repo, events)

	t.Run("mensagem alheia ou inexistente", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), UpdateMessageInput{
			ContentID: uuid.New(),
			UserID:    uuid.New(),
			Content:   "editado",
		})
		if !errors.Is(err, ErrMessageNotEditable) {
			t.Errorf("erro = %v, want ErrMessageNotEditable", err)
		}
	})

	t.Run("conteúdo vazio", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), UpdateMessageInput{Content: ""})
		if !errors.Is(err, ErrEmptyContent) {
			t.Errorf("erro = %v, want ErrEmptyContent", err)
		}
	})

	t.Run("consulta e publica na sala da URL", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		var queriedRoom uuid.UUID
		repo.updateMessage = func(ctx context.Context, arg UpdateMessageParams) (UpdateMessageRow, error) {
			queriedRoom = arg.RoomID
			return UpdateMessageRow{ID: arg.ID, RoomID: arg.RoomID}, nil
		}
		events := &fakePublisher{}
		svc := NewUpdateMessageService(repo, events)

		_, err := svc.Execute(context.Background(), UpdateMessageInput{
			RoomID:    roomID,
			ContentID: uuid.New(),
			UserID:    uuid.New(),
			Content:   "editado",
		})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}

		if queriedRoom != roomID {
			t.Errorf("consulta na sala %v, want %v", queriedRoom, roomID)
		}
		if len(events.topics) != 1 {
			t.Fatalf("eventos publicados = %d, want 1", len(events.topics))
		}
		if events.topics[0] != realtime.RoomTopic(roomID) {
			t.Errorf("topic = %q, want %q", events.topics[0], realtime.RoomTopic(roomID))
		}
		var payload MessageUpdatedPayload
		if err := json.Unmarshal(events.envelopes[0].Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.RoomID != roomID {
			t.Errorf("room_id = %v, want %v", payload.RoomID, roomID)
		}
	})
}
