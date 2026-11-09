package chat

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/Luzin7/vozzera-backend/internal/transport/ws"
	"github.com/google/uuid"
)

const (
	roomAuthorizationTimeout = 3 * time.Second
	snapshotTimeout          = 3 * time.Second
)

type RoomSnapshotProvider interface {
	RoomSnapshot(ctx context.Context, roomID uuid.UUID) ([]realtime.Envelope, error)
}

type ChatHandlerDeps struct {
	Sender        *SendMessageService
	Registerer    realtime.Registerer
	Publisher     realtime.Publisher
	Authorizer    realtime.SubscriptionAuthorizer
	RoomSnapshots []RoomSnapshotProvider
}

func RegisterChatHandlers(r *ws.Router, deps ChatHandlerDeps) {
	r.Handle(CmdPing, handlePing)

	r.Handle(CmdSubscribe, func(ctx context.Context, c *realtime.Client, data json.RawMessage) error {
		var cmd struct {
			RoomID uuid.UUID `json:"room_id"`
		}
		if err := json.Unmarshal(data, &cmd); err != nil || cmd.RoomID == uuid.Nil {
			return nil
		}
		topic := realtime.RoomTopic(cmd.RoomID)
		allowed, err := deps.roomAccess(ctx, c.UserID, topic)
		if err != nil {
			return err
		}
		if !allowed {
			return nil
		}
		deps.Registerer.Subscribe(c, topic)

		deps.publishRoomSnapshots(cmd.RoomID, topic)
		return nil
	})

	r.Handle(CmdUnsubscribe, func(ctx context.Context, c *realtime.Client, data json.RawMessage) error {
		var cmd struct {
			RoomID uuid.UUID `json:"room_id"`
		}
		if err := json.Unmarshal(data, &cmd); err != nil || cmd.RoomID == uuid.Nil {
			return nil
		}
		deps.Registerer.Unsubscribe(c, realtime.RoomTopic(cmd.RoomID))
		return nil
	})

	r.Handle(CmdTypingStart, func(ctx context.Context, c *realtime.Client, data json.RawMessage) error {
		return handleTyping(ctx, c, CmdTypingStart, data, deps)
	})
	r.Handle(CmdTypingStop, func(ctx context.Context, c *realtime.Client, data json.RawMessage) error {
		return handleTyping(ctx, c, CmdTypingStop, data, deps)
	})

	r.Handle(CmdMessage, func(ctx context.Context, c *realtime.Client, data json.RawMessage) error {
		var cmd struct {
			RoomID  uuid.UUID `json:"room_id"`
			Content string    `json:"content"`
		}
		if err := json.Unmarshal(data, &cmd); err != nil {
			return nil
		}
		if cmd.RoomID == uuid.Nil || cmd.Content == "" {
			return nil
		}
		allowed, err := deps.roomAccess(ctx, c.UserID, realtime.RoomTopic(cmd.RoomID))
		if err != nil {
			return err
		}
		if !allowed {
			return nil
		}
		ctxMsg, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err = deps.Sender.Execute(ctxMsg, SendMessageInput{
			RoomID:   cmd.RoomID,
			UserID:   c.UserID,
			Username: c.Username,
			Content:  cmd.Content,
		})
		return err
	})
}

func handleTyping(ctx context.Context, c *realtime.Client, typingType string, data json.RawMessage, deps ChatHandlerDeps) error {
	var cmd struct {
		RoomID uuid.UUID `json:"room_id"`
	}
	if err := json.Unmarshal(data, &cmd); err != nil || cmd.RoomID == uuid.Nil {
		return nil
	}
	topic := realtime.RoomTopic(cmd.RoomID)
	allowed, err := deps.roomAccess(ctx, c.UserID, topic)
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}
	typingData, _ := json.Marshal(map[string]interface{}{
		"user_id":  c.UserID,
		"username": c.Username,
	})
	env := realtime.Envelope{
		V:     1,
		Type:  typingType,
		Topic: topic,
		TS:    time.Now(),
		Data:  typingData,
	}

	return deps.Publisher.Publish(ctx, topic, env)
}

func (deps ChatHandlerDeps) roomAccess(ctx context.Context, userID uuid.UUID, topic realtime.Topic) (bool, error) {
	if deps.Authorizer == nil {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(ctx, roomAuthorizationTimeout)
	defer cancel()

	err := deps.Authorizer.CanSubscribe(ctx, userID.String(), topic)
	if err == nil {
		return true, nil
	}
	if isAccessDenied(err) {
		return false, nil
	}
	return false, ErrAuthorizeRoom(err)
}

func isAccessDenied(err error) bool {
	var httpErr *httpx.Error
	return errors.As(err, &httpErr) && httpErr.Status < http.StatusInternalServerError
}

func handlePing(_ context.Context, c *realtime.Client, _ json.RawMessage) error {
	c.Send(pongFrame)
	return nil
}

func (deps ChatHandlerDeps) publishRoomSnapshots(roomID uuid.UUID, topic realtime.Topic) {
	for _, provider := range deps.RoomSnapshots {
		ctx, cancel := context.WithTimeout(context.Background(), snapshotTimeout)
		deps.publishRoomSnapshot(ctx, provider, roomID, topic)
		cancel()
	}
}

func (deps ChatHandlerDeps) publishRoomSnapshot(ctx context.Context, provider RoomSnapshotProvider, roomID uuid.UUID, topic realtime.Topic) {
	envelopes, err := provider.RoomSnapshot(ctx, roomID)
	if err != nil {
		log.Printf("chat.room_snapshot level=error room_id=%s err=%q", roomID, err)
		return
	}

	for _, env := range envelopes {
		if err := deps.Publisher.Publish(ctx, topic, env); err != nil {
			log.Printf("chat.room_snapshot level=error room_id=%s err=%q", roomID, err)
		}
	}
}
