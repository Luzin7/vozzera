package voice

import (
	"sync"
	"time"
)

const recentLeaveTTL = 2 * time.Minute

type RecentLeaves struct {
	now func() time.Time

	mu   sync.Mutex
	sids map[string]time.Time
}

func NewRecentLeaves() *RecentLeaves {
	return &RecentLeaves{now: time.Now, sids: make(map[string]time.Time)}
}

func (r *RecentLeaves) Record(sid string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	for known, at := range r.sids {
		if now.Sub(at) >= recentLeaveTTL {
			delete(r.sids, known)
		}
	}
	r.sids[sid] = now
}

func (r *RecentLeaves) Consume(sid string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	at, ok := r.sids[sid]
	if !ok {
		return false
	}
	delete(r.sids, sid)
	return r.now().Sub(at) < recentLeaveTTL
}
