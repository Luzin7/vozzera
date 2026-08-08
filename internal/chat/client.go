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

		var in InboundEvent
		if err := json.Unmarshal(message, &in); err != nil {
			log.Printf("Erro ao desserializar evento: %v", err)
			continue
		}

		out := OutboundEvent{
			Type:     EventMessage,
			UserID:   c.UserID,
			Username: c.Username,
			Content:  in.Content,
		}
		data, err := json.Marshal(out)
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
