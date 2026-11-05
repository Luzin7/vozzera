package voice

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const (
	CmdAwayEnter = "voice.away.enter"
	CmdAwayLeave = "voice.away.leave"

	EventAwaySnapshot = "voice.away.snapshot"
	EventAwayEnded    = "voice.away.ended"
	EventAwayResync   = "voice.away.resync"

	publishTimeout = 3 * time.Second
)

type AwayEndReason string

const (
	AwayEndedRoomEmpty        AwayEndReason = "room_empty"
	AwayEndedReturned         AwayEndReason = "returned"
	AwayEndedConnectionClosed AwayEndReason = "connection_closed"
)

type awaySnapshotItem struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Since    time.Time `json:"since"`
}

type awayEndedPayload struct {
	UserID uuid.UUID     `json:"user_id"`
	Reason AwayEndReason `json:"reason"`
}

type awayEvents struct {
	state     VoiceRoomState
	publisher realtime.Publisher
}

func newAwayEvents(state VoiceRoomState, publisher realtime.Publisher) *awayEvents {
	return &awayEvents{state: state, publisher: publisher}
}

func (e *awayEvents) publish(ctx context.Context, topic realtime.Topic, eventType string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		log.Printf("voice.publish level=error type=%s topic=%s err=%q", eventType, topic, err)
		return
	}

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()

	err = e.publisher.Publish(ctx, topic, realtime.Envelope{V: 1, Type: eventType, Topic: topic, TS: time.Now(), Data: raw})
	if err != nil {
		log.Printf("voice.publish level=error type=%s topic=%s err=%q", eventType, topic, err)
	}
}

func (e *awayEvents) publishAwayList(ctx context.Context, roomID uuid.UUID, entries []AwayEntry) {
	items := make([]awaySnapshotItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, awaySnapshotItem{UserID: entry.UserID, Username: entry.Username, Since: entry.Since})
	}
	e.publish(ctx, realtime.RoomTopic(roomID), EventAwaySnapshot, items)
}

func (e *awayEvents) publishCurrentAwayList(ctx context.Context, roomID uuid.UUID) error {
	snapshot, err := e.state.Snapshot(ctx, roomID)
	if err != nil {
		log.Printf("voice.away.error level=error op=snapshot user_id= room_id=%s err=%q", roomID, err)
		return err
	}

	e.publishAwayList(ctx, roomID, snapshot.Away)
	return nil
}

func (e *awayEvents) publishEnded(ctx context.Context, roomID, userID uuid.UUID, reason AwayEndReason) {
	e.publish(ctx, realtime.RoomTopic(roomID), EventAwayEnded, awayEndedPayload{UserID: userID, Reason: reason})
}

func (e *awayEvents) publishPresence(ctx context.Context, roomID uuid.UUID, eventType string, participants []Participant) {
	if participants == nil {
		participants = []Participant{}
	}
	e.publish(ctx, realtime.RoomTopic(roomID), eventType, participants)
}

func (e *awayEvents) publishResync(ctx context.Context) {
	e.publish(ctx, realtime.GlobalPresenceTopic, EventAwayResync, struct{}{})
}

func logAwayCleared(entry AwayEntry, cause string) {
	log.Printf("voice.away.cleared user_id=%s room_id=%s cause=%s since_ms=%d", entry.UserID, entry.RoomID, cause, entry.Since.UnixMilli())
}
