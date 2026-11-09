package chat

import (
	"context"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

type Repository interface {
	GetMessagesByRoom(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error)
	GetMessagesByRoomBefore(ctx context.Context, arg GetMessagesByRoomBeforeParams) ([]GetMessagesByRoomBeforeRow, error)
	UpdateMessage(ctx context.Context, arg UpdateMessageParams) (UpdateMessageRow, error)
	DeleteMessage(ctx context.Context, arg DeleteMessageParams) (DeleteMessageRow, error)
	CreateMessage(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error)
	GetReplyTarget(ctx context.Context, arg GetReplyTargetParams) (GetReplyTargetRow, error)
	LockMessage(ctx context.Context, messageID uuid.UUID) (uuid.UUID, error)
	GetReactionState(ctx context.Context, arg GetReactionStateParams) (GetReactionStateRow, error)
	AddReaction(ctx context.Context, arg AddReactionParams) (int64, error)
	RemoveReaction(ctx context.Context, arg RemoveReactionParams) (int64, error)
	GetReactionsByMessageIDs(ctx context.Context, messageIDs []uuid.UUID) ([]GetReactionsByMessageIDsRow, error)
}

type RoomAccess interface {
	CanAccess(ctx context.Context, roomID uuid.UUID, claims httpx.UserClaims) (bool, error)
}

type Transactor interface {
	WithinTx(ctx context.Context, limits TxLimits, fn func(Repository) error) error
}

type TxLimits struct {
	LockTimeout      time.Duration
	StatementTimeout time.Duration
}
