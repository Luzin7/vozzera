package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type UpdateEmailInput struct {
	UserID uuid.UUID
	Email  string
}

type UpdateEmailOutput struct {
	Email string
}

type UpdateEmailService struct {
	repo Repository
}

func NewUpdateEmailService(repo Repository) *UpdateEmailService {
	return &UpdateEmailService{repo: repo}
}

func (s *UpdateEmailService) Execute(ctx context.Context, in UpdateEmailInput) (UpdateEmailOutput, error) {
	email := normalizeEmail(in.Email)
	if !validEmail(email) {
		return UpdateEmailOutput{}, ErrInvalidEmail()
	}

	existing, err := s.repo.GetUserByEmail(ctx, email)
	if err == nil && existing.ID != in.UserID {
		return UpdateEmailOutput{}, ErrEmailTaken()
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return UpdateEmailOutput{}, ErrGetUser(err)
	}

	if err := s.repo.UpdateUserEmail(ctx, UpdateUserEmailParams{ID: in.UserID, Email: email}); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return UpdateEmailOutput{}, ErrEmailTaken()
		}
		return UpdateEmailOutput{}, ErrUpdateEmail(err)
	}

	return UpdateEmailOutput{Email: email}, nil
}
