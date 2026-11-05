package voice

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/livekit/protocol/livekit"
)

func leftEvent(joinedAt, createdAt int64) *livekit.WebhookEvent {
	return &livekit.WebhookEvent{
		Id:          "EV_1",
		Event:       "participant_left",
		CreatedAt:   createdAt,
		Participant: &livekit.ParticipantInfo{Sid: "PA_1", Identity: "user-1", JoinedAt: joinedAt},
	}
}

func TestParticipantSessionSeconds(t *testing.T) {
	tests := []struct {
		name       string
		event      *livekit.WebhookEvent
		wantSecs   int64
		wantReason sessionSkipReason
		wantOK     bool
	}{
		{"normal", leftEvent(1000, 1150), 150, "", true},
		{"missing joined", leftEvent(0, 1150), 0, skipMissingJoinedAt, false},
		{"missing created", leftEvent(1000, 0), 0, skipMissingCreatedAt, false},
		{"clock skew negative", leftEvent(1200, 1150), 0, skipNegativeDuration, false},
		{"zero length", leftEvent(1000, 1000), 0, "", true},
		{"nil participant", &livekit.WebhookEvent{CreatedAt: 10}, 0, skipMissingJoinedAt, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secs, reason, ok := participantSessionSeconds(tt.event)
			if secs != tt.wantSecs || reason != tt.wantReason || ok != tt.wantOK {
				t.Errorf("got (%d, %q, %v), want (%d, %q, %v)", secs, reason, ok, tt.wantSecs, tt.wantReason, tt.wantOK)
			}
		})
	}
}

func TestLogSessionEnded(t *testing.T) {
	tests := []struct {
		name  string
		event *livekit.WebhookEvent
		want  string
	}{
		{"ok", leftEvent(1000, 1150), "voice.session_ended event_id=EV_1 room_id=room-1 user_id=user-1 sid=PA_1 joined_at=1000 seconds=150 disconnect_reason=CLIENT_INITIATED"},
		{"skipped", leftEvent(0, 1150), "voice.session_ended level=warn event_id=EV_1 room_id=room-1 user_id=user-1 sid=PA_1 skip_reason=missing_joined_at disconnect_reason=CLIENT_INITIATED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := log.New(&buf, "", 0)
			tt.event.Participant.DisconnectReason = livekit.DisconnectReason_CLIENT_INITIATED
			logSessionEnded(logger, "room-1", tt.event)
			if got := strings.TrimSpace(buf.String()); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLogWebhookDropped(t *testing.T) {
	tests := []struct {
		name       string
		numDropped int32
		want       string
	}{
		{"dropped", 3, "voice.webhook_dropped level=warn event_id=EV_1 event=participant_left num_dropped=3"},
		{"none dropped", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			event := leftEvent(1000, 1150)
			event.NumDropped = tt.numDropped
			logWebhookDropped(log.New(&buf, "", 0), event)
			if got := strings.TrimSpace(buf.String()); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
