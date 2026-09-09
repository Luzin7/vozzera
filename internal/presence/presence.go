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
	publisher   realtime.Publisher
}

func NewService(publisher realtime.Publisher) *Service {
	return &Service{
		onlineUsers: make(map[uuid.UUID]*onlineUser),
		publisher:   publisher,
	}
}

var _ realtime.PresenceHook = (*Service)(nil)

func (s *Service) HandleClientConnected(userID uuid.UUID, username string) {
	s.mu.Lock()
	u, exists := s.onlineUsers[userID]
	if !exists {
		u = &onlineUser{Username: username}
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

	data, _ := json.Marshal(realtime.UserPresence{UserID: userID, Username: username})
	s.publisher.Publish(context.Background(), realtime.GlobalPresenceTopic, realtime.Envelope{
		V: 1, Type: UserOnline, Topic: realtime.GlobalPresenceTopic,
		TS: time.Now(), Data: data,
	})
}

func (s *Service) HandleClientDisconnected(userID uuid.UUID, username string) {
	s.mu.Lock()
	u, ok := s.onlineUsers[userID]
	if !ok {
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

	data, _ := json.Marshal(realtime.UserPresence{UserID: userID, Username: username})
	s.publisher.Publish(context.Background(), realtime.GlobalPresenceTopic, realtime.Envelope{
		V: 1, Type: UserOffline, Topic: realtime.GlobalPresenceTopic,
		TS: time.Now(), Data: data,
	})
}

func (s *Service) HandleTopicSubscribed(topic realtime.Topic) []byte {
	if topic != realtime.GlobalPresenceTopic {
		return nil
	}

	s.mu.RLock()
	users := make([]realtime.UserPresence, 0, len(s.onlineUsers))
	for id, u := range s.onlineUsers {
		users = append(users, realtime.UserPresence{UserID: id, Username: u.Username})
	}
	s.mu.RUnlock()

	data, _ := json.Marshal(users)
	env, _ := json.Marshal(realtime.Envelope{
		V: 1, Type: PresenceSnapshot, Topic: realtime.GlobalPresenceTopic,
		TS: time.Now(), Data: data,
	})
	return env
}

func (s *Service) OnlineUsers() []realtime.UserPresence {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]realtime.UserPresence, 0, len(s.onlineUsers))
	for id, u := range s.onlineUsers {
		users = append(users, realtime.UserPresence{UserID: id, Username: u.Username})
	}
	return users
}
