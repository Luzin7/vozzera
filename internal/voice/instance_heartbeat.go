package voice

import (
	"context"
	"log"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const (
	heartbeatInterval         = 10 * time.Second
	sweepInterval             = 30 * time.Second
	presenceReconcileInterval = 30 * time.Second
	hydrationBackoffMax       = 5 * time.Minute
)

type presenceHydrator interface {
	Execute(ctx context.Context) error
}

type InstanceHeartbeatDeps struct {
	State       VoiceRoomState
	InstanceID  uuid.UUID
	Hydrator    presenceHydrator
	Publisher   realtime.Publisher
	StartedDown bool
}

type InstanceHeartbeat struct {
	state      VoiceRoomState
	instanceID uuid.UUID
	hydrator   presenceHydrator
	events     *awayEvents
	interval   time.Duration
	down       bool
	now        func() time.Time

	hydrationAttempts int
	hydrationBackoff  time.Duration
	nextHydration     time.Time
}

func NewInstanceHeartbeat(deps InstanceHeartbeatDeps) *InstanceHeartbeat {
	return &InstanceHeartbeat{
		state:      deps.State,
		instanceID: deps.InstanceID,
		hydrator:   deps.Hydrator,
		events:     newAwayEvents(deps.State, deps.Publisher),
		interval:   heartbeatInterval,
		down:       deps.StartedDown,
		now:        time.Now,
	}
}

func (h *InstanceHeartbeat) Run(ctx context.Context) {
	h.tick(ctx)

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.tick(ctx)
		}
	}
}

func (h *InstanceHeartbeat) tick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}

	result, err := h.state.Beat(ctx, h.instanceID)
	if err != nil {
		h.markDown(err)
		return
	}

	h.markUp()
	if result.Wiped {
		log.Printf("voice.kv_wiped instance_id=%s", h.instanceID)
		h.scheduleHydrationNow()
		h.hydrate(ctx)
		h.events.publishResync(ctx)
		return
	}
	if h.hydrationDue() && h.hydrate(ctx) {
		h.events.publishResync(ctx)
	}
}

func (h *InstanceHeartbeat) hydrationDue() bool {
	return h.nextHydration.IsZero() || !h.now().Before(h.nextHydration)
}

func (h *InstanceHeartbeat) scheduleHydrationNow() {
	h.hydrationAttempts = 0
	h.hydrationBackoff = 0
	h.nextHydration = time.Time{}
}

func (h *InstanceHeartbeat) hydrate(ctx context.Context) bool {
	h.hydrationAttempts++
	now := h.now()
	if err := h.hydrator.Execute(ctx); err != nil {
		h.hydrationBackoff = min(max(h.hydrationBackoff*2, heartbeatInterval), hydrationBackoffMax)
		h.nextHydration = now.Add(h.hydrationBackoff)
		log.Printf("voice.hydrate level=error attempts=%d retry_in=%s err=%q", h.hydrationAttempts, h.hydrationBackoff, err)
		return false
	}

	if h.hydrationAttempts > 1 {
		log.Printf("voice.hydrate recovered attempts=%d", h.hydrationAttempts)
	}
	h.hydrationAttempts = 0
	h.hydrationBackoff = 0
	h.nextHydration = now.Add(presenceReconcileInterval)
	return true
}

func (h *InstanceHeartbeat) markDown(err error) {
	if h.down {
		return
	}
	h.down = true
	log.Printf("kv.state level=error transition=down err=%q", err)
}

func (h *InstanceHeartbeat) markUp() {
	if !h.down {
		return
	}
	h.down = false
	log.Printf("kv.state transition=up")
}
