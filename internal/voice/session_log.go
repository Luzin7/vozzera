package voice

import (
	"log"

	"github.com/livekit/protocol/livekit"
)

type sessionSkipReason string

const (
	skipMissingJoinedAt  sessionSkipReason = "missing_joined_at"
	skipMissingCreatedAt sessionSkipReason = "missing_created_at"
	skipNegativeDuration sessionSkipReason = "negative_duration"
)

func participantSessionSeconds(event *livekit.WebhookEvent) (int64, sessionSkipReason, bool) {
	joinedAt := event.GetParticipant().GetJoinedAt()
	if joinedAt <= 0 {
		return 0, skipMissingJoinedAt, false
	}
	createdAt := event.GetCreatedAt()
	if createdAt <= 0 {
		return 0, skipMissingCreatedAt, false
	}
	if createdAt < joinedAt {
		return 0, skipNegativeDuration, false
	}
	return createdAt - joinedAt, "", true
}

func logSessionEnded(logger *log.Logger, roomID string, event *livekit.WebhookEvent) {
	p := event.GetParticipant()
	seconds, reason, ok := participantSessionSeconds(event)
	if !ok {
		logger.Printf("voice.session_ended level=warn event_id=%s room_id=%s user_id=%s sid=%s skip_reason=%s disconnect_reason=%s",
			event.GetId(), roomID, p.GetIdentity(), p.GetSid(), reason, p.GetDisconnectReason())
		return
	}
	logger.Printf("voice.session_ended event_id=%s room_id=%s user_id=%s sid=%s joined_at=%d seconds=%d disconnect_reason=%s",
		event.GetId(), roomID, p.GetIdentity(), p.GetSid(), p.GetJoinedAt(), seconds, p.GetDisconnectReason())
}

func logWebhookDropped(logger *log.Logger, event *livekit.WebhookEvent) {
	if event.GetNumDropped() <= 0 {
		return
	}
	logger.Printf("voice.webhook_dropped level=warn event_id=%s event=%s num_dropped=%d",
		event.GetId(), event.GetEvent(), event.GetNumDropped())
}
