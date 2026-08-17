package chat

import "context"

type CreateRoomInput struct {
	Name  string
	Type  string
	IsMod bool
}

type CreateRoomOutput struct {
	Room Room
}

type CreateRoomService struct {
	repo   Repository
	events RoomBroadcaster
}

func NewCreateRoomService(repo Repository, events RoomBroadcaster) *CreateRoomService {
	return &CreateRoomService{repo: repo, events: events}
}

func (s *CreateRoomService) Execute(ctx context.Context, in CreateRoomInput) (CreateRoomOutput, error) {
	if !in.IsMod {
		return CreateRoomOutput{}, ErrNotAuthorized
	}

	if in.Name == "" {
		return CreateRoomOutput{}, ErrNameRequired
	}
	if len(in.Name) > 100 {
		return CreateRoomOutput{}, ErrNameTooLong
	}
	if in.Type != "text" && in.Type != "voice" {
		return CreateRoomOutput{}, ErrInvalidRoomType
	}

	room, err := s.repo.CreateRoom(ctx, CreateRoomParams{Name: in.Name, Type: in.Type})
	if err != nil {
		return CreateRoomOutput{}, ErrCreateRoom(err)
	}

	s.events.Broadcast(OutboundEvent{
		Type:     EventRoom,
		Action:   RoomCreated,
		ID:       room.ID,
		RoomName: room.Name,
		RoomType: room.Type,
	})

	return CreateRoomOutput{Room: room}, nil
}
