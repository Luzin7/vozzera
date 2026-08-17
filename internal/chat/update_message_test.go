package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestUpdateMessageService_Execute(t *testing.T) {
	repo := newFakeRepo()
	repo.updateMessage = func(ctx context.Context, arg UpdateMessageParams) (UpdateMessageRow, error) {
		return UpdateMessageRow{}, pgx.ErrNoRows
	}

	events := &fakeBroadcaster{}
	svc := NewUpdateMessageService(repo, events)

	t.Run("mensagem alheia ou inexistente", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), UpdateMessageInput{
			RoomID:    uuid.New(),
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
}
