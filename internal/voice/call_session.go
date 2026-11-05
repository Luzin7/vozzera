package voice

import (
	"context"
	"strconv"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
)

const EventSessionSnapshot = "voice.session.snapshot"

const (
	sessionOriginExact    = "exact"
	sessionOriginFallback = "fallback"
)

const sessionTimeLayout = "2006-01-02T15:04:05.000Z"

type sessionSnapshotPayload struct {
	StartedAt *string `json:"started_at"`
}

func roomCreationTime(room *livekit.Room) time.Time {
	if ms := room.GetCreationTimeMs(); ms > 0 {
		return time.UnixMilli(ms).UTC()
	}
	if seconds := room.GetCreationTime(); seconds > 0 {
		return time.Unix(seconds, 0).UTC()
	}
	return time.Time{}
}

func (e *awayEvents) publishSession(ctx context.Context, roomID uuid.UUID, startedAt *time.Time) {
	e.publish(ctx, realtime.RoomTopic(roomID), EventSessionSnapshot, newSessionSnapshotPayload(startedAt))
}

func newSessionSnapshotPayload(startedAt *time.Time) sessionSnapshotPayload {
	if startedAt == nil {
		return sessionSnapshotPayload{}
	}
	formatted := startedAt.UTC().Format(sessionTimeLayout)
	return sessionSnapshotPayload{StartedAt: &formatted}
}

func sessionOrigin(created, fallback time.Time) (millis int64, origin string) {
	if created.IsZero() {
		return fallback.UnixMilli(), sessionOriginFallback
	}
	return created.UnixMilli(), sessionOriginExact
}

func sessionTimeFromStored(value any) (*time.Time, error) {
	raw, ok := value.(string)
	if !ok {
		return nil, errUnexpectedScriptReply
	}
	if raw == "" {
		return nil, nil
	}
	millis, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, errUnexpectedScriptReply
	}
	started := time.UnixMilli(millis).UTC()
	return &started, nil
}
