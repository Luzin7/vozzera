package voice

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type RoomSnapshotProvider struct {
	state VoiceRoomState
}

func NewRoomSnapshotProvider(state VoiceRoomState) *RoomSnapshotProvider {
	return &RoomSnapshotProvider{state: state}
}

func (p *RoomSnapshotProvider) RoomSnapshot(ctx context.Context, roomID uuid.UUID) ([]realtime.Envelope, error) {
	snapshot, err := p.state.Snapshot(ctx, roomID)
	if err != nil {
		return nil, err
	}

	topic := realtime.RoomTopic(roomID)
	participants := snapshot.Participants
	if participants == nil {
		participants = []Participant{}
	}
	presenceData, err := json.Marshal(participants)
	if err != nil {
		return nil, err
	}

	away := make([]awaySnapshotItem, 0, len(snapshot.Away))
	for _, entry := range snapshot.Away {
		away = append(away, awaySnapshotItem{UserID: entry.UserID, Username: entry.Username, Since: entry.Since})
	}
	awayData, err := json.Marshal(away)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return []realtime.Envelope{
		{V: 1, Type: realtime.PresenceSnapshot, Topic: topic, TS: now, Data: presenceData},
		{V: 1, Type: EventAwaySnapshot, Topic: topic, TS: now, Data: awayData},
	}, nil
}
