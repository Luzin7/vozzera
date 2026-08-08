package chat

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	UserID   uuid.UUID
	Username string
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Erro de leitura: %v", err)
			}
			break
		}
		event := OutboundEvent{
			UserID:   c.UserID,
			Content:  string(message),
			Type:     EventMessage,
			Username: c.Username,
		}
		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("Erro ao serializar evento: %v", err)
			continue
		}
		c.hub.broadcast <- data
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}
