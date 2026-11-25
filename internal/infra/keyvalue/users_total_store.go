package keyvalue

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type UsersTotalHandler interface {
	TotalChanged(ctx context.Context, total int64)
	RefreshTotal(ctx context.Context) error
}

type UsersTotalStore struct {
	client  *redis.Client
	key     string
	channel string
	ttl     time.Duration
}

func NewUsersTotalStore(client *redis.Client, keyPrefix string, ttl time.Duration) *UsersTotalStore {
	return &UsersTotalStore{
		client:  client,
		key:     keyPrefix + ":users:total",
		channel: keyPrefix + ":users-total",
		ttl:     ttl,
	}
}

func (s *UsersTotalStore) Get(ctx context.Context) (int64, bool, error) {
	value, err := s.client.Get(ctx, s.key).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

func (s *UsersTotalStore) Set(ctx context.Context, total int64) error {
	return s.client.Set(ctx, s.key, total, s.ttl).Err()
}

func (s *UsersTotalStore) Publish(ctx context.Context, total int64) error {
	return s.client.Publish(ctx, s.channel, strconv.FormatInt(total, 10)).Err()
}

func (s *UsersTotalStore) Subscribe(ctx context.Context, handler UsersTotalHandler) {
	subscribeLoop(ctx, s.client, s.channel, "users_total",
		func(payload string) { dispatchUsersTotal(ctx, handler, payload) },
		func() { refreshTotal(ctx, handler, reconnectAttemptTimeout) },
	)
}

func dispatchUsersTotal(ctx context.Context, handler UsersTotalHandler, payload string) {
	total, err := strconv.ParseInt(payload, 10, 64)
	if err != nil || total < 0 {
		log.Printf("kv.users_total_message_invalid level=warn payload=%q", payload)
		return
	}
	handler.TotalChanged(ctx, total)
}

func refreshTotal(ctx context.Context, handler UsersTotalHandler, timeout time.Duration) {
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := handler.RefreshTotal(attemptCtx); err != nil {
		log.Printf("kv.users_total_refresh_failed level=error err=%q", err)
	}
}
