package presence

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

type TotalCache interface {
	Get(ctx context.Context) (int64, bool, error)
	Set(ctx context.Context, total int64) error
	Publish(ctx context.Context, total int64) error
}

type UserStatsProvider interface {
	TotalUsers(ctx context.Context) (int64, error)
	ListUsers(ctx context.Context) ([]realtime.UserPresence, error)
}
