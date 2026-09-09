package realtime

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestVoiceRoomPresence_Join_AdicionaParticipante(t *testing.T) {
	ps := NewVoiceRoomPresence()
	roomID := uuid.New()
	p := Participant{SID: "sid-1", UserID: "user-1", Username: "Alice"}

	snapshot := ps.Join(roomID, p)

	if len(snapshot) != 1 {
		t.Fatalf("snapshot len = %d, want 1", len(snapshot))
	}
	if snapshot[0].SID != "sid-1" {
		t.Errorf("SID = %q, want %q", snapshot[0].SID, "sid-1")
	}
}

func TestVoiceRoomPresence_JoinMultiplosParticipantes(t *testing.T) {
	ps := NewVoiceRoomPresence()
	roomID := uuid.New()

	ps.Join(roomID, Participant{SID: "sid-1", UserID: "user-1", Username: "Alice"})
	snapshot := ps.Join(roomID, Participant{SID: "sid-2", UserID: "user-2", Username: "Bob"})

	if len(snapshot) != 2 {
		t.Fatalf("snapshot len = %d, want 2", len(snapshot))
	}
}

func TestVoiceRoomPresence_LeaveExistente(t *testing.T) {
	ps := NewVoiceRoomPresence()
	roomID := uuid.New()
	p := Participant{SID: "sid-1", UserID: "user-1", Username: "Alice"}

	ps.Join(roomID, p)
	snapshot, left := ps.Leave(roomID, "sid-1")

	if !left {
		t.Fatal("Leave retornou false, esperava true")
	}
	if len(snapshot) != 0 {
		t.Errorf("snapshot len = %d, want 0", len(snapshot))
	}
}

func TestVoiceRoomPresence_LeaveInexistente(t *testing.T) {
	ps := NewVoiceRoomPresence()
	roomID := uuid.New()

	_, left := ps.Leave(roomID, "sid-inexistente")

	if left {
		t.Fatal("Leave retornou true, esperava false")
	}
}

func TestVoiceRoomPresence_ClearRemoveSalaToda(t *testing.T) {
	ps := NewVoiceRoomPresence()
	roomID := uuid.New()

	ps.Join(roomID, Participant{SID: "sid-1", UserID: "user-1", Username: "Alice"})
	ps.Join(roomID, Participant{SID: "sid-2", UserID: "user-2", Username: "Bob"})
	ps.Clear(roomID)

	snapshot := ps.Snapshot(roomID)

	if len(snapshot) != 0 {
		t.Errorf("snapshot len = %d, want 0", len(snapshot))
	}
}

func TestVoiceRoomPresence_SnapshotJSONComParticipantes(t *testing.T) {
	ps := NewVoiceRoomPresence()
	roomID := uuid.New()

	ps.Join(roomID, Participant{SID: "sid-1", UserID: "user-1", Username: "Alice"})
	data := ps.SnapshotJSON(roomID)

	if data == nil {
		t.Fatal("SnapshotJSON retornou nil")
	}
	var participants []Participant
	if err := json.Unmarshal(data, &participants); err != nil {
		t.Fatalf("json.Unmarshal erro: %v", err)
	}
	if len(participants) != 1 {
		t.Errorf("len = %d, want 1", len(participants))
	}
}

func TestVoiceRoomPresence_SnapshotJSONVazio(t *testing.T) {
	ps := NewVoiceRoomPresence()
	roomID := uuid.New()

	data := ps.SnapshotJSON(roomID)

	if data != nil {
		t.Errorf("SnapshotJSON = %v, want nil", data)
	}
}
