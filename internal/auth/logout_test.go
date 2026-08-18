package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type revokeRecorder struct {
	revoked uuid.UUID
}

func (r *revokeRecorder) Revoke(sessionID uuid.UUID) {
	r.revoked = sessionID
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
