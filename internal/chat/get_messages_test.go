package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestGetMessagesService_Execute(t *testing.T) {
	t.Run("limite padrao", func(t *testing.T) {
		repo := newFakeRepo()
		var calledLimit int32
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			calledLimit = arg.Limit
			return nil, nil
		}
		svc := NewGetMessagesService(repo)

		_, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 0})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if calledLimit != 50 {
			t.Errorf("limit = %d, want 50", calledLimit)
		}
	})

	t.Run("limite maximo", func(t *testing.T) {
		repo := newFakeRepo()
		var calledLimit int32
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			calledLimit = arg.Limit
			return nil, nil
		}
		svc := NewGetMessagesService(repo)

		_, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 999})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if calledLimit != 200 {
			t.Errorf("limit = %d, want 200", calledLimit)
		}
	})

	t.Run("ordem reversa", func(t *testing.T) {
		repo := newFakeRepo()
		msg1 := GetMessagesByRoomRow{ID: uuid.New(), Content: pgtype.Text{String: "primeira", Valid: true}}
		msg2 := GetMessagesByRoomRow{ID: uuid.New(), Content: pgtype.Text{String: "segunda", Valid: true}}
		msg3 := GetMessagesByRoomRow{ID: uuid.New(), Content: pgtype.Text{String: "terceira", Valid: true}}
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return []GetMessagesByRoomRow{msg3, msg2, msg1}, nil
		}
		svc := NewGetMessagesService(repo)

		out, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Messages) != 3 {
			t.Fatalf("len(messages) = %d, want 3", len(out.Messages))
		}
		if out.Messages[0].ID != msg1.ID {
			t.Errorf("messages[0].id = %v, want %v", out.Messages[0].ID, msg1.ID)
		}
		if out.Messages[1].ID != msg2.ID {
			t.Errorf("messages[1].id = %v, want %v", out.Messages[1].ID, msg2.ID)
		}
		if out.Messages[2].ID != msg3.ID {
			t.Errorf("messages[2].id = %v, want %v", out.Messages[2].ID, msg3.ID)
		}
	})

	t.Run("sem mensagens", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return nil, nil
		}
		svc := NewGetMessagesService(repo)

		out, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Messages == nil {
			t.Error("Messages = nil, want empty slice")
		}
		if len(out.Messages) != 0 {
			t.Errorf("len(Messages) = %d, want 0", len(out.Messages))
		}
	})

	t.Run("erro do repositorio", func(t *testing.T) {
		repo := newFakeRepo()
		sentinelErr := errors.New("db error")
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return nil, sentinelErr
		}
		svc := NewGetMessagesService(repo)

		_, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
