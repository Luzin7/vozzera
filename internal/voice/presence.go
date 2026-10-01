package voice

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

func (ps *VoiceRoomPresence) snapshotLocked(roomID uuid.UUID) []Participant {
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

	return ps.snapshotLocked(roomID)
}

func (ps *VoiceRoomPresence) SnapshotJSON(roomID uuid.UUID) []byte {
	participants := ps.Snapshot(roomID)
	if len(participants) == 0 {
		return nil
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

	return ps.snapshotLocked(roomID)
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

	return ps.snapshotLocked(roomID), true
}
