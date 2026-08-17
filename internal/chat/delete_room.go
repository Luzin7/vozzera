package chat

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DeleteRoomInput struct {
	ID    uuid.UUID
	IsMod bool
}

type DeleteRoomService struct {
	repo   Repository
	events RoomBroadcaster
}

func NewDeleteRoomService(repo Repository, events RoomBroadcaster) *DeleteRoomService {
	return &DeleteRoomService{repo: repo, events: events}
}

func (s *DeleteRoomService) Execute(ctx context.Context, in DeleteRoomInput) error {
	if !in.IsMod {
		return ErrNotAuthorized
	}

	_, err := s.repo.DeleteRoom(ctx, in.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRoomNotFound
	}
	if err != nil {
		return ErrDeleteRoom(err)
	}

	s.events.CloseRoom(in.ID)
	return nil
}
