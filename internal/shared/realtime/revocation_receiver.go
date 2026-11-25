package realtime

import (
	"context"
	"log"

	"github.com/google/uuid"
)

type SessionLiveness interface {
	ActiveSessionIDs(ctx context.Context, ids []uuid.UUID) ([]uuid.UUID, error)
}

type SessionRevocationReceiver struct {
	hub      *Hub
	liveness SessionLiveness
}

func NewSessionRevocationReceiver(hub *Hub, liveness SessionLiveness) *SessionRevocationReceiver {
	return &SessionRevocationReceiver{hub: hub, liveness: liveness}
}

func (r *SessionRevocationReceiver) Revoke(ctx context.Context, sessionID uuid.UUID) error {
	return r.hub.Revoke(ctx, sessionID)
}

func (r *SessionRevocationReceiver) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	return r.hub.RevokeUser(ctx, userID)
}

func (r *SessionRevocationReceiver) Resync(ctx context.Context) error {
	connected, err := r.hub.SessionIDs(ctx)
	if err != nil {
		return err
	}
	if len(connected) == 0 {
		return nil
	}

	active, err := r.liveness.ActiveSessionIDs(ctx, connected)
	if err != nil {
		return err
	}

	activeSet := make(map[uuid.UUID]bool, len(active))
	for _, id := range active {
		activeSet[id] = true
	}

	closed := 0
	for _, id := range connected {
		if activeSet[id] {
			continue
		}
		if err := r.hub.Revoke(ctx, id); err != nil {
			return err
		}
		closed++
	}

	log.Printf("realtime.resync checked=%d closed=%d", len(connected), closed)
	return nil
}
