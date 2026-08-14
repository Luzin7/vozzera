package chat

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
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

	c.conn.SetReadDeadline(time.Now().Add(pongWait))

	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.conn.SetReadLimit(maxMessageSize)
		return nil
	})

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
			content := strings.TrimSpace(in.Content)
			if content == "" || len(content) > 4000 {
				c.send <- jsonMessage(OutboundEvent{
					Type:  EventError,
					Error: "Mensagem deve ter entre 1 e 4000 caracteres",
				})
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			msgDB, err := c.hub.queries.CreateMessage(ctx, CreateMessageParams{
				RoomID:  in.RoomID,
				UserID:  c.UserID,
				Content: pgtype.Text{String: in.Content, Valid: true},
			})
			cancel()

			if err != nil {
				log.Printf("Erro ao salvar mensagem no banco de dados (timeout ou falha): %v", err)
				continue
			}

			out := OutboundEvent{
				Type:      EventMessage,
				Action:    MessageCreated,
				ID:        msgDB.ID,
				RoomID:    msgDB.RoomID,
				UserID:    c.UserID,
				Username:  c.Username,
				Content:   msgDB.Content.String,
				CreatedAt: msgDB.CreatedAt.Time,
			}

			c.hub.broadcast <- out
			continue

		default:
			continue
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
