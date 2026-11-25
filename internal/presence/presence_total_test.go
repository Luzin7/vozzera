package presence

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

type countingStats struct {
	total int64
	calls int
	err   error
}

func (c *countingStats) TotalUsers(context.Context) (int64, error) {
	c.calls++
	return c.total, c.err
}

func (c *countingStats) ListUsers(context.Context) ([]realtime.UserPresence, error) {
	return nil, nil
}

type fakeTotalCache struct {
	value      int64
	found      bool
	getErr     error
	setErr     error
	publishErr error
	sets       []int64
	published  []int64
}

func (f *fakeTotalCache) Get(context.Context) (int64, bool, error) {
	return f.value, f.found, f.getErr
}

func (f *fakeTotalCache) Set(_ context.Context, total int64) error {
	f.sets = append(f.sets, total)
	return f.setErr
}

func (f *fakeTotalCache) Publish(_ context.Context, total int64) error {
	f.published = append(f.published, total)
	return f.publishErr
}

func snapshotTotal(t *testing.T, svc *Service) int64 {
	t.Helper()
	var snapshot PresenceSnapshotPayload
	if err := json.Unmarshal(svc.OnConnect(nil)[0].Data, &snapshot); err != nil {
		t.Fatalf("Unmarshal erro: %v", err)
	}
	return snapshot.Total
}

func TestService_RefreshTotalCacheMissCountsAndSets(t *testing.T) {
	stats := &countingStats{total: 42}
	cache := &fakeTotalCache{}
	svc := NewService(&fakePublisher{}, stats, WithTotalCache(cache))

	if err := svc.RefreshTotal(context.Background()); err != nil {
		t.Fatalf("RefreshTotal erro: %v", err)
	}

	if stats.calls != 1 {
		t.Errorf("COUNT calls = %d, want 1", stats.calls)
	}
	if len(cache.sets) != 1 || cache.sets[0] != 42 {
		t.Errorf("sets = %v, want [42]", cache.sets)
	}
	if len(cache.published) != 0 {
		t.Errorf("published = %v, want none", cache.published)
	}
}

func TestService_RefreshTotalCacheHitSkipsDatabase(t *testing.T) {
	stats := &countingStats{total: 42}
	cache := &fakeTotalCache{value: 77, found: true}
	svc := NewService(&fakePublisher{}, stats, WithTotalCache(cache))

	if err := svc.RefreshTotal(context.Background()); err != nil {
		t.Fatalf("RefreshTotal erro: %v", err)
	}

	if stats.calls != 0 {
		t.Errorf("COUNT calls = %d, want 0", stats.calls)
	}
	if got := snapshotTotal(t, svc); got != 77 {
		t.Errorf("Total = %d, want 77", got)
	}
}

func TestService_UserCreatedSetsAndPublishesTotal(t *testing.T) {
	stats := &countingStats{total: 43}
	cache := &fakeTotalCache{}
	svc := NewService(&fakePublisher{}, stats, WithTotalCache(cache))

	svc.UserCreated(context.Background())

	if len(cache.sets) != 1 || cache.sets[0] != 43 {
		t.Errorf("sets = %v, want [43]", cache.sets)
	}
	if len(cache.published) != 1 || cache.published[0] != 43 {
		t.Errorf("published = %v, want [43]", cache.published)
	}
	if got := snapshotTotal(t, svc); got != 43 {
		t.Errorf("Total = %d, want 43", got)
	}
}

func TestService_UserCreatedSurvivesCacheFailure(t *testing.T) {
	stats := &countingStats{total: 43}
	cache := &fakeTotalCache{setErr: errors.New("down"), publishErr: errors.New("down")}
	pub := &fakePublisher{}
	svc := NewService(pub, stats, WithTotalCache(cache))

	svc.UserCreated(context.Background())

	if got := snapshotTotal(t, svc); got != 43 {
		t.Errorf("Total = %d, want 43", got)
	}
	if pub.Len() != 1 {
		t.Errorf("broadcasts = %d, want 1", pub.Len())
	}
}

func TestService_TotalChangedUpdatesAndBroadcastsSnapshot(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &countingStats{}, WithTotalCache(&fakeTotalCache{}))

	svc.TotalChanged(context.Background(), 55)

	topic, env := pub.Last()
	if topic != realtime.GlobalPresenceTopic || env.Type != PresenceSnapshot {
		t.Fatalf("broadcast = %q %q, want global presence.snapshot", topic, env.Type)
	}
	if got := snapshotTotal(t, svc); got != 55 {
		t.Errorf("Total = %d, want 55", got)
	}
}

func TestService_TotalChangedSameValueDoesNotBroadcast(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &countingStats{}, WithTotalCache(&fakeTotalCache{}))
	svc.TotalChanged(context.Background(), 55)

	svc.TotalChanged(context.Background(), 55)

	if pub.Len() != 1 {
		t.Errorf("broadcasts = %d, want 1", pub.Len())
	}
}

func TestService_UserCreatedCountFailureBumpsLocallyWithoutCaching(t *testing.T) {
	stats := &countingStats{total: 10, err: errors.New("db down")}
	cache := &fakeTotalCache{}
	pub := &fakePublisher{}
	svc := NewService(pub, stats, WithTotalCache(cache))
	svc.setTotal(10)

	svc.UserCreated(context.Background())

	if got := snapshotTotal(t, svc); got != 11 {
		t.Errorf("Total = %d, want 11", got)
	}
	if pub.Len() != 1 {
		t.Errorf("broadcasts = %d, want 1", pub.Len())
	}
	if len(cache.sets) != 0 || len(cache.published) != 0 {
		t.Errorf("sets = %v published = %v, want none", cache.sets, cache.published)
	}
}

func TestService_UserCreatedAfterCountFailureSetsAbsoluteTotal(t *testing.T) {
	stats := &countingStats{total: 11, err: errors.New("db down")}
	cache := &fakeTotalCache{}
	svc := NewService(&fakePublisher{}, stats, WithTotalCache(cache))
	svc.setTotal(10)
	svc.UserCreated(context.Background())
	stats.err = nil
	stats.total = 12

	svc.UserCreated(context.Background())

	if got := snapshotTotal(t, svc); got != 12 {
		t.Errorf("Total = %d, want 12", got)
	}
	if len(cache.sets) != 1 || cache.sets[0] != 12 {
		t.Errorf("sets = %v, want [12]", cache.sets)
	}
}

func TestService_RefreshTotalAfterCountFailureSetsAbsoluteTotal(t *testing.T) {
	stats := &countingStats{total: 12, err: errors.New("db down")}
	cache := &fakeTotalCache{}
	svc := NewService(&fakePublisher{}, stats, WithTotalCache(cache))
	svc.setTotal(10)
	svc.UserCreated(context.Background())
	stats.err = nil

	if err := svc.RefreshTotal(context.Background()); err != nil {
		t.Fatalf("RefreshTotal erro: %v", err)
	}

	if got := snapshotTotal(t, svc); got != 12 {
		t.Errorf("Total = %d, want 12", got)
	}
}

func TestService_RefreshTotalBroadcastsWhenValueChanged(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &countingStats{}, WithTotalCache(&fakeTotalCache{value: 77, found: true}))

	if err := svc.RefreshTotal(context.Background()); err != nil {
		t.Fatalf("RefreshTotal erro: %v", err)
	}

	if pub.Len() != 1 {
		t.Errorf("broadcasts = %d, want 1", pub.Len())
	}
}

func TestService_RefreshTotalSameValueDoesNotBroadcast(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &countingStats{}, WithTotalCache(&fakeTotalCache{value: 77, found: true}))
	svc.setTotal(77)

	if err := svc.RefreshTotal(context.Background()); err != nil {
		t.Fatalf("RefreshTotal erro: %v", err)
	}

	if pub.Len() != 0 {
		t.Errorf("broadcasts = %d, want 0", pub.Len())
	}
}

func TestService_LoadTotalDoesNotBroadcast(t *testing.T) {
	pub := &fakePublisher{}
	svc := NewService(pub, &countingStats{total: 5})

	if err := svc.LoadTotal(context.Background()); err != nil {
		t.Fatalf("LoadTotal erro: %v", err)
	}

	if pub.Len() != 0 {
		t.Errorf("broadcasts = %d, want 0", pub.Len())
	}
}

func TestService_RunTotalRefreshReturnsOnCancel(t *testing.T) {
	svc := NewService(&fakePublisher{}, &countingStats{}, WithTotalCache(&fakeTotalCache{}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		svc.RunTotalRefresh(ctx, time.Hour)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunTotalRefresh did not return on cancel")
	}
}

func TestService_RunTotalRefreshWithoutCacheReturnsImmediately(t *testing.T) {
	svc := NewService(&fakePublisher{}, &countingStats{})
	done := make(chan struct{})

	go func() {
		svc.RunTotalRefresh(context.Background(), time.Hour)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunTotalRefresh blocked without cache")
	}
}

func TestService_RunTotalRefreshTickRefreshesTotal(t *testing.T) {
	cache := &fakeTotalCache{value: 99, found: true}
	pub := &fakePublisher{}
	svc := NewService(pub, &countingStats{}, WithTotalCache(cache))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go svc.RunTotalRefresh(ctx, 5*time.Millisecond)

	deadline := time.After(2 * time.Second)
	for pub.Len() == 0 {
		select {
		case <-deadline:
			t.Fatal("tick did not refresh total")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if got := snapshotTotal(t, svc); got != 99 {
		t.Errorf("Total = %d, want 99", got)
	}
}

type blockingGetCache struct {
	fakeTotalCache
}

func (b *blockingGetCache) Get(ctx context.Context) (int64, bool, error) {
	<-ctx.Done()
	return 0, false, ctx.Err()
}

func TestService_LoadTotalFallsBackToCountWhenCacheGetBlocks(t *testing.T) {
	stats := &countingStats{total: 42}
	svc := NewService(&fakePublisher{}, stats, WithTotalCache(&blockingGetCache{}))
	svc.cacheTimeout = 10 * time.Millisecond

	if err := svc.LoadTotal(context.Background()); err != nil {
		t.Fatalf("LoadTotal erro: %v", err)
	}

	if stats.calls != 1 {
		t.Errorf("COUNT calls = %d, want 1", stats.calls)
	}
	if got := snapshotTotal(t, svc); got != 42 {
		t.Errorf("Total = %d, want 42", got)
	}
}

func TestService_LoadTotalFallsBackToCountWhenCacheGetFails(t *testing.T) {
	stats := &countingStats{total: 42}
	svc := NewService(&fakePublisher{}, stats, WithTotalCache(&fakeTotalCache{getErr: errors.New("kv down")}))

	if err := svc.LoadTotal(context.Background()); err != nil {
		t.Fatalf("LoadTotal erro: %v", err)
	}

	if got := snapshotTotal(t, svc); got != 42 {
		t.Errorf("Total = %d, want 42", got)
	}
}
