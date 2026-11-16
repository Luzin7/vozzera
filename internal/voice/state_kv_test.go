package voice

import (
	"context"
	"testing"
	"time"

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

func TestKVVoiceRoomState_JoinAndLeaveSetRoomKeysTTL(t *testing.T) {
	client := newKVTestClient(t)
	prefix, roomID, caller, other := "vz:test-"+uuid.NewString(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	mustJoin(t, state, roomID, participantOf(caller, "sid-caller"), at(0))
	mustJoin(t, state, roomID, participantOf(other, "sid-other"), at(0))
	keys := []string{state.presenceKey(roomID), state.sessionKey(roomID), state.revisionKey(roomID)}

	assertRoomKeysTTL(t, client, keys)

	if _, err := state.Leave(context.Background(), LeaveInput{RoomID: roomID, SID: "sid-other", At: at(0)}); err != nil {
		t.Fatalf("Leave erro: %v", err)
	}

	assertRoomKeysTTL(t, client, keys)
}

func TestKVVoiceRoomState_HydrateRefreshesRoomKeysTTL(t *testing.T) {
	client := newKVTestClient(t)
	prefix, roomID, caller, other := "vz:test-"+uuid.NewString(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	participants := []Participant{participantOf(caller, "sid-caller"), participantOf(other, "sid-other")}
	mustJoin(t, state, roomID, participants[0], at(0))
	mustJoin(t, state, roomID, participants[1], at(0))
	mustEnterAway(t, state, newAwayEntry(roomID, forUser(caller)))
	service := newHydrateService(listerWithParticipants(roomID, participants...), state, &lockedPublisher{})
	keys := roomKeys(state, roomID)

	mustHydrate(t, service)

	assertRoomKeysTTL(t, client, keys)

	for _, key := range keys {
		if err := client.PExpire(context.Background(), key, 500*time.Millisecond).Err(); err != nil {
			t.Fatalf("PExpire erro: %v", err)
		}
	}
	mustHydrate(t, service)

	assertRoomKeysTTL(t, client, keys)
}

func roomKeys(state *kvVoiceRoomState, roomID uuid.UUID) []string {
	return []string{state.presenceKey(roomID), state.sessionKey(roomID), state.awayKey(roomID), state.revisionKey(roomID)}
}

func assertRoomKeysTTL(t *testing.T, client *redis.Client, keys []string) {
	t.Helper()

	for _, key := range keys {
		ttl, err := client.PTTL(context.Background(), key).Result()
		if err != nil {
			t.Fatalf("PTTL %s erro: %v", key, err)
		}
		if ttl <= roomStateTTL-5*time.Second || ttl > roomStateTTL {
			t.Errorf("pttl %s = %v, want perto de %v", key, ttl, roomStateTTL)
		}
	}
}

func TestEnterAwayService_LiberaOrfaoMesmoComContextoDaConexaoCancelado_KV(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	runEnterAwayOrphanRelease(t, NewKVVoiceRoomState(client, prefix))
}
