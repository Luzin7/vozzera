package voice

import (
	"context"
	"log"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type SweepAwayService struct {
	state  VoiceRoomState
	events *awayEvents
}

func NewSweepAwayService(state VoiceRoomState, publisher realtime.Publisher) *SweepAwayService {
	return &SweepAwayService{state: state, events: newAwayEvents(state, publisher)}
}

func (s *SweepAwayService) Execute(ctx context.Context) error {
	removed, err := s.state.SweepDeadOwners(ctx)
	if err != nil {
		log.Printf("voice.away.error level=error op=sweep user_id= room_id= err=%q", err)
		return ErrVoiceStateUnavailable(err)
	}

	byRoom := make(map[uuid.UUID][]AwayEntry)
	var rooms []uuid.UUID
	for _, entry := range removed {
		logAwayCleared(entry, "swept")
		if _, seen := byRoom[entry.RoomID]; !seen {
			rooms = append(rooms, entry.RoomID)
		}
		byRoom[entry.RoomID] = append(byRoom[entry.RoomID], entry)
	}

	for _, roomID := range rooms {
		if err := s.events.publishCurrentAwayList(ctx, roomID); err != nil {
			continue
		}
		for _, entry := range byRoom[roomID] {
			s.events.publishEnded(ctx, roomID, entry.UserID, AwayEndedConnectionClosed)
		}
	}
	return nil
}
