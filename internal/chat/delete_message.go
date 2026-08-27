package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DeleteMessageInput struct {
	RoomID    uuid.UUID
	ContentID uuid.UUID
	UserID    uuid.UUID
	IsMod     bool
}

type DeleteMessageOutput struct {
	Message DeleteMessageRow
}

type DeleteMessageService struct {
	repo      Repository
	publisher realtime.Publisher
}

func NewDeleteMessageService(repo Repository, publisher realtime.Publisher) *DeleteMessageService {
	return &DeleteMessageService{repo: repo, publisher: publisher}
}

func (s *DeleteMessageService) Execute(ctx context.Context, in DeleteMessageInput) (DeleteMessageOutput, error) {
	msg, err := s.repo.DeleteMessage(ctx, DeleteMessageParams{
		ID:     in.ContentID,
		UserID: in.UserID,
		IsMod:  in.IsMod,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return DeleteMessageOutput{}, ErrMessageNotDeletable
	}
	if err != nil {
		return DeleteMessageOutput{}, ErrDeleteMessage(err)
	}

	payload := MessageDeletedPayload{
		ContentID: msg.ID,
		RoomID:    msg.RoomID,
		UserID:    msg.UserID,
		IsMod:     in.IsMod,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return DeleteMessageOutput{}, ErrDeleteMessage(err)
	}

	topic := realtime.Topic(fmt.Sprintf("room:%s", msg.RoomID.String()))

	err = s.publisher.Publish(ctx, topic, realtime.Envelope{
		V:     1,
		Type:  EventMessageDeleted,
		Topic: topic,
		TS:    time.Now(),
		Data:  payloadBytes,
	})
	if err != nil {
		return DeleteMessageOutput{}, ErrDeleteMessage(err)
	}

	return DeleteMessageOutput{Message: msg}, nil
}
