package voice

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const leaveVoiceMinInterval = time.Second

type LeaveVoiceInput struct {
	UserID uuid.UUID
	RoomID uuid.UUID
	SID    string
}

type LeaveVoiceService struct {
	state  VoiceRoomState
	events *awayEvents
	leaves *RecentLeaves
	now    func() time.Time

	mu        sync.Mutex
	lastBySID map[string]time.Time
}

func NewLeaveVoiceService(state VoiceRoomState, publisher realtime.Publisher, leaves *RecentLeaves) *LeaveVoiceService {
	return &LeaveVoiceService{
		state:     state,
		events:    newAwayEvents(state, publisher),
		leaves:    leaves,
		now:       time.Now,
		lastBySID: make(map[string]time.Time),
	}
}

func (s *LeaveVoiceService) Execute(ctx context.Context, in LeaveVoiceInput) error {
	if in.RoomID == uuid.Nil || in.SID == "" {
		return nil
	}

	snapshot, err := s.state.Snapshot(ctx, in.RoomID)
	if err != nil {
		log.Printf("voice.leave level=error step=snapshot room_id=%s user_id=%s err=%q", in.RoomID, in.UserID, err)
		return ErrVoiceStateUnavailable(err)
	}
	if !ownsSession(snapshot.Participants, in.UserID, in.SID) || !s.allow(in.SID) {
		return nil
	}
	return s.leave(ctx, in, in.SID)
}

func ownsSession(participants []Participant, userID uuid.UUID, sid string) bool {
	for _, p := range participants {
		if p.SID == sid && p.UserID == userID.String() {
			return true
		}
	}
	return false
}

func (s *LeaveVoiceService) leave(ctx context.Context, in LeaveVoiceInput, sid string) error {
	startedAt := s.now()
	result, err := s.state.Leave(ctx, LeaveInput{RoomID: in.RoomID, SID: sid, At: startedAt})
	if err != nil {
		log.Printf("voice.leave level=error step=leave room_id=%s user_id=%s err=%q", in.RoomID, in.UserID, err)
		return ErrVoiceStateUnavailable(err)
	}
	if !result.Left {
		return nil
	}

	s.leaves.Record(sid)

	s.events.publishPresence(ctx, in.RoomID, realtime.PresenceLeft, result.Participants)
	s.events.publishSession(ctx, in.RoomID, result.SessionStartedAt)
	if result.Emptied {
		s.events.publishAwayList(ctx, in.RoomID, nil)
	}
	for _, entry := range result.RemovedAway {
		logAwayCleared(entry, "room_empty")
		s.events.publishEnded(ctx, in.RoomID, entry.UserID, AwayEndedRoomEmpty)
	}
	log.Printf("voice.leave room_id=%s user_id=%s sid=%s publish_ms=%d", in.RoomID, in.UserID, sid, s.now().Sub(startedAt).Milliseconds())
	return nil
}

func (s *LeaveVoiceService) allow(sid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	for id, at := range s.lastBySID {
		if now.Sub(at) >= leaveVoiceMinInterval {
			delete(s.lastBySID, id)
		}
	}
	if _, throttled := s.lastBySID[sid]; throttled {
		return false
	}
	s.lastBySID[sid] = now
	return true
}
