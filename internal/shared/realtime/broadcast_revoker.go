package realtime

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type LocalRevoker interface {
	Revoke(ctx context.Context, sessionID uuid.UUID) error
	RevokeUser(ctx context.Context, userID uuid.UUID) error
}

type RevocationPublisher interface {
	PublishSessionRevoked(ctx context.Context, sessionID uuid.UUID) error
	PublishUserRevoked(ctx context.Context, userID uuid.UUID) error
}

type BroadcastRevoker struct {
	local     LocalRevoker
	publisher RevocationPublisher
}

func NewBroadcastRevoker(local LocalRevoker, publisher RevocationPublisher) *BroadcastRevoker {
	return &BroadcastRevoker{local: local, publisher: publisher}
}

func (r *BroadcastRevoker) Revoke(ctx context.Context, sessionID uuid.UUID) error {
	localErr := r.local.Revoke(ctx, sessionID)
	if r.publisher == nil {
		return localErr
	}
	return errors.Join(localErr, r.publisher.PublishSessionRevoked(ctx, sessionID))
}

func (r *BroadcastRevoker) RevokeUser(ctx context.Context, userID uuid.UUID) error {
	localErr := r.local.RevokeUser(ctx, userID)
	if r.publisher == nil {
		return localErr
	}
	return errors.Join(localErr, r.publisher.PublishUserRevoked(ctx, userID))
}
