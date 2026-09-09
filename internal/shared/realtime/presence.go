package realtime

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

type Participant struct {
	SID      string `json:"sid"`
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

type VoiceRoomPresence struct {
	mu    sync.RWMutex
	rooms map[uuid.UUID]map[string]Participant
}

func NewVoiceRoomPresence() *VoiceRoomPresence {
	return &VoiceRoomPresence{
		rooms: make(map[uuid.UUID]map[string]Participant),
	}
}

func (ps *VoiceRoomPresence) snapshot(roomID uuid.UUID) []Participant {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return ps.snapshotUnsafe(roomID)
}

func (ps *VoiceRoomPresence) snapshotUnsafe(roomID uuid.UUID) []Participant {
	room, exists := ps.rooms[roomID]
	if !exists {
		return []Participant{}
	}

	participants := make([]Participant, 0, len(room))
	for _, participant := range room {
		participants = append(participants, participant)
	}

	return participants
}

func (ps *VoiceRoomPresence) Snapshot(roomID uuid.UUID) []Participant {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return ps.snapshot(roomID)
}

func (ps *VoiceRoomPresence) SnapshotJSON(roomID uuid.UUID) []byte {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	room, exists := ps.rooms[roomID]
	if !exists || len(room) == 0 {
		return nil
	}

	participants := make([]Participant, 0, len(room))
	for _, p := range room {
		participants = append(participants, p)
	}

	data, _ := json.Marshal(participants)
	return data
}

func (ps *VoiceRoomPresence) Clear(roomID uuid.UUID) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	delete(ps.rooms, roomID)
}

func (ps *VoiceRoomPresence) Join(roomID uuid.UUID, participant Participant) []Participant {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.rooms[roomID] == nil {
		ps.rooms[roomID] = make(map[string]Participant)
	}

	ps.rooms[roomID][participant.SID] = participant

	return ps.snapshotUnsafe(roomID)
}

func (ps *VoiceRoomPresence) Leave(roomID uuid.UUID, sid string) ([]Participant, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	room, exists := ps.rooms[roomID]
	if !exists {
		return nil, false
	}
	_, SIDExists := room[sid]
	if !SIDExists {
		return nil, false
	}
	delete(room, sid)

	return ps.snapshotUnsafe(roomID), true
}
