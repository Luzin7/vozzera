package voice

import (
	"context"
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const memoryHeartbeatTTL = 30 * time.Second

type memoryVoiceRoomState struct {
	mu         sync.Mutex
	now        func() time.Time
	presence   map[uuid.UUID]map[string]Participant
	away       map[uuid.UUID]map[uuid.UUID]AwayEntry
	index      map[uuid.UUID]uuid.UUID
	sessions   map[uuid.UUID]int64
	revisions  map[uuid.UUID]int64
	heartbeats map[uuid.UUID]time.Time
	epochSet   bool
}

var _ VoiceRoomState = (*memoryVoiceRoomState)(nil)

func newMemoryVoiceRoomState(now func() time.Time) *memoryVoiceRoomState {
	return &memoryVoiceRoomState{
		now:        now,
		presence:   make(map[uuid.UUID]map[string]Participant),
		away:       make(map[uuid.UUID]map[uuid.UUID]AwayEntry),
		index:      make(map[uuid.UUID]uuid.UUID),
		sessions:   make(map[uuid.UUID]int64),
		revisions:  make(map[uuid.UUID]int64),
		heartbeats: make(map[uuid.UUID]time.Time),
	}
}

func normalizedAwayEntry(entry AwayEntry) AwayEntry {
	entry.Since = time.UnixMilli(entry.Since.UnixMilli()).UTC()
	return entry
}

func (s *memoryVoiceRoomState) participantsLocked(roomID uuid.UUID) []Participant {
	participants := make([]Participant, 0, len(s.presence[roomID]))
	for _, p := range s.presence[roomID] {
		participants = append(participants, p)
	}
	sort.Slice(participants, func(i, j int) bool { return participants[i].SID < participants[j].SID })
	return participants
}

func (s *memoryVoiceRoomState) awayEntriesLocked(roomID uuid.UUID) []AwayEntry {
	entries := make([]AwayEntry, 0, len(s.away[roomID]))
	for _, entry := range s.away[roomID] {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].Since.Equal(entries[j].Since) {
			return entries[i].Since.Before(entries[j].Since)
		}
		return entries[i].UserID.String() < entries[j].UserID.String()
	})
	return entries
}

func (s *memoryVoiceRoomState) openSessionLocked(roomID uuid.UUID, created, fallback time.Time) {
	millis, origin := sessionOrigin(created, fallback)
	current, open := s.sessions[roomID]
	if !open || (origin == sessionOriginExact && current < millis) {
		s.sessions[roomID] = millis
	}
}

func (s *memoryVoiceRoomState) sessionLocked(roomID uuid.UUID) *time.Time {
	millis, open := s.sessions[roomID]
	if !open {
		return nil
	}
	started := time.UnixMilli(millis).UTC()
	return &started
}

func (s *memoryVoiceRoomState) removeAwayLocked(userID uuid.UUID) *AwayEntry {
	roomID, indexed := s.index[userID]
	if !indexed {
		return nil
	}
	delete(s.index, userID)

	entry, exists := s.away[roomID][userID]
	if !exists {
		return nil
	}
	delete(s.away[roomID], userID)
	if len(s.away[roomID]) == 0 {
		delete(s.away, roomID)
	}
	return &entry
}

func (s *memoryVoiceRoomState) clearRoomAwayLocked(roomID uuid.UUID) []AwayEntry {
	removed := s.awayEntriesLocked(roomID)
	for _, entry := range removed {
		if s.index[entry.UserID] == roomID {
			delete(s.index, entry.UserID)
		}
	}
	delete(s.away, roomID)
	return removed
}

func (s *memoryVoiceRoomState) activeOthersLocked(roomID, excludedUserID uuid.UUID) int {
	active := 0
	for _, p := range s.presence[roomID] {
		userID, err := uuid.Parse(p.UserID)
		if err == nil && userID == excludedUserID {
			continue
		}
		if err == nil && s.isAwayLocked(roomID, userID) {
			continue
		}
		active++
	}
	return active
}

func (s *memoryVoiceRoomState) isAwayLocked(roomID, userID uuid.UUID) bool {
	_, away := s.away[roomID][userID]
	return away
}

func (s *memoryVoiceRoomState) Join(_ context.Context, in JoinInput) (JoinResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	roomID, p, at := in.RoomID, in.Participant, in.At

	if s.presence[roomID] == nil {
		s.presence[roomID] = make(map[string]Participant)
	}
	s.presence[roomID][p.SID] = p
	s.revisions[roomID]++
	s.openSessionLocked(roomID, in.RoomCreatedAt, at)

	return JoinResult{Participants: s.participantsLocked(roomID), ClearedAway: s.clearAwayOnJoinLocked(p.UserID, at), SessionStartedAt: s.sessionLocked(roomID)}, nil
}

func (s *memoryVoiceRoomState) clearAwayOnJoinLocked(rawUserID string, at time.Time) *AwayEntry {
	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return nil
	}
	awayRoomID, indexed := s.index[userID]
	if !indexed {
		return nil
	}
	entry, exists := s.away[awayRoomID][userID]
	if !exists || entry.Since.UnixMilli() > at.UnixMilli() {
		return nil
	}
	return s.removeAwayLocked(userID)
}

func (s *memoryVoiceRoomState) Leave(_ context.Context, in LeaveInput) (LeaveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	roomID, sid := in.RoomID, in.SID

	s.revisions[roomID]++
	_, left := s.presence[roomID][sid]
	delete(s.presence[roomID], sid)
	if len(s.presence[roomID]) == 0 {
		delete(s.presence, roomID)
	}

	result := LeaveResult{Participants: s.participantsLocked(roomID), Left: left}
	if s.activeOthersLocked(roomID, uuid.Nil) == 0 {
		result.Emptied = true
		result.RemovedAway = s.clearRoomAwayLocked(roomID)
		delete(s.sessions, roomID)
		return result, nil
	}

	s.openSessionLocked(roomID, in.RoomCreatedAt, in.At)
	result.SessionStartedAt = s.sessionLocked(roomID)
	return result, nil
}

func (s *memoryVoiceRoomState) Finish(_ context.Context, roomID uuid.UUID) ([]AwayEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.revisions[roomID]++
	delete(s.presence, roomID)
	delete(s.sessions, roomID)
	return s.clearRoomAwayLocked(roomID), nil
}

func (s *memoryVoiceRoomState) Snapshot(_ context.Context, roomID uuid.UUID) (RoomSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return RoomSnapshot{Participants: s.participantsLocked(roomID), Away: s.awayEntriesLocked(roomID), SessionStartedAt: s.sessionLocked(roomID)}, nil
}

func (s *memoryVoiceRoomState) EnterAway(_ context.Context, entry AwayEntry) (EnterAwayResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeOthersLocked(entry.RoomID, entry.UserID) == 0 {
		return EnterAwayResult{}, nil
	}

	entry = normalizedAwayEntry(entry)
	var movedFrom *uuid.UUID
	previousRoomID, indexed := s.index[entry.UserID]
	if indexed && previousRoomID != entry.RoomID {
		s.removeAwayLocked(entry.UserID)
		movedFrom = &previousRoomID
	}
	if previous, exists := s.away[entry.RoomID][entry.UserID]; exists {
		entry.Since = previous.Since
	}

	if s.away[entry.RoomID] == nil {
		s.away[entry.RoomID] = make(map[uuid.UUID]AwayEntry)
	}
	s.away[entry.RoomID][entry.UserID] = entry
	s.index[entry.UserID] = entry.RoomID
	s.revisions[entry.RoomID]++

	return EnterAwayResult{Accepted: true, MovedFrom: movedFrom}, nil
}

func (s *memoryVoiceRoomState) LeaveAway(_ context.Context, userID uuid.UUID) (*AwayEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.removeAwayLocked(userID), nil
}

func (s *memoryVoiceRoomState) ReleaseAway(_ context.Context, userID uuid.UUID, owner AwayOwner) (*AwayEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	roomID, indexed := s.index[userID]
	if !indexed {
		return nil, nil
	}
	if entry, exists := s.away[roomID][userID]; !exists || entry.Owner != owner {
		return nil, nil
	}
	return s.removeAwayLocked(userID), nil
}

func (s *memoryVoiceRoomState) SweepDeadOwners(_ context.Context) ([]AwayEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var removed []AwayEntry
	visited := 0
	for userID, roomID := range s.index {
		if visited == sweepMaxEntries {
			break
		}
		visited++

		entry, exists := s.away[roomID][userID]
		if !exists {
			delete(s.index, userID)
			continue
		}
		if s.instanceAliveLocked(entry.Owner.InstanceID) {
			continue
		}
		removed = append(removed, *s.removeAwayLocked(userID))
	}
	return removed, nil
}

func (s *memoryVoiceRoomState) instanceAliveLocked(instanceID uuid.UUID) bool {
	expiresAt, beating := s.heartbeats[instanceID]
	return beating && s.now().Before(expiresAt)
}

func (s *memoryVoiceRoomState) PresenceRevision(_ context.Context, roomID uuid.UUID) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.revisions[roomID], nil
}

func (s *memoryVoiceRoomState) HydrateRoom(_ context.Context, in RoomHydration) (HydrateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	roomID, participants := in.RoomID, in.Participants
	if s.revisions[roomID] != in.ExpectedRevision {
		return HydrateResult{Conflict: true, SessionStartedAt: s.sessionLocked(roomID)}, nil
	}
	if len(participants) == 0 && in.PreserveEmpty {
		return HydrateResult{SessionStartedAt: s.sessionLocked(roomID)}, nil
	}
	hadPresence := len(s.presence[roomID]) > 0
	changed := s.reconcilePresenceLocked(roomID, participants)
	if changed {
		s.revisions[roomID]++
	}
	if !hadPresence && len(participants) > 0 {
		s.openSessionLocked(roomID, in.CreatedAt, s.now())
	}
	var removedAway []AwayEntry
	if len(participants) == 0 {
		removedAway = s.clearRoomAwayLocked(roomID)
		delete(s.sessions, roomID)
	}
	return HydrateResult{Written: changed, SessionStartedAt: s.sessionLocked(roomID), RemovedAway: removedAway}, nil
}

func (s *memoryVoiceRoomState) reconcilePresenceLocked(roomID uuid.UUID, participants []Participant) bool {
	next := make(map[string]Participant, len(participants))
	for _, p := range participants {
		next[p.SID] = p
	}
	if maps.Equal(s.presence[roomID], next) {
		return false
	}
	if len(next) == 0 {
		delete(s.presence, roomID)
		return true
	}
	s.presence[roomID] = next
	return true
}

func (s *memoryVoiceRoomState) Beat(_ context.Context, instanceID uuid.UUID) (BeatResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.heartbeats[instanceID] = s.now().Add(memoryHeartbeatTTL)

	wiped := !s.epochSet
	s.epochSet = true
	return BeatResult{Wiped: wiped}, nil
}

func (s *memoryVoiceRoomState) expireInstance(instanceID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.heartbeats, instanceID)
}
