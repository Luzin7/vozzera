package chat

import (
	"context"
	"errors"

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
	repo   Repository
	events RoomBroadcaster
}

func NewDeleteMessageService(repo Repository, events RoomBroadcaster) *DeleteMessageService {
	return &DeleteMessageService{repo: repo, events: events}
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

	s.events.Broadcast(OutboundEvent{
		Type:   EventMessage,
		Action: MessageDeleted,
		ID:     msg.ID,
		RoomID: in.RoomID,
		UserID: in.UserID,
	})

	return DeleteMessageOutput{Message: msg}, nil
}
