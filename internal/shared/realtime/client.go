package realtime

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait                = 10 * time.Second
	pongWait                 = 60 * time.Second
	pingPeriod               = (pongWait * 9) / 10
	maxMessageSize           = 4096
	sessionRevalidateTimeout = 5 * time.Second
)

type Client struct {
	registerer          Registerer
	conn                *websocket.Conn
	send                chan []byte
	ConnID              uuid.UUID
	UserID              uuid.UUID
	Username            string
	SessionID           uuid.UUID
	Topics              map[Topic]bool
	Handler             InboundHandler
	sessionRevalidation SessionRevalidation
	context             context.Context
	ctxCancel           context.CancelFunc
	closeCode           int
}

type ClientOption func(*Client)

func WithSessionRevalidation(revalidation SessionRevalidation) ClientOption {
	return func(c *Client) {
		c.sessionRevalidation = revalidation
	}
}

func NewClient(registerer Registerer, conn *websocket.Conn, userID uuid.UUID, username string, sessionID uuid.UUID, handler InboundHandler, opts ...ClientOption) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{
		registerer: registerer,
		conn:       conn,
		send:       make(chan []byte, 256),
		ConnID:     uuid.New(),
		UserID:     userID,
		Username:   username,
		SessionID:  sessionID,
		Topics:     make(map[Topic]bool),
		Handler:    handler,
		context:    ctx,
		ctxCancel:  cancel,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func (c *Client) Context() context.Context {
	return c.context
}

func (c *Client) Send(data []byte) {
	c.send <- data
}

func (c *Client) ReadPump() {
	defer func() {
		c.ctxCancel()
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
	pingTicker := time.NewTicker(pingPeriod)
	defer func() {
		pingTicker.Stop()
		c.conn.Close()
	}()

	revalidateTicker, revalidate := c.sessionRevalidateTicker()
	defer func() {
		if revalidateTicker != nil {
			revalidateTicker.Stop()
		}
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(c.closeCode, ""))
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
		case <-pingTicker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-revalidate:
			if c.sessionValid() {
				continue
			}
			c.revokeSession()
		}
	}
}

func (c *Client) sessionRevalidateTicker() (*time.Ticker, <-chan time.Time) {
	if c.sessionRevalidation.Revalidator == nil || c.sessionRevalidation.Revoker == nil || c.sessionRevalidation.Interval <= 0 {
		return nil, nil
	}

	ticker := time.NewTicker(c.sessionRevalidation.Interval)
	return ticker, ticker.C
}

func (c *Client) sessionValid() bool {
	ctx, cancel := context.WithTimeout(c.context, sessionRevalidateTimeout)
	defer cancel()

	err := c.sessionRevalidation.Revalidator.RevalidateSession(ctx, c.SessionID)
	if err == nil {
		return true
	}
	if IsSessionInvalid(err) {
		return false
	}

	log.Printf("falha ao revalidar sessão do websocket %s: %v", c.ConnID, err)
	return true
}

func (c *Client) revokeSession() {
	ctx, cancel := context.WithTimeout(c.context, sessionRevalidateTimeout)
	defer cancel()

	if err := c.sessionRevalidation.Revoker.Revoke(ctx, c.SessionID); err != nil {
		log.Printf("falha ao revogar sessão do websocket %s: %v", c.ConnID, err)
	}
}
