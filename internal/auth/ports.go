package auth

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	CreateUser(ctx context.Context, arg CreateUserParams) (CreateUserRow, error)
	GetUserByUsername(ctx context.Context, username string) (GetUserByUsernameRow, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
	GetUserByEmail(ctx context.Context, email string) (GetUserByEmailRow, error)
	GetUserByIdentifier(ctx context.Context, arg GetUserByIdentifierParams) (User, error)
	InsertSession(ctx context.Context, arg InsertSessionParams) (InsertSessionRow, error)
	DeleteSessionByID(ctx context.Context, id uuid.UUID) error
	DeleteSessionsByUser(ctx context.Context, userID uuid.UUID) error
	UpdateUserPassword(ctx context.Context, arg UpdateUserPasswordParams) error
	UpdateUserEmail(ctx context.Context, arg UpdateUserEmailParams) error
	InsertPasswordResetToken(ctx context.Context, arg InsertPasswordResetTokenParams) (PasswordResetToken, error)
	ConsumePasswordResetToken(ctx context.Context, tokenHash string) (uuid.UUID, error)
	CleanupExpiredPasswordResetTokens(ctx context.Context) error
	GetSessionByID(ctx context.Context, id uuid.UUID) (GetSessionByIDRow, error)
	TouchSession(ctx context.Context, arg TouchSessionParams) error
}

type Transactor interface {
	WithinTx(ctx context.Context, fn func(Repository) error) error
}

type SessionRevoker interface {
	Revoke(ctx context.Context, sessionID uuid.UUID) error
	RevokeUser(ctx context.Context, userID uuid.UUID) error
}

type MailSender interface {
	Send(to, subject, html string) error
}
