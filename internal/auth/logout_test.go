package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type revokeRecorder struct {
	revoked     uuid.UUID
	revokedUser uuid.UUID
	ctxErr      error
	err         error
}

func (r *revokeRecorder) Revoke(ctx context.Context, sessionID uuid.UUID) error {
	r.revoked = sessionID
	r.ctxErr = ctx.Err()
	return r.err
}

func (r *revokeRecorder) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	r.revokedUser = userID
	r.ctxErr = ctx.Err()
	return r.err
}

func TestLogoutService_Execute(t *testing.T) {
	repo := newFakeRepo()
	repo.deleteSessionByID = func(ctx context.Context, id uuid.UUID) error { return nil }

	revoker := &revokeRecorder{}
	svc := NewLogoutService(repo, revoker)

	sessionID := uuid.New()
	if err := svc.Execute(context.Background(), LogoutInput{SessionID: sessionID}); err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}

	if revoker.revoked != sessionID {
		t.Errorf("revoked = %v, want %v", revoker.revoked, sessionID)
	}
}

func TestLogoutService_Execute_RevokeFailureStillSucceeds(t *testing.T) {
	repo := newFakeRepo()
	repo.deleteSessionByID = func(context.Context, uuid.UUID) error { return nil }
	svc := NewLogoutService(repo, &revokeRecorder{err: errors.New("hub encerrado")})

	if err := svc.Execute(context.Background(), LogoutInput{SessionID: uuid.New()}); err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}
}

func TestLogoutService_Execute_RevokesAfterRequestIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := newFakeRepo()
	repo.deleteSessionByID = func(context.Context, uuid.UUID) error { return nil }
	revoker := &revokeRecorder{}

	_ = NewLogoutService(repo, revoker).Execute(ctx, LogoutInput{SessionID: uuid.New()})

	if revoker.ctxErr != nil {
		t.Errorf("revogação rodou com o contexto da requisição cancelado: %v", revoker.ctxErr)
	}
}
