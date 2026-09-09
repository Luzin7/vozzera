package realtime

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

type subscription struct {
	client *Client
	topic  Topic
}

type broadcastPayload struct {
	topic Topic
	data  []byte
}

type Hub struct {
	clients     map[*Client]bool
	topics      map[Topic]map[*Client]bool
	presence    PresenceHook
	broadcast   chan broadcastPayload
	register    chan *Client
	unregister  chan *Client
	subscribe   chan subscription
	unsubscribe chan subscription
	revoke      chan uuid.UUID
	sync        chan struct{}
	stop        chan struct{}
	done        chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		topics:      make(map[Topic]map[*Client]bool),
		broadcast:   make(chan broadcastPayload, 256),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		subscribe:   make(chan subscription),
		unsubscribe: make(chan subscription),
		revoke:      make(chan uuid.UUID),
		sync:        make(chan struct{}),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
}

func (h *Hub) SetPresence(p PresenceHook) {
	h.presence = p
}

func (h *Hub) Shutdown(ctx context.Context) error {
	select {
	case <-h.stop:
	default:
		close(h.stop)
	}

	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hub) Publish(ctx context.Context, topic Topic, env Envelope) error {
	bytesData, err := json.Marshal(env)
	if err != nil {
		return err
	}

	payload := broadcastPayload{
		topic: topic,
		data:  bytesData,
	}

	select {
	case h.broadcast <- payload:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-h.done:
		return errors.New("hub encerrado")
	}
}

func (h *Hub) Subscribe(c *Client, topic Topic) {
	select {
	case h.subscribe <- subscription{client: c, topic: topic}:
	case <-h.done:
	}
}

func (h *Hub) Unsubscribe(c *Client, topic Topic) {
	select {
	case h.unsubscribe <- subscription{client: c, topic: topic}:
	case <-h.done:
	}
}

func (h *Hub) Register(c *Client) {
	select {
	case h.register <- c:
	case <-h.done:
	}
}

func (h *Hub) Unregister(c *Client) {
	select {
	case h.unregister <- c:
	case <-h.done:
	}
}

func (h *Hub) Revoke(ctx context.Context, sessionID uuid.UUID) error {
	select {
	case h.revoke <- sessionID:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-h.done:
		return errors.New("hub encerrado")
	}
}

// Sync bloqueia até a goroutine do Run() processar todos os comandos pendentes.
func (h *Hub) Sync(ctx context.Context) error {
	select {
	case h.sync <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-h.done:
		return errors.New("hub encerrado")
	}

	select {
	case <-h.sync:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-h.done:
		return errors.New("hub encerrado")
	}
}

func (h *Hub) removeClient(c *Client) {
	if _, ok := h.clients[c]; !ok {
		return
	}

	delete(h.clients, c)
	close(c.send)

	for topic := range c.Topics {
		if clients, ok := h.topics[topic]; ok {
			delete(clients, c)
			if len(clients) == 0 {
				delete(h.topics, topic)
			}
		}
	}
	c.Topics = make(map[Topic]bool)

	if h.presence != nil {
		h.presence.HandleClientDisconnected(c.UserID, c.Username)
	}
}

func (h *Hub) drain() {
	for client := range h.clients {
		h.removeClient(client)
	}
}

func (h *Hub) Run() {
	defer close(h.done)

	for {
		select {
		case <-h.stop:
			h.drain()
			return

		case client := <-h.register:
			h.clients[client] = true
			if h.presence != nil {
				h.presence.HandleClientConnected(client.UserID, client.Username)
			}

		case client := <-h.unregister:
			h.removeClient(client)

		case sessionID := <-h.revoke:
			for client := range h.clients {
				if client.SessionID == sessionID {
					h.removeClient(client)
				}
			}

		case sub := <-h.subscribe:
			if h.topics[sub.topic] == nil {
				h.topics[sub.topic] = make(map[*Client]bool)
			}
			h.topics[sub.topic][sub.client] = true
			sub.client.Topics[sub.topic] = true
			if h.presence != nil {
				data := h.presence.HandleTopicSubscribed(sub.topic)
				if data != nil {
					select {
					case sub.client.send <- data:
					default:
						h.removeClient(sub.client)
					}
				}
			}

		case unsub := <-h.unsubscribe:
			if clients, ok := h.topics[unsub.topic]; ok {
				delete(clients, unsub.client)
				if len(clients) == 0 {
					delete(h.topics, unsub.topic)
				}
			}
			delete(unsub.client.Topics, unsub.topic)

		case payload := <-h.broadcast:
			clients := h.topics[payload.topic]
			for client := range clients {
				select {
				case client.send <- payload.data:
				default:
					h.removeClient(client)
				}
			}

		case <-h.sync:
			h.sync <- struct{}{}
		}
	}
}
