package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Topic string

const CloseSessionRevoked = 4001

const (
	GlobalPresenceTopic Topic = "__presence__"
	MemberRoomListTopic Topic = "app:rooms:members"
	StaffRoomListTopic  Topic = "app:rooms:staff"
)

func RoomTopic(roomID uuid.UUID) Topic {
	return Topic("room:" + roomID.String())
}

func RoomListTopicFor(canModerate bool) Topic {
	if canModerate {
		return StaffRoomListTopic
	}
	return MemberRoomListTopic
}

type UserPresence struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
}

type Envelope struct {
	V     int             `json:"v"`
	Type  string          `json:"type"`
	Topic Topic           `json:"topic"`
	TS    time.Time       `json:"ts"`
	Data  json.RawMessage `json:"data,omitempty"`
}

type Publisher interface {
	Publish(ctx context.Context, topic Topic, env Envelope) error
}

type Registerer interface {
	Register(c *Client)
	Unregister(c *Client)
	Subscribe(c *Client, topic Topic)
	Unsubscribe(c *Client, topic Topic)
}

type InboundHandler interface {
	HandleMessage(c *Client, env Envelope) error
}

type SubscriptionAuthorizer interface {
	CanSubscribe(ctx context.Context, userID string, topic Topic) error
}

type PresenceHook interface {
	HandleClientConnected(userID uuid.UUID, username string)
	HandleClientDisconnected(userID uuid.UUID, username string)
}
