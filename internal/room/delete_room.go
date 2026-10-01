package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	repo      Repository
	publisher realtime.Publisher
}

func NewDeleteRoomService(repo Repository, publisher realtime.Publisher) *DeleteRoomService {
	return &DeleteRoomService{repo: repo, publisher: publisher}
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

	payload := RoomDeletedPayload{ID: in.ID, IsMod: true}
	data, err := json.Marshal(payload)
	if err != nil {
		return ErrDeleteRoom(err)
	}

	topic := realtime.Topic(fmt.Sprintf("room:%s", in.ID.String()))
	return s.publisher.Publish(ctx, topic, realtime.Envelope{
		V:     1,
		Type:  EventRoomDeleted,
		Topic: topic,
		TS:    time.Now(),
		Data:  data,
	})
}
