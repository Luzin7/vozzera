package voice

import (
	"context"
	"time"
)

type awaySweepExecutor interface {
	Execute(ctx context.Context) error
}

type AwaySweeper struct {
	sweep    awaySweepExecutor
	interval time.Duration
}

func NewAwaySweeper(sweep awaySweepExecutor) *AwaySweeper {
	return &AwaySweeper{sweep: sweep, interval: sweepInterval}
}

func (s *AwaySweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *AwaySweeper) tick(ctx context.Context) {
	_ = s.sweep.Execute(ctx)
}
