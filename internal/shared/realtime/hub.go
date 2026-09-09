package realtime

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

type onlineUser struct {
	Username    string
	Connections int32
}

type subscription struct {
	client *Client
	topic  Topic
}

type broadcastPayload struct {
	topic Topic
	data  []byte
}

type UserPresence struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
}

type Hub struct {
	clients     map[*Client]bool
	onlineUsers map[uuid.UUID]*onlineUser
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
		onlineUsers: make(map[uuid.UUID]*onlineUser),
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

func (h *Hub) OnlineUsers() []UserPresence {
	users := make([]UserPresence, 0, len(h.onlineUsers))
	for id, u := range h.onlineUsers {
		users = append(users, UserPresence{
			UserID:   id,
			Username: u.Username,
		})
	}
	return users
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

func (h *Hub) broadcastPresence(env Envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	for client := range h.topics[GlobalPresenceTopic] {
		select {
		case client.send <- data:
		default:
			h.RemoveClient(client)
		}
	}
}

func (h *Hub) RemoveClient(c *Client) {
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

	u, ok := h.onlineUsers[c.UserID]
	if !ok {
		return
	}
	u.Connections--
	if u.Connections == 0 {
		delete(h.onlineUsers, c.UserID)

		data, _ := json.Marshal(map[string]interface{}{
			"user_id":  c.UserID,
			"username": u.Username,
		})
		h.broadcastPresence(Envelope{
			V:     1,
			Type:  UserOffline,
			Topic: GlobalPresenceTopic,
			TS:    time.Now(),
			Data:  data,
		})
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

			u, exists := h.onlineUsers[client.UserID]
			if !exists {
				u = &onlineUser{Username: client.Username}
				h.onlineUsers[client.UserID] = u
			} else {
				u.Username = client.Username
			}
			u.Connections++

			if u.Connections == 1 {
				data, _ := json.Marshal(map[string]interface{}{
					"user_id":  client.UserID,
					"username": u.Username,
				})
				h.broadcastPresence(Envelope{
					V:     1,
					Type:  UserOnline,
					Topic: GlobalPresenceTopic,
					TS:    time.Now(),
					Data:  data,
				})
			}

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
