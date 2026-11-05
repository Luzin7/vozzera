package voice

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func TestMemoryVoiceRoomState_Contract(t *testing.T) {
	runVoiceRoomStateContract(t, func(t *testing.T) contractHarness {
		state := newMemoryVoiceRoomState((&fakeClock{now: contractEpoch}).Now)
		return contractHarness{state: state, expireInstance: state.expireInstance}
	})
}

func TestMemoryVoiceRoomState_HeartbeatExpiraAposTTL(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: contractEpoch}
	state := newMemoryVoiceRoomState(clock.Now)
	roomID, caller, other, instance := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(other, "sid-b"), at(0))
	state.Beat(ctx, instance)
	mustEnterAway(t, state, newAwayEntry(roomID, forUser(caller), ownedBy(AwayOwner{InstanceID: instance, ConnID: uuid.New()})))

	clock.Advance(memoryHeartbeatTTL - time.Millisecond)
	removed, _ := state.SweepDeadOwners(ctx)
	if len(removed) != 0 {
		t.Fatalf("removed = %+v antes do TTL, want nenhum", removed)
	}

	clock.Advance(time.Millisecond)
	removed, _ = state.SweepDeadOwners(ctx)
	if len(removed) != 1 {
		t.Fatalf("removed = %+v após o TTL, want 1", removed)
	}
}

func TestMemoryVoiceRoomState_BeatRenovaHeartbeat(t *testing.T) {
	ctx := context.Background()
	clock := &fakeClock{now: contractEpoch}
	state := newMemoryVoiceRoomState(clock.Now)
	roomID, caller, other, instance := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(other, "sid-b"), at(0))
	state.Beat(ctx, instance)
	mustEnterAway(t, state, newAwayEntry(roomID, forUser(caller), ownedBy(AwayOwner{InstanceID: instance, ConnID: uuid.New()})))

	clock.Advance(memoryHeartbeatTTL - time.Second)
	state.Beat(ctx, instance)
	clock.Advance(memoryHeartbeatTTL - time.Second)
	removed, _ := state.SweepDeadOwners(ctx)

	if len(removed) != 0 {
		t.Fatalf("removed = %+v, want nenhum após renovar o heartbeat", removed)
	}
}
