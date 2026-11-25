package keyvalue

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	sessionMessagePrefix    = "session:"
	userMessagePrefix       = "user:"
	resubscribeBackoff      = 2 * time.Second
	resyncBackoff           = 2 * time.Second
	resyncMaxAttempts       = 5
	reconnectAttemptTimeout = 10 * time.Second
)

type RevocationHandler interface {
	Revoke(ctx context.Context, sessionID uuid.UUID) error
	RevokeUser(ctx context.Context, userID uuid.UUID) error
	Resync(ctx context.Context) error
}

type RevocationBus struct {
	client  *redis.Client
	channel string
}

func NewRevocationBus(client *redis.Client, keyPrefix string) *RevocationBus {
	return &RevocationBus{client: client, channel: keyPrefix + ":session-revoked"}
}

func (b *RevocationBus) PublishSessionRevoked(ctx context.Context, sessionID uuid.UUID) error {
	return b.client.Publish(ctx, b.channel, sessionMessagePrefix+sessionID.String()).Err()
}

func (b *RevocationBus) PublishUserRevoked(ctx context.Context, userID uuid.UUID) error {
	return b.client.Publish(ctx, b.channel, userMessagePrefix+userID.String()).Err()
}

func (b *RevocationBus) Subscribe(ctx context.Context, handler RevocationHandler) {
	subscribeLoop(ctx, b.client, b.channel, "revocation",
		func(payload string) { dispatchRevocation(ctx, handler, payload) },
		func() { resync(ctx, handler) },
	)
}

func resync(ctx context.Context, handler RevocationHandler) {
	resyncWithBackoff(ctx, handler, resyncBackoff, reconnectAttemptTimeout)
}

func resyncWithBackoff(ctx context.Context, handler RevocationHandler, backoff, attemptTimeout time.Duration) {
	for attempt := 1; attempt <= resyncMaxAttempts; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		err := handler.Resync(attemptCtx)
		cancel()
		if err == nil {
			return
		}
		log.Printf("kv.revocation_resync_failed level=error attempt=%d err=%q", attempt, err)
		if attempt == resyncMaxAttempts {
			return
		}
		sleep(ctx, backoff<<(attempt-1))
		if ctx.Err() != nil {
			return
		}
	}
}

func dispatchRevocation(ctx context.Context, handler RevocationHandler, payload string) {
	if id, ok := strings.CutPrefix(payload, sessionMessagePrefix); ok {
		revoke(id, func(parsed uuid.UUID) error { return handler.Revoke(ctx, parsed) })
		return
	}
	if id, ok := strings.CutPrefix(payload, userMessagePrefix); ok {
		revoke(id, func(parsed uuid.UUID) error { return handler.RevokeUser(ctx, parsed) })
		return
	}
	log.Printf("kv.revocation_message_unknown level=warn payload=%q", payload)
}

func revoke(rawID string, apply func(uuid.UUID) error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		log.Printf("kv.revocation_message_invalid level=warn id=%q err=%q", rawID, err)
		return
	}
	if err := apply(id); err != nil {
		log.Printf("kv.revocation_apply_failed level=warn id=%s err=%q", id, err)
	}
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
