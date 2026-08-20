package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

type Client struct {
	hub       *Hub
	sender    *SendMessageService
	conn      *websocket.Conn
	send      chan []byte
	UserID    uuid.UUID
	Username  string
	SessionID uuid.UUID
	Rooms     map[uuid.UUID]bool
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
				log.Printf("erro de leitura: %v", err)
			}
			break
		}

		var in InboundEvent
		if err := json.Unmarshal(message, &in); err != nil {
			log.Printf("erro ao desserializar evento: %v", err)
			continue
		}

		switch in.Type {
		case EventJoin:
			c.hub.join <- roomJoin{client: c, roomID: in.RoomID}
			continue

		case EventTyping:
			if in.Action != EventTypingStart && in.Action != EventTypingStop {
				log.Printf("ação de digitação inválida: %s", in.Action)
				continue
			}
			if _, ok := c.Rooms[in.RoomID]; !ok {
				log.Printf("usuário %s tentou enviar evento de digitação para sala %s sem estar presente", c.UserID, in.RoomID)
				continue
			}
			if in.Action == EventTypingStart {
				c.hub.broadcast <- OutboundEvent{
					Type:     EventTyping,
					RoomID:   in.RoomID,
					UserID:   c.UserID,
					Username: c.Username,
					Action:   EventTypingStart,
				}
			}
			if in.Action == EventTypingStop {
				c.hub.broadcast <- OutboundEvent{
					Type:     EventTyping,
					RoomID:   in.RoomID,
					UserID:   c.UserID,
					Username: c.Username,
					Action:   EventTypingStop,
				}
			}

			continue

		case EventMessage:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := c.sender.Execute(ctx, SendMessageInput{
				RoomID:   in.RoomID,
				UserID:   c.UserID,
				Username: c.Username,
				Content:  in.Content,
			})
			cancel()

			if err != nil {
				if errors.Is(err, ErrInvalidContent) {
					c.send <- jsonMessage(OutboundEvent{Type: EventError, Error: err.Error()})
					continue
				}
				log.Printf("erro ao salvar mensagem: %v", err)
			}
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
