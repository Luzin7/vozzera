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
	hydrationTimeout        = 5 * time.Second
	roomHydrationTimeout    = 1 * time.Second
	emptyConfirmationCycles = 2
)

var errPresenceRevisionConflict = errors.New("presença alterada durante a reconciliação")

type HydratePresenceService struct {
	lister            RoomLister
	state             VoiceRoomState
	events            *awayEvents
	timeout           time.Duration
	cursor            int
	emptyObservations map[uuid.UUID]int
}

func NewHydratePresenceService(lister RoomLister, state VoiceRoomState, publisher realtime.Publisher) *HydratePresenceService {
	return &HydratePresenceService{
		lister:            lister,
		state:             state,
		events:            newAwayEvents(state, publisher),
		timeout:           hydrationTimeout,
		emptyObservations: make(map[uuid.UUID]int),
	}
}

type hydrationTally struct {
	rooms           int
	participants    int
	emptyPending    int
	skippedBudget   int
	vanished        int
	vanishedPending int
	failed          int
	touchFailed     int
	invalidRooms    int
}

type roomReconcile struct {
	participants int
	emptyPending bool
	touchFailed  bool
}

func (s *HydratePresenceService) Execute(ctx context.Context) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	tally, err := s.hydrateRooms(ctx)

	level := "info"
	if tally.touchFailed > 0 {
		level = "warn"
	}
	log.Printf("voice.hydrate level=%s rooms=%d participants=%d empty_pending=%d skipped_budget=%d vanished=%d vanished_pending=%d failed=%d touch_failed=%d invalid_rooms=%d duration_ms=%d",
		level, tally.rooms, tally.participants, tally.emptyPending, tally.skippedBudget, tally.vanished, tally.vanishedPending, tally.failed, tally.touchFailed, tally.invalidRooms, time.Since(start).Milliseconds())
	return err
}

func (s *HydratePresenceService) hydrateRooms(ctx context.Context) (hydrationTally, error) {
	rooms, err := s.lister.ListRooms(ctx)
	if err != nil {
		log.Printf("voice.hydrate level=error step=list_rooms err=%q", err)
		return hydrationTally{}, err
	}

	s.indexRooms(ctx)
	tally, attempted, failures := s.reconcileListedRooms(ctx, rooms)
	s.cursor += attempted

	listed := listedRoomIDs(rooms)
	tracked, err := s.state.RoomsWithState(ctx)
	if err != nil {
		log.Printf("voice.hydrate level=error step=rooms_with_state err=%q", err)
		s.pruneEmptyObservations(listed)
		return tally, errors.Join(failures, fmt.Errorf("salas com estado: %w", err))
	}

	tally.vanished, tally.vanishedPending, err = s.sweepVanished(ctx, listed, tracked)
	s.pruneEmptyObservations(trackedRooms(listed, tracked))
	return tally, errors.Join(failures, err)
}

func (s *HydratePresenceService) reconcileListedRooms(ctx context.Context, rooms []LiveKitRoom) (hydrationTally, int, error) {
	var tally hydrationTally
	var failures []error
	ordered := s.rotateRooms(rooms)
	attempted := 0
	for i, room := range ordered {
		if ctx.Err() != nil {
			tally.skippedBudget += len(ordered) - i
			break
		}

		roomID, err := uuid.Parse(room.Name)
		if err != nil {
			tally.invalidRooms++
			continue
		}

		attempted++
		result, err := s.reconcileRoom(ctx, room, roomID)
		if result.touchFailed {
			tally.touchFailed++
		}
		if err != nil {
			if ctx.Err() != nil {
				tally.skippedBudget += len(ordered) - i
				break
			}
			log.Printf("voice.hydrate level=error room_id=%s err=%q", roomID, err)
			tally.failed++
			failures = append(failures, fmt.Errorf("sala %s: %w", roomID, err))
			continue
		}

		tally.rooms++
		tally.participants += result.participants
		if result.emptyPending {
			tally.emptyPending++
		}
	}
	return tally, attempted, errors.Join(failures...)
}

func (s *HydratePresenceService) indexRooms(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, kvOperationTimeout)
	defer cancel()

	indexed, err := s.state.IndexRoomsFromPresence(ctx)
	if err != nil {
		log.Printf("voice.hydrate level=error step=index_rooms err=%q", err)
		return
	}
	if indexed > 0 {
		log.Printf("voice.hydrate level=info step=index_rooms rooms=%d", indexed)
	}
}

func (s *HydratePresenceService) sweepVanished(ctx context.Context, listed map[uuid.UUID]struct{}, tracked []uuid.UUID) (int, int, error) {
	vanished, pending := 0, 0
	var failures []error
	for _, roomID := range tracked {
		if _, ok := listed[roomID]; ok {
			continue
		}
		applyEmpty := s.emptyObservations[roomID]+1 >= emptyConfirmationCycles
		if s.trackEmptyObservation(roomID, true, applyEmpty) {
			pending++
			continue
		}
		if err := s.sweepRoom(ctx, roomID); err != nil {
			log.Printf("voice.hydrate level=error step=vanished room_id=%s err=%q", roomID, err)
			failures = append(failures, fmt.Errorf("sala ausente %s: %w", roomID, err))
			continue
		}
		vanished++
	}
	return vanished, pending, errors.Join(failures...)
}

func (s *HydratePresenceService) sweepRoom(ctx context.Context, roomID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), kvOperationTimeout)
	defer cancel()

	removed, err := s.state.Finish(ctx, roomID)
	if err != nil {
		return err
	}
	s.announceRemovedAway(ctx, roomID, removed, "room_gone")
	s.announcePresence(ctx, roomID, nil, nil)
	return nil
}

func listedRoomIDs(rooms []LiveKitRoom) map[uuid.UUID]struct{} {
	listed := make(map[uuid.UUID]struct{}, len(rooms))
	for _, room := range rooms {
		roomID, err := uuid.Parse(room.Name)
		if err != nil {
			continue
		}
		listed[roomID] = struct{}{}
	}
	return listed
}

func trackedRooms(listed map[uuid.UUID]struct{}, tracked []uuid.UUID) map[uuid.UUID]struct{} {
	keep := make(map[uuid.UUID]struct{}, len(listed)+len(tracked))
	for roomID := range listed {
		keep[roomID] = struct{}{}
	}
	for _, roomID := range tracked {
		keep[roomID] = struct{}{}
	}
	return keep
}

func (s *HydratePresenceService) reconcileRoom(ctx context.Context, room LiveKitRoom, roomID uuid.UUID) (result roomReconcile, err error) {
	ctx, cancel := context.WithTimeout(ctx, roomHydrationTimeout)
	defer cancel()

	revision, err := s.state.PresenceRevision(ctx, roomID)
	if err != nil {
		return result, err
	}

	result.touchFailed = !s.touchRoom(ctx, roomID)

	observed, err := s.lister.ListParticipants(ctx, room.Name)
	if err != nil {
		return result, err
	}

	participants := withUUIDIdentity(observed)
	empty := len(participants) == 0
	applyEmpty := empty && s.emptyObservations[roomID]+1 >= emptyConfirmationCycles

	hydrated, err := s.state.HydrateRoom(ctx, RoomHydration{
		RoomID:           roomID,
		Participants:     participants,
		CreatedAt:        room.CreatedAt,
		ExpectedRevision: revision,
		PreserveEmpty:    empty && !applyEmpty,
	})
	if err != nil {
		return result, err
	}
	if hydrated.Written && !s.touchRoom(ctx, roomID) {
		result.touchFailed = true
	}
	if hydrated.Conflict {
		s.announceConflictState(ctx, roomID)
		return result, errPresenceRevisionConflict
	}

	s.announceRemovedAway(ctx, roomID, hydrated.RemovedAway, "room_empty")

	if hydrated.Written && room.CreatedAt.IsZero() {
		log.Printf("voice.session level=warn cause=missing_room_creation_time step=hydrate room_id=%s", roomID)
	}

	s.announcePresence(ctx, roomID, participants, hydrated.SessionStartedAt)

	if s.trackEmptyObservation(roomID, empty, applyEmpty) {
		result.emptyPending = true
		return result, nil
	}
	result.participants = len(participants)
	return result, nil
}

func (s *HydratePresenceService) trackEmptyObservation(roomID uuid.UUID, empty, applyEmpty bool) bool {
	if !empty {
		delete(s.emptyObservations, roomID)
		return false
	}
	if !applyEmpty {
		s.emptyObservations[roomID]++
		return true
	}
	delete(s.emptyObservations, roomID)
	return false
}

func (s *HydratePresenceService) announcePresence(ctx context.Context, roomID uuid.UUID, participants []Participant, startedAt *time.Time) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishTimeout)
	defer cancel()

	publishedAt := startedAt
	if len(participants) == 0 {
		publishedAt = nil
	}
	s.events.publishPresence(ctx, roomID, realtime.PresenceSnapshot, participants)
	s.events.publishSession(ctx, roomID, publishedAt)
}

func (s *HydratePresenceService) announceConflictState(ctx context.Context, roomID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), kvOperationTimeout)
	defer cancel()

	snapshot, err := s.state.Snapshot(ctx, roomID)
	if err != nil {
		log.Printf("voice.hydrate level=error step=conflict_snapshot room_id=%s err=%q", roomID, err)
		return
	}
	s.announcePresence(ctx, roomID, snapshot.Participants, snapshot.SessionStartedAt)
}

func (s *HydratePresenceService) touchRoom(ctx context.Context, roomID uuid.UUID) bool {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), kvOperationTimeout)
	defer cancel()

	if err := s.state.TouchRoom(ctx, roomID); err != nil {
		log.Printf("voice.room.ttl level=error room_id=%s err=%q", roomID, err)
		return false
	}
	return true
}

func (s *HydratePresenceService) rotateRooms(rooms []LiveKitRoom) []LiveKitRoom {
	if len(rooms) < 2 {
		return rooms
	}

	offset := s.cursor % len(rooms)
	if offset == 0 {
		return rooms
	}
	ordered := make([]LiveKitRoom, 0, len(rooms))
	ordered = append(ordered, rooms[offset:]...)
	return append(ordered, rooms[:offset]...)
}

func (s *HydratePresenceService) pruneEmptyObservations(seen map[uuid.UUID]struct{}) {
	for roomID := range s.emptyObservations {
		if _, ok := seen[roomID]; !ok {
			delete(s.emptyObservations, roomID)
		}
	}
}

func (s *HydratePresenceService) announceRemovedAway(ctx context.Context, roomID uuid.UUID, removed []AwayEntry, cause string) {
	if len(removed) == 0 {
		return
	}
	s.events.publishAwayList(ctx, roomID, nil)
	for _, entry := range removed {
		logAwayCleared(entry, cause)
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
