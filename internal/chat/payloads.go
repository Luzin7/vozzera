package chat

import (
	"time"

	"github.com/google/uuid"
)

var pongFrame = []byte(`{"type":"` + EventPong + `"}`)

type ReplyToPayload struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Content  string    `json:"content"`
}

type MessagePayload struct {
	ID        uuid.UUID         `json:"id"`
	RoomID    uuid.UUID         `json:"room_id"`
	UserID    uuid.UUID         `json:"user_id"`
	Username  string            `json:"username"`
	Content   string            `json:"content"`
	CreatedAt time.Time         `json:"created_at"`
	ReplyTo   *ReplyToPayload   `json:"reply_to"`
	Reactions []ReactionSummary `json:"reactions"`
}

type ReactionPayload struct {
	MessageID uuid.UUID `json:"message_id"`
	Emoji     string    `json:"emoji"`
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Count     int32     `json:"count"`
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
