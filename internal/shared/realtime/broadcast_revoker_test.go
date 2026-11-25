package realtime

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type localRevokerRecorder struct {
	sessions []uuid.UUID
	users    []uuid.UUID
}

func (r *localRevokerRecorder) Revoke(_ context.Context, id uuid.UUID) error {
	r.sessions = append(r.sessions, id)
	return nil
}

func (r *localRevokerRecorder) RevokeUser(_ context.Context, id uuid.UUID) error {
	r.users = append(r.users, id)
	return nil
}

type publisherRecorder struct {
	sessions []uuid.UUID
	users    []uuid.UUID
	err      error
}

func (p *publisherRecorder) PublishSessionRevoked(_ context.Context, id uuid.UUID) error {
	p.sessions = append(p.sessions, id)
	return p.err
}

func (p *publisherRecorder) PublishUserRevoked(_ context.Context, id uuid.UUID) error {
	p.users = append(p.users, id)
	return p.err
}

func TestBroadcastRevoker_RevokeRevokesLocallyAndPublishes(t *testing.T) {
	local, publisher := &localRevokerRecorder{}, &publisherRecorder{}
	id := uuid.New()

	if err := NewBroadcastRevoker(local, publisher).Revoke(context.Background(), id); err != nil {
		t.Fatalf("Revoke() erro inesperado: %v", err)
	}

	if len(local.sessions) != 1 || local.sessions[0] != id {
		t.Errorf("local.sessions = %v, want [%v]", local.sessions, id)
	}
	if len(publisher.sessions) != 1 || publisher.sessions[0] != id {
		t.Errorf("publisher.sessions = %v, want [%v]", publisher.sessions, id)
	}
}

func TestBroadcastRevoker_RevokeUserRevokesLocallyAndPublishes(t *testing.T) {
	local, publisher := &localRevokerRecorder{}, &publisherRecorder{}
	id := uuid.New()

	if err := NewBroadcastRevoker(local, publisher).RevokeUser(context.Background(), id); err != nil {
		t.Fatalf("RevokeUser() erro inesperado: %v", err)
	}

	if len(local.users) != 1 || len(publisher.users) != 1 {
		t.Errorf("local.users = %v, publisher.users = %v, want 1 each", local.users, publisher.users)
	}
}

func TestBroadcastRevoker_PublishFailureStillRevokesLocally(t *testing.T) {
	boom := errors.New("kv fora")
	local, publisher := &localRevokerRecorder{}, &publisherRecorder{err: boom}

	err := NewBroadcastRevoker(local, publisher).Revoke(context.Background(), uuid.New())

	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
	if len(local.sessions) != 1 {
		t.Errorf("local.sessions = %v, want 1 entry", local.sessions)
	}
}

func TestBroadcastRevoker_WithoutPublisherRevokesLocally(t *testing.T) {
	local := &localRevokerRecorder{}

	if err := NewBroadcastRevoker(local, nil).Revoke(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Revoke() erro inesperado: %v", err)
	}

	if len(local.sessions) != 1 {
		t.Errorf("local.sessions = %v, want 1 entry", local.sessions)
	}
}
