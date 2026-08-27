package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/livekit/protocol/webhook"
)

const maxWebhookBodyBytes = 1 << 16 // 64 KB

type WebhookHandler struct {
	keyProvider auth.KeyProvider
	presence    *realtime.PresenceStore
	publisher   realtime.Publisher
}

func NewWebhookHandler(apiKey, apiSecret string, presence *realtime.PresenceStore, publisher realtime.Publisher) *WebhookHandler {
	return &WebhookHandler{
		keyProvider: auth.NewSimpleKeyProvider(apiKey, apiSecret),
		presence:    presence,
		publisher:   publisher,
	}
}

func participantFromEvent(p *livekit.ParticipantInfo) realtime.Participant {
	return realtime.Participant{
		SID:      p.GetSid(),
		UserID:   p.GetIdentity(),
		Username: p.GetName(),
	}
}

func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)

	event, err := webhook.ReceiveWebhookEvent(r, h.keyProvider)
	if err != nil {
		http.Error(w, "failed to receive webhook event: "+err.Error(), http.StatusBadRequest)
		return
	}

	roomID, err := uuid.Parse(event.Room.GetName())
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch event.Event {
	case webhook.EventParticipantJoined:
		p := participantFromEvent(event.Participant)
		participants := h.presence.Join(roomID, p)

		data, _ := json.Marshal(participants)
		topic := realtime.Topic("room:" + roomID.String())

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		h.publisher.Publish(ctx, topic, realtime.Envelope{
			V:     1,
			Type:  realtime.PresenceJoined,
			Topic: topic,
			TS:    time.Now(),
			Data:  data,
		})

	case webhook.EventParticipantLeft:
		p := participantFromEvent(event.Participant)
		participants, left := h.presence.Leave(roomID, p.SID)
		if !left {
			w.WriteHeader(http.StatusOK)
			return
		}

		data, _ := json.Marshal(participants)
		topic := realtime.Topic("room:" + roomID.String())

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		h.publisher.Publish(ctx, topic, realtime.Envelope{
			V:     1,
			Type:  realtime.PresenceLeft,
			Topic: topic,
			TS:    time.Now(),
			Data:  data,
		})

	case webhook.EventRoomFinished:
		h.presence.Clear(roomID)

		topic := realtime.Topic("room:" + roomID.String())

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		h.publisher.Publish(ctx, topic, realtime.Envelope{
			V:     1,
			Type:  realtime.PresenceSnapshot,
			Topic: topic,
			TS:    time.Now(),
			Data:  []byte("[]"),
		})

	default:
		w.WriteHeader(http.StatusOK)
	}
}
