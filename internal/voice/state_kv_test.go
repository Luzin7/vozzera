package voice

import (
	"context"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/infra/keyvalue"
	"github.com/Luzin7/vozzera-backend/internal/infra/keyvalue/kvtest"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func newKVTestClient(t *testing.T) *redis.Client {
	t.Helper()

	client, err := keyvalue.Connect(context.Background(), kvtest.URL(t))
	if err != nil {
		t.Fatalf("Connect erro: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func removeKeysWithPrefix(t *testing.T, client *redis.Client, prefix string) {
	t.Helper()

	ctx := context.Background()
	iter := client.Scan(ctx, 0, prefix+":*", 100).Iterator()
	for iter.Next(ctx) {
		client.Del(ctx, iter.Val())
	}
}

func TestKVVoiceRoomState_Contract(t *testing.T) {
	client := newKVTestClient(t)

	runVoiceRoomStateContract(t, func(t *testing.T) contractHarness {
		prefix := "vz:test-" + uuid.NewString()
		t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

		return contractHarness{
			state: NewKVVoiceRoomState(client, prefix),
			expireInstance: func(instanceID uuid.UUID) {
				client.Del(context.Background(), prefix+":instance:"+instanceID.String())
			},
			dropSession: func(roomID uuid.UUID) {
				client.Del(context.Background(), prefix+":room:"+roomID.String()+":session")
			},
		}
	})
}

func TestEnterAwayService_LiberaOrfaoMesmoComContextoDaConexaoCancelado_KV(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	runEnterAwayOrphanRelease(t, NewKVVoiceRoomState(client, prefix))
}
