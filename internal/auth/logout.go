package auth

import (
	"context"

	"github.com/google/uuid"
)

type LogoutInput struct {
	SessionID uuid.UUID
}

type LogoutService struct {
	repo    Repository
	revoker SessionRevoker
}

func NewLogoutService(repo Repository, revoker SessionRevoker) *LogoutService {
	return &LogoutService{repo: repo, revoker: revoker}
}

func (s *LogoutService) Execute(ctx context.Context, in LogoutInput) error {
	if err := s.repo.DeleteSessionByID(ctx, in.SessionID); err != nil {
		return ErrRevokeSession(err)
	}

	s.revoker.Revoke(in.SessionID)
	return nil
}
