package realtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

const (
	testTouchWindow = 24 * time.Hour
	testSessionTTL  = 7 * 24 * time.Hour
)

type countingRevalidator struct {
	calls int
	ttl   time.Duration
	now   func() time.Time
	err   error
}

func (r *countingRevalidator) RevalidateSession(context.Context, uuid.UUID) (time.Time, error) {
	r.calls++
	if r.err != nil {
		return time.Time{}, r.err
	}
	return r.now().Add(r.ttl), nil
}

type fakeClock struct{ current time.Time }

func (c *fakeClock) Now() time.Time          { return c.current }
func (c *fakeClock) Advance(d time.Duration) { c.current = c.current.Add(d) }

func newTestMonitor(clock *fakeClock, revalidator *countingRevalidator, expiresAt time.Time) *sessionMonitor {
	revalidator.now = clock.Now
	return newSessionMonitor(
		SessionRevalidation{Revalidator: revalidator, TouchWindow: testTouchWindow},
		uuid.New(), expiresAt, clock.Now,
	)
}

func TestSessionMonitor_NoDatabaseCallsOutsideTouchWindow(t *testing.T) {
	clock := &fakeClock{current: time.Unix(1_700_000_000, 0)}
	revalidator := &countingRevalidator{ttl: testSessionTTL}
	expiresAt := clock.Now().Add(testSessionTTL)

	monitors := make([]*sessionMonitor, 50)
	for i := range monitors {
		monitors[i] = newTestMonitor(clock, revalidator, expiresAt)
	}

	for tick := 0; tick < 12*5*5; tick++ {
		clock.Advance(5 * time.Minute)
		for _, m := range monitors {
			if !m.Valid(context.Background(), uuid.Nil) {
				t.Fatalf("sessão inválida no tick %d", tick)
			}
		}
	}

	if revalidator.calls != 0 {
		t.Errorf("chamadas ao banco = %d, want 0", revalidator.calls)
	}
}

func TestSessionMonitor_RevalidatesOncePerClientInsideTouchWindow(t *testing.T) {
	clock := &fakeClock{current: time.Unix(1_700_000_000, 0)}
	revalidator := &countingRevalidator{ttl: testSessionTTL}
	m := newTestMonitor(clock, revalidator, clock.Now().Add(testTouchWindow-time.Minute))

	for tick := 0; tick < 10; tick++ {
		clock.Advance(5 * time.Minute)
		if !m.Valid(context.Background(), uuid.Nil) {
			t.Fatalf("sessão inválida no tick %d", tick)
		}
	}

	if revalidator.calls != 1 {
		t.Errorf("chamadas ao banco = %d, want 1", revalidator.calls)
	}
}

func TestSessionMonitor_ExpiredSessionIsInvalidWithoutDatabase(t *testing.T) {
	clock := &fakeClock{current: time.Unix(1_700_000_000, 0)}
	revalidator := &countingRevalidator{ttl: testSessionTTL}
	m := newTestMonitor(clock, revalidator, clock.Now().Add(time.Minute))

	clock.Advance(2 * time.Minute)

	if m.Valid(context.Background(), uuid.Nil) {
		t.Error("sessão expirada deveria ser inválida")
	}
	if revalidator.calls != 0 {
		t.Errorf("chamadas ao banco = %d, want 0", revalidator.calls)
	}
}

func TestSessionMonitor_UnknownExpiryAlwaysRevalidates(t *testing.T) {
	clock := &fakeClock{current: time.Unix(1_700_000_000, 0)}
	revalidator := &countingRevalidator{ttl: testSessionTTL}
	m := newTestMonitor(clock, revalidator, time.Time{})

	m.Valid(context.Background(), uuid.Nil)

	if revalidator.calls != 1 {
		t.Errorf("chamadas ao banco = %d, want 1", revalidator.calls)
	}
}

func TestSessionMonitor_InvalidSessionFromDatabaseIsInvalid(t *testing.T) {
	clock := &fakeClock{current: time.Unix(1_700_000_000, 0)}
	revalidator := &countingRevalidator{err: ErrSessionInvalid(errors.New("sessão removida"))}
	m := newTestMonitor(clock, revalidator, clock.Now().Add(time.Hour))

	if m.Valid(context.Background(), uuid.Nil) {
		t.Error("sessão removida deveria ser inválida")
	}
}

func TestSessionMonitor_TransientDatabaseErrorKeepsSession(t *testing.T) {
	clock := &fakeClock{current: time.Unix(1_700_000_000, 0)}
	revalidator := &countingRevalidator{err: errors.New("banco fora")}
	m := newTestMonitor(clock, revalidator, clock.Now().Add(time.Hour))

	if !m.Valid(context.Background(), uuid.Nil) {
		t.Error("erro transitório não deveria derrubar a sessão")
	}
}
