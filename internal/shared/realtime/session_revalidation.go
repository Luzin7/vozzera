package realtime

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type SessionRevalidator interface {
	RevalidateSession(ctx context.Context, sessionID uuid.UUID) error
}

type SessionRevoker interface {
	Revoke(ctx context.Context, sessionID uuid.UUID) error
}

type SessionRevalidation struct {
	Revalidator SessionRevalidator
	Revoker     SessionRevoker
	Interval    time.Duration
}

type SessionInvalidError struct {
	Err error
}

func (e *SessionInvalidError) Error() string {
	return e.Err.Error()
}

func (e *SessionInvalidError) Unwrap() error {
	return e.Err
}

func ErrSessionInvalid(cause error) error {
	return &SessionInvalidError{Err: cause}
}

func IsSessionInvalid(err error) bool {
	var target *SessionInvalidError
	return errors.As(err, &target)
}
