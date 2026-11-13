package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	emailtemplates "github.com/Luzin7/vozzera-backend/internal/email"
)

type RequestPasswordResetInput struct {
	Email string
}

type RequestPasswordResetOutput struct{}

type RequestPasswordResetService struct {
	repo     Repository
	mailer   MailSender
	appURL   string
	tokenTTL time.Duration
}

func NewRequestPasswordResetService(repo Repository, mailer MailSender, appURL string, tokenTTL time.Duration) *RequestPasswordResetService {
	return &RequestPasswordResetService{repo: repo, mailer: mailer, appURL: appURL, tokenTTL: tokenTTL}
}

func (s *RequestPasswordResetService) Execute(ctx context.Context, in RequestPasswordResetInput) (RequestPasswordResetOutput, error) {
	email := normalizeEmail(in.Email)
	if !validEmail(email) {
		return RequestPasswordResetOutput{}, ErrInvalidEmail()
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return RequestPasswordResetOutput{}, nil
	}
	if err != nil {
		return RequestPasswordResetOutput{}, ErrGetUser(err)
	}

	token := generateResetToken()
	if _, err := s.repo.InsertPasswordResetToken(ctx, InsertPasswordResetTokenParams{
		UserID:    user.ID,
		TokenHash: hashResetToken(token),
		ExpiresAt: time.Now().Add(s.tokenTTL),
	}); err != nil {
		return RequestPasswordResetOutput{}, ErrCreateResetToken(err)
	}

	subject, body := emailtemplates.PasswordReset(s.appURL, token)
	_ = s.mailer.Send(user.Email, subject, body)

	return RequestPasswordResetOutput{}, nil
}
