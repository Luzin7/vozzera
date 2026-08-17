package chat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSendMessageService_Execute(t *testing.T) {
	repo := newFakeRepo()
	repo.createMessage = func(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error) {
		return CreateMessageRow{
			ID:        uuid.New(),
			RoomID:    arg.RoomID,
			UserID:    arg.UserID,
			Content:   arg.Content,
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}, nil
	}

	events := &fakeBroadcaster{}
	svc := NewSendMessageService(repo, events)

	t.Run("conteúdo em branco", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), SendMessageInput{RoomID: uuid.New(), Content: "   "})
		if !errors.Is(err, ErrInvalidContent) {
			t.Errorf("erro = %v, want ErrInvalidContent", err)
		}
	})

	t.Run("sucesso emite broadcast", func(t *testing.T) {
		out, err := svc.Execute(context.Background(), SendMessageInput{
			RoomID:   uuid.New(),
			UserID:   uuid.New(),
			Username: "luand",
			Content:  " oi ",
		})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Content != "oi" {
			t.Errorf("content = %q, want %q", out.Content, "oi")
		}

		if len(events.events) != 1 {
			t.Fatalf("events = %d, want 1", len(events.events))
		}
		ev := events.events[0]
		if ev.Action != MessageCreated {
			t.Errorf("action = %q, want %q", ev.Action, MessageCreated)
		}
		if ev.Username != "luand" {
			t.Errorf("username = %q, want %q", ev.Username, "luand")
		}
	})
}
