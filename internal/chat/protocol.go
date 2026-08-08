package chat

import (
	"time"

	"github.com/google/uuid"
)

// Tipos de evento que trafegam no WS (nos dois sentidos).
const (
	EventMessage  = "message"  // cliente → servidor → todos
	EventJoin     = "join"     // cliente → servidor (entrar numa sala)
	EventPresence = "presence" // servidor → clientes
	EventError    = "error"    // servidor → cliente
)

// InboundEvent é o que o cliente manda.
type InboundEvent struct {
	Type    string    `json:"type"`
	RoomID  uuid.UUID `json:"room_id"`
	Content string    `json:"content,omitempty"`
}

// OutboundEvent é o que o servidor devolve. Sempre com identidade resolvida
// no servidor — NUNCA confie no username que o cliente mandar.
type OutboundEvent struct {
	Type      string    `json:"type"`
	ID        uuid.UUID `json:"id,omitempty"`
	RoomID    uuid.UUID `json:"room_id,omitempty"`
	UserID    uuid.UUID `json:"user_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}
