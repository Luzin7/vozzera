package chat

import (
	"context"
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
	Rooms    map[uuid.UUID]bool
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

		switch in.Type {
		case EventJoin:
			c.hub.join <- roomJoin{client: c, roomID: in.RoomID}
			continue
		case EventMessage:
		default:
			continue
		}

		msgDB, err := c.hub.queries.CreateMessage(context.Background(), CreateMessageParams{
			RoomID:  in.RoomID,
			UserID:  c.UserID,
			Content: in.Content,
		})
		if err != nil {
			log.Printf("Erro ao salvar mensagem no banco de dados: %v", err)
			continue
		}

		out := OutboundEvent{
			Type:      EventMessage,
			ID:        msgDB.ID,
			RoomID:    msgDB.RoomID,
			UserID:    c.UserID,
			Username:  c.Username,
			Content:   msgDB.Content,
			CreatedAt: msgDB.CreatedAt.Time,
		}

		c.hub.broadcast <- out
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
