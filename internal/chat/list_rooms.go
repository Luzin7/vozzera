package chat

import "context"

type ListRoomsOutput struct {
	Rooms []Room
}

type ListRoomsService struct {
	repo Repository
}

func NewListRoomsService(repo Repository) *ListRoomsService {
	return &ListRoomsService{repo: repo}
}

func (s *ListRoomsService) Execute(ctx context.Context) (ListRoomsOutput, error) {
	rooms, err := s.repo.ListRooms(ctx)
	if err != nil {
		return ListRoomsOutput{}, ErrListRooms(err)
	}

	if rooms == nil {
		rooms = []Room{}
	}

	return ListRoomsOutput{Rooms: rooms}, nil
}
