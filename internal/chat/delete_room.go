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

type DeleteRoomInput struct {
	ID    uuid.UUID
	IsMod bool
}

type DeleteRoomService struct {
	repo      Repository
	publisher realtime.Publisher
}

func NewDeleteRoomService(repo Repository, publisher realtime.Publisher) *DeleteRoomService {
	return &DeleteRoomService{repo: repo, publisher: publisher}
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

	payload := RoomDeletedPayload{
		ID:    in.ID,
		IsMod: in.IsMod,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return ErrDeleteRoom(err)
	}

	topic := realtime.Topic(fmt.Sprintf("room:%s", in.ID.String()))

	env := realtime.Envelope{
		V:     1,
		Type:  EventRoomDeleted,
		Topic: topic,
		TS:    time.Now(),
		Data:  payloadBytes,
	}

	err = s.publisher.Publish(ctx, topic, env)
	if err != nil {
		return ErrDeleteRoom(err)
	}

	return nil
}
