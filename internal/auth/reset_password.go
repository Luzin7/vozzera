package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ResetPasswordInput struct {
	Token    string
	Password string
}

type ResetPasswordOutput struct{}

type passwordChange struct {
	tokenHash    string
	passwordHash string
}

type ResetPasswordService struct {
	transactor Transactor
	revoker    SessionRevoker
}

func NewResetPasswordService(transactor Transactor, revoker SessionRevoker) *ResetPasswordService {
	return &ResetPasswordService{transactor: transactor, revoker: revoker}
}

func (s *ResetPasswordService) Execute(ctx context.Context, in ResetPasswordInput) (ResetPasswordOutput, error) {
	if in.Token == "" {
		return ResetPasswordOutput{}, ErrInvalidResetToken()
	}

	if len(in.Password) < 8 || len(in.Password) > 72 {
		return ResetPasswordOutput{}, ErrInvalidPassword()
	}

	hashedPassword, err := HashPassword(in.Password)
	if err != nil {
		return ResetPasswordOutput{}, ErrHashPassword(err)
	}

	change := passwordChange{tokenHash: hashResetToken(in.Token), passwordHash: hashedPassword}
	var userID uuid.UUID
	err = s.transactor.WithinTx(ctx, func(repo Repository) error {
		id, err := applyPasswordChange(ctx, repo, change)
		userID = id
		return err
	})
	if err != nil {
		return ResetPasswordOutput{}, ErrResetPassword(err)
	}

	revokeSocketsDetached(ctx, func(revokeCtx context.Context) error {
		return s.revoker.RevokeUser(revokeCtx, userID)
	})

	return ResetPasswordOutput{}, nil
}

func applyPasswordChange(ctx context.Context, repo Repository, change passwordChange) (uuid.UUID, error) {
	userID, err := repo.ConsumePasswordResetToken(ctx, change.tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrInvalidResetToken()
	}
	if err != nil {
		return uuid.Nil, ErrResetPassword(err)
	}

	if err := repo.UpdateUserPassword(ctx, UpdateUserPasswordParams{ID: userID, PasswordHash: change.passwordHash}); err != nil {
		return uuid.Nil, ErrResetPassword(err)
	}

	if err := repo.DeleteSessionsByUser(ctx, userID); err != nil {
		return uuid.Nil, ErrResetPassword(err)
	}

	return userID, nil
}
