package room

import (
	"context"
	"errors"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type TopicRevoker interface {
	RevokeTopic(ctx context.Context, topic realtime.Topic) error
}

type UpdateRoomInput struct {
	ID        uuid.UUID
	Name      string
	StaffOnly *bool
	Claims    httpx.UserClaims
}

type UpdateRoomOutput struct {
	Room Room
}

type UpdateRoomService struct {
	repo      Repository
	publisher realtime.Publisher
	revoker   TopicRevoker
}

func NewUpdateRoomService(repo Repository, publisher realtime.Publisher, revoker TopicRevoker) *UpdateRoomService {
	return &UpdateRoomService{repo: repo, publisher: publisher, revoker: revoker}
}

func (s *UpdateRoomService) Execute(ctx context.Context, in UpdateRoomInput) (UpdateRoomOutput, error) {
	if err := validateRoomWrite(in.Claims, in.Name); err != nil {
		return UpdateRoomOutput{}, err
	}

	room, err := s.repo.UpdateRoom(ctx, UpdateRoomParams{
		Name:      in.Name,
		ID:        in.ID,
		StaffOnly: toPgBool(in.StaffOnly),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return UpdateRoomOutput{}, ErrRoomNotFound
	}
	if err != nil {
		return UpdateRoomOutput{}, ErrUpdateRoom(err)
	}

	if in.StaffOnly != nil && *in.StaffOnly {
		s.revokeSubscribers(ctx, room.ID)
	}

	topic := realtime.Topic("app:rooms")
	if err := publishRoom(ctx, s.publisher, EventRoomUpdated, topic, room); err != nil {
		return UpdateRoomOutput{}, ErrUpdateRoom(err)
	}

	return UpdateRoomOutput{Room: room}, nil
}

func (s *UpdateRoomService) revokeSubscribers(ctx context.Context, roomID uuid.UUID) {
	if s.revoker == nil {
		return
	}
	s.revoker.RevokeTopic(ctx, realtime.Topic("room:"+roomID.String()))
}

func toPgBool(value *bool) pgtype.Bool {
	if value == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *value, Valid: true}
}
