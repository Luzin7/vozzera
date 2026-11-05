package voice

import (
	"context"
	"log"
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
	state       VoiceRoomState
	publisher   realtime.Publisher
	events      *awayEvents
}

func NewWebhookHandler(apiKey, apiSecret string, state VoiceRoomState, publisher realtime.Publisher) *WebhookHandler {
	return &WebhookHandler{
		keyProvider: auth.NewSimpleKeyProvider(apiKey, apiSecret),
		state:       state,
		publisher:   publisher,
		events:      newAwayEvents(state, publisher),
	}
}

func participantFromEvent(p *livekit.ParticipantInfo) Participant {
	return Participant{
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

	logWebhookDropped(log.Default(), event)

	roomID, err := uuid.Parse(event.Room.GetName())
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	if err := h.dispatch(r.Context(), roomID, event); err != nil {
		log.Printf("voice.webhook level=error event_id=%s event=%s room_id=%s status=503 err=%q", event.GetId(), event.GetEvent(), roomID, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func logMissingRoomCreationTime(event *livekit.WebhookEvent, roomID uuid.UUID) {
	if !roomCreationTime(event.GetRoom()).IsZero() {
		return
	}
	log.Printf("voice.session level=warn cause=missing_room_creation_time event_id=%s event=%s room_id=%s", event.GetId(), event.GetEvent(), roomID)
}

func (h *WebhookHandler) dispatch(ctx context.Context, roomID uuid.UUID, event *livekit.WebhookEvent) error {
	switch event.Event {
	case webhook.EventParticipantJoined:
		logMissingRoomCreationTime(event, roomID)
		return h.handleJoined(ctx, roomID, event)
	case webhook.EventParticipantLeft:
		logMissingRoomCreationTime(event, roomID)
		return h.handleLeft(ctx, roomID, event)
	case webhook.EventRoomFinished:
		return h.handleFinished(ctx, roomID)
	default:
		return nil
	}
}

func (h *WebhookHandler) handleJoined(ctx context.Context, roomID uuid.UUID, event *livekit.WebhookEvent) error {
	p := participantFromEvent(event.Participant)

	result, err := h.state.Join(ctx, JoinInput{RoomID: roomID, Participant: p, At: unixMillis(event.GetCreatedAt()), RoomCreatedAt: roomCreationTime(event.GetRoom())})
	if err != nil {
		return err
	}

	h.events.publishPresence(ctx, roomID, realtime.PresenceJoined, result.Participants)
	h.events.publishSession(ctx, roomID, result.SessionStartedAt)
	if result.ClearedAway == nil {
		return nil
	}

	logAwayCleared(*result.ClearedAway, "joined")
	_ = h.events.publishCurrentAwayList(ctx, roomID)
	h.events.publishEnded(ctx, result.ClearedAway.RoomID, result.ClearedAway.UserID, AwayEndedReturned)
	return nil
}

func (h *WebhookHandler) handleLeft(ctx context.Context, roomID uuid.UUID, event *livekit.WebhookEvent) error {
	logSessionEnded(log.Default(), roomID.String(), event)

	p := participantFromEvent(event.Participant)

	result, err := h.state.Leave(ctx, LeaveInput{RoomID: roomID, SID: p.SID, At: unixMillis(event.GetCreatedAt()), RoomCreatedAt: roomCreationTime(event.GetRoom())})
	if err != nil {
		return err
	}

	h.events.publishPresence(ctx, roomID, realtime.PresenceLeft, result.Participants)
	h.events.publishSession(ctx, roomID, result.SessionStartedAt)
	if result.Emptied {
		h.events.publishAwayList(ctx, roomID, nil)
	}
	h.announceRemovedAwayEnded(ctx, roomID, result.RemovedAway, "room_empty")
	return nil
}

func (h *WebhookHandler) handleFinished(ctx context.Context, roomID uuid.UUID) error {
	removed, err := h.state.Finish(ctx, roomID)
	if err != nil {
		return err
	}

	h.events.publishPresence(ctx, roomID, realtime.PresenceSnapshot, nil)
	h.events.publishSession(ctx, roomID, nil)
	h.events.publishAwayList(ctx, roomID, nil)
	h.announceRemovedAwayEnded(ctx, roomID, removed, "room_finished")
	return nil
}

func (h *WebhookHandler) announceRemovedAwayEnded(ctx context.Context, roomID uuid.UUID, removed []AwayEntry, cause string) {
	for _, entry := range removed {
		logAwayCleared(entry, cause)
		h.events.publishEnded(ctx, roomID, entry.UserID, AwayEndedRoomEmpty)
	}
}

func unixMillis(seconds int64) time.Time {
	return time.UnixMilli(seconds * 1000)
}
