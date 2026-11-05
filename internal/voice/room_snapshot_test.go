package voice

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

func TestRoomSnapshotProvider_SalaVaziaPublicaAsListasVaziasESessaoNula(t *testing.T) {
	roomID := uuid.New()

	envelopes, err := NewRoomSnapshotProvider(newTestState()).RoomSnapshot(context.Background(), roomID)

	if err != nil || len(envelopes) != 3 {
		t.Fatalf("err = %v, envelopes = %d, want 3", err, len(envelopes))
	}
	if envelopes[0].Type != realtime.PresenceSnapshot || string(envelopes[0].Data) != "[]" {
		t.Errorf("presença = %s %s, want snapshot []", envelopes[0].Type, envelopes[0].Data)
	}
	if envelopes[1].Type != EventAwaySnapshot || string(envelopes[1].Data) != "[]" {
		t.Errorf("ausentes = %s %s, want snapshot []", envelopes[1].Type, envelopes[1].Data)
	}
	if envelopes[2].Type != EventSessionSnapshot || string(envelopes[2].Data) != `{"started_at":null}` {
		t.Errorf("sessão = %s %s, want snapshot nulo", envelopes[2].Type, envelopes[2].Data)
	}
	for _, envelope := range envelopes {
		if envelope.Topic != realtime.RoomTopic(roomID) {
			t.Errorf("tópico = %s, want o da sala", envelope.Topic)
		}
	}
}

func TestRoomSnapshotProvider_IncluiParticipantesEAusentes(t *testing.T) {
	state := newTestState()
	roomID, other := joinedRoom(t, state)
	away := uuid.New()
	mustEnterAway(t, state, AwayEntry{RoomID: roomID, UserID: away, Username: "bob", Since: at(0), Owner: AwayOwner{InstanceID: uuid.New(), ConnID: uuid.New()}})

	envelopes, err := NewRoomSnapshotProvider(state).RoomSnapshot(context.Background(), roomID)

	if err != nil {
		t.Fatalf("RoomSnapshot erro: %v", err)
	}
	var participants []Participant
	json.Unmarshal(envelopes[0].Data, &participants)
	if len(participants) != 1 || participants[0].UserID != other.String() {
		t.Errorf("participantes = %+v, want o ativo", participants)
	}
	var items []awaySnapshotItem
	json.Unmarshal(envelopes[1].Data, &items)
	if len(items) != 1 || items[0].UserID != away || items[0].Username != "bob" {
		t.Errorf("ausentes = %+v, want bob", items)
	}
}

func TestRoomSnapshotProvider_ErroDoEstadoNaoDevolveEnvelopes(t *testing.T) {
	state := &erroringState{VoiceRoomState: newTestState(), snapshotErr: errKVDown}

	envelopes, err := NewRoomSnapshotProvider(state).RoomSnapshot(context.Background(), uuid.New())

	if err == nil || len(envelopes) != 0 {
		t.Fatalf("err = %v, envelopes = %d, want erro e nenhum envelope", err, len(envelopes))
	}
}

func TestRoomSnapshotProvider_IncluiInicioDaSessao(t *testing.T) {
	state := newTestState()
	roomID := uuid.New()
	joinWithCreation(t, state, roomID, newParticipant(), time.UnixMilli(sessionCreatedMs), at(0))

	envelopes, err := NewRoomSnapshotProvider(state).RoomSnapshot(context.Background(), roomID)

	if err != nil || len(envelopes) != 3 {
		t.Fatalf("err = %v, envelopes = %d, want 3", err, len(envelopes))
	}
	if got := string(envelopes[2].Data); got != `{"started_at":"2023-11-14T22:13:20.123Z"}` {
		t.Errorf("sessão = %s, want o início em UTC", got)
	}
}

type staleSessionState struct {
	VoiceRoomState
	startedAt time.Time
}

func (s staleSessionState) Snapshot(context.Context, uuid.UUID) (RoomSnapshot, error) {
	return RoomSnapshot{SessionStartedAt: &s.startedAt}, nil
}

func TestRoomSnapshotProvider_SessaoSemParticipantesVaiNula(t *testing.T) {
	state := staleSessionState{VoiceRoomState: newTestState(), startedAt: time.UnixMilli(sessionCreatedMs)}

	envelopes, err := NewRoomSnapshotProvider(state).RoomSnapshot(context.Background(), uuid.New())

	if err != nil || len(envelopes) != 3 {
		t.Fatalf("err = %v, envelopes = %d, want 3", err, len(envelopes))
	}
	if got := string(envelopes[2].Data); got != `{"started_at":null}` {
		t.Errorf("sessão = %s, want nula sem participantes", got)
	}
}
