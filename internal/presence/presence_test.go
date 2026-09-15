package presence

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type fakePublisher struct {
	mu        sync.Mutex
	envelopes []realtime.Envelope
	topics    []realtime.Topic
}

func (f *fakePublisher) Publish(_ context.Context, topic realtime.Topic, env realtime.Envelope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.topics = append(f.topics, topic)
	f.envelopes = append(f.envelopes, env)
	return nil
}

func (f *fakePublisher) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.envelopes)
}

func (f *fakePublisher) Last() (realtime.Topic, realtime.Envelope) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.envelopes) == 0 {
		return "", realtime.Envelope{}
	}
	return f.topics[len(f.topics)-1], f.envelopes[len(f.envelopes)-1]
}

var _ realtime.Publisher = (*fakePublisher)(nil)

type fakeStats struct{}

func (f *fakeStats) TotalUsers(_ context.Context) (int64, error) {
	return 100, nil
}

var _ UserStatsProvider = (*fakeStats)(nil)

func TestService_HandleClientConnected_FirstConnectionBroadcastsOnline(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &fakeStats{})
	userID := uuid.New()

	svc.HandleClientConnected(userID, "alice")

	if pub.Len() != 1 {
		t.Fatalf("envelopes = %d, want 1", pub.Len())
	}
	topic, env := pub.Last()
	if topic != realtime.GlobalPresenceTopic {
		t.Errorf("topic = %q, want %q", topic, realtime.GlobalPresenceTopic)
	}
	if env.Type != UserOnline {
		t.Errorf("type = %q, want %q", env.Type, UserOnline)
	}
}

func TestService_HandleClientConnected_SecondConnectionDoesNotBroadcast(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &fakeStats{})
	userID := uuid.New()

	svc.HandleClientConnected(userID, "alice")
	svc.HandleClientConnected(userID, "alice")

	if pub.Len() != 1 {
		t.Fatalf("envelopes = %d, want 1 (segunda conexÃ£o nÃ£o notifica)", pub.Len())
	}
}

func TestService_HandleClientDisconnected_LastLeavesBroadcastsOffline(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &fakeStats{})
	userID := uuid.New()

	svc.HandleClientConnected(userID, "alice")
	svc.HandleClientConnected(userID, "alice")
	svc.HandleClientDisconnected(userID, "alice")
	svc.HandleClientDisconnected(userID, "alice")

	if pub.Len() != 2 {
		t.Fatalf("envelopes = %d, want 2 (online + offline)", pub.Len())
	}
	topic, env := pub.Last()
	if topic != realtime.GlobalPresenceTopic {
		t.Errorf("topic = %q, want %q", topic, realtime.GlobalPresenceTopic)
	}
	if env.Type != UserOffline {
		t.Errorf("type = %q, want %q", env.Type, UserOffline)
	}
}

func TestService_HandleClientDisconnected_StillHasConnections(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &fakeStats{})
	userID := uuid.New()

	svc.HandleClientConnected(userID, "alice")
	svc.HandleClientConnected(userID, "alice")
	svc.HandleClientDisconnected(userID, "alice")

	if pub.Len() != 1 {
		t.Fatalf("envelopes = %d, want 1 (sÃ³ online, sem offline)", pub.Len())
	}
}

func TestService_OnConnect_ReturnsSnapshot(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &fakeStats{})

	svc.RefreshTotal(context.Background())
	svc.HandleClientConnected(uuid.New(), "alice")

	client := realtime.NewClient(nil, nil, uuid.New(), "", uuid.New(), nil)
	envelopes := svc.OnConnect(client)

	if len(envelopes) == 0 {
		t.Fatal("OnConnect retornou 0 envelopes, esperava 1")
	}
	env := envelopes[0]
	if env.Type != PresenceSnapshot {
		t.Errorf("type = %q, want %q", env.Type, PresenceSnapshot)
	}
	if env.Topic != realtime.GlobalPresenceTopic {
		t.Errorf("topic = %q, want %q", env.Topic, realtime.GlobalPresenceTopic)
	}

	var payload PresenceSnapshotPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	if payload.Online != 1 {
		t.Errorf("online = %d, want 1", payload.Online)
	}
	if payload.Total != 100 {
		t.Errorf("total = %d, want 100", payload.Total)
	}
}

func TestService_OnConnect_Empty(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &fakeStats{})

	svc.RefreshTotal(context.Background())

	client := realtime.NewClient(nil, nil, uuid.New(), "", uuid.New(), nil)
	envelopes := svc.OnConnect(client)

	if len(envelopes) == 0 {
		t.Fatal("OnConnect retornou 0 envelopes, esperava 1")
	}
	env := envelopes[0]

	var payload PresenceSnapshotPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	if payload.Online != 0 {
		t.Errorf("online = %d, want 0", payload.Online)
	}
	if payload.Total != 100 {
		t.Errorf("total = %d, want 100", payload.Total)
	}
}

func TestService_OnlineUsers_ReturnsOnline(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &fakeStats{})
	uid1 := uuid.New()
	uid2 := uuid.New()

	svc.HandleClientConnected(uid1, "alice")
	svc.HandleClientConnected(uid2, "bob")

	users := svc.OnlineUsers()
	if len(users) != 2 {
		t.Fatalf("OnlineUsers len = %d, want 2", len(users))
	}
}
