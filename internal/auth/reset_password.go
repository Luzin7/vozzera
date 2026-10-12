package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type ResetPasswordInput struct {
	Token    string
	Password string
}

type ResetPasswordOutput struct{}

type ResetPasswordService struct {
	repo    Repository
	revoker SessionRevoker
}

func NewResetPasswordService(repo Repository, revoker SessionRevoker) *ResetPasswordService {
	return &ResetPasswordService{repo: repo, revoker: revoker}
}

func (s *ResetPasswordService) Execute(ctx context.Context, in ResetPasswordInput) (ResetPasswordOutput, error) {
	if in.Token == "" {
		return ResetPasswordOutput{}, ErrInvalidResetToken()
	}

	if len(in.Password) < 8 || len(in.Password) > 72 {
		return ResetPasswordOutput{}, ErrInvalidPassword()
	}

	token, err := s.repo.GetPasswordResetTokenByHash(ctx, hashResetToken(in.Token))
	if errors.Is(err, pgx.ErrNoRows) {
		return ResetPasswordOutput{}, ErrInvalidResetToken()
	}
	if err != nil {
		return ResetPasswordOutput{}, ErrGetUser(err)
	}

	hashedPassword, err := HashPassword(in.Password)
	if err != nil {
		return ResetPasswordOutput{}, ErrHashPassword(err)
	}

	if err := s.repo.UpdateUserPassword(ctx, UpdateUserPasswordParams{
		ID:           token.UserID,
		PasswordHash: hashedPassword,
	}); err != nil {
		return ResetPasswordOutput{}, ErrResetPassword(err)
	}

	if err := s.repo.DeletePasswordResetTokensByUser(ctx, token.UserID); err != nil {
		return ResetPasswordOutput{}, ErrResetPassword(err)
	}

	if err := s.repo.DeleteSessionsByUser(ctx, token.UserID); err != nil {
		return ResetPasswordOutput{}, ErrResetPassword(err)
	}

	if err := s.revoker.RevokeUser(ctx, token.UserID); err != nil {
		return ResetPasswordOutput{}, ErrResetPassword(err)
	}

	return ResetPasswordOutput{}, nil
}
