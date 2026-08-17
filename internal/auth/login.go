package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type LoginInput struct {
	Username string
	Password string
}

type LoginOutput struct {
	UserID    uuid.UUID
	Username  string
	SessionID uuid.UUID
	ExpiresAt time.Time
}

type LoginService struct {
	repo       Repository
	sessionTTL time.Duration
}

func NewLoginService(repo Repository, sessionTTL time.Duration) *LoginService {
	return &LoginService{repo: repo, sessionTTL: sessionTTL}
}

func (s *LoginService) Execute(ctx context.Context, in LoginInput) (LoginOutput, error) {
	username := strings.TrimSpace(in.Username)
	if len(username) > 50 {
		return LoginOutput{}, ErrUsernameTooLong()
	}

	if len(in.Password) > 72 {
		return LoginOutput{}, ErrPasswordTooLong()
	}

	user, err := s.repo.GetUserByUsername(ctx, username)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginOutput{}, ErrInvalidCredentials()
	}
	if err != nil {
		return LoginOutput{}, ErrGetUser(err)
	}

	if err := CheckPassword(user.PasswordHash, in.Password); err != nil {
		return LoginOutput{}, ErrInvalidCredentials()
	}

	session, err := s.repo.InsertSession(ctx, InsertSessionParams{
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(s.sessionTTL),
	})
	if err != nil {
		return LoginOutput{}, ErrCreateSession(err)
	}

	return LoginOutput{
		UserID:    user.ID,
		Username:  user.Username,
		SessionID: session.ID,
		ExpiresAt: session.ExpiresAt,
	}, nil
}
