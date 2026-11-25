package realtime

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
)

type sessionMonitor struct {
	revalidator SessionRevalidator
	touchWindow time.Duration
	sessionID   uuid.UUID
	expiresAt   time.Time
	now         func() time.Time
}

func newSessionMonitor(revalidation SessionRevalidation, sessionID uuid.UUID, expiresAt time.Time, now func() time.Time) *sessionMonitor {
	return &sessionMonitor{
		revalidator: revalidation.Revalidator,
		touchWindow: revalidation.TouchWindow,
		sessionID:   sessionID,
		expiresAt:   expiresAt,
		now:         now,
	}
}

func (m *sessionMonitor) Valid(ctx context.Context, connID uuid.UUID) bool {
	if m.expired() {
		return false
	}
	if !m.nearExpiry() {
		return true
	}

	expiresAt, err := m.revalidator.RevalidateSession(ctx, m.sessionID)
	if err == nil {
		m.expiresAt = expiresAt
		return true
	}
	if IsSessionInvalid(err) {
		return false
	}

	log.Printf("falha ao revalidar sessão do websocket %s: %v", connID, err)
	return true
}

func (m *sessionMonitor) expired() bool {
	return !m.expiresAt.IsZero() && m.now().After(m.expiresAt)
}

func (m *sessionMonitor) nearExpiry() bool {
	return m.expiresAt.IsZero() || m.expiresAt.Sub(m.now()) < m.touchWindow
}
