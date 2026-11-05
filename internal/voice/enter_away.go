package voice

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

const (
	AwayReasonIdle       = "idle"
	AwayReasonConnection = "connection"

	roomAuthorizationTimeout = 3 * time.Second
	orphanReleaseTimeout     = kvOperationTimeout + publishTimeout
)

type EnterAwayInput struct {
	UserID   uuid.UUID
	Username string
	RoomID   uuid.UUID
	Reason   string
	Owner    AwayOwner
	ConnCtx  context.Context
}

type EnterAwayService struct {
	state      VoiceRoomState
	authorizer realtime.SubscriptionAuthorizer
	events     *awayEvents
	now        func() time.Time
}

func NewEnterAwayService(state VoiceRoomState, authorizer realtime.SubscriptionAuthorizer, publisher realtime.Publisher) *EnterAwayService {
	return &EnterAwayService{state: state, authorizer: authorizer, events: newAwayEvents(state, publisher), now: time.Now}
}

func (s *EnterAwayService) Execute(ctx context.Context, in EnterAwayInput) error {
	if in.Reason != AwayReasonIdle && in.Reason != AwayReasonConnection {
		return ErrInvalidAwayReason
	}
	if in.RoomID == uuid.Nil {
		return ErrInvalidAwayRoom
	}
	if err := s.authorize(ctx, in); err != nil {
		return err
	}

	result, err := s.state.EnterAway(ctx, AwayEntry{
		RoomID:   in.RoomID,
		UserID:   in.UserID,
		Username: in.Username,
		Since:    s.now().UTC(),
		Owner:    in.Owner,
	})
	if err != nil {
		log.Printf("voice.away.error level=error op=enter user_id=%s room_id=%s err=%q", in.UserID, in.RoomID, err)
		return ErrVoiceStateUnavailable(err)
	}
	if !result.Accepted {
		log.Printf("voice.away.rejected user_id=%s room_id=%s cause=room_empty", in.UserID, in.RoomID)
		s.events.publishEnded(ctx, in.RoomID, in.UserID, AwayEndedRoomEmpty)
		return nil
	}

	return s.announceAccepted(ctx, in, result)
}

func (s *EnterAwayService) authorize(ctx context.Context, in EnterAwayInput) error {
	ctx, cancel := context.WithTimeout(ctx, roomAuthorizationTimeout)
	defer cancel()

	err := s.authorizer.CanSubscribe(ctx, in.UserID.String(), realtime.RoomTopic(in.RoomID))
	if err == nil {
		return nil
	}
	if isAccessDenied(err) {
		log.Printf("voice.away.denied user_id=%s room_id=%s", in.UserID, in.RoomID)
		return ErrAwayRoomDenied
	}
	log.Printf("voice.away.error level=error op=enter user_id=%s room_id=%s err=%q", in.UserID, in.RoomID, err)
	return ErrVoiceStateUnavailable(err)
}

func (s *EnterAwayService) announceAccepted(ctx context.Context, in EnterAwayInput, result EnterAwayResult) error {
	if in.ConnCtx.Err() != nil {
		return s.releaseOrphan(ctx, in, result)
	}

	movedFrom := ""
	if result.MovedFrom != nil {
		movedFrom = result.MovedFrom.String()
	}
	log.Printf("voice.away.entered user_id=%s room_id=%s reason=%s conn_id=%s moved_from=%s", in.UserID, in.RoomID, in.Reason, in.Owner.ConnID, movedFrom)

	if err := s.events.publishCurrentAwayList(ctx, in.RoomID); err != nil {
		return ErrVoiceStateUnavailable(err)
	}
	if result.MovedFrom == nil {
		return nil
	}
	if err := s.events.publishCurrentAwayList(ctx, *result.MovedFrom); err != nil {
		return ErrVoiceStateUnavailable(err)
	}
	return nil
}

func (s *EnterAwayService) releaseOrphan(ctx context.Context, in EnterAwayInput, result EnterAwayResult) error {
	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), orphanReleaseTimeout)
	defer cancel()

	removed, err := s.state.ReleaseAway(detached, in.UserID, in.Owner)
	if err != nil {
		log.Printf("voice.away.error level=error op=release user_id=%s room_id=%s err=%q", in.UserID, in.RoomID, err)
		return ErrVoiceStateUnavailable(err)
	}
	if removed != nil {
		logAwayCleared(*removed, "connection_closed")
	}
	if result.MovedFrom == nil {
		return nil
	}
	if err := s.events.publishCurrentAwayList(detached, *result.MovedFrom); err != nil {
		return ErrVoiceStateUnavailable(err)
	}
	return nil
}

func isAccessDenied(err error) bool {
	var httpErr *httpx.Error
	return errors.As(err, &httpErr) && httpErr.Status < http.StatusInternalServerError
}
