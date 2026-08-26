package realtime

import (
	"context"
	"encoding/json"
	"log"

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
	broadcast   chan broadcastPayload
	register    chan *Client
	unregister  chan *Client
	subscribe   chan subscription
	unsubscribe chan subscription
	revoke      chan uuid.UUID
}

func NewHub() *Hub {
	return &Hub{
		broadcast:   make(chan broadcastPayload),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		clients:     make(map[*Client]bool),
		topics:      make(map[Topic]map[*Client]bool),
		subscribe:   make(chan subscription),
		unsubscribe: make(chan subscription),
		revoke:      make(chan uuid.UUID),
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
	}
}

func (h *Hub) Subscribe(c *Client, topic Topic) {
	h.subscribe <- subscription{client: c, topic: topic}
}

func (h *Hub) Unsubscribe(c *Client, topic Topic) {
	h.unsubscribe <- subscription{client: c, topic: topic}
}

func (h *Hub) Register(c *Client) {
	h.register <- c
}

func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

func (h *Hub) Revoke(ctx context.Context, sessionID uuid.UUID) error {
	h.revoke <- sessionID
	return nil
}

func (h *Hub) RemoveClient(c *Client) {
	if _, ok := h.clients[c]; ok {
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
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Hub encerrado")
			return

		case client := <-h.register:
			h.clients[client] = true

		case client := <-h.unregister:
			h.RemoveClient(client)

		case sessionID := <-h.revoke:
			for client := range h.clients {
				if client.SessionID == sessionID {
					h.RemoveClient(client)
				}
			}

		case sub := <-h.subscribe:
			if h.topics[sub.topic] == nil {
				h.topics[sub.topic] = make(map[*Client]bool)
			}
			h.topics[sub.topic][sub.client] = true
			sub.client.Topics[sub.topic] = true

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
					h.RemoveClient(client)
				}
			}
		}
	}
}
