package presence

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const (
	totalRefreshTimeout   = 5 * time.Second
	cacheOperationTimeout = 2 * time.Second
)

type onlineUser struct {
	Username    string
	Connections int32
}

type Service struct {
	mu          sync.RWMutex
	onlineUsers map[uuid.UUID]*onlineUser

	publisher realtime.Publisher
	stats     UserStatsProvider

	totalCache   TotalCache
	cacheTimeout time.Duration

	total   int64
	totalMu sync.RWMutex
}

type Option func(*Service)

func WithTotalCache(cache TotalCache) Option {
	return func(s *Service) {
		s.totalCache = cache
	}
}

func NewService(
	publisher realtime.Publisher,
	stats UserStatsProvider,
	opts ...Option,
) *Service {
	s := &Service{
		onlineUsers:  make(map[uuid.UUID]*onlineUser),
		publisher:    publisher,
		stats:        stats,
		cacheTimeout: cacheOperationTimeout,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

var _ interface {
	OnConnect(*realtime.Client) []realtime.Envelope
} = (*Service)(nil)

func (s *Service) OnConnect(_ *realtime.Client) []realtime.Envelope {
	return []realtime.Envelope{s.snapshotEnvelope()}
}

func (s *Service) snapshotEnvelope() realtime.Envelope {
	onlineList := s.OnlineUsers()
	online := int64(len(onlineList))

	s.totalMu.RLock()
	total := s.total
	s.totalMu.RUnlock()

	data, _ := json.Marshal(
		PresenceSnapshotPayload{
			Online:      online,
			Offline:     total - online,
			Total:       total,
			OnlineUsers: onlineList,
		},
	)

	return realtime.Envelope{
		V:     1,
		Type:  PresenceSnapshot,
		Topic: realtime.GlobalPresenceTopic,
		TS:    time.Now(),
		Data:  data,
	}
}

func (s *Service) RefreshTotal(ctx context.Context) error {
	previous := s.currentTotal()
	if err := s.LoadTotal(ctx); err != nil {
		return err
	}
	if s.currentTotal() != previous {
		s.broadcastTotal(ctx)
	}
	return nil
}

func (s *Service) LoadTotal(ctx context.Context) error {
	if s.totalCache == nil {
		return s.countTotal(ctx)
	}

	getCtx, cancel := context.WithTimeout(ctx, s.cacheTimeout)
	total, found, err := s.totalCache.Get(getCtx)
	cancel()
	if err != nil {
		log.Printf("presence.total_cache_get_failed level=warn err=%q", err)
	}
	if err == nil && found {
		s.setTotal(total)
		return nil
	}

	if err := s.countTotal(ctx); err != nil {
		return err
	}
	s.cacheTotal(ctx, s.currentTotal(), false)
	return nil
}

func (s *Service) RunTotalRefresh(ctx context.Context, interval time.Duration) {
	if s.totalCache == nil {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(ctx, totalRefreshTimeout)
			if err := s.RefreshTotal(refreshCtx); err != nil {
				log.Printf("presence.total_refresh_failed level=error err=%q", err)
			}
			cancel()
		}
	}
}

func (s *Service) UserCreated(ctx context.Context) {
	if s.totalCache == nil {
		s.addToTotal(1)
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), totalRefreshTimeout)
	defer cancel()

	if err := s.countTotal(ctx); err != nil {
		log.Printf("presence.total_count_failed level=error err=%q", err)
		s.addToTotal(1)
		s.broadcastTotal(ctx)
		return
	}

	s.cacheTotal(ctx, s.currentTotal(), true)
	s.broadcastTotal(ctx)
}

func (s *Service) TotalChanged(ctx context.Context, total int64) {
	if s.currentTotal() == total {
		return
	}
	s.setTotal(total)
	s.broadcastTotal(ctx)
}

func (s *Service) countTotal(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, totalRefreshTimeout)
	defer cancel()

	total, err := s.stats.TotalUsers(ctx)
	if err != nil {
		return err
	}
	s.setTotal(total)
	return nil
}

func (s *Service) cacheTotal(ctx context.Context, total int64, publish bool) {
	s.cacheCall(ctx, "set", func(callCtx context.Context) error { return s.totalCache.Set(callCtx, total) })
	if !publish {
		return
	}
	s.cacheCall(ctx, "publish", func(callCtx context.Context) error { return s.totalCache.Publish(callCtx, total) })
}

func (s *Service) cacheCall(ctx context.Context, op string, call func(context.Context) error) {
	callCtx, cancel := context.WithTimeout(ctx, s.cacheTimeout)
	defer cancel()

	if err := call(callCtx); err != nil {
		log.Printf("presence.total_cache_%s_failed level=warn err=%q", op, err)
	}
}

func (s *Service) broadcastTotal(ctx context.Context) {
	env := s.snapshotEnvelope()
	if err := s.publisher.Publish(ctx, realtime.GlobalPresenceTopic, env); err != nil {
		log.Printf("presence.total_broadcast_failed level=warn err=%q", err)
	}
}

func (s *Service) currentTotal() int64 {
	s.totalMu.RLock()
	defer s.totalMu.RUnlock()
	return s.total
}

func (s *Service) setTotal(total int64) {
	s.totalMu.Lock()
	s.total = total
	s.totalMu.Unlock()
}

func (s *Service) addToTotal(delta int64) {
	s.totalMu.Lock()
	s.total += delta
	s.totalMu.Unlock()
}

func (s *Service) HandleClientConnected(
	userID uuid.UUID,
	username string,
) {
	s.mu.Lock()

	u, exists := s.onlineUsers[userID]

	if !exists {
		u = &onlineUser{
			Username: username,
		}

		s.onlineUsers[userID] = u
	} else {
		u.Username = username
	}

	u.Connections++

	isFirstConn := u.Connections == 1

	s.mu.Unlock()

	if !isFirstConn {
		return
	}

	data, _ := json.Marshal(
		realtime.UserPresence{
			UserID:   userID,
			Username: username,
		},
	)

	s.publisher.Publish(
		context.Background(),
		realtime.GlobalPresenceTopic,
		realtime.Envelope{
			V:     1,
			Type:  UserOnline,
			Topic: realtime.GlobalPresenceTopic,
			TS:    time.Now(),
			Data:  data,
		},
	)
}

func (s *Service) HandleClientDisconnected(
	userID uuid.UUID,
	username string,
) {
	s.mu.Lock()

	u, exists := s.onlineUsers[userID]

	if !exists {
		s.mu.Unlock()
		return
	}

	u.Connections--

	wasLast := u.Connections <= 0

	if wasLast {
		delete(s.onlineUsers, userID)
	}

	s.mu.Unlock()

	if !wasLast {
		return
	}

	data, _ := json.Marshal(
		realtime.UserPresence{
			UserID:   userID,
			Username: username,
		},
	)

	s.publisher.Publish(
		context.Background(),
		realtime.GlobalPresenceTopic,
		realtime.Envelope{
			V:     1,
			Type:  UserOffline,
			Topic: realtime.GlobalPresenceTopic,
			TS:    time.Now(),
			Data:  data,
		},
	)
}

func (s *Service) OnlineUsers() []realtime.UserPresence {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make(
		[]realtime.UserPresence,
		0,
		len(s.onlineUsers),
	)

	for id, u := range s.onlineUsers {
		users = append(
			users,
			realtime.UserPresence{
				UserID:   id,
				Username: u.Username,
			},
		)
	}

	return users
}
