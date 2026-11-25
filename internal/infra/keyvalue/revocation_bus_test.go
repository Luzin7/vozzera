package keyvalue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

func newHubWithClients(t *testing.T, clients ...*realtime.Client) *realtime.Hub {
	t.Helper()
	hub := realtime.NewHub()
	go hub.Run()
	t.Cleanup(func() { hub.Shutdown(context.Background()) })

	for _, c := range clients {
		hub.Register(c)
	}
	if err := hub.Sync(context.Background()); err != nil {
		t.Fatalf("Sync erro: %v", err)
	}
	return hub
}

func newClient(hub *realtime.Hub, userID, sessionID uuid.UUID) *realtime.Client {
	return realtime.NewClient(hub, nil, userID, "user", sessionID, nil)
}

func closed(c *realtime.Client) bool {
	select {
	case <-c.Context().Done():
		return true
	case <-time.After(time.Second):
		return false
	}
}

func open(c *realtime.Client) bool {
	select {
	case <-c.Context().Done():
		return false
	case <-time.After(50 * time.Millisecond):
		return true
	}
}

func TestDispatchRevocation_SessionMessageClosesOnlyMatchingClient(t *testing.T) {
	hub := realtime.NewHub()
	target := newClient(hub, uuid.New(), uuid.New())
	other := newClient(hub, uuid.New(), uuid.New())
	hub = newHubWithClients(t, target, other)
	handler := realtime.NewSessionRevocationReceiver(hub, nil)

	dispatchRevocation(context.Background(), handler, sessionMessagePrefix+target.SessionID.String())

	if !closed(target) {
		t.Error("cliente da sessão revogada deveria ter sido fechado")
	}
	if !open(other) {
		t.Error("cliente de outra sessão deveria permanecer aberto")
	}
}

func TestDispatchRevocation_UserMessageClosesAllClientsOfUser(t *testing.T) {
	hub := realtime.NewHub()
	userID := uuid.New()
	first := newClient(hub, userID, uuid.New())
	second := newClient(hub, userID, uuid.New())
	other := newClient(hub, uuid.New(), uuid.New())
	hub = newHubWithClients(t, first, second, other)
	handler := realtime.NewSessionRevocationReceiver(hub, nil)

	dispatchRevocation(context.Background(), handler, userMessagePrefix+userID.String())

	if !closed(first) || !closed(second) {
		t.Error("clientes do usuário revogado deveriam ter sido fechados")
	}
	if !open(other) {
		t.Error("cliente de outro usuário deveria permanecer aberto")
	}
}

func TestDispatchRevocation_InvalidPayloadIsIgnored(t *testing.T) {
	hub := realtime.NewHub()
	client := newClient(hub, uuid.New(), uuid.New())
	hub = newHubWithClients(t, client)
	handler := realtime.NewSessionRevocationReceiver(hub, nil)

	dispatchRevocation(context.Background(), handler, "session:nao-e-uuid")
	dispatchRevocation(context.Background(), handler, "outro:"+client.SessionID.String())

	if !open(client) {
		t.Error("payload inválido não deveria fechar clientes")
	}
}

type resyncFake struct {
	results []error
	calls   int
}

func (r *resyncFake) Revoke(context.Context, uuid.UUID) error     { return nil }
func (r *resyncFake) RevokeUser(context.Context, uuid.UUID) error { return nil }
func (r *resyncFake) Resync(context.Context) error {
	err := r.results[r.calls]
	r.calls++
	return err
}

func TestResyncWithBackoff_RetriesAfterFailureUntilSuccess(t *testing.T) {
	handler := &resyncFake{results: []error{errors.New("banco fora"), nil}}

	resyncWithBackoff(context.Background(), handler, time.Millisecond, time.Second)

	if handler.calls != 2 {
		t.Errorf("calls = %d, want 2", handler.calls)
	}
}

func TestResyncWithBackoff_StopsAfterMaxAttempts(t *testing.T) {
	failure := errors.New("banco fora")
	handler := &resyncFake{results: []error{failure, failure, failure, failure, failure, failure}}

	resyncWithBackoff(context.Background(), handler, time.Millisecond, time.Second)

	if handler.calls != resyncMaxAttempts {
		t.Errorf("calls = %d, want %d", handler.calls, resyncMaxAttempts)
	}
}

func TestResyncWithBackoff_StopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	handler := &resyncFake{results: []error{errors.New("banco fora"), nil}}

	resyncWithBackoff(ctx, handler, time.Hour, time.Second)

	if handler.calls != 1 {
		t.Errorf("calls = %d, want 1", handler.calls)
	}
}

type blockingResync struct {
	calls int
}

func (b *blockingResync) Revoke(context.Context, uuid.UUID) error     { return nil }
func (b *blockingResync) RevokeUser(context.Context, uuid.UUID) error { return nil }
func (b *blockingResync) Resync(ctx context.Context) error {
	b.calls++
	<-ctx.Done()
	return ctx.Err()
}

func TestResyncWithBackoffBoundsEachAttemptByTimeout(t *testing.T) {
	handler := &blockingResync{}
	done := make(chan struct{})

	go func() {
		resyncWithBackoff(context.Background(), handler, time.Millisecond, 10*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("resync travou sem timeout por tentativa")
	}
	if handler.calls != resyncMaxAttempts {
		t.Errorf("calls = %d, want %d", handler.calls, resyncMaxAttempts)
	}
}

func TestJitterStaysWithinHalfAndOneAndAHalfOfBase(t *testing.T) {
	base := 2 * time.Second

	for range 200 {
		got := jitter(base)
		if got < base/2 || got >= base+base/2 {
			t.Fatalf("jitter = %v, want [%v, %v)", got, base/2, base+base/2)
		}
	}
}
