package voice

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

type beatingState struct {
	VoiceRoomState
	results []beatOutcome
	calls   int
}

type beatOutcome struct {
	result BeatResult
	err    error
}

func (s *beatingState) Beat(context.Context, uuid.UUID) (BeatResult, error) {
	outcome := s.results[min(s.calls, len(s.results)-1)]
	s.calls++
	return outcome.result, outcome.err
}

type countingHydrator struct {
	calls    atomic.Int32
	failures int32
}

func (h *countingHydrator) Execute(context.Context) error {
	if h.calls.Add(1) <= h.failures {
		return errKVDown
	}
	return nil
}

var heartbeatPublisher = &lockedPublisher{}

func newHeartbeat(state VoiceRoomState, hydrator presenceHydrator, startedDown bool) *InstanceHeartbeat {
	heartbeatPublisher = &lockedPublisher{}
	return NewInstanceHeartbeat(InstanceHeartbeatDeps{State: state, InstanceID: uuid.New(), Hydrator: hydrator, Publisher: heartbeatPublisher, StartedDown: startedDown})
}

func TestInstanceHeartbeat_WipedLogaEHidrata(t *testing.T) {
	buf := captureLog(t)
	hydrator := &countingHydrator{}
	state := &beatingState{results: []beatOutcome{{result: BeatResult{Wiped: true}}}}
	heartbeat := newHeartbeat(state, hydrator, false)

	heartbeat.tick(context.Background())

	if hydrator.calls.Load() != 1 {
		t.Errorf("hidratações = %d, want 1", hydrator.calls.Load())
	}
	if !strings.Contains(buf.String(), "voice.kv_wiped instance_id="+heartbeat.instanceID.String()) {
		t.Errorf("log = %q, want voice.kv_wiped", buf.String())
	}
}

func TestInstanceHeartbeat_SemWipedNaoHidrata(t *testing.T) {
	hydrator := &countingHydrator{}
	state := &beatingState{results: []beatOutcome{{result: BeatResult{}}}}

	newHeartbeat(state, hydrator, false).tick(context.Background())

	if hydrator.calls.Load() != 0 {
		t.Errorf("hidratações = %d, want 0", hydrator.calls.Load())
	}
}

func TestInstanceHeartbeat_LogaSoNasTransicoes(t *testing.T) {
	buf := captureLog(t)
	state := &beatingState{results: []beatOutcome{
		{},
		{err: errKVDown},
		{err: errKVDown},
		{},
		{},
	}}
	heartbeat := newHeartbeat(state, &countingHydrator{}, false)

	for range 5 {
		heartbeat.tick(context.Background())
	}

	if got := strings.Count(buf.String(), "kv.state level=error transition=down"); got != 1 {
		t.Errorf("transições down = %d, want 1; log = %q", got, buf.String())
	}
	if got := strings.Count(buf.String(), "kv.state transition=up"); got != 1 {
		t.Errorf("transições up = %d, want 1; log = %q", got, buf.String())
	}
}

func TestInstanceHeartbeat_IniciadoForaDoArLogaUpNaPrimeiraBatidaOk(t *testing.T) {
	buf := captureLog(t)
	state := &beatingState{results: []beatOutcome{{err: errKVDown}, {result: BeatResult{Wiped: true}}}}
	heartbeat := newHeartbeat(state, &countingHydrator{}, true)

	heartbeat.tick(context.Background())
	heartbeat.tick(context.Background())

	if strings.Contains(buf.String(), "transition=down") {
		t.Errorf("log = %q, não deveria repetir down", buf.String())
	}
	if !strings.Contains(buf.String(), "kv.state transition=up") {
		t.Errorf("log = %q, want transition=up", buf.String())
	}
}

func TestInstanceHeartbeat_RunBateLogoEParaComOContexto(t *testing.T) {
	state := &beatingState{results: []beatOutcome{{}}}
	heartbeat := newHeartbeat(state, &countingHydrator{}, false)
	heartbeat.interval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		heartbeat.Run(ctx)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run não parou com o contexto cancelado")
	}
	if state.calls < 2 {
		t.Errorf("batidas = %d, want pelo menos 2", state.calls)
	}
}

func TestInstanceHeartbeat_BeatReal_ComEstadoEmMemoriaDetectaWipedUmaVez(t *testing.T) {
	hydrator := &countingHydrator{}
	heartbeat := newHeartbeat(newTestState(), hydrator, false)

	heartbeat.tick(context.Background())
	heartbeat.tick(context.Background())

	if hydrator.calls.Load() != 1 {
		t.Errorf("hidratações = %d, want 1", hydrator.calls.Load())
	}
}

type countingSweeper struct {
	calls atomic.Int32
	err   error
}

func (s *countingSweeper) Execute(context.Context) error {
	s.calls.Add(1)
	return s.err
}

func TestAwaySweeper_TickExecutaOServico(t *testing.T) {
	sweeper := &countingSweeper{}

	NewAwaySweeper(sweeper).tick(context.Background())

	if sweeper.calls.Load() != 1 {
		t.Errorf("chamadas = %d, want 1", sweeper.calls.Load())
	}
}

func TestAwaySweeper_ErroNaoParaOLoop(t *testing.T) {
	sweeper := &countingSweeper{err: errKVDown}
	away := NewAwaySweeper(sweeper)
	away.interval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		away.Run(ctx)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run não parou com o contexto cancelado")
	}
	if sweeper.calls.Load() < 2 {
		t.Errorf("chamadas = %d, want pelo menos 2 apesar do erro", sweeper.calls.Load())
	}
}

func TestInstanceHeartbeat_HidratacaoFalhaTentaDeNovoACadaBatidaAteConseguir(t *testing.T) {
	hydrator := &countingHydrator{failures: 2}
	state := &beatingState{results: []beatOutcome{{result: BeatResult{Wiped: true}}, {}}}
	heartbeat := newHeartbeat(state, hydrator, false)

	for range 5 {
		heartbeat.tick(context.Background())
	}

	if hydrator.calls.Load() != 3 {
		t.Errorf("hidratações = %d, want 3 (2 falhas e 1 sucesso, sem repetir depois)", hydrator.calls.Load())
	}
}

func TestInstanceHeartbeat_PublicaResyncAposWipedEOutroAoRecuperarAHidratacao(t *testing.T) {
	hydrator := &countingHydrator{failures: 1}
	state := &beatingState{results: []beatOutcome{{result: BeatResult{Wiped: true}}, {}}}
	heartbeat := newHeartbeat(state, hydrator, false)

	for range 3 {
		heartbeat.tick(context.Background())
	}

	got := heartbeatPublisher.types()
	if len(got) != 2 || got[0] != EventAwayResync || got[1] != EventAwayResync {
		t.Errorf("publicado = %v, want 2x %s (após o wipe e após a recuperação)", got, EventAwayResync)
	}
}

func TestInstanceHeartbeat_LogaFalhaDeHidratacaoSoNaTransicaoERecuperacao(t *testing.T) {
	buf := captureLog(t)
	hydrator := &countingHydrator{failures: 3}
	state := &beatingState{results: []beatOutcome{{result: BeatResult{Wiped: true}}, {}}}
	heartbeat := newHeartbeat(state, hydrator, false)

	for range 5 {
		heartbeat.tick(context.Background())
	}

	if got := strings.Count(buf.String(), "voice.hydrate level=error"); got != 1 {
		t.Errorf("logs de erro = %d, want 1; log = %q", got, buf.String())
	}
	if !strings.Contains(buf.String(), "voice.hydrate recovered attempts=4") {
		t.Errorf("log = %q, want voice.hydrate recovered attempts=4", buf.String())
	}
}

func TestInstanceHeartbeat_HidratacaoNoWipeSemFalhaPublicaUmResyncSo(t *testing.T) {
	state := &beatingState{results: []beatOutcome{{result: BeatResult{Wiped: true}}, {}}}
	heartbeat := newHeartbeat(state, &countingHydrator{}, false)

	for range 3 {
		heartbeat.tick(context.Background())
	}

	if got := heartbeatPublisher.types(); len(got) != 1 {
		t.Errorf("publicado = %v, want um resync", got)
	}
}
