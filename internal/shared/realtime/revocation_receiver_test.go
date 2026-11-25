package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type livenessFake struct {
	active  map[uuid.UUID]bool
	queries [][]uuid.UUID
	err     error
}

func (l *livenessFake) ActiveSessionIDs(_ context.Context, ids []uuid.UUID) ([]uuid.UUID, error) {
	l.queries = append(l.queries, ids)
	if l.err != nil {
		return nil, l.err
	}
	var active []uuid.UUID
	for _, id := range ids {
		if l.active[id] {
			active = append(active, id)
		}
	}
	return active, nil
}

func runningHub(t *testing.T, clients ...*Client) *Hub {
	t.Helper()
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() { hub.Shutdown(context.Background()) })
	for _, c := range clients {
		hub.Register(c)
	}
	hub.Sync(context.Background())
	return hub
}

func isClosed(c *Client) bool {
	select {
	case <-c.Context().Done():
		return true
	case <-time.After(time.Second):
		return false
	}
}

func TestSessionRevocationReceiver_ResyncClosesOnlyInactiveSessionsWithOneQuery(t *testing.T) {
	alive := NewClient(nil, nil, uuid.New(), "alice", uuid.New(), nil)
	revoked := NewClient(nil, nil, uuid.New(), "bob", uuid.New(), nil)
	hub := runningHub(t, alive, revoked)
	liveness := &livenessFake{active: map[uuid.UUID]bool{alive.SessionID: true}}

	if err := NewSessionRevocationReceiver(hub, liveness).Resync(context.Background()); err != nil {
		t.Fatalf("Resync() erro inesperado: %v", err)
	}

	if !isClosed(revoked) {
		t.Error("sessão inativa deveria ter sido fechada")
	}
	select {
	case <-alive.Context().Done():
		t.Error("sessão ativa não deveria ser fechada")
	case <-time.After(50 * time.Millisecond):
	}
	if len(liveness.queries) != 1 || len(liveness.queries[0]) != 2 {
		t.Errorf("queries = %v, want 1 consulta com 2 ids", liveness.queries)
	}
}

func TestSessionRevocationReceiver_ResyncFailureKeepsClientsOpen(t *testing.T) {
	client := NewClient(nil, nil, uuid.New(), "alice", uuid.New(), nil)
	hub := runningHub(t, client)
	liveness := &livenessFake{err: errors.New("banco fora")}

	if err := NewSessionRevocationReceiver(hub, liveness).Resync(context.Background()); err == nil {
		t.Fatal("Resync() deveria retornar o erro do banco")
	}

	select {
	case <-client.Context().Done():
		t.Error("falha de infraestrutura não deveria fechar clientes")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSessionRevocationReceiver_ResyncWithoutClientsSkipsDatabase(t *testing.T) {
	hub := runningHub(t)
	liveness := &livenessFake{}

	if err := NewSessionRevocationReceiver(hub, liveness).Resync(context.Background()); err != nil {
		t.Fatalf("Resync() erro inesperado: %v", err)
	}

	if len(liveness.queries) != 0 {
		t.Errorf("queries = %v, want nenhuma", liveness.queries)
	}
}
