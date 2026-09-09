package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
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

	presenceMu sync.RWMutex

	broadcast   chan broadcastPayload
	register    chan *Client
	unregister  chan *Client
	subscribe   chan subscription
	unsubscribe chan subscription
	revoke      chan uuid.UUID

	stop chan struct{}
	done chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		onlineUsers: make(map[uuid.UUID]*onlineUser),
		topics:      make(map[Topic]map[*Client]bool),
		broadcast:   make(chan broadcastPayload),
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		subscribe:   make(chan subscription),
		unsubscribe: make(chan subscription),
		revoke:      make(chan uuid.UUID),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
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

func (h *Hub) OnlineUsers() []UserPresence {
	h.presenceMu.RLock()
	defer h.presenceMu.RUnlock()

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

func (h *Hub) broadcastPresence(env Envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	for client := range h.topics[GlobalPresenceTopic] {
		select {
		case client.send <- data:
		default:
			h.removeClient(client, true)
		}
	}
}

func (h *Hub) removeClient(c *Client, notifyPresence bool) {
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

	h.presenceMu.Lock()
	u, ok := h.onlineUsers[c.UserID]
	if ok {
		u.Connections--
		if u.Connections <= 0 {
			delete(h.onlineUsers, c.UserID)
		}
	}
	h.presenceMu.Unlock()

	if ok && u.Connections <= 0 && notifyPresence {
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

func (h *Hub) drain() {
	for client := range h.clients {
		h.removeClient(client, false)
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

			h.presenceMu.Lock()
			u, exists := h.onlineUsers[client.UserID]
			if !exists {
				u = &onlineUser{Username: client.Username}
				h.onlineUsers[client.UserID] = u
			} else {
				u.Username = client.Username
			}
			u.Connections++
			isFirstConn := u.Connections == 1
			h.presenceMu.Unlock()

			if isFirstConn {
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
			h.removeClient(client, true)

		case sessionID := <-h.revoke:
			for client := range h.clients {
				if client.SessionID == sessionID {
					h.removeClient(client, true)
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
					h.removeClient(client, true)
				}
			}
		}
	}
}
