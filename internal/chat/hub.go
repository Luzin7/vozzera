package chat

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
)

type roomJoin struct {
	client *Client
	roomID uuid.UUID
}

type Hub struct {
	clients    map[*Client]bool
	rooms      map[uuid.UUID]map[*Client]bool
	broadcast  chan OutboundEvent
	join       chan roomJoin
	register   chan *Client
	unregister chan *Client
	queries    *Queries
}

func NewHub(queries *Queries) *Hub {
	return &Hub{
		broadcast:  make(chan OutboundEvent),
		join:       make(chan roomJoin),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[uuid.UUID]map[*Client]bool),
		queries:    queries,
	}
}

func (h *Hub) RemoveClientFromRoom(c *Client) {
	_, ok := h.clients[c]

	if ok {
		delete(h.clients, c)
		close(c.send)

		for roomID := range c.Rooms {
			room, ok := h.rooms[roomID]

			if ok {
				delete(room, c)
				if len(room) == 0 {
					delete(h.rooms, roomID)
				}
			}
		}
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			h.RemoveClientFromRoom(client)
		case j := <-h.join:
			if h.rooms[j.roomID] == nil {
				h.rooms[j.roomID] = make(map[*Client]bool)
			}
			h.rooms[j.roomID][j.client] = true

			j.client.Rooms[j.roomID] = true
		case message := <-h.broadcast:
			payload, err := json.Marshal(message)
			if err != nil {
				log.Printf("Erro ao serializar evento: %v", err)
				continue
			}
			for client := range h.rooms[message.RoomID] {
				select {
				case client.send <- payload:
				default:
					h.RemoveClientFromRoom(client)
				}
			}
		}
	}
}
