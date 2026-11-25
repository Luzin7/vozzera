package keyvalue

import (
	"context"
	"log"
	"math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"
)

func subscribeLoop(ctx context.Context, client *redis.Client, channel, name string, onMessage func(payload string), onReconnect func()) {
	pubsub := client.Subscribe(ctx, channel)
	defer pubsub.Close()

	for ctx.Err() == nil {
		message, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("kv.%s_subscriber_dropped level=warn err=%q", name, err)
			awaitReconnect(ctx, pubsub, name)
			onReconnect()
			continue
		}
		onMessage(message.Payload)
	}
}

func awaitReconnect(ctx context.Context, pubsub *redis.PubSub, name string) {
	for ctx.Err() == nil {
		sleep(ctx, jitter(resubscribeBackoff))
		if err := pubsub.Ping(ctx); err == nil {
			log.Printf("kv.%s_subscriber_reconnected level=info", name)
			return
		}
	}
}

func jitter(base time.Duration) time.Duration {
	return base/2 + rand.N(base)
}
