package chat

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
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
	repo      Repository
	publisher realtime.Publisher
}

func NewUpdateRoomService(repo Repository, publisher realtime.Publisher) *UpdateRoomService {
	return &UpdateRoomService{repo: repo, publisher: publisher}
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

	payload := RoomPayload{
		ID:        room.ID,
		Name:      room.Name,
		Type:      room.Type,
		CreatedAt: room.CreatedAt.Time,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return UpdateRoomOutput{}, ErrUpdateRoom(err)
	}

	topic := realtime.Topic("app:rooms")

	env := realtime.Envelope{
		V:     1,
		Type:  EventRoomUpdated,
		Topic: topic,
		TS:    time.Now(),
		Data:  payloadBytes,
	}

	err = s.publisher.Publish(ctx, topic, env)
	if err != nil {
		return UpdateRoomOutput{}, ErrUpdateRoom(err)
	}

	return UpdateRoomOutput{Room: room}, nil
}
