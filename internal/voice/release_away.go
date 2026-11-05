package voice

import (
	"context"
	"log"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type ReleaseAwayInput struct {
	UserID uuid.UUID
	Owner  AwayOwner
}

type ReleaseAwayService struct {
	state  VoiceRoomState
	events *awayEvents
}

func NewReleaseAwayService(state VoiceRoomState, publisher realtime.Publisher) *ReleaseAwayService {
	return &ReleaseAwayService{state: state, events: newAwayEvents(state, publisher)}
}

func (s *ReleaseAwayService) Execute(ctx context.Context, in ReleaseAwayInput) error {
	removed, err := s.state.ReleaseAway(ctx, in.UserID, in.Owner)
	if err != nil {
		log.Printf("voice.away.error level=error op=release user_id=%s room_id= err=%q", in.UserID, err)
		return ErrVoiceStateUnavailable(err)
	}
	if removed == nil {
		return nil
	}

	logAwayCleared(*removed, "connection_closed")
	if err := s.events.publishCurrentAwayList(ctx, removed.RoomID); err != nil {
		return ErrVoiceStateUnavailable(err)
	}
	s.events.publishEnded(ctx, removed.RoomID, removed.UserID, AwayEndedConnectionClosed)
	return nil
}

type awayReleaser interface {
	Execute(ctx context.Context, in ReleaseAwayInput) error
}

const releaseOperationTimeout = 10 * time.Second

var defaultReleaseRetryDelays = []time.Duration{100 * time.Millisecond, 300 * time.Millisecond, 900 * time.Millisecond}

type ReleaseAwayOnCloseHook struct {
	release     awayReleaser
	instanceID  uuid.UUID
	retryDelays []time.Duration
}

var _ realtime.ConnectionClosedHook = (*ReleaseAwayOnCloseHook)(nil)

func NewReleaseAwayOnCloseHook(release awayReleaser, instanceID uuid.UUID) *ReleaseAwayOnCloseHook {
	return &ReleaseAwayOnCloseHook{release: release, instanceID: instanceID, retryDelays: defaultReleaseRetryDelays}
}

func (h *ReleaseAwayOnCloseHook) HandleConnectionClosed(c *realtime.Client) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf("voice.away.error level=error op=release user_id=%s room_id= err=%q", c.UserID, recovered)
		}
	}()

	in := ReleaseAwayInput{UserID: c.UserID, Owner: AwayOwner{InstanceID: h.instanceID, ConnID: c.ConnID}}

	err := h.attempt(in)
	for attempt, delay := range h.retryDelays {
		if err == nil {
			return
		}
		log.Printf("voice.away.release_retry attempt=%d user_id=%s err=%q", attempt+1, c.UserID, err)
		time.Sleep(delay)
		err = h.attempt(in)
	}
	if err != nil {
		log.Printf("voice.away.error level=error op=release user_id=%s room_id= err=%q", c.UserID, err)
	}
}

func (h *ReleaseAwayOnCloseHook) attempt(in ReleaseAwayInput) error {
	ctx, cancel := context.WithTimeout(context.Background(), releaseOperationTimeout)
	defer cancel()

	return h.release.Execute(ctx, in)
}
