package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type MeInput struct {
	UserID uuid.UUID
}

type MeOutput struct {
	ID       uuid.UUID
	Username string
	Role     string
}

type MeService struct {
	repo Repository
}

func NewMeService(repo Repository) *MeService {
	return &MeService{repo: repo}
}

func (s *MeService) Execute(ctx context.Context, in MeInput) (MeOutput, error) {
	user, err := s.repo.GetUserByID(ctx, in.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return MeOutput{}, ErrUserNotFound()
	}
	if err != nil {
		return MeOutput{}, ErrGetUser(err)
	}

	return MeOutput{ID: user.ID, Username: user.Username, Role: user.Role}, nil
}
