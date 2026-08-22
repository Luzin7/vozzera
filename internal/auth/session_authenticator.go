package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SessionAuthenticator struct {
	repo        Repository
	touchWindow time.Duration
	ttl         time.Duration
}

type UserClaims struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	Role      string
	Username  string
}

func NewSessionAuthenticator(repo Repository, touchWindow, ttl time.Duration) *SessionAuthenticator {
	return &SessionAuthenticator{repo: repo, touchWindow: touchWindow, ttl: ttl}
}

func (a *SessionAuthenticator) AuthenticateSession(ctx context.Context, sessionID string) (UserClaims, error) {
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return UserClaims{}, ErrInvalidSessionID()
	}

	session, err := a.repo.GetSessionByID(ctx, sid)
	if err != nil {
		return UserClaims{}, err
	}

	if time.Now().After(session.ExpiresAt) {
		return UserClaims{}, ErrSessionExpired()
	}

	if time.Until(session.ExpiresAt) < a.touchWindow {
		newExpiresAt := time.Now().Add(a.ttl)
		err = a.repo.TouchSession(ctx, TouchSessionParams{
			ID:        sid,
			ExpiresAt: newExpiresAt,
		})
		if err != nil {
			return UserClaims{}, err
		}
		return UserClaims{
			ID:        session.ID,
			UserID:    session.UserID,
			ExpiresAt: newExpiresAt,
			Role:      session.Role,
			Username:  session.Username,
		}, nil
	}

	return UserClaims{
		ID:        session.ID,
		UserID:    session.UserID,
		ExpiresAt: session.ExpiresAt,
		Role:      session.Role,
		Username:  session.Username,
	}, nil
}
