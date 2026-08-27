package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

type CreateRoomInput struct {
	Name  string
	Type  string
	IsMod bool
}

type CreateRoomOutput struct {
	Room Room
}

type CreateRoomService struct {
	repo      Repository
	publisher realtime.Publisher
}

func NewCreateRoomService(repo Repository, publisher realtime.Publisher) *CreateRoomService {
	return &CreateRoomService{repo: repo, publisher: publisher}
}

func (s *CreateRoomService) Execute(ctx context.Context, in CreateRoomInput) (CreateRoomOutput, error) {
	if !in.IsMod {
		return CreateRoomOutput{}, ErrNotAuthorized
	}

	if in.Name == "" {
		return CreateRoomOutput{}, ErrNameRequired
	}
	if len(in.Name) > 100 {
		return CreateRoomOutput{}, ErrNameTooLong
	}
	if in.Type != "text" && in.Type != "voice" {
		return CreateRoomOutput{}, ErrInvalidRoomType
	}

	room, err := s.repo.CreateRoom(ctx, CreateRoomParams{Name: in.Name, Type: in.Type})
	if err != nil {
		return CreateRoomOutput{}, ErrCreateRoom(err)
	}

	payload := RoomPayload{
		ID:        room.ID,
		Name:      room.Name,
		Type:      room.Type,
		CreatedAt: room.CreatedAt.Time,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return CreateRoomOutput{}, fmt.Errorf("failed to marshal room payload: %w", err)
	}

	topic := realtime.Topic("app:rooms")

	env := realtime.Envelope{
		V:     1,
		Type:  EventRoomCreated,
		Topic: topic,
		TS:    time.Now(),
		Data:  payloadBytes,
	}

	s.publisher.Publish(ctx, topic, env)

	return CreateRoomOutput{Room: room}, nil
}
