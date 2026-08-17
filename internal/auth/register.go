package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type RegisterInput struct {
	Username   string
	Password   string
	InviteCode string
}

type RegisterOutput struct {
	ID       uuid.UUID
	Username string
}

type RegisterService struct {
	repo       Repository
	inviteCode string
}

func NewRegisterService(repo Repository, inviteCode string) *RegisterService {
	return &RegisterService{repo: repo, inviteCode: inviteCode}
}

func (s *RegisterService) Execute(ctx context.Context, in RegisterInput) (RegisterOutput, error) {
	if in.InviteCode != s.inviteCode || s.inviteCode == "" {
		return RegisterOutput{}, ErrInvalidInviteCode()
	}

	username := strings.TrimSpace(in.Username)
	if len(username) < 3 || len(username) > 50 {
		return RegisterOutput{}, ErrInvalidUsername()
	}

	if len(in.Password) < 8 || len(in.Password) > 72 {
		return RegisterOutput{}, ErrInvalidPassword()
	}

	hashedPassword, err := HashPassword(in.Password)
	if err != nil {
		return RegisterOutput{}, ErrHashPassword(err)
	}

	user, err := s.repo.CreateUser(ctx, CreateUserParams{
		Username:     username,
		PasswordHash: hashedPassword,
	})
	if err != nil {
		return RegisterOutput{}, ErrUsernameTaken(err)
	}

	return RegisterOutput{ID: user.ID, Username: user.Username}, nil
}
