package voice

import "context"

type ListVoiceRoomsOutput struct {
	Rooms []Room
}

type ListVoiceRoomsService struct {
	repo Repository
}

func NewListVoiceRoomsService(repo Repository) *ListVoiceRoomsService {
	return &ListVoiceRoomsService{repo: repo}
}

func (s *ListVoiceRoomsService) Execute(ctx context.Context) (ListVoiceRoomsOutput, error) {
	rooms, err := s.repo.ListVoiceRooms(ctx)
	if err != nil {
		return ListVoiceRoomsOutput{}, ErrListVoiceRooms(err)
	}

	if rooms == nil {
		rooms = []Room{}
	}

	return ListVoiceRoomsOutput{Rooms: rooms}, nil
}
