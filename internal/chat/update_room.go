package chat

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UpdateRoomInput struct {
	ID    uuid.UUID
	Name  string
	IsMod bool
}

type UpdateRoomOutput struct {
	Room Room
}

type UpdateRoomService struct {
	repo   Repository
	events RoomBroadcaster
}

func NewUpdateRoomService(repo Repository, events RoomBroadcaster) *UpdateRoomService {
	return &UpdateRoomService{repo: repo, events: events}
}

func (s *UpdateRoomService) Execute(ctx context.Context, in UpdateRoomInput) (UpdateRoomOutput, error) {
	if !in.IsMod {
		return UpdateRoomOutput{}, ErrNotAuthorized
	}

	if in.Name == "" {
		return UpdateRoomOutput{}, ErrNameRequired
	}
	if len(in.Name) > 100 {
		return UpdateRoomOutput{}, ErrNameTooLong
	}

	room, err := s.repo.UpdateRoom(ctx, UpdateRoomParams{Name: in.Name, ID: in.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return UpdateRoomOutput{}, ErrRoomNotFound
	}
	if err != nil {
		return UpdateRoomOutput{}, ErrUpdateRoom(err)
	}

	s.events.Broadcast(OutboundEvent{
		Type:     EventRoom,
		Action:   RoomUpdated,
		ID:       room.ID,
		RoomName: room.Name,
		RoomType: room.Type,
	})

	return UpdateRoomOutput{Room: room}, nil
}
