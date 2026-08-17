package voice

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	GetRoomByID(ctx context.Context, id uuid.UUID) (Room, error)
	ListVoiceRooms(ctx context.Context) ([]Room, error)
}
