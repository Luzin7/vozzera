package chat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type ChatRouter struct {
	sender     *SendMessageService
	registerer realtime.Registerer
	publisher  realtime.Publisher
	authorizer realtime.SubscriptionAuthorizer
	presence   *realtime.VoiceRoomPresence
}

func NewChatRouter(sender *SendMessageService, hub *realtime.Hub, authorizer realtime.SubscriptionAuthorizer, presence *realtime.VoiceRoomPresence) *ChatRouter {
	return &ChatRouter{sender: sender, registerer: hub, publisher: hub, authorizer: authorizer, presence: presence}
}

func (h *ChatRouter) HandleMessage(c *realtime.Client, env realtime.Envelope) error {
	switch env.Type {
	case CmdSubscribe:
		var cmd struct {
			RoomID uuid.UUID `json:"room_id"`
		}
		if err := json.Unmarshal(env.Data, &cmd); err != nil || cmd.RoomID == uuid.Nil {
			return nil
		}
		topic := realtime.Topic("room:" + cmd.RoomID.String())
		if h.authorizer != nil {
			if err := h.authorizer.CanSubscribe(context.Background(), c.UserID.String(), topic); err != nil {
				return nil
			}
		}
		h.registerer.Subscribe(c, topic)

		if h.presence != nil {
			data := h.presence.SnapshotJSON(cmd.RoomID)
			if data != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				h.publisher.Publish(ctx, topic, realtime.Envelope{
					V:     1,
					Type:  EventVoicePresenceSnapshot,
					Topic: topic,
					TS:    time.Now(),
					Data:  data,
				})
			}
		}

	case CmdUnsubscribe:
		var cmd struct {
			RoomID uuid.UUID `json:"room_id"`
		}
		if err := json.Unmarshal(env.Data, &cmd); err != nil || cmd.RoomID == uuid.Nil {
			return nil
		}
		h.registerer.Unsubscribe(c, realtime.Topic("room:"+cmd.RoomID.String()))

	case CmdTypingStart, CmdTypingStop:
		var cmd struct {
			RoomID uuid.UUID `json:"room_id"`
		}
		if err := json.Unmarshal(env.Data, &cmd); err != nil || cmd.RoomID == uuid.Nil {
			return nil
		}
		topic := realtime.Topic("room:" + cmd.RoomID.String())
		env.Topic = topic
		env.Data, _ = json.Marshal(map[string]interface{}{
			"user_id":  c.UserID,
			"username": c.Username,
		})
		return h.publisher.Publish(context.Background(), topic, env)

	case CmdMessage:
		var cmd struct {
			RoomID  uuid.UUID `json:"room_id"`
			Content string    `json:"content"`
		}
		if err := json.Unmarshal(env.Data, &cmd); err != nil {
			return nil
		}
		if cmd.RoomID == uuid.Nil || cmd.Content == "" {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := h.sender.Execute(ctx, SendMessageInput{
			RoomID:   cmd.RoomID,
			UserID:   c.UserID,
			Username: c.Username,
			Content:  cmd.Content,
		})
		return err
	}

	return nil
}
