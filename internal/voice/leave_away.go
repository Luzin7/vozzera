package voice

import (
	"context"
	"log"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type LeaveAwayInput struct {
	UserID uuid.UUID
}

type LeaveAwayService struct {
	state  VoiceRoomState
	events *awayEvents
}

func NewLeaveAwayService(state VoiceRoomState, publisher realtime.Publisher) *LeaveAwayService {
	return &LeaveAwayService{state: state, events: newAwayEvents(state, publisher)}
}

func (s *LeaveAwayService) Execute(ctx context.Context, in LeaveAwayInput) error {
	removed, err := s.state.LeaveAway(ctx, in.UserID)
	if err != nil {
		log.Printf("voice.away.error level=error op=leave user_id=%s room_id= err=%q", in.UserID, err)
		return ErrVoiceStateUnavailable(err)
	}
	if removed == nil {
		return nil
	}

	logAwayCleared(*removed, "returned")
	if err := s.events.publishCurrentAwayList(ctx, removed.RoomID); err != nil {
		return ErrVoiceStateUnavailable(err)
	}
	s.events.publishEnded(ctx, removed.RoomID, removed.UserID, AwayEndedReturned)
	return nil
}
