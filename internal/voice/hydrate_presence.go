package voice

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const (
	hydrationTimeout     = 5 * time.Second
	roomHydrationTimeout = 1 * time.Second
)

var errPresenceRevisionConflict = errors.New("presença alterada durante a reconciliação")

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

	log.Printf("voice.hydrate rooms=%d participants=%d skipped_rooms=%d duration_ms=%d",
		tally.rooms, tally.participants, tally.skippedRooms, time.Since(start).Milliseconds())
	return err
}

func (s *HydratePresenceService) hydrateRooms(ctx context.Context) (hydrationTally, error) {
	var tally hydrationTally

	rooms, err := s.lister.ListRooms(ctx)
	if err != nil {
		log.Printf("voice.hydrate level=error step=list_rooms err=%q", err)
		return tally, err
	}

	var failures []error
	for _, room := range rooms {
		roomID, err := uuid.Parse(room.Name)
		if err != nil {
			tally.skippedRooms++
			continue
		}

		participants, written, err := s.reconcileRoom(ctx, room, roomID)
		if err != nil {
			log.Printf("voice.hydrate level=error room_id=%s err=%q", roomID, err)
			tally.skippedRooms++
			failures = append(failures, fmt.Errorf("sala %s: %w", roomID, err))
			continue
		}
		if !written {
			continue
		}

		tally.rooms++
		tally.participants += participants
	}
	return tally, errors.Join(failures...)
}

func (s *HydratePresenceService) reconcileRoom(ctx context.Context, room LiveKitRoom, roomID uuid.UUID) (int, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, roomHydrationTimeout)
	defer cancel()

	revision, err := s.state.PresenceRevision(ctx, roomID)
	if err != nil {
		return 0, false, err
	}

	participants, err := s.lister.ListParticipants(ctx, room.Name)
	if err != nil {
		return 0, false, err
	}

	participants = withUUIDIdentity(participants)
	hydrated, err := s.state.HydrateRoom(ctx, RoomHydration{RoomID: roomID, Participants: participants, CreatedAt: room.CreatedAt, ExpectedRevision: revision})
	if err != nil {
		return 0, false, err
	}
	if hydrated.Conflict {
		return 0, false, errPresenceRevisionConflict
	}

	s.announceRemovedAway(ctx, roomID, hydrated.RemovedAway)

	if !hydrated.Written {
		return 0, false, nil
	}

	if room.CreatedAt.IsZero() {
		log.Printf("voice.session level=warn cause=missing_room_creation_time step=hydrate room_id=%s", roomID)
	}

	s.events.publishPresence(ctx, roomID, realtime.PresenceSnapshot, participants)
	s.events.publishSession(ctx, roomID, hydrated.SessionStartedAt)
	return len(participants), true, nil
}

func (s *HydratePresenceService) announceRemovedAway(ctx context.Context, roomID uuid.UUID, removed []AwayEntry) {
	if len(removed) == 0 {
		return
	}
	s.events.publishAwayList(ctx, roomID, nil)
	for _, entry := range removed {
		logAwayCleared(entry, "room_empty")
		s.events.publishEnded(ctx, roomID, entry.UserID, AwayEndedRoomEmpty)
	}
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
