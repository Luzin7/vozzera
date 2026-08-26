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
	"github.com/jackc/pgx/v5/pgtype"
)

type UpdateMessageInput struct {
	RoomID    uuid.UUID
	ContentID uuid.UUID
	UserID    uuid.UUID
	Content   string
}

type UpdateMessageOutput struct {
	Message UpdateMessageRow
}

type UpdateMessageService struct {
	repo      Repository
	publisher realtime.Publisher
}

func NewUpdateMessageService(repo Repository, publisher realtime.Publisher) *UpdateMessageService {
	return &UpdateMessageService{repo: repo, publisher: publisher}
}

func (s *UpdateMessageService) Execute(ctx context.Context, in UpdateMessageInput) (UpdateMessageOutput, error) {
	if in.Content == "" {
		return UpdateMessageOutput{}, ErrEmptyContent
	}
	if len(in.Content) > 4000 {
		return UpdateMessageOutput{}, ErrContentTooLong
	}

	msg, err := s.repo.UpdateMessage(ctx, UpdateMessageParams{
		Content: pgtype.Text{String: in.Content, Valid: true},
		ID:      in.ContentID,
		UserID:  in.UserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return UpdateMessageOutput{}, ErrMessageNotEditable
	}
	if err != nil {
		return UpdateMessageOutput{}, ErrUpdateMessage(err)
	}

	payload := MessageUpdatedPayload{
		ContentID: in.ContentID,
		RoomID:    in.RoomID,
		UserID:    in.UserID,
		Content:   in.Content,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return UpdateMessageOutput{}, ErrUpdateMessage(err)
	}

	topic := realtime.Topic(fmt.Sprintf("room:%s", in.RoomID.String()))

	env := realtime.Envelope{
		V:     1,
		Type:  EventMessageUpdated,
		Topic: topic,
		TS:    time.Now(),
		Data:  payloadBytes,
	}

	err = s.publisher.Publish(ctx, topic, env)
	if err != nil {
		return UpdateMessageOutput{}, ErrUpdateMessage(err)
	}

	return UpdateMessageOutput{Message: msg}, nil
}
