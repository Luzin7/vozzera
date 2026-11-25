package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type recordingVoiceLeaver struct {
	inputs []LeaveVoiceInput
}

func (r *recordingVoiceLeaver) Execute(_ context.Context, in LeaveVoiceInput) error {
	r.inputs = append(r.inputs, in)
	return nil
}

func newLeaveVoiceFixture(state VoiceRoomState) (*LeaveVoiceService, *RecentLeaves, *lockedPublisher) {
	publisher := &lockedPublisher{}
	leaves := NewRecentLeaves()
	return NewLeaveVoiceService(state, publisher, leaves), leaves, publisher
}

func leaveVoiceInput(roomID, userID uuid.UUID, sid string) LeaveVoiceInput {
	return LeaveVoiceInput{UserID: userID, RoomID: roomID, SID: sid}
}

func TestLeaveVoiceService_RemoveSoOProprioParticipanteEPublicaUmaVez(t *testing.T) {
	state := newTestState()
	service, _, publisher := newLeaveVoiceFixture(state)
	roomID, alice, bob := uuid.New(), uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(alice, "sid-a"), at(0))
	mustJoin(t, state, roomID, participantOf(bob, "sid-b"), at(0))

	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, alice, "sid-a")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	assertParticipantSIDs(t, state, roomID, "sid-b")
	want := []string{"voice.presence.left", EventSessionSnapshot}
	if got := publisher.types(); !reflect.DeepEqual(got, want) {
		t.Fatalf("publicado = %v, want %v", got, want)
	}
}

func TestLeaveVoiceService_UsuarioNaoRemoveOutroParticipante(t *testing.T) {
	state := newTestState()
	service, _, publisher := newLeaveVoiceFixture(state)
	roomID := uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-b"), at(0))

	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, uuid.New(), "sid-b")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	assertParticipantSIDs(t, state, roomID, "sid-b")
	if got := publisher.types(); len(got) != 0 {
		t.Errorf("publicado = %v, want nada", got)
	}
}

func TestLeaveVoiceService_WebhookDeSaidaDepoisNaoRepublica(t *testing.T) {
	state := newTestState()
	service, leaves, publisher := newLeaveVoiceFixture(state)
	handler := NewWebhookHandler(webhookKey, webhookSecret, state, publisher, leaves)
	roomID, alice := uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(alice, "sid-a"), at(0))
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-b"), at(0))
	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, alice, "sid-a")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	published := len(publisher.types())
	left := webhookEventSpec{event: "participant_left", eventID: "EV_left", roomID: roomID, userID: alice, sid: "sid-a", createdAt: 1_700_000_100}

	status := serveWebhook(handler, signedWebhookRequest(t, left))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if got := len(publisher.types()); got != published {
		t.Errorf("publicações = %d, want %d", got, published)
	}
	assertParticipantSIDs(t, state, roomID, "sid-b")
}

func TestLeaveVoiceService_ReconciliacaoRecolocaUsuarioAindaConectado(t *testing.T) {
	state := newTestState()
	service, _, publisher := newLeaveVoiceFixture(state)
	roomID, alice := uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(alice, "sid-a"), at(0))
	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, alice, "sid-a")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	assertParticipantSIDs(t, state, roomID)
	lister := listerWithParticipants(roomID, participantOf(alice, "sid-a"))

	mustHydrate(t, newHydrateService(lister, state, publisher))

	assertParticipantSIDs(t, state, roomID, "sid-a")
}

func TestLeaveVoiceService_IgnoraRepeticaoDoMesmoSID(t *testing.T) {
	state := newTestState()
	service, _, _ := newLeaveVoiceFixture(state)
	now := time.Unix(1000, 0)
	service.now = func() time.Time { return now }
	roomID, alice := uuid.New(), uuid.New()
	in := leaveVoiceInput(roomID, alice, "sid-a")
	mustJoin(t, state, roomID, participantOf(alice, "sid-a"), at(0))
	if err := service.Execute(context.Background(), in); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	mustJoin(t, state, roomID, participantOf(alice, "sid-a"), at(0))

	if err := service.Execute(context.Background(), in); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	assertParticipantSIDs(t, state, roomID, "sid-a")

	now = now.Add(leaveVoiceMinInterval)
	if err := service.Execute(context.Background(), in); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	assertParticipantSIDs(t, state, roomID)
}

func TestAwayHandlers_VoiceLeaveUsaIdentidadeDaSessaoEIgnoraUserIDDoPayload(t *testing.T) {
	roomID, spoofed := uuid.New(), uuid.New()
	leaver := &recordingVoiceLeaver{}
	router := newAwayRouterWithVoiceLeaver(leaver)
	client := newTestClient()
	data := fmt.Sprintf(`{"room_id":%q,"sid":"sid-a","user_id":%q}`, roomID, spoofed)

	err := router.Dispatch(client, realtime.Envelope{Type: CmdVoiceLeave, Data: json.RawMessage(data)})

	if err != nil || len(leaver.inputs) != 1 {
		t.Fatalf("err = %v, chamadas = %d, want 1", err, len(leaver.inputs))
	}
	want := LeaveVoiceInput{UserID: client.UserID, RoomID: roomID, SID: "sid-a"}
	if leaver.inputs[0] != want {
		t.Errorf("input = %+v, want %+v", leaver.inputs[0], want)
	}
}

func TestLeaveVoiceService_RemoveSoOSIDNomeadoDoMesmoUsuario(t *testing.T) {
	state := newTestState()
	service, _, _ := newLeaveVoiceFixture(state)
	roomID, alice := uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(alice, "sid-a1"), at(0))
	mustJoin(t, state, roomID, participantOf(alice, "sid-a2"), at(0))

	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, alice, "sid-a1")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	assertParticipantSIDs(t, state, roomID, "sid-a2")
}

func TestLeaveVoiceService_IgnoraSIDDeOutroUsuario(t *testing.T) {
	state := newTestState()
	service, _, publisher := newLeaveVoiceFixture(state)
	roomID := uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-b"), at(0))

	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, uuid.New(), "sid-b")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	assertParticipantSIDs(t, state, roomID, "sid-b")
	if got := publisher.types(); len(got) != 0 {
		t.Errorf("publicado = %v, want nada", got)
	}
}

func TestLeaveVoiceService_SIDAusenteNaoRemoveNinguem(t *testing.T) {
	state := newTestState()
	service, _, _ := newLeaveVoiceFixture(state)
	roomID, alice := uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(alice, "sid-a"), at(0))

	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, alice, "")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	assertParticipantSIDs(t, state, roomID, "sid-a")
}

func TestLeaveVoiceService_SaidaAtrasadaDeSIDAntigoNaoAfetaSIDNovo(t *testing.T) {
	state := newTestState()
	service, _, _ := newLeaveVoiceFixture(state)
	roomID, alice := uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(alice, "sid-new"), at(0))

	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, alice, "sid-old")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	assertParticipantSIDs(t, state, roomID, "sid-new")
}

type failingSnapshotState struct {
	VoiceRoomState
}

func (failingSnapshotState) Snapshot(context.Context, uuid.UUID) (RoomSnapshot, error) {
	return RoomSnapshot{}, errKVDown
}

func TestLeaveVoiceService_FalhaNoSnapshotNaoConsomeJanelaDeRepeticao(t *testing.T) {
	state := newTestState()
	roomID, alice := uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(alice, "sid-a"), at(0))
	service, _, _ := newLeaveVoiceFixture(failingSnapshotState{state})

	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, alice, "sid-a")); err == nil {
		t.Fatal("Execute erro = nil, want erro do estado")
	}
	service.state = state

	if err := service.Execute(context.Background(), leaveVoiceInput(roomID, alice, "sid-a")); err != nil {
		t.Fatalf("Execute erro: %v", err)
	}

	assertParticipantSIDs(t, state, roomID)
}
