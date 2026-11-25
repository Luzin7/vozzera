package voice

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type fakeRoomLister struct {
	rooms            []LiveKitRoom
	participants     map[string][]Participant
	listRoomsErr     error
	participantErr   map[string]error
	blockUntilDone   bool
	participantBlock bool
	visited          []string
}

func (f *fakeRoomLister) ListRooms(ctx context.Context) ([]LiveKitRoom, error) {
	if f.blockUntilDone {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.rooms, f.listRoomsErr
}

func (f *fakeRoomLister) ListParticipants(ctx context.Context, roomName string) ([]Participant, error) {
	f.visited = append(f.visited, roomName)
	if f.participantBlock {
		<-ctx.Done()
		return nil, ctx.Err()
	}
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
	if !strings.Contains(buf.String(), "voice.hydrate level=info rooms=1 participants=1 empty_pending=0 skipped_budget=0 vanished=0 vanished_pending=0 failed=0 touch_failed=0 invalid_rooms=0 duration_ms=") {
		t.Errorf("log = %q, want resumo do voice.hydrate", buf.String())
	}
}

func TestHydratePresenceService_RepublishesPresenceAndSessionEveryCycle(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID, userID := uuid.New(), uuid.New()
	lister := &fakeRoomLister{
		rooms:        []LiveKitRoom{{Name: roomID.String(), CreatedAt: time.UnixMilli(sessionCreatedMs)}},
		participants: map[string][]Participant{roomID.String(): {participantOf(userID, "sid-a")}},
	}
	service := newHydrateService(lister, state, publisher)

	for cycle := range 2 {
		if err := service.Execute(context.Background()); err != nil {
			t.Fatalf("Execute ciclo %d erro: %v", cycle, err)
		}
	}

	wantTypes := []string{realtime.PresenceSnapshot, EventSessionSnapshot, realtime.PresenceSnapshot, EventSessionSnapshot}
	if got := publisher.types(); !reflect.DeepEqual(got, wantTypes) {
		t.Fatalf("publicado = %v, want %v", got, wantTypes)
	}
	for _, index := range []int{0, 2} {
		got := string(publisher.envelopes[index].Data)
		if !strings.Contains(got, `"sid":"sid-a"`) {
			t.Errorf("presença do ciclo %d = %s, want o participante observado", index/2+1, got)
		}
	}
	for _, index := range []int{1, 3} {
		got := string(publisher.envelopes[index].Data)
		if got != `{"started_at":"2023-11-14T22:13:20.123Z"}` {
			t.Errorf("sessão do ciclo %d = %s, want o horário de criação da sala", index/2+1, got)
		}
	}
}

func TestHydratePresenceService_RepublishesEmptyRoomPresenceAndSession(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID := uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-ghost"), at(0))
	service := newHydrateService(listerWithParticipants(roomID), state, publisher)

	for cycle := range 2 {
		if err := service.Execute(context.Background()); err != nil {
			t.Fatalf("Execute ciclo %d erro: %v", cycle, err)
		}
	}

	wantTypes := []string{realtime.PresenceSnapshot, EventSessionSnapshot, realtime.PresenceSnapshot, EventSessionSnapshot}
	if got := publisher.types(); !reflect.DeepEqual(got, wantTypes) {
		t.Fatalf("publicado = %v, want %v", got, wantTypes)
	}
	for _, index := range []int{0, 2} {
		if got := string(publisher.envelopes[index].Data); got != "[]" {
			t.Errorf("presença do ciclo %d = %s, want []", index/2+1, got)
		}
	}
	for _, index := range []int{1, 3} {
		if got := string(publisher.envelopes[index].Data); got != `{"started_at":null}` {
			t.Errorf("sessão do ciclo %d = %s, want started_at null", index/2+1, got)
		}
	}
}

func TestHydratePresenceService_ResumesAtFirstRoomSkippedByBudget(t *testing.T) {
	first, second, third := uuid.New(), uuid.New(), uuid.New()
	lister := &fakeRoomLister{
		rooms:            []LiveKitRoom{{Name: first.String()}, {Name: second.String()}, {Name: third.String()}},
		participantBlock: true,
	}
	service := newHydrateService(lister, newTestState(), &lockedPublisher{})
	service.timeout = 20 * time.Millisecond
	captureLog(t)

	for cycle := range 3 {
		if err := service.Execute(context.Background()); err != nil {
			t.Fatalf("Execute ciclo %d erro: %v", cycle, err)
		}
	}

	want := []string{first.String(), second.String(), third.String()}
	if got := lister.visited; !reflect.DeepEqual(got, want) {
		t.Fatalf("salas visitadas = %v, want %v", got, want)
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
	if !strings.Contains(buf.String(), "rooms=1 participants=1 empty_pending=0 skipped_budget=0 vanished=0 vanished_pending=0 failed=0 touch_failed=0 invalid_rooms=1") {
		t.Errorf("log = %q, want invalid_rooms=1", buf.String())
	}
}

func TestHydratePresenceService_ReconciliaSalaExistente(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID := uuid.New()
	stale := participantOf(uuid.New(), "sid-webhook")
	mustJoin(t, state, roomID, stale, at(0))
	lister := &fakeRoomLister{
		rooms:        []LiveKitRoom{{Name: roomID.String()}},
		participants: map[string][]Participant{roomID.String(): {participantOf(uuid.New(), "sid-hydrated")}},
	}

	newHydrateService(lister, state, publisher).Execute(context.Background())

	participants := mustSnapshot(t, state, roomID).Participants
	if len(participants) != 1 || participants[0].SID != "sid-hydrated" {
		t.Errorf("participantes = %+v, want só o do LiveKit", participants)
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

func TestHydratePresenceService_FalhaEmUmaSalaContaComoFalhaEContinua(t *testing.T) {
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

	if err == nil {
		t.Fatal("Execute deveria devolver o erro da sala")
	}
	if len(mustSnapshot(t, state, healthy).Participants) != 1 {
		t.Error("sala saudável deveria ter sido hidratada")
	}
	if !strings.Contains(buf.String(), "rooms=1 participants=1 empty_pending=0 skipped_budget=0 vanished=0 vanished_pending=0 failed=1 touch_failed=0 invalid_rooms=0") {
		t.Errorf("log = %q, want failed=1", buf.String())
	}
	if !strings.Contains(buf.String(), "voice.hydrate level=error room_id="+broken.String()) {
		t.Errorf("log = %q, want a sala com falha", buf.String())
	}
}

func TestHydratePresenceService_TouchFailureCountsInSummary(t *testing.T) {
	roomID, userID := uuid.New(), uuid.New()
	lister := &fakeRoomLister{
		rooms:        []LiveKitRoom{{Name: roomID.String(), CreatedAt: time.UnixMilli(sessionCreatedMs)}},
		participants: map[string][]Participant{roomID.String(): {participantOf(userID, "sid-a")}},
	}
	publisher := &lockedPublisher{}
	buf := captureLog(t)
	state := touchFailingState{VoiceRoomState: newTestState()}

	if err := newHydrateService(lister, state, publisher).Execute(context.Background()); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	if !strings.Contains(buf.String(), "voice.hydrate level=warn rooms=1 participants=1 empty_pending=0 skipped_budget=0 vanished=0 vanished_pending=0 failed=0 touch_failed=1 invalid_rooms=0") {
		t.Errorf("log = %q, want touch_failed=1 no resumo em warn", buf.String())
	}
	if !strings.Contains(buf.String(), "voice.room.ttl level=error room_id="+roomID.String()) {
		t.Errorf("log = %q, want o erro do TouchRoom", buf.String())
	}
	if got := publisher.types(); len(got) != 2 {
		t.Errorf("publicado = %v, want presença e sessão mesmo com falha no TTL", got)
	}
}

type touchFailingState struct {
	VoiceRoomState
}

func (s touchFailingState) TouchRoom(context.Context, uuid.UUID) error {
	return errors.New("expire falhou")
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

func TestHydratePresenceService_ContaSalasPuladasPeloOrcamento(t *testing.T) {
	first, second := uuid.New(), uuid.New()
	lister := &fakeRoomLister{
		rooms:            []LiveKitRoom{{Name: first.String()}, {Name: second.String()}},
		participantBlock: true,
	}
	service := newHydrateService(lister, newTestState(), &lockedPublisher{})
	service.timeout = 20 * time.Millisecond
	buf := captureLog(t)

	if err := service.Execute(context.Background()); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	if !strings.Contains(buf.String(), "rooms=0 participants=0 empty_pending=0 skipped_budget=2 vanished=0 vanished_pending=0 failed=0") {
		t.Errorf("log = %q, want skipped_budget=2", buf.String())
	}
}

func TestHydratePresenceService_AnunciaSalaAusenteAposDuasObservacoes(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			publisher := &lockedPublisher{}
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-ghost"), at(0))
			service := newHydrateService(&fakeRoomLister{}, state, publisher)
			buf := captureLog(t)

			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID, "sid-ghost")
			if got := publisher.types(); len(got) != 0 {
				t.Fatalf("publicado = %v, want nada na primeira observação", got)
			}
			if !strings.Contains(buf.String(), "vanished=0 vanished_pending=1") {
				t.Errorf("log = %q, want vanished_pending=1", buf.String())
			}

			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID)
			want := []string{realtime.PresenceSnapshot, EventSessionSnapshot}
			if got := publisher.types(); !reflect.DeepEqual(got, want) {
				t.Fatalf("publicado = %v, want %v", got, want)
			}
			if got := string(publisher.envelopes[0].Data); got != "[]" {
				t.Errorf("presença = %s, want []", got)
			}
			if got := string(publisher.envelopes[1].Data); got != `{"started_at":null}` {
				t.Errorf("sessão = %s, want started_at null", got)
			}
			if !strings.Contains(buf.String(), "vanished=1 vanished_pending=0") {
				t.Errorf("log = %q, want vanished=1", buf.String())
			}
		})
	}
}

func TestHydratePresenceService_NaoAnunciaVaziaQuandoSalaVolta(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			publisher := &lockedPublisher{}
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-keep"), at(0))
			lister := listerWithParticipants(roomID, participantOf(uuid.New(), "sid-keep"))
			lister.rooms = nil
			service := newHydrateService(lister, state, publisher)

			mustHydrate(t, service)

			lister.rooms = []LiveKitRoom{{Name: roomID.String(), CreatedAt: time.UnixMilli(sessionCreatedMs)}}
			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID, "sid-keep")
			want := []string{realtime.PresenceSnapshot, EventSessionSnapshot}
			if got := publisher.types(); !reflect.DeepEqual(got, want) {
				t.Fatalf("publicado = %v, want %v", got, want)
			}
			if got := string(publisher.envelopes[0].Data); !strings.Contains(got, `"sid":"sid-keep"`) {
				t.Errorf("presença = %s, want sid-keep", got)
			}
		})
	}
}

func TestHydratePresenceService_NaoApagaSalaListadaComFalhaNosParticipantes(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-present"), at(0))
			lister := &fakeRoomLister{
				rooms:          []LiveKitRoom{{Name: roomID.String()}},
				participantErr: map[string]error{roomID.String(): errors.New("timeout")},
			}
			service := newHydrateService(lister, state, &lockedPublisher{})
			captureLog(t)
			sawError := false
			for range 3 {
				if err := service.Execute(context.Background()); err != nil {
					sawError = true
				}
			}

			assertParticipantSIDs(t, state, roomID, "sid-present")
			if !sawError {
				t.Fatal("Execute deveria devolver o erro da sala")
			}
		})
	}
}

func TestHydratePresenceService_NaoApagaSalaQuandoListRoomsFalha(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-present"), at(0))
			lister := &fakeRoomLister{listRoomsErr: errors.New("livekit fora")}
			service := newHydrateService(lister, state, &lockedPublisher{})
			buf := captureLog(t)
			sawError := false
			for range 3 {
				if err := service.Execute(context.Background()); err != nil {
					sawError = true
				}
			}

			if !strings.Contains(buf.String(), "voice.hydrate level=error rooms=0 participants=0") {
				t.Fatalf("log = %q, want resumo do ciclo em level=error", buf.String())
			}
			assertParticipantSIDs(t, state, roomID, "sid-present")
			if !sawError {
				t.Fatal("Execute deveria devolver o erro do ListRooms")
			}
		})
	}
}

func TestHydratePresenceService_AnunciaSalaAusenteSemeadaPorHidratacao(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			publisher := &lockedPublisher{}
			roomID := uuid.New()
			lister := listerWithParticipants(roomID, participantOf(uuid.New(), "sid-a"))
			service := newHydrateService(lister, state, publisher)

			mustHydrate(t, service)
			lister.rooms = nil
			mustHydrate(t, service)
			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID)
			if got := publisher.types(); len(got) != 4 {
				t.Fatalf("publicado = %v, want dois anúncios da sala e dois do desaparecimento", got)
			}
			if got := string(publisher.last().Data); got != `{"started_at":null}` {
				t.Errorf("sessão = %s, want started_at null", got)
			}
		})
	}
}

func TestHydratePresenceService_SalaVaziaUmaVezEAusenteUmaVezNaoEVarrida(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-ghost"), at(0))
			lister := listerWithParticipants(roomID)
			service := newHydrateService(lister, state, &lockedPublisher{})

			mustHydrate(t, service)

			lister.rooms = nil
			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID, "sid-ghost")

			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID)
		})
	}
}

func TestHydratePresenceService_ObservacaoAusenteNaoConfirmaHidratacaoVazia(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-ghost"), at(0))
			lister := listerWithParticipants(roomID)
			service := newHydrateService(lister, state, &lockedPublisher{})

			lister.rooms = nil
			mustHydrate(t, service)

			lister.rooms = []LiveKitRoom{{Name: roomID.String(), CreatedAt: time.UnixMilli(sessionCreatedMs)}}
			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID, "sid-ghost")

			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID)
		})
	}
}

type finishFailingState struct {
	VoiceRoomState
	failures int
}

func (s *finishFailingState) Finish(ctx context.Context, roomID uuid.UUID) ([]AwayEntry, error) {
	if s.failures > 0 {
		s.failures--
		return nil, errors.New("finish falhou")
	}
	return s.VoiceRoomState.Finish(ctx, roomID)
}

type ctxCheckingState struct {
	VoiceRoomState
}

func (s *ctxCheckingState) Finish(ctx context.Context, roomID uuid.UUID) ([]AwayEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.VoiceRoomState.Finish(ctx, roomID)
}

type delayingPublisher struct {
	*lockedPublisher
	delay time.Duration
}

func (p *delayingPublisher) Publish(ctx context.Context, topic realtime.Topic, env realtime.Envelope) error {
	if env.Type == realtime.PresenceSnapshot {
		time.Sleep(p.delay)
	}
	return p.lockedPublisher.Publish(ctx, topic, env)
}

func TestHydratePresenceService_VarreSalaAusenteComContextoProprioNoFinish(t *testing.T) {
	state := newTestState()
	roomID := uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-ghost"), at(0))
	checked := &ctxCheckingState{VoiceRoomState: state}
	publisher := &delayingPublisher{lockedPublisher: &lockedPublisher{}, delay: kvOperationTimeout + 100*time.Millisecond}
	service := newHydrateService(&fakeRoomLister{}, checked, publisher)

	if err := service.sweepRoom(context.Background(), roomID); err != nil {
		t.Fatalf("sweepRoom err = %v, want Finish com contexto vivo após publicação lenta", err)
	}

	assertParticipantSIDs(t, state, roomID)
}

func TestHydratePresenceService_AnunciaAntesDeApagarSalaAusente(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			publisher := &lockedPublisher{}
			roomID, awayUser := uuid.New(), uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-active"), at(0))
			mustJoin(t, state, roomID, participantOf(awayUser, "sid-away"), at(0))
			mustEnterAway(t, state, newAwayEntry(roomID, forUser(awayUser)))
			failing := &finishFailingState{VoiceRoomState: state, failures: 1}
			service := newHydrateService(&fakeRoomLister{}, failing, publisher)

			mustHydrate(t, service)

			buf := captureLog(t)
			if err := service.Execute(context.Background()); err == nil {
				t.Fatal("Execute deveria devolver o erro do Finish")
			}
			if !strings.Contains(buf.String(), "vanished=0 vanished_pending=0 failed=1") {
				t.Fatalf("log = %q, want falha da varredura contada no resumo", buf.String())
			}
			if !strings.Contains(buf.String(), "voice.hydrate level=error rooms=0") {
				t.Fatalf("log = %q, want resumo do ciclo em level=error", buf.String())
			}
			want := []string{EventAwaySnapshot, EventAwayEnded, realtime.PresenceSnapshot, EventSessionSnapshot}
			if got := publisher.types(); !reflect.DeepEqual(got, want) {
				t.Fatalf("publicado = %v, want anúncio antes do Finish: %v", got, want)
			}
			assertParticipantSIDs(t, state, roomID, "sid-active", "sid-away")

			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID)
			repeated := []string{
				EventAwaySnapshot, EventAwayEnded, realtime.PresenceSnapshot, EventSessionSnapshot,
				EventAwaySnapshot, EventAwayEnded, realtime.PresenceSnapshot, EventSessionSnapshot,
			}
			if got := publisher.types(); !reflect.DeepEqual(got, repeated) {
				t.Fatalf("publicado = %v, want anúncio repetido no ciclo seguinte à falha: %v", got, repeated)
			}
		})
	}
}

func TestHydratePresenceService_AnunciaAusenteLimpoAoVarrerSalaAusente(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			publisher := &lockedPublisher{}
			roomID, awayUser := uuid.New(), uuid.New()
			mustJoin(t, state, roomID, participantOf(awayUser, "sid-away"), at(0))
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-active"), at(0))
			mustEnterAway(t, state, newAwayEntry(roomID, forUser(awayUser)))
			service := newHydrateService(&fakeRoomLister{}, state, publisher)
			buf := captureLog(t)

			mustHydrate(t, service)
			mustHydrate(t, service)

			snapshot := mustSnapshot(t, state, roomID)
			if len(snapshot.Participants) != 0 || len(snapshot.Away) != 0 || snapshot.SessionStartedAt != nil {
				t.Fatalf("snapshot = %+v, want tudo limpo", snapshot)
			}
			want := []string{EventAwaySnapshot, EventAwayEnded, realtime.PresenceSnapshot, EventSessionSnapshot}
			if got := publisher.types(); !reflect.DeepEqual(got, want) {
				t.Fatalf("publicado = %v, want %v", got, want)
			}
			var payload awayEndedPayload
			if err := json.Unmarshal(publisher.envelopes[1].Data, &payload); err != nil {
				t.Fatalf("payload erro: %v", err)
			}
			if payload.UserID != awayUser || payload.Reason != AwayEndedRoomEmpty {
				t.Errorf("away ended = %+v, want user=%s reason=%s", payload, awayUser, AwayEndedRoomEmpty)
			}
			if !strings.Contains(buf.String(), "voice.away.cleared user_id="+awayUser.String()+" room_id="+roomID.String()+" cause=room_gone") {
				t.Errorf("log = %q, want cause=room_gone", buf.String())
			}
		})
	}
}

func TestHydratePresenceService_RecheckPendingLimpaSalaVaziaSemWebhookDeSaida(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-ghost"), at(0))
			service := newHydrateService(listerWithParticipants(roomID), state, &lockedPublisher{})

			mustHydrate(t, service)
			assertParticipantSIDs(t, state, roomID, "sid-ghost")

			if err := service.RecheckPending(context.Background()); err != nil {
				t.Fatalf("RecheckPending erro: %v", err)
			}

			assertParticipantSIDs(t, state, roomID)
		})
	}
}

func TestHydratePresenceService_RecheckPendingNaoLimpaSobListagemVaziaTransitoria(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-keep"), at(0))
			lister := listerWithParticipants(roomID)
			service := newHydrateService(lister, state, &lockedPublisher{})

			mustHydrate(t, service)
			lister.participants[roomID.String()] = []Participant{participantOf(uuid.New(), "sid-keep")}
			if err := service.RecheckPending(context.Background()); err != nil {
				t.Fatalf("RecheckPending erro: %v", err)
			}
			lister.participants[roomID.String()] = nil
			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID, "sid-keep")
		})
	}
}

func TestHydratePresenceService_RecheckPendingNaoConsultaSalasNaoPendentes(t *testing.T) {
	state := newTestState()
	roomID := uuid.New()
	lister := listerWithParticipants(roomID, participantOf(uuid.New(), "sid-live"))
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-live"), at(0))
	service := newHydrateService(lister, state, &lockedPublisher{})

	mustHydrate(t, service)
	lister.visited = nil
	if err := service.RecheckPending(context.Background()); err != nil {
		t.Fatalf("RecheckPending erro: %v", err)
	}

	if len(lister.visited) != 0 {
		t.Errorf("chamadas ao LiveKit = %v, want nenhuma", lister.visited)
	}
}
