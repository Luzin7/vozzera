package auth

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	CreateUser(ctx context.Context, arg CreateUserParams) (CreateUserRow, error)
	GetUserByUsername(ctx context.Context, username string) (GetUserByUsernameRow, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
	InsertSession(ctx context.Context, arg InsertSessionParams) (InsertSessionRow, error)
	DeleteSessionByID(ctx context.Context, id uuid.UUID) error
}

type SessionRevoker interface {
	Revoke(sessionID uuid.UUID)
}

type MailSender interface {
	Send(to, subject, html string) error
}
