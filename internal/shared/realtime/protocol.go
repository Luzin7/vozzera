package realtime

import (
	"context"
	"encoding/json"
	"time"
)

type Topic string

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
