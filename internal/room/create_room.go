package room

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateRoomInput struct {
	Name      string
	HasVoice  bool
	StaffOnly bool
	Claims    httpx.UserClaims
}

type CreateRoomOutput struct {
	Room Room
}

type CreateRoomService struct {
	repo      Repository
	publisher realtime.Publisher
}

func NewCreateRoomService(repo Repository, publisher realtime.Publisher) *CreateRoomService {
	return &CreateRoomService{repo: repo, publisher: publisher}
}

func (s *CreateRoomService) Execute(ctx context.Context, in CreateRoomInput) (CreateRoomOutput, error) {
	if err := validateRoomWrite(in.Claims, in.Name); err != nil {
		return CreateRoomOutput{}, err
	}

	room, err := s.repo.CreateRoom(ctx, CreateRoomParams{
		Name:      in.Name,
		CreatedBy: pgtype.UUID{Bytes: in.Claims.UserID, Valid: true},
		HasVoice:  in.HasVoice,
		StaffOnly: in.StaffOnly,
	})
	if err != nil {
		return CreateRoomOutput{}, ErrCreateRoom(err)
	}

	topic := realtime.Topic("app:rooms")
	if err := publishRoom(ctx, s.publisher, EventRoomCreated, topic, room); err != nil {
		return CreateRoomOutput{}, ErrCreateRoom(err)
	}

	return CreateRoomOutput{Room: room}, nil
}

func validateRoomWrite(claims httpx.UserClaims, name string) error {
	if !claims.CanModerate() {
		return ErrNotAuthorized
	}
	if name == "" {
		return ErrNameRequired
	}
	if len(name) > 100 {
		return ErrNameTooLong
	}
	return nil
}
