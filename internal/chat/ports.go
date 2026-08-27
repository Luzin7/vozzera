package chat

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	ListRooms(ctx context.Context) ([]Room, error)
	GetRoomByID(ctx context.Context, id uuid.UUID) (Room, error)
	CreateRoom(ctx context.Context, arg CreateRoomParams) (Room, error)
	UpdateRoom(ctx context.Context, arg UpdateRoomParams) (Room, error)
	DeleteRoom(ctx context.Context, id uuid.UUID) (Room, error)
	GetMessagesByRoom(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error)
	UpdateMessage(ctx context.Context, arg UpdateMessageParams) (UpdateMessageRow, error)
	DeleteMessage(ctx context.Context, arg DeleteMessageParams) (DeleteMessageRow, error)
	CreateMessage(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error)
}
