package presence

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
)

type UserStatsProvider interface {
	TotalUsers(ctx context.Context) (int64, error)
	ListUsers(ctx context.Context) ([]realtime.UserPresence, error)
}
