package chat

import (
	"time"

	"github.com/google/uuid"
)

type RoomPayload struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type RoomDeletedPayload struct {
	ID    uuid.UUID `json:"id"`
	IsMod bool      `json:"is_mod"`
}

type MessagePayload struct {
	ID        uuid.UUID `json:"id"`
	RoomID    uuid.UUID `json:"room_id"`
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type MessageUpdatedPayload struct {
	ContentID uuid.UUID `json:"content_id"`
	RoomID    uuid.UUID `json:"room_id"`
	UserID    uuid.UUID `json:"user_id"`
	Content   string    `json:"content"`
}

type MessageDeletedPayload struct {
	ContentID uuid.UUID `json:"content_id"`
	RoomID    uuid.UUID `json:"room_id"`
	UserID    uuid.UUID `json:"user_id"`
	IsMod     bool      `json:"is_mod"`
}
