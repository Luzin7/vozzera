package keyvalue

import (
	"context"
	"testing"
	"time"
)

type totalRecorder struct {
	totals []int64
}

func (r *totalRecorder) TotalChanged(_ context.Context, total int64) {
	r.totals = append(r.totals, total)
}

func (r *totalRecorder) RefreshTotal(context.Context) error { return nil }

func TestDispatchUsersTotalAppliesValidPayload(t *testing.T) {
	handler := &totalRecorder{}

	dispatchUsersTotal(context.Background(), handler, "123")

	if len(handler.totals) != 1 || handler.totals[0] != 123 {
		t.Errorf("totals = %v, want [123]", handler.totals)
	}
}

func TestDispatchUsersTotalIgnoresInvalidPayload(t *testing.T) {
	handler := &totalRecorder{}

	dispatchUsersTotal(context.Background(), handler, "abc")
	dispatchUsersTotal(context.Background(), handler, "-1")

	if len(handler.totals) != 0 {
		t.Errorf("totals = %v, want none", handler.totals)
	}
}

type blockingTotalHandler struct {
	deadlineSet bool
}

func (b *blockingTotalHandler) TotalChanged(context.Context, int64) {}

func (b *blockingTotalHandler) RefreshTotal(ctx context.Context) error {
	_, b.deadlineSet = ctx.Deadline()
	<-ctx.Done()
	return ctx.Err()
}

func TestRefreshTotalBoundsAttemptByTimeout(t *testing.T) {
	handler := &blockingTotalHandler{}
	done := make(chan struct{})

	go func() {
		refreshTotal(context.Background(), handler, 10*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh travou sem timeout")
	}
	if !handler.deadlineSet {
		t.Error("contexto sem deadline")
	}
}
