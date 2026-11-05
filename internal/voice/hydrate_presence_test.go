package voice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type fakeRoomLister struct {
	rooms          []LiveKitRoom
	participants   map[string][]Participant
	listRoomsErr   error
	participantErr map[string]error
	blockUntilDone bool
}

func (f *fakeRoomLister) ListRooms(ctx context.Context) ([]LiveKitRoom, error) {
	if f.blockUntilDone {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.rooms, f.listRoomsErr
}

func (f *fakeRoomLister) ListParticipants(_ context.Context, roomName string) ([]Participant, error) {
	return f.participants[roomName], f.participantErr[roomName]
}

func newHydrateService(lister RoomLister, state VoiceRoomState, publisher realtime.Publisher) *HydratePresenceService {
	return NewHydratePresenceService(lister, state, publisher)
}

func TestHydratePresenceService_PreencheSalasEPublicaSnapshot(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	buf := captureLog(t)
	roomID, userID := uuid.New(), uuid.New()
	lister := &fakeRoomLister{
		rooms:        []LiveKitRoom{{Name: roomID.String(), CreatedAt: time.UnixMilli(sessionCreatedMs)}},
		participants: map[string][]Participant{roomID.String(): {participantOf(userID, "sid-a")}},
	}

	err := newHydrateService(lister, state, publisher).Execute(context.Background())

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	if got := mustSnapshot(t, state, roomID).Participants; len(got) != 1 || got[0].SID != "sid-a" {
		t.Errorf("participantes = %+v, want sid-a", got)
	}
	if got := publisher.types(); len(got) != 2 || got[0] != realtime.PresenceSnapshot || got[1] != EventSessionSnapshot {
		t.Errorf("publicado = %v, want snapshot de presença e de sessão", got)
	}
	if got := string(publisher.last().Data); got != `{"started_at":"2023-11-14T22:13:20.123Z"}` {
		t.Errorf("sessão = %s, want o horário de criação da sala", got)
	}
	if !strings.Contains(buf.String(), "voice.hydrate rooms=1 participants=1 skipped_rooms=0 duration_ms=") {
		t.Errorf("log = %q, want resumo do voice.hydrate", buf.String())
	}
}

func TestHydratePresenceService_IgnoraSalasENomesQueNaoSaoUUID(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID, userID := uuid.New(), uuid.New()
	lister := &fakeRoomLister{
		rooms: []LiveKitRoom{{Name: "sala-legada"}, {Name: roomID.String()}},
		participants: map[string][]Participant{
			roomID.String(): {participantOf(userID, "sid-a"), {SID: "sid-bot", UserID: "agent-123", Username: "bot"}},
		},
	}
	buf := captureLog(t)

	err := newHydrateService(lister, state, publisher).Execute(context.Background())

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	if got := mustSnapshot(t, state, roomID).Participants; len(got) != 1 || got[0].SID != "sid-a" {
		t.Errorf("participantes = %+v, want só identidades UUID", got)
	}
	if !strings.Contains(buf.String(), "rooms=1 participants=1 skipped_rooms=1") {
		t.Errorf("log = %q, want skipped_rooms=1", buf.String())
	}
}

func TestHydratePresenceService_NaoSobrescreveSalaExistente(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID := uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-webhook"), at(0))
	lister := &fakeRoomLister{
		rooms:        []LiveKitRoom{{Name: roomID.String()}},
		participants: map[string][]Participant{roomID.String(): {participantOf(uuid.New(), "sid-hydrated")}},
	}

	newHydrateService(lister, state, publisher).Execute(context.Background())

	if got := mustSnapshot(t, state, roomID).Participants; len(got) != 1 || got[0].SID != "sid-webhook" {
		t.Errorf("participantes = %+v, want só o do webhook", got)
	}
}

func TestHydratePresenceService_FalhaDoLiveKitDevolveErroENaoPublica(t *testing.T) {
	publisher := &lockedPublisher{}
	lister := &fakeRoomLister{listRoomsErr: errors.New("livekit fora")}

	err := newHydrateService(lister, newTestState(), publisher).Execute(context.Background())

	if err == nil {
		t.Fatal("Execute deveria devolver o erro")
	}
	if got := publisher.types(); len(got) != 0 {
		t.Errorf("publicado = %v, want nada", got)
	}
}

func TestHydratePresenceService_FalhaEmUmaSalaContaComoPuladaEContinua(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	broken, healthy := uuid.New(), uuid.New()
	lister := &fakeRoomLister{
		rooms:          []LiveKitRoom{{Name: broken.String()}, {Name: healthy.String()}},
		participants:   map[string][]Participant{healthy.String(): {participantOf(uuid.New(), "sid-a")}},
		participantErr: map[string]error{broken.String(): errors.New("timeout")},
	}
	buf := captureLog(t)

	err := newHydrateService(lister, state, publisher).Execute(context.Background())

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	if len(mustSnapshot(t, state, healthy).Participants) != 1 {
		t.Error("sala saudável deveria ter sido hidratada")
	}
	if !strings.Contains(buf.String(), "rooms=1 participants=1 skipped_rooms=1") {
		t.Errorf("log = %q, want skipped_rooms=1", buf.String())
	}
}

func TestHydratePresenceService_RespeitaOTetoDeCincoSegundos(t *testing.T) {
	captureLog(t)
	service := newHydrateService(&fakeRoomLister{blockUntilDone: true}, newTestState(), &lockedPublisher{})
	service.timeout = 20 * time.Millisecond
	start := time.Now()

	err := service.Execute(context.Background())

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("Execute demorou %v", time.Since(start))
	}
}

func TestHydratePresenceService_SalaSemCriacaoLogaAviso(t *testing.T) {
	buf := captureLog(t)
	roomID := uuid.New()
	lister := &fakeRoomLister{
		rooms:        []LiveKitRoom{{Name: roomID.String()}},
		participants: map[string][]Participant{roomID.String(): {participantOf(uuid.New(), "sid-a")}},
	}

	newHydrateService(lister, newTestState(), &lockedPublisher{}).Execute(context.Background())

	want := "voice.session level=warn cause=missing_room_creation_time step=hydrate room_id=" + roomID.String()
	if !strings.Contains(buf.String(), want) {
		t.Errorf("log = %q, want %q", buf.String(), want)
	}
}
