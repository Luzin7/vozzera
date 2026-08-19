package chat

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	EventMessage  = "message"  // cliente → servidor → todos
	EventTyping   = "typing"   // cliente → servidor → todos
	EventLeave    = "leave"    // cliente → servidor (sair de uma sala)
	EventRoom     = "room"     // servidor → clientes
	EventJoin     = "join"     // cliente → servidor (entrar numa sala)
	EventPresence = "presence" // servidor → clientes
	EventError    = "error"    // servidor → cliente
)

const (
	EventTypingStart = "start"
	EventTypingStop  = "stop"
)

const (
	MessageCreated = "created"
	MessageUpdated = "updated"
	MessageDeleted = "deleted"
)

const (
	RoomCreated = "created"
	RoomUpdated = "updated"
	RoomDeleted = "deleted"
)

type InboundEvent struct {
	Type    string    `json:"type"`
	RoomID  uuid.UUID `json:"room_id"`
	Content string    `json:"content,omitempty"`
	Action  string    `json:"action,omitempty"`
}

type OutboundEvent struct {
	Type      string    `json:"type"`
	Action    string    `json:"action,omitempty"`
	ID        uuid.UUID `json:"id,omitempty"`
	RoomID    uuid.UUID `json:"room_id,omitempty"`
	RoomName  string    `json:"name,omitempty"`
	RoomType  string    `json:"room_type,omitempty"`
	UserID    uuid.UUID `json:"user_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func jsonMessage(v OutboundEvent) []byte {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return payload
}
