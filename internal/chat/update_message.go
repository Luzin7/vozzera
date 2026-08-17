package chat

import (
	"context"
	"errors"

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
	repo   Repository
	events RoomBroadcaster
}

func NewUpdateMessageService(repo Repository, events RoomBroadcaster) *UpdateMessageService {
	return &UpdateMessageService{repo: repo, events: events}
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

	s.events.Broadcast(OutboundEvent{
		Type:      EventMessage,
		Action:    MessageUpdated,
		ID:        msg.ID,
		RoomID:    in.RoomID,
		UserID:    in.UserID,
		Content:   msg.Content.String,
		UpdatedAt: msg.UpdatedAt.Time,
	})

	return UpdateMessageOutput{Message: msg}, nil
}
