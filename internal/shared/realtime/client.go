package realtime

import (
	"encoding/json"
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
	registerer Registerer
	conn       *websocket.Conn
	send       chan []byte
	UserID     uuid.UUID
	Username   string
	SessionID  uuid.UUID
	Topics     map[Topic]bool
	Handler    InboundHandler
}

func NewClient(registerer Registerer, conn *websocket.Conn, userID uuid.UUID, username string, sessionID uuid.UUID, handler InboundHandler) *Client {
	return &Client{
		registerer: registerer,
		conn:       conn,
		send:       make(chan []byte, 256),
		UserID:     userID,
		Username:   username,
		SessionID:  sessionID,
		Topics:     make(map[Topic]bool),
		Handler:    handler,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.registerer.Unregister(c)
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
				log.Printf("erro de leitura de websocket: %v", err)
			}
			break
		}

		var in Envelope
		if err := json.Unmarshal(message, &in); err != nil {
			log.Printf("envelope mal formado recebido da rede: %v", err)
			continue
		}

		if c.Handler != nil {
			if err := c.Handler.HandleMessage(c, in); err != nil {
				log.Printf("erro ao lidar com a mensagem recebida: %v", err)
			}
		}
	}
}

func (c *Client) WritePump() {
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
