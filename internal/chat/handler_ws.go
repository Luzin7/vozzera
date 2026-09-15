package chat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/Luzin7/vozzera-backend/internal/transport/ws"
	"github.com/google/uuid"
)

type VoicePresenceProvider interface {
	SnapshotJSON(roomID uuid.UUID) []byte
}

type ChatHandlerDeps struct {
	Sender     *SendMessageService
	Registerer realtime.Registerer
	Publisher  realtime.Publisher
	Authorizer realtime.SubscriptionAuthorizer
	Presence   VoicePresenceProvider
}

func RegisterChatHandlers(r *ws.Router, deps ChatHandlerDeps) {
	r.Handle(CmdSubscribe, func(ctx context.Context, c *realtime.Client, data json.RawMessage) error {
		var cmd struct {
			RoomID uuid.UUID `json:"room_id"`
		}
		if err := json.Unmarshal(data, &cmd); err != nil || cmd.RoomID == uuid.Nil {
			return nil
		}
		topic := realtime.Topic("room:" + cmd.RoomID.String())
		if deps.Authorizer != nil {
			if err := deps.Authorizer.CanSubscribe(ctx, c.UserID.String(), topic); err != nil {
				return nil
			}
		}
		deps.Registerer.Subscribe(c, topic)

		if deps.Presence != nil {
			pdata := deps.Presence.SnapshotJSON(cmd.RoomID)
			if pdata != nil {
				ctxPub, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				deps.Publisher.Publish(ctxPub, topic, realtime.Envelope{
					V:     1,
					Type:  realtime.PresenceSnapshot,
					Topic: topic,
					TS:    time.Now(),
					Data:  pdata,
				})
			}
		}
		return nil
	})

	r.Handle(CmdUnsubscribe, func(ctx context.Context, c *realtime.Client, data json.RawMessage) error {
		var cmd struct {
			RoomID uuid.UUID `json:"room_id"`
		}
		if err := json.Unmarshal(data, &cmd); err != nil || cmd.RoomID == uuid.Nil {
			return nil
		}
		deps.Registerer.Unsubscribe(c, realtime.Topic("room:"+cmd.RoomID.String()))
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
		ctxMsg, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := deps.Sender.Execute(ctxMsg, SendMessageInput{
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
	topic := realtime.Topic("room:" + cmd.RoomID.String())
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
