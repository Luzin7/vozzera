package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

var errKVDown = errors.New("kv indisponível")

type erroringState struct {
	VoiceRoomState
	enterErr    error
	leaveAway   error
	releaseErr  error
	snapshotErr error
	sweepErr    error
	webhookErr  error
}

func (s *erroringState) EnterAway(ctx context.Context, entry AwayEntry) (EnterAwayResult, error) {
	if s.enterErr != nil {
		return EnterAwayResult{}, s.enterErr
	}
	return s.VoiceRoomState.EnterAway(ctx, entry)
}

func (s *erroringState) LeaveAway(ctx context.Context, userID uuid.UUID) (*AwayEntry, error) {
	if s.leaveAway != nil {
		return nil, s.leaveAway
	}
	return s.VoiceRoomState.LeaveAway(ctx, userID)
}

func (s *erroringState) ReleaseAway(ctx context.Context, userID uuid.UUID, owner AwayOwner) (*AwayEntry, error) {
	if s.releaseErr != nil {
		return nil, s.releaseErr
	}
	return s.VoiceRoomState.ReleaseAway(ctx, userID, owner)
}

func (s *erroringState) Snapshot(ctx context.Context, roomID uuid.UUID) (RoomSnapshot, error) {
	if s.snapshotErr != nil {
		return RoomSnapshot{}, s.snapshotErr
	}
	return s.VoiceRoomState.Snapshot(ctx, roomID)
}

func (s *erroringState) SweepDeadOwners(ctx context.Context) ([]AwayEntry, error) {
	if s.sweepErr != nil {
		return nil, s.sweepErr
	}
	return s.VoiceRoomState.SweepDeadOwners(ctx)
}

type fakeRoomAuthorizer struct {
	err error
}

func (f fakeRoomAuthorizer) CanSubscribe(context.Context, string, realtime.Topic) error {
	return f.err
}

type lockedPublisher struct {
	mu        sync.Mutex
	envelopes []realtime.Envelope
	err       error
}

func (p *lockedPublisher) Publish(_ context.Context, _ realtime.Topic, env realtime.Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.envelopes = append(p.envelopes, env)
	return p.err
}

func (p *lockedPublisher) types() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	types := make([]string, 0, len(p.envelopes))
	for _, env := range p.envelopes {
		types = append(types, env.Type)
	}
	return types
}

func (p *lockedPublisher) last() realtime.Envelope {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.envelopes[len(p.envelopes)-1]
}

func newTestState() *memoryVoiceRoomState {
	return newMemoryVoiceRoomState((&fakeClock{now: contractEpoch}).Now)
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := log.Default().Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buf
}

func forbiddenError() error {
	return httpx.Errorf(http.StatusForbidden, "negado")
}

func joinedRoom(t *testing.T, state VoiceRoomState) (roomID, other uuid.UUID) {
	t.Helper()
	roomID, other = uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(other, "sid-other"), at(0))
	return roomID, other
}

func enterInput(roomID uuid.UUID) EnterAwayInput {
	return EnterAwayInput{
		UserID:   uuid.New(),
		Username: "alice",
		RoomID:   roomID,
		Reason:   "idle",
		Owner:    AwayOwner{InstanceID: uuid.New(), ConnID: uuid.New()},
		ConnCtx:  context.Background(),
	}
}

func TestEnterAwayService_AceitoPublicaSnapshotDepoisDaEscrita(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID, _ := joinedRoom(t, state)
	in := enterInput(roomID)

	err := NewEnterAwayService(state, fakeRoomAuthorizer{}, publisher).Execute(context.Background(), in)

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	if got := publisher.types(); len(got) != 1 || got[0] != EventAwaySnapshot {
		t.Fatalf("publicado = %v, want [%s]", got, EventAwaySnapshot)
	}
	var items []awaySnapshotItem
	json.Unmarshal(publisher.last().Data, &items)
	if len(items) != 1 || items[0].UserID != in.UserID || items[0].Username != "alice" {
		t.Errorf("snapshot = %+v, want o usuário do input", items)
	}
}

func TestEnterAwayService_RejeitadoPublicaEndedRoomEmpty(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	in := enterInput(uuid.New())

	err := NewEnterAwayService(state, fakeRoomAuthorizer{}, publisher).Execute(context.Background(), in)

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	if got := publisher.types(); len(got) != 1 || got[0] != EventAwayEnded {
		t.Fatalf("publicado = %v, want [%s]", got, EventAwayEnded)
	}
	var payload awayEndedPayload
	json.Unmarshal(publisher.last().Data, &payload)
	if payload.UserID != in.UserID || payload.Reason != AwayEndedRoomEmpty {
		t.Errorf("payload = %+v, want room_empty do usuário", payload)
	}
}

func TestEnterAwayService_MotivoInvalido(t *testing.T) {
	publisher := &lockedPublisher{}
	in := enterInput(uuid.New())
	in.Reason = "foo"

	err := NewEnterAwayService(newTestState(), fakeRoomAuthorizer{}, publisher).Execute(context.Background(), in)

	if !errors.Is(err, ErrInvalidAwayReason) {
		t.Fatalf("err = %v, want ErrInvalidAwayReason", err)
	}
}

func TestEnterAwayService_SalaNula(t *testing.T) {
	err := NewEnterAwayService(newTestState(), fakeRoomAuthorizer{}, &lockedPublisher{}).Execute(context.Background(), enterInput(uuid.Nil))

	if !errors.Is(err, ErrInvalidAwayRoom) {
		t.Fatalf("err = %v, want ErrInvalidAwayRoom", err)
	}
}

func TestEnterAwayService_SalaNaoAutorizadaNaoEscreveNemPublica(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID, _ := joinedRoom(t, state)
	in := enterInput(roomID)

	err := NewEnterAwayService(state, fakeRoomAuthorizer{err: forbiddenError()}, publisher).Execute(context.Background(), in)

	if !errors.Is(err, ErrAwayRoomDenied) {
		t.Fatalf("err = %v, want ErrAwayRoomDenied", err)
	}
	assertAwayUsers(t, mustSnapshot(t, state, roomID))
	if got := publisher.types(); len(got) != 0 {
		t.Errorf("publicado = %v, want nada", got)
	}
}

func TestEnterAwayService_FalhaDeAutorizacaoNaoVirNegado(t *testing.T) {
	in := enterInput(uuid.New())

	err := NewEnterAwayService(newTestState(), fakeRoomAuthorizer{err: errKVDown}, &lockedPublisher{}).Execute(context.Background(), in)

	if err == nil || errors.Is(err, ErrAwayRoomDenied) {
		t.Fatalf("err = %v, want erro que não seja negado", err)
	}
}

func TestEnterAwayService_ErroDoEstadoNaoPublicaNada(t *testing.T) {
	publisher := &lockedPublisher{}
	state := &erroringState{VoiceRoomState: newTestState(), enterErr: errKVDown}

	err := NewEnterAwayService(state, fakeRoomAuthorizer{}, publisher).Execute(context.Background(), enterInput(uuid.New()))

	if !errors.Is(err, errKVDown) {
		t.Fatalf("err = %v, want erro do estado embrulhado", err)
	}
	var httpErr *httpx.Error
	if !errors.As(err, &httpErr) || httpErr.Status != http.StatusServiceUnavailable {
		t.Errorf("err = %v, want 503", err)
	}
	if got := publisher.types(); len(got) != 0 {
		t.Errorf("publicado = %v, want nada", got)
	}
}

func TestEnterAwayService_ConexaoJaFechadaLiberaEntradaSemPublicar(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID, _ := joinedRoom(t, state)
	in := enterInput(roomID)
	closed, cancel := context.WithCancel(context.Background())
	cancel()
	in.ConnCtx = closed

	err := NewEnterAwayService(state, fakeRoomAuthorizer{}, publisher).Execute(context.Background(), in)

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	assertAwayUsers(t, mustSnapshot(t, state, roomID))
	if got := publisher.types(); len(got) != 0 {
		t.Errorf("publicado = %v, want nada", got)
	}
}

func TestEnterAwayService_MovimentoPublicaSnapshotDasDuasSalas(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	first, _ := joinedRoom(t, state)
	second, _ := joinedRoom(t, state)
	service := NewEnterAwayService(state, fakeRoomAuthorizer{}, publisher)
	in := enterInput(first)
	service.Execute(context.Background(), in)
	publisher.envelopes = nil
	in.RoomID = second

	err := service.Execute(context.Background(), in)

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	if got := publisher.types(); len(got) != 2 || got[0] != EventAwaySnapshot || got[1] != EventAwaySnapshot {
		t.Fatalf("publicado = %v, want 2 snapshots", got)
	}
}

func TestLeaveAwayService_PublicaSnapshotEEndedReturned(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID, _ := joinedRoom(t, state)
	in := enterInput(roomID)
	mustEnterAway(t, state, AwayEntry{RoomID: roomID, UserID: in.UserID, Username: "alice", Since: at(0), Owner: in.Owner})

	err := NewLeaveAwayService(state, publisher).Execute(context.Background(), LeaveAwayInput{UserID: in.UserID})

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	if got := publisher.types(); len(got) != 2 || got[0] != EventAwaySnapshot || got[1] != EventAwayEnded {
		t.Fatalf("publicado = %v, want snapshot e ended", got)
	}
	var payload awayEndedPayload
	json.Unmarshal(publisher.last().Data, &payload)
	if payload.Reason != AwayEndedReturned || payload.UserID != in.UserID {
		t.Errorf("payload = %+v", payload)
	}
}

func TestLeaveAwayService_SemEntradaNaoPublica(t *testing.T) {
	publisher := &lockedPublisher{}

	err := NewLeaveAwayService(newTestState(), publisher).Execute(context.Background(), LeaveAwayInput{UserID: uuid.New()})

	if err != nil || len(publisher.types()) != 0 {
		t.Fatalf("err = %v, publicado = %v, want nada", err, publisher.types())
	}
}

func TestLeaveAwayService_ErroDoEstadoNaoPublica(t *testing.T) {
	publisher := &lockedPublisher{}
	state := &erroringState{VoiceRoomState: newTestState(), leaveAway: errKVDown}

	err := NewLeaveAwayService(state, publisher).Execute(context.Background(), LeaveAwayInput{UserID: uuid.New()})

	if !errors.Is(err, errKVDown) || len(publisher.types()) != 0 {
		t.Fatalf("err = %v, publicado = %v", err, publisher.types())
	}
}

func TestLeaveAwayService_ErroNoSnapshotNaoPublicaListaVazia(t *testing.T) {
	inner := newTestState()
	publisher := &lockedPublisher{}
	roomID, _ := joinedRoom(t, inner)
	in := enterInput(roomID)
	mustEnterAway(t, inner, AwayEntry{RoomID: roomID, UserID: in.UserID, Username: "alice", Since: at(0), Owner: in.Owner})
	state := &erroringState{VoiceRoomState: inner, snapshotErr: errKVDown}

	err := NewLeaveAwayService(state, publisher).Execute(context.Background(), LeaveAwayInput{UserID: in.UserID})

	if !errors.Is(err, errKVDown) || len(publisher.types()) != 0 {
		t.Fatalf("err = %v, publicado = %v", err, publisher.types())
	}
}

func TestReleaseAwayService_ExigeDonoEPublicaConnectionClosed(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID, _ := joinedRoom(t, state)
	in := enterInput(roomID)
	mustEnterAway(t, state, AwayEntry{RoomID: roomID, UserID: in.UserID, Username: "alice", Since: at(0), Owner: in.Owner})
	service := NewReleaseAwayService(state, publisher)

	service.Execute(context.Background(), ReleaseAwayInput{UserID: in.UserID, Owner: AwayOwner{InstanceID: in.Owner.InstanceID, ConnID: uuid.New()}})
	if len(publisher.types()) != 0 {
		t.Fatalf("dono errado publicou %v", publisher.types())
	}
	err := service.Execute(context.Background(), ReleaseAwayInput{UserID: in.UserID, Owner: in.Owner})

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	if got := publisher.types(); len(got) != 2 || got[0] != EventAwaySnapshot || got[1] != EventAwayEnded {
		t.Fatalf("publicado = %v, want snapshot e ended", got)
	}
	var payload awayEndedPayload
	json.Unmarshal(publisher.last().Data, &payload)
	if payload.Reason != AwayEndedConnectionClosed {
		t.Errorf("reason = %s, want connection_closed", payload.Reason)
	}
}

type countingReleaser struct {
	calls    atomic.Int32
	failures int32
	panics   bool
	last     ReleaseAwayInput
}

func (r *countingReleaser) Execute(_ context.Context, in ReleaseAwayInput) error {
	if r.panics {
		panic("boom")
	}
	r.last = in
	if r.calls.Add(1) <= r.failures {
		return errKVDown
	}
	return nil
}

func newTestClient() *realtime.Client {
	return realtime.NewClient(nil, nil, uuid.New(), "alice", uuid.New(), nil)
}

func newTestHook(release awayReleaser, instanceID uuid.UUID) *ReleaseAwayOnCloseHook {
	hook := NewReleaseAwayOnCloseHook(release, instanceID)
	hook.retryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	return hook
}

func TestReleaseAwayOnCloseHook_UsaIdentidadeDaConexao(t *testing.T) {
	releaser := &countingReleaser{}
	instanceID := uuid.New()
	client := newTestClient()

	newTestHook(releaser, instanceID).HandleConnectionClosed(client)

	want := ReleaseAwayInput{UserID: client.UserID, Owner: AwayOwner{InstanceID: instanceID, ConnID: client.ConnID}}
	if releaser.last != want {
		t.Errorf("input = %+v, want %+v", releaser.last, want)
	}
}

func TestReleaseAwayOnCloseHook_TentaNovamenteAteTresVezesEDesiste(t *testing.T) {
	releaser := &countingReleaser{failures: 100}
	buf := captureLog(t)

	newTestHook(releaser, uuid.New()).HandleConnectionClosed(newTestClient())

	if got := releaser.calls.Load(); got != 4 {
		t.Errorf("chamadas = %d, want 1 + 3 retentativas", got)
	}
	if !strings.Contains(buf.String(), "voice.away.release_retry attempt=3") || !strings.Contains(buf.String(), "level=error op=release") {
		t.Errorf("log = %q, want retries e erro final", buf.String())
	}
}

func TestReleaseAwayOnCloseHook_ParaAoTerSucesso(t *testing.T) {
	releaser := &countingReleaser{failures: 1}

	newTestHook(releaser, uuid.New()).HandleConnectionClosed(newTestClient())

	if got := releaser.calls.Load(); got != 2 {
		t.Errorf("chamadas = %d, want 2", got)
	}
}

func TestReleaseAwayOnCloseHook_RecuperaPanicELoga(t *testing.T) {
	buf := captureLog(t)

	newTestHook(&countingReleaser{panics: true}, uuid.New()).HandleConnectionClosed(newTestClient())

	if !strings.Contains(buf.String(), "op=release") || !strings.Contains(buf.String(), "boom") {
		t.Errorf("log = %q, want o panic registrado", buf.String())
	}
}

func TestSweepAwayService_PublicaSnapshotPorSalaEEndedPorEntrada(t *testing.T) {
	state := newTestState()
	publisher := &lockedPublisher{}
	roomID, _ := joinedRoom(t, state)
	deadInstance := uuid.New()
	for range 2 {
		mustEnterAway(t, state, AwayEntry{RoomID: roomID, UserID: uuid.New(), Username: "alice", Since: at(0), Owner: AwayOwner{InstanceID: deadInstance, ConnID: uuid.New()}})
	}

	err := NewSweepAwayService(state, publisher).Execute(context.Background())

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	got := publisher.types()
	if len(got) != 3 || got[0] != EventAwaySnapshot || got[1] != EventAwayEnded || got[2] != EventAwayEnded {
		t.Fatalf("publicado = %v, want 1 snapshot e 2 ended", got)
	}
}

func TestSweepAwayService_NadaParaLimparNaoPublica(t *testing.T) {
	publisher := &lockedPublisher{}

	err := NewSweepAwayService(newTestState(), publisher).Execute(context.Background())

	if err != nil || len(publisher.types()) != 0 {
		t.Fatalf("err = %v, publicado = %v", err, publisher.types())
	}
}

func TestSweepAwayService_ErroDoEstado(t *testing.T) {
	state := &erroringState{VoiceRoomState: newTestState(), sweepErr: errKVDown}

	err := NewSweepAwayService(state, &lockedPublisher{}).Execute(context.Background())

	if !errors.Is(err, errKVDown) {
		t.Fatalf("err = %v, want erro do estado", err)
	}
}

func (s *erroringState) Join(ctx context.Context, roomID uuid.UUID, p Participant, at time.Time) (JoinResult, error) {
	if s.webhookErr != nil {
		return JoinResult{}, s.webhookErr
	}
	return s.VoiceRoomState.Join(ctx, roomID, p, at)
}

func (s *erroringState) Leave(ctx context.Context, roomID uuid.UUID, sid string) (LeaveResult, error) {
	if s.webhookErr != nil {
		return LeaveResult{}, s.webhookErr
	}
	return s.VoiceRoomState.Leave(ctx, roomID, sid)
}

func (s *erroringState) Finish(ctx context.Context, roomID uuid.UUID) ([]AwayEntry, error) {
	if s.webhookErr != nil {
		return nil, s.webhookErr
	}
	return s.VoiceRoomState.Finish(ctx, roomID)
}

type cancelAfterEnterState struct {
	VoiceRoomState
	cancel context.CancelFunc
}

func (s *cancelAfterEnterState) EnterAway(ctx context.Context, entry AwayEntry) (EnterAwayResult, error) {
	result, err := s.VoiceRoomState.EnterAway(ctx, entry)
	s.cancel()
	return result, err
}

type contextAwareState struct {
	VoiceRoomState
}

func (s contextAwareState) ReleaseAway(ctx context.Context, userID uuid.UUID, owner AwayOwner) (*AwayEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.VoiceRoomState.ReleaseAway(ctx, userID, owner)
}

func (s contextAwareState) Snapshot(ctx context.Context, roomID uuid.UUID) (RoomSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return RoomSnapshot{}, err
	}
	return s.VoiceRoomState.Snapshot(ctx, roomID)
}

func runEnterAwayOrphanRelease(t *testing.T, state VoiceRoomState) {
	t.Helper()
	background := context.Background()
	publisher := &lockedPublisher{}
	first, second := uuid.New(), uuid.New()
	for _, roomID := range []uuid.UUID{first, second} {
		mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-"+roomID.String()), at(0))
	}
	in := enterInput(first)
	if err := NewEnterAwayService(state, fakeRoomAuthorizer{}, publisher).Execute(background, in); err != nil {
		t.Fatalf("enter inicial erro: %v", err)
	}
	publisher.envelopes = nil

	connCtx, cancel := context.WithCancel(background)
	in.RoomID, in.ConnCtx = second, connCtx
	service := NewEnterAwayService(&cancelAfterEnterState{VoiceRoomState: state, cancel: cancel}, fakeRoomAuthorizer{}, publisher)

	err := service.Execute(connCtx, in)

	if err != nil {
		t.Fatalf("Execute erro: %v", err)
	}
	assertAwayUsers(t, mustSnapshot(t, state, second))
	assertAwayUsers(t, mustSnapshot(t, state, first))
	got := publisher.types()
	if len(got) != 1 || got[0] != EventAwaySnapshot {
		t.Errorf("publicado = %v, want snapshot da sala anterior", got)
	}
}

func TestEnterAwayService_LiberaOrfaoMesmoComContextoDaConexaoCancelado_Memoria(t *testing.T) {
	runEnterAwayOrphanRelease(t, contextAwareState{newTestState()})
}
