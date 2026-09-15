package presence

import "context"

type UserStatsProvider interface {
	TotalUsers(ctx context.Context) (int64, error)
}
