package presence

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
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

	total   int64
	totalMu sync.RWMutex
}

func NewService(
	publisher realtime.Publisher,
	stats UserStatsProvider,
) *Service {
	return &Service{
		onlineUsers: make(map[uuid.UUID]*onlineUser),
		publisher:   publisher,
		stats:       stats,
	}
}

var _ interface {
	OnConnect(*realtime.Client) []realtime.Envelope
} = (*Service)(nil)

func (s *Service) OnConnect(_ *realtime.Client) []realtime.Envelope {
	s.mu.RLock()

	online := int64(len(s.onlineUsers))

	onlineList := make(
		[]realtime.UserPresence,
		0,
		online,
	)

	for id, u := range s.onlineUsers {
		onlineList = append(
			onlineList,
			realtime.UserPresence{
				UserID:   id,
				Username: u.Username,
			},
		)
	}

	s.mu.RUnlock()

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

	return []realtime.Envelope{
		{
			V:     1,
			Type:  PresenceSnapshot,
			Topic: realtime.GlobalPresenceTopic,
			TS:    time.Now(),
			Data:  data,
		},
	}
}

func (s *Service) RefreshTotal(ctx context.Context) error {
	total, err := s.stats.TotalUsers(ctx)
	if err != nil {
		return err
	}

	s.totalMu.Lock()
	s.total = total
	s.totalMu.Unlock()

	return nil
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
