package room

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
)

type RoomFilter struct {
	HasVoice *bool
}

type ListRoomsOutput struct {
	Rooms []Room
}

type ListRoomsService struct {
	repo Repository
}

func NewListRoomsService(repo Repository) *ListRoomsService {
	return &ListRoomsService{repo: repo}
}

func (s *ListRoomsService) Execute(ctx context.Context, claims httpx.UserClaims, filter RoomFilter) (ListRoomsOutput, error) {
	rooms, err := s.repo.ListRooms(ctx)
	if err != nil {
		return ListRoomsOutput{}, ErrListRooms(err)
	}

	visible := make([]Room, 0, len(rooms))
	for _, room := range rooms {
		if !canAccess(room, claims) {
			continue
		}
		if filter.HasVoice != nil && room.HasVoice != *filter.HasVoice {
			continue
		}
		visible = append(visible, room)
	}

	return ListRoomsOutput{Rooms: visible}, nil
}
