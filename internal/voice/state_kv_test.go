package voice

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
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
	keys := []string{state.presenceKey(roomID), state.sessionKey(roomID), state.revisionKey(roomID), state.roomsKey()}

	assertRoomKeysTTL(t, client, keys)

	if _, err := state.Leave(context.Background(), LeaveInput{RoomID: roomID, SID: "sid-other", At: at(0)}); err != nil {
		t.Fatalf("Leave erro: %v", err)
	}

	assertRoomKeysTTL(t, client, keys)
}

func TestKVVoiceRoomState_EnterAwaySetsRoomKeysTTL(t *testing.T) {
	client := newKVTestClient(t)
	prefix, roomID, caller, other := "vz:test-"+uuid.NewString(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	mustJoin(t, state, roomID, participantOf(other, "sid-other"), at(0))
	client.Del(context.Background(), state.revisionKey(roomID))
	mustEnterAway(t, state, newAwayEntry(roomID, forUser(caller)))

	assertRoomKeysTTL(t, client, roomKeys(state, roomID))
}

func TestKVVoiceRoomState_HydrateWriteRestoresRoomKeysTTL(t *testing.T) {
	client := newKVTestClient(t)
	prefix, roomID, caller, other, extra := "vz:test-"+uuid.NewString(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	mustJoin(t, state, roomID, participantOf(caller, "sid-caller"), at(0))
	mustJoin(t, state, roomID, participantOf(other, "sid-other"), at(0))
	mustEnterAway(t, state, newAwayEntry(roomID, forUser(caller)))
	service := newHydrateService(listerWithParticipants(roomID, participantOf(other, "sid-other"), participantOf(extra, "sid-extra")), state, &lockedPublisher{})
	keys := append(roomKeys(state, roomID), state.roomsKey())

	for _, key := range keys {
		if err := client.PExpire(context.Background(), key, 500*time.Millisecond).Err(); err != nil {
			t.Fatalf("PExpire erro: %v", err)
		}
	}

	mustHydrate(t, service)

	assertParticipantSIDs(t, state, roomID, "sid-other", "sid-extra")
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
	keys := append(roomKeys(state, roomID), state.roomsKey())

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

func TestKVVoiceRoomState_FinishRemovesIndexedRoom(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	roomID := uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-a"), at(0))

	if member, err := client.SIsMember(ctx, state.roomsKey(), roomID.String()).Result(); err != nil || !member {
		t.Fatalf("rooms member = %v, err = %v, want true", member, err)
	}
	before := mustPresenceRevision(t, state, roomID)
	if err := client.PExpire(ctx, state.revisionKey(roomID), 500*time.Millisecond).Err(); err != nil {
		t.Fatalf("PExpire erro: %v", err)
	}

	removed, err := state.Finish(ctx, roomID)

	if err != nil {
		t.Fatalf("Finish erro: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v, want vazio", removed)
	}
	for _, key := range []string{state.presenceKey(roomID), state.sessionKey(roomID), state.awayKey(roomID)} {
		exists, err := client.Exists(ctx, key).Result()
		if err != nil || exists != 0 {
			t.Errorf("exists %s = %d, err = %v, want ausente", key, exists, err)
		}
	}
	if member, err := client.SIsMember(ctx, state.roomsKey(), roomID.String()).Result(); err != nil || member {
		t.Errorf("rooms member = %v, err = %v, want removido", member, err)
	}
	if after := mustPresenceRevision(t, state, roomID); after != before+1 {
		t.Errorf("revision = %d, want %d", after, before+1)
	}
	if ttl, err := client.TTL(ctx, state.revisionKey(roomID)).Result(); err != nil || ttl < roomStateTTL-time.Second {
		t.Errorf("ttl revisão = %v, err = %v, want perto de %v", ttl, err, roomStateTTL)
	}
}

func TestKVVoiceRoomState_IndexRoomsFromPresenceSeedsRoomsSet(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	first, second := uuid.New(), uuid.New()
	for _, roomID := range []uuid.UUID{first, second} {
		value := `{"user_id":"` + uuid.NewString() + `","username":"alice"}`
		if err := client.HSet(ctx, state.presenceKey(roomID), "sid-a", value).Err(); err != nil {
			t.Fatalf("HSet erro: %v", err)
		}
	}

	indexed, err := state.IndexRoomsFromPresence(ctx)

	if err != nil || indexed != 2 {
		t.Fatalf("indexed = %d, err = %v, want 2", indexed, err)
	}
	members, err := client.SMembers(ctx, state.roomsKey()).Result()
	if err != nil {
		t.Fatalf("SMembers erro: %v", err)
	}
	want := []string{first.String(), second.String()}
	sort.Strings(members)
	sort.Strings(want)
	if !reflect.DeepEqual(members, want) {
		t.Errorf("members = %v, want %v", members, want)
	}
	if ttl, err := client.TTL(ctx, state.roomsKey()).Result(); err != nil || ttl <= 0 {
		t.Errorf("ttl rooms = %v, err = %v, want positivo", ttl, err)
	}
	if ttl, err := client.TTL(ctx, state.roomsSeededKey()).Result(); err != nil || ttl <= 0 {
		t.Errorf("ttl seeded = %v, err = %v, want positivo", ttl, err)
	}
	if exists, err := client.Exists(ctx, state.roomsSeedKey()).Result(); err != nil || exists != 0 {
		t.Errorf("marker do cursor = %d, err = %v, want removido na conclusão", exists, err)
	}

	again, err := state.IndexRoomsFromPresence(ctx)

	if err != nil || again != 0 {
		t.Fatalf("segunda chamada = %d, err = %v, want 0 com o índice já semeado", again, err)
	}
}

func TestKVVoiceRoomState_IndexRoomsFromPresenceNaoRepeteDepoisDeSemeado(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	seeded := uuid.New()
	if err := client.HSet(ctx, state.presenceKey(seeded), "sid-a", `{"user_id":"`+uuid.NewString()+`","username":"alice"}`).Err(); err != nil {
		t.Fatalf("HSet erro: %v", err)
	}
	if _, err := state.IndexRoomsFromPresence(ctx); err != nil {
		t.Fatalf("IndexRoomsFromPresence erro: %v", err)
	}
	if err := client.Del(ctx, state.roomsKey()).Err(); err != nil {
		t.Fatalf("Del erro: %v", err)
	}
	late := uuid.New()
	if err := client.HSet(ctx, state.presenceKey(late), "sid-a", `{"user_id":"`+uuid.NewString()+`","username":"alice"}`).Err(); err != nil {
		t.Fatalf("HSet erro: %v", err)
	}

	indexed, err := state.IndexRoomsFromPresence(ctx)

	if err != nil || indexed != 0 {
		t.Fatalf("com a flag semeada = %d, err = %v, want 0 sem varrer de novo", indexed, err)
	}
	if member, err := client.SIsMember(ctx, state.roomsKey(), late.String()).Result(); err != nil || member {
		t.Errorf("member = %v, err = %v, want sala fora do índice", member, err)
	}

	if err := client.Del(ctx, state.roomsSeededKey()).Err(); err != nil {
		t.Fatalf("Del erro: %v", err)
	}
	if _, err := state.IndexRoomsFromPresence(ctx); err != nil {
		t.Fatalf("IndexRoomsFromPresence erro: %v", err)
	}

	if member, err := client.SIsMember(ctx, state.roomsKey(), late.String()).Result(); err != nil || !member {
		t.Fatalf("member = %v, err = %v, want sala indexada após expirar a flag", member, err)
	}
}

func TestKVVoiceRoomState_IndexRoomsFromPresenceRetomaCicloIncompleto(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	state.seedBatchesPerRun = 1
	const rooms = 1200
	pipe := client.Pipeline()
	for range rooms {
		pipe.HSet(ctx, state.presenceKey(uuid.New()), "sid-a", `{"user_id":"`+uuid.NewString()+`","username":"alice"}`)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("HSet erro: %v", err)
	}

	first, err := state.IndexRoomsFromPresence(ctx)

	if !errors.Is(err, errRoomSeedIncomplete) {
		t.Fatalf("primeiro ciclo err = %v, want %v", err, errRoomSeedIncomplete)
	}
	if first <= 0 || first >= rooms {
		t.Fatalf("primeiro ciclo indexed = %d, want parcial", first)
	}
	cursor, err := state.seedCursor(ctx)
	if err != nil || cursor == 0 {
		t.Fatalf("cursor = %d, err = %v, want cursor persistido", cursor, err)
	}
	if exists, err := client.Exists(ctx, state.roomsSeededKey()).Result(); err != nil || exists != 0 {
		t.Fatalf("flag semeada = %d, err = %v, want ausente antes de concluir", exists, err)
	}

	for range rooms {
		if _, err := state.IndexRoomsFromPresence(ctx); err == nil {
			break
		}
	}

	if exists, err := client.Exists(ctx, state.roomsSeededKey()).Result(); err != nil || exists != 1 {
		t.Fatalf("flag semeada = %d, err = %v, want presente após concluir", exists, err)
	}
	if exists, err := client.Exists(ctx, state.roomsSeedKey()).Result(); err != nil || exists != 0 {
		t.Fatalf("marker do cursor = %d, err = %v, want removido após concluir", exists, err)
	}
	members, err := client.SCard(ctx, state.roomsKey()).Result()
	if err != nil {
		t.Fatalf("SCard erro: %v", err)
	}
	if members != rooms {
		t.Errorf("salas no índice = %d, want %d", members, rooms)
	}
}

func TestKVVoiceRoomState_HydrateLogaErroEnquantoIndexacaoIncompleta(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	state.seedBatchesPerRun = 1
	pipe := client.Pipeline()
	for range 1200 {
		pipe.HSet(ctx, state.presenceKey(uuid.New()), "sid-a", `{"user_id":"`+uuid.NewString()+`","username":"alice"}`)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("HSet erro: %v", err)
	}
	buf := captureLog(t)

	mustHydrate(t, newHydrateService(&fakeRoomLister{}, state, &lockedPublisher{}))

	if !strings.Contains(buf.String(), "voice.hydrate level=error step=index_rooms") {
		t.Errorf("log = %q, want erro da indexação incompleta", buf.String())
	}
}

func TestKVVoiceRoomState_RoomsWithStateDescartaMembroFantasma(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	live, phantom := uuid.New(), uuid.New()
	mustJoin(t, state, live, participantOf(uuid.New(), "sid-a"), at(0))
	if err := client.SAdd(ctx, state.roomsKey(), phantom.String()).Err(); err != nil {
		t.Fatalf("SAdd erro: %v", err)
	}

	rooms := mustRoomsWithState(t, state)

	if len(rooms) != 1 || rooms[0] != live {
		t.Fatalf("salas com estado = %v, want só %s", rooms, live)
	}
	if member, err := client.SIsMember(ctx, state.roomsKey(), phantom.String()).Result(); err != nil || member {
		t.Errorf("member fantasma = %v, err = %v, want removido do índice", member, err)
	}
}

func TestKVVoiceRoomState_HydratePreservedEmptyRoomKeepsRoomsTTL(t *testing.T) {
	client := newKVTestClient(t)
	prefix, roomID := "vz:test-"+uuid.NewString(), uuid.New()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-a"), at(0))
	if err := client.PExpire(context.Background(), state.roomsKey(), 500*time.Millisecond).Err(); err != nil {
		t.Fatalf("PExpire erro: %v", err)
	}
	service := newHydrateService(listerWithParticipants(roomID), state, &lockedPublisher{})

	mustHydrate(t, service)

	assertRoomKeysTTL(t, client, []string{state.roomsKey()})
}

func TestKVVoiceRoomState_HydrateIndexesRoomWithRoomsSetPopulated(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	joined, hydrated := uuid.New(), uuid.New()
	mustJoin(t, state, joined, participantOf(uuid.New(), "sid-joined"), at(0))
	service := newHydrateService(listerWithParticipants(hydrated, participantOf(uuid.New(), "sid-a")), state, &lockedPublisher{})

	mustHydrate(t, service)

	if member, err := client.SIsMember(ctx, state.roomsKey(), hydrated.String()).Result(); err != nil || !member {
		t.Fatalf("rooms member = %v, err = %v, want sala hidratada indexada", member, err)
	}
}

func TestKVVoiceRoomState_LeaveOfLastParticipantRemovesRoomFromIndex(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	roomID := uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-a"), at(0))
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-b"), at(0))

	for _, sid := range []string{"sid-a", "sid-b"} {
		if _, err := state.Leave(ctx, LeaveInput{RoomID: roomID, SID: sid, At: at(0)}); err != nil {
			t.Fatalf("Leave erro: %v", err)
		}
	}

	if member, err := client.SIsMember(ctx, state.roomsKey(), roomID.String()).Result(); err != nil || member {
		t.Fatalf("rooms member = %v, err = %v, want removido", member, err)
	}
}

func TestKVVoiceRoomState_HydrateSecondEmptyObservationRemovesRoomFromIndex(t *testing.T) {
	client := newKVTestClient(t)
	prefix := "vz:test-" + uuid.NewString()
	t.Cleanup(func() { removeKeysWithPrefix(t, client, prefix) })

	ctx := context.Background()
	state := NewKVVoiceRoomState(client, prefix).(*kvVoiceRoomState)
	roomID := uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-a"), at(0))
	service := newHydrateService(listerWithParticipants(roomID), state, &lockedPublisher{})

	mustHydrate(t, service)
	mustHydrate(t, service)

	if member, err := client.SIsMember(ctx, state.roomsKey(), roomID.String()).Result(); err != nil || member {
		t.Fatalf("rooms member = %v, err = %v, want removido", member, err)
	}
}
