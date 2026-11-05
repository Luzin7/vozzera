package voice

import (
	"context"
	"log"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const hydrationTimeout = 5 * time.Second

type HydratePresenceService struct {
	lister  RoomLister
	state   VoiceRoomState
	events  *awayEvents
	timeout time.Duration
}

func NewHydratePresenceService(lister RoomLister, state VoiceRoomState, publisher realtime.Publisher) *HydratePresenceService {
	return &HydratePresenceService{lister: lister, state: state, events: newAwayEvents(state, publisher), timeout: hydrationTimeout}
}

type hydrationTally struct {
	rooms        int
	participants int
	skippedRooms int
}

func (s *HydratePresenceService) Execute(ctx context.Context) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	tally, err := s.hydrateRooms(ctx)
	if err != nil {
		return err
	}

	log.Printf("voice.hydrate rooms=%d participants=%d skipped_rooms=%d duration_ms=%d",
		tally.rooms, tally.participants, tally.skippedRooms, time.Since(start).Milliseconds())
	return nil
}

func (s *HydratePresenceService) hydrateRooms(ctx context.Context) (hydrationTally, error) {
	var tally hydrationTally

	names, err := s.lister.ListRoomNames(ctx)
	if err != nil {
		return tally, err
	}

	for _, name := range names {
		roomID, err := uuid.Parse(name)
		if err != nil {
			tally.skippedRooms++
			continue
		}

		participants, err := s.lister.ListParticipants(ctx, name)
		if err != nil {
			log.Printf("voice.hydrate level=error room_id=%s err=%q", roomID, err)
			tally.skippedRooms++
			continue
		}

		participants = withUUIDIdentity(participants)
		written, err := s.state.HydrateRoom(ctx, roomID, participants)
		if err != nil {
			return tally, err
		}
		if !written {
			continue
		}

		tally.rooms++
		tally.participants += len(participants)
		s.events.publishPresence(ctx, roomID, realtime.PresenceSnapshot, participants)
	}
	return tally, nil
}

func withUUIDIdentity(participants []Participant) []Participant {
	valid := make([]Participant, 0, len(participants))
	for _, p := range participants {
		if _, err := uuid.Parse(p.UserID); err == nil {
			valid = append(valid, p)
		}
	}
	return valid
}
