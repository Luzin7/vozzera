package room

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const (
	EventRoomCreated = "room.created"
	EventRoomUpdated = "room.updated"
	EventRoomDeleted = "room.deleted"
)

type RoomPayload struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	HasVoice  bool      `json:"has_voice"`
	StaffOnly bool      `json:"staff_only"`
	CreatedAt time.Time `json:"created_at"`
}

type RoomDeletedPayload struct {
	ID    uuid.UUID `json:"id"`
	IsMod bool      `json:"is_mod"`
}

func publishRoom(ctx context.Context, publisher realtime.Publisher, event string, topic realtime.Topic, room Room) error {
	payload := RoomPayload{
		ID:        room.ID,
		Name:      room.Name,
		HasVoice:  room.HasVoice,
		StaffOnly: room.StaffOnly,
		CreatedAt: room.CreatedAt.Time,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return publisher.Publish(ctx, topic, realtime.Envelope{
		V:     1,
		Type:  event,
		Topic: topic,
		TS:    time.Now(),
		Data:  data,
	})
}
