package chat

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

type Repository interface {
	GetMessagesByRoom(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error)
	UpdateMessage(ctx context.Context, arg UpdateMessageParams) (UpdateMessageRow, error)
	DeleteMessage(ctx context.Context, arg DeleteMessageParams) (DeleteMessageRow, error)
	CreateMessage(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error)
}

type RoomAccess interface {
	CanAccess(ctx context.Context, roomID uuid.UUID, claims httpx.UserClaims) (bool, error)
}
