package voice

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type reconcileHarness struct {
	name  string
	state func(t *testing.T) VoiceRoomState
}

func reconcileHarnesses() []reconcileHarness {
	return []reconcileHarness{
		{name: "memoria", state: func(*testing.T) VoiceRoomState { return newTestState() }},
		{
			name: "kv",
			state: func(t *testing.T) VoiceRoomState {
				client := newKVTestClient(t)
				prefix := "vz:test-" + uuid.NewString()
				t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })
				return NewKVVoiceRoomState(client, prefix)
			},
		},
	}
}

func listerWithParticipants(roomID uuid.UUID, participants ...Participant) *fakeRoomLister {
	return &fakeRoomLister{
		rooms:        []LiveKitRoom{{Name: roomID.String(), CreatedAt: time.UnixMilli(sessionCreatedMs)}},
		participants: map[string][]Participant{roomID.String(): participants},
	}
}

func assertParticipantSIDs(t *testing.T, state VoiceRoomState, roomID uuid.UUID, want ...string) {
	t.Helper()

	participants := mustSnapshot(t, state, roomID).Participants
	got := make([]string, 0, len(participants))
	for _, p := range participants {
		got = append(got, p.SID)
	}
	if len(got) != len(want) {
		t.Fatalf("sids = %v, want %v", got, want)
	}
	for _, sid := range want {
		if !containsString(got, sid) {
			t.Fatalf("sids = %v, want %v", got, want)
		}
	}
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

type joinBeforeHydrateState struct {
	VoiceRoomState
	join JoinInput
	once sync.Once
}

func (s *joinBeforeHydrateState) HydrateRoom(ctx context.Context, in RoomHydration) (HydrateResult, error) {
	s.once.Do(func() { s.VoiceRoomState.Join(ctx, s.join) })
	return s.VoiceRoomState.HydrateRoom(ctx, in)
}

func TestHydratePresenceService_NaoApagaJoinNaoObservado(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			base := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, base, roomID, participantOf(uuid.New(), "sid-stale"), at(0))
			joined := participantOf(uuid.New(), "sid-joined")
			state := &joinBeforeHydrateState{
				VoiceRoomState: base,
				join:           JoinInput{RoomID: roomID, Participant: joined, At: at(0)},
			}
			keep := participantOf(uuid.New(), "sid-keep")
			lister := listerWithParticipants(roomID, keep)

			err := newHydrateService(lister, state, &lockedPublisher{}).Execute(context.Background())

			if err == nil {
				t.Fatal("Execute deveria sinalizar conflito para repetir")
			}
			assertParticipantSIDs(t, base, roomID, "sid-joined", "sid-stale")

			lister.participants[roomID.String()] = []Participant{keep, joined}
			if err := newHydrateService(lister, base, &lockedPublisher{}).Execute(context.Background()); err != nil {
				t.Fatalf("segundo Execute erro: %v", err)
			}
			assertParticipantSIDs(t, base, roomID, "sid-keep", "sid-joined")
		})
	}
}

func TestHydratePresenceService_ReconciliaPresencaComLiveKit(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			t.Run("remove presença obsoleta e mantém a verdadeira", func(t *testing.T) {
				state := harness.state(t)
				publisher := &lockedPublisher{}
				roomID := uuid.New()
				keepUser, staleUser, newUser := uuid.New(), uuid.New(), uuid.New()
				mustJoin(t, state, roomID, participantOf(keepUser, "sid-keep"), at(0))
				mustJoin(t, state, roomID, participantOf(staleUser, "sid-stale"), at(0))
				lister := listerWithParticipants(roomID, participantOf(keepUser, "sid-keep"), participantOf(newUser, "sid-new"))

				err := newHydrateService(lister, state, publisher).Execute(context.Background())

				if err != nil {
					t.Fatalf("Execute erro: %v", err)
				}
				assertParticipantSIDs(t, state, roomID, "sid-keep", "sid-new")
			})

			t.Run("preenche presença vazia", func(t *testing.T) {
				state := harness.state(t)
				roomID := uuid.New()
				lister := listerWithParticipants(roomID, participantOf(uuid.New(), "sid-a"), participantOf(uuid.New(), "sid-b"))

				err := newHydrateService(lister, state, &lockedPublisher{}).Execute(context.Background())

				if err != nil {
					t.Fatalf("Execute erro: %v", err)
				}
				assertParticipantSIDs(t, state, roomID, "sid-a", "sid-b")
			})

			t.Run("repetir produz o mesmo estado", func(t *testing.T) {
				state := harness.state(t)
				roomID := uuid.New()
				mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-stale"), at(0))
				lister := listerWithParticipants(roomID, participantOf(uuid.New(), "sid-a"), participantOf(uuid.New(), "sid-b"))
				service := newHydrateService(lister, state, &lockedPublisher{})

				if err := service.Execute(context.Background()); err != nil {
					t.Fatalf("primeiro Execute erro: %v", err)
				}
				first := mustSnapshot(t, state, roomID).Participants
				if err := service.Execute(context.Background()); err != nil {
					t.Fatalf("segundo Execute erro: %v", err)
				}
				second := mustSnapshot(t, state, roomID).Participants

				if !reflect.DeepEqual(first, second) {
					t.Fatalf("primeiro = %+v, segundo = %+v, want iguais", first, second)
				}
			})

			t.Run("erro do LiveKit mantém a presença e loga", func(t *testing.T) {
				state := harness.state(t)
				roomID := uuid.New()
				mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-stale"), at(0))
				buf := captureLog(t)

				err := newHydrateService(&fakeRoomLister{listRoomsErr: errors.New("livekit fora")}, state, &lockedPublisher{}).Execute(context.Background())

				if err == nil {
					t.Fatal("Execute deveria devolver o erro")
				}
				assertParticipantSIDs(t, state, roomID, "sid-stale")
				if !strings.Contains(buf.String(), "voice.hydrate level=error step=list_rooms") {
					t.Errorf("log = %q, want erro de ListRooms", buf.String())
				}
			})

			t.Run("erro ao listar participantes mantém a presença e loga a sala", func(t *testing.T) {
				state := harness.state(t)
				roomID := uuid.New()
				mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-stale"), at(0))
				lister := listerWithParticipants(roomID)
				lister.participantErr = map[string]error{roomID.String(): errors.New("timeout")}
				buf := captureLog(t)

				err := newHydrateService(lister, state, &lockedPublisher{}).Execute(context.Background())

				if err == nil {
					t.Fatal("Execute deveria devolver o erro")
				}
				assertParticipantSIDs(t, state, roomID, "sid-stale")
				if !strings.Contains(buf.String(), "voice.hydrate level=error room_id="+roomID.String()) {
					t.Errorf("log = %q, want a sala e a causa", buf.String())
				}
			})
		})
	}
}

func TestHydratePresenceService_ResolveAusenteAoEsvaziarSala(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			publisher := &lockedPublisher{}
			roomID, awayUser := uuid.New(), uuid.New()
			mustJoin(t, state, roomID, participantOf(awayUser, "sid-away"), at(0))
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-active"), at(0))
			mustEnterAway(t, state, newAwayEntry(roomID, forUser(awayUser)))
			service := newHydrateService(listerWithParticipants(roomID), state, publisher)

			if err := service.Execute(context.Background()); err != nil {
				t.Fatalf("primeiro Execute erro: %v", err)
			}
			if err := service.Execute(context.Background()); err != nil {
				t.Fatalf("segundo Execute erro: %v", err)
			}

			snapshot := mustSnapshot(t, state, roomID)
			if len(snapshot.Away) != 0 {
				t.Fatalf("away = %v, want vazio", awayUserIDs(snapshot))
			}
			if snapshot.SessionStartedAt != nil {
				t.Fatalf("sessão = %v, want nil", snapshot.SessionStartedAt)
			}
			stale, err := state.LeaveAway(context.Background(), awayUser)
			if err != nil {
				t.Fatalf("LeaveAway erro: %v", err)
			}
			if stale != nil {
				t.Fatalf("LeaveAway devolveu %+v, want nil", stale)
			}
			assertAwayEnded(t, publisher, awayUser)
		})
	}
}

func mustHydrate(t *testing.T, service *HydratePresenceService) {
	t.Helper()
	if err := service.Execute(context.Background()); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
}

func TestHydratePresenceService_MantemSalaVaziaAteASegundaObservacao(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			publisher := &lockedPublisher{}
			roomID, awayUser := uuid.New(), uuid.New()
			mustJoin(t, state, roomID, participantOf(awayUser, "sid-away"), at(0))
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-active"), at(0))
			mustEnterAway(t, state, newAwayEntry(roomID, forUser(awayUser)))
			service := newHydrateService(listerWithParticipants(roomID), state, publisher)
			buf := captureLog(t)

			mustHydrate(t, service)

			snapshot := mustSnapshot(t, state, roomID)
			if len(snapshot.Participants) != 2 || len(snapshot.Away) != 1 || snapshot.SessionStartedAt == nil {
				t.Fatalf("snapshot = %+v, want presença, sessão e ausente preservados", snapshot)
			}
			if got := publisher.types(); len(got) != 0 {
				t.Errorf("publicado = %v, want nada na primeira observação", got)
			}
			if !strings.Contains(buf.String(), "empty_pending=1") {
				t.Errorf("log = %q, want empty_pending=1", buf.String())
			}

			mustHydrate(t, service)

			snapshot = mustSnapshot(t, state, roomID)
			if len(snapshot.Participants) != 0 || len(snapshot.Away) != 0 || snapshot.SessionStartedAt != nil {
				t.Fatalf("snapshot = %+v, want tudo limpo", snapshot)
			}
			assertAwayEnded(t, publisher, awayUser)
		})
	}
}

func TestHydratePresenceService_NaoVazioResetaContagemDeSalaVazia(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-stale"), at(0))
			keep := participantOf(uuid.New(), "sid-keep")
			lister := listerWithParticipants(roomID, keep)
			service := newHydrateService(lister, state, &lockedPublisher{})

			lister.participants[roomID.String()] = nil
			mustHydrate(t, service)

			lister.participants[roomID.String()] = []Participant{keep}
			mustHydrate(t, service)

			lister.participants[roomID.String()] = nil
			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID, "sid-keep")
		})
	}
}

func TestHydratePresenceService_ConflitoNaoContaComoObservacaoVazia(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			base := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, base, roomID, participantOf(uuid.New(), "sid-present"), at(0))
			state := &joinBeforeHydrateState{
				VoiceRoomState: base,
				join:           JoinInput{RoomID: roomID, Participant: participantOf(uuid.New(), "sid-late"), At: at(0)},
			}
			service := newHydrateService(listerWithParticipants(roomID), state, &lockedPublisher{})

			if err := service.Execute(context.Background()); err == nil {
				t.Fatal("Execute deveria sinalizar conflito")
			}
			assertParticipantSIDs(t, base, roomID, "sid-present", "sid-late")

			mustHydrate(t, service)

			assertParticipantSIDs(t, base, roomID, "sid-present", "sid-late")
		})
	}
}

func TestHydratePresenceService_DescartaContagemDeSalaForaDaReconciliacao(t *testing.T) {
	for _, harness := range reconcileHarnesses() {
		t.Run(harness.name, func(t *testing.T) {
			state := harness.state(t)
			roomID := uuid.New()
			mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-present"), at(0))
			lister := listerWithParticipants(roomID)
			lister.participants[roomID.String()] = nil
			service := newHydrateService(lister, state, &lockedPublisher{})

			mustHydrate(t, service)

			lister.rooms = nil
			mustHydrate(t, service)

			lister.rooms = []LiveKitRoom{{Name: roomID.String(), CreatedAt: time.UnixMilli(sessionCreatedMs)}}
			mustHydrate(t, service)

			assertParticipantSIDs(t, state, roomID, "sid-present")
		})
	}
}

func assertAwayEnded(t *testing.T, publisher *lockedPublisher, userID uuid.UUID) {
	t.Helper()

	want := []string{EventAwaySnapshot, EventAwayEnded, realtime.PresenceSnapshot, EventSessionSnapshot}
	if got := publisher.types(); !reflect.DeepEqual(got, want) {
		t.Fatalf("publicado = %v, want %v", got, want)
	}
	if got := string(publisher.envelopes[0].Data); got != "[]" {
		t.Errorf("away snapshot = %s, want []", got)
	}
	var payload awayEndedPayload
	if err := json.Unmarshal(publisher.envelopes[1].Data, &payload); err != nil {
		t.Fatalf("payload erro: %v", err)
	}
	if payload.UserID != userID || payload.Reason != AwayEndedRoomEmpty {
		t.Errorf("away ended = %+v, want user=%s reason=%s", payload, userID, AwayEndedRoomEmpty)
	}
	if got := string(publisher.envelopes[3].Data); got != `{"started_at":null}` {
		t.Errorf("sessão = %s, want started_at nil", got)
	}
}
