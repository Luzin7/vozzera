package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/Luzin7/vozzera-backend/internal/transport/ws"
	"github.com/google/uuid"
)

type fakeRoomSnapshotProvider struct {
	envelopes []realtime.Envelope
	err       error
	roomIDs   []uuid.UUID
}

func (f *fakeRoomSnapshotProvider) RoomSnapshot(_ context.Context, roomID uuid.UUID) ([]realtime.Envelope, error) {
	f.roomIDs = append(f.roomIDs, roomID)
	return f.envelopes, f.err
}

func subscribeWithSnapshots(t *testing.T, roomID uuid.UUID, providers ...RoomSnapshotProvider) *fakePublisher {
	t.Helper()
	reg := newFakeRegisterer()
	pub := &fakePublisher{}
	r := ws.NewRouter()
	RegisterChatHandlers(r, ChatHandlerDeps{Registerer: reg, Publisher: pub, Authorizer: &fakeAuthorizer{}, RoomSnapshots: providers})
	client := realtime.NewClient(reg, nil, uuid.New(), "alice", uuid.New(), nil)
	data, _ := json.Marshal(map[string]any{"room_id": roomID.String()})

	if err := r.HandleMessage(client, realtime.Envelope{V: 1, Type: CmdSubscribe, Data: data}); err != nil {
		t.Fatalf("HandleMessage erro: %v", err)
	}
	return pub
}

func snapshotEnvelope(roomID uuid.UUID, eventType, data string) realtime.Envelope {
	return realtime.Envelope{V: 1, Type: eventType, Topic: realtime.RoomTopic(roomID), TS: time.Now(), Data: json.RawMessage(data)}
}

func TestSubscribe_PublicaEnvelopesDosProvedoresDeSnapshot(t *testing.T) {
	roomID := uuid.New()
	provider := &fakeRoomSnapshotProvider{envelopes: []realtime.Envelope{
		snapshotEnvelope(roomID, realtime.PresenceSnapshot, "[]"),
		snapshotEnvelope(roomID, "voice.away.snapshot", "[]"),
	}}

	pub := subscribeWithSnapshots(t, roomID, provider)

	if len(provider.roomIDs) != 1 || provider.roomIDs[0] != roomID {
		t.Fatalf("provider chamado com %v, want %s", provider.roomIDs, roomID)
	}
	if len(pub.envelopes) != 2 || pub.envelopes[0].Type != realtime.PresenceSnapshot || pub.envelopes[1].Type != "voice.away.snapshot" {
		t.Fatalf("publicado = %+v, want presença e ausentes", pub.envelopes)
	}
}

func TestSubscribe_ErroNoSnapshotNaoPublicaNadaELoga(t *testing.T) {
	var buf bytes.Buffer
	previous := log.Default().Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(previous)
	roomID := uuid.New()
	provider := &fakeRoomSnapshotProvider{err: errors.New("kv indisponível")}

	pub := subscribeWithSnapshots(t, roomID, provider)

	if len(pub.envelopes) != 0 {
		t.Errorf("publicado = %+v, want nada", pub.envelopes)
	}
	if !strings.Contains(buf.String(), "chat.room_snapshot level=error room_id="+roomID.String()) {
		t.Errorf("log = %q, want chat.room_snapshot level=error", buf.String())
	}
}
