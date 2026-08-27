package chat

import (
	"context"
	"errors"
	"fmt"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

var ErrNotTextRoom = errors.New("apenas salas de texto e voz podem ser assinadas via websocket")

type RoomAuthorizer struct {
	repo Repository
}

func NewRoomAuthorizer(repo Repository) *RoomAuthorizer {
	return &RoomAuthorizer{repo: repo}
}

func (a *RoomAuthorizer) CanSubscribe(ctx context.Context, userID string, topic realtime.Topic) error {
	room, err := a.repo.GetRoomByID(ctx, roomIDFromTopic(topic))
	if err != nil {
		return fmt.Errorf("sala não encontrada: %w", err)
	}
	if room.Type != "text" && room.Type != "voice" {
		return ErrNotTextRoom
	}
	return nil
}

func roomIDFromTopic(topic realtime.Topic) uuid.UUID {
	raw := string(topic)
	if len(raw) < 5 || raw[:5] != "room:" {
		return uuid.Nil
	}
	id, err := uuid.Parse(raw[5:])
	if err != nil {
		return uuid.Nil
	}
	return id
}

var _ realtime.SubscriptionAuthorizer = (*RoomAuthorizer)(nil)
