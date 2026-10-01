package room

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type RoomAuthorizer struct {
	repo Repository
}

func NewRoomAuthorizer(repo Repository) *RoomAuthorizer {
	return &RoomAuthorizer{repo: repo}
}

func (a *RoomAuthorizer) CanSubscribe(ctx context.Context, userID string, topic realtime.Topic) error {
	roomID := roomIDFromTopic(topic)
	if roomID == uuid.Nil {
		return ErrRoomNotFound
	}

	room, err := a.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return ErrRoomNotFound
	}
	if !room.StaffOnly {
		return nil
	}

	return a.authorizeStaff(ctx, userID)
}

func (a *RoomAuthorizer) authorizeStaff(ctx context.Context, userID string) error {
	id, err := uuid.Parse(userID)
	if err != nil {
		return ErrRoomNotFound
	}

	role, err := a.repo.GetUserRole(ctx, id)
	if err != nil {
		return ErrRoomNotFound
	}
	if role == httpx.RoleMod || role == httpx.RoleAdmin {
		return nil
	}

	return ErrRoomNotFound
}

func roomIDFromTopic(topic realtime.Topic) uuid.UUID {
	raw := string(topic)
	if len(raw) < 5 || raw[:5] != "room:" {
		return uuid.Nil
	}
	id, err := uuid.Parse(raw[5:])
	if err != nil {
		return uuid.Nil
	}
	return id
}

var _ realtime.SubscriptionAuthorizer = (*RoomAuthorizer)(nil)
