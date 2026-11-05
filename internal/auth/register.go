package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	pgUniqueViolationCode = "23505"
	usernameUniqueKey     = "users_username_key"
	emailUniqueKey        = "users_email_key"
)

type RegisterInput struct {
	Username   string
	Password   string
	Email      string
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

	email := normalizeEmail(in.Email)
	if !validEmail(email) {
		return RegisterOutput{}, ErrInvalidEmail()
	}

	hashedPassword, err := HashPassword(in.Password)
	if err != nil {
		return RegisterOutput{}, ErrHashPassword(err)
	}

	user, err := s.repo.CreateUser(ctx, CreateUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: hashedPassword,
	})
	if err != nil {
		return RegisterOutput{}, mapCreateUserError(err)
	}

	return RegisterOutput{ID: user.ID, Username: user.Username}, nil
}

func mapCreateUserError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgUniqueViolationCode {
		return ErrCreateUser(err)
	}

	switch pgErr.ConstraintName {
	case usernameUniqueKey:
		return ErrUsernameTaken(err)
	case emailUniqueKey:
		return ErrEmailTaken()
	}
	return ErrCreateUser(err)
}
