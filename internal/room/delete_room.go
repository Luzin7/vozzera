package room

import (
	"context"
	"errors"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DeleteRoomInput struct {
	ID     uuid.UUID
	Claims httpx.UserClaims
}

type DeleteRoomService struct {
	repo   Repository
	events roomEventPublisher
}

func NewDeleteRoomService(repo Repository, publisher realtime.Publisher) *DeleteRoomService {
	return &DeleteRoomService{repo: repo, events: roomEventPublisher{publisher: publisher}}
}

func (s *DeleteRoomService) Execute(ctx context.Context, in DeleteRoomInput) error {
	if !in.Claims.CanModerate() {
		return ErrNotAuthorized
	}

	_, err := s.repo.DeleteRoom(ctx, in.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrRoomNotFound
	}
	if err != nil {
		return ErrDeleteRoom(err)
	}

	if err := s.events.deleted(ctx, in.ID); err != nil {
		return ErrDeleteRoom(err)
	}
	return nil
}
