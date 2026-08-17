package chat

import (
	"context"
	"slices"

	"github.com/google/uuid"
)

type GetMessagesInput struct {
	RoomID uuid.UUID
	Limit  int
}

type GetMessagesOutput struct {
	Messages []GetMessagesByRoomRow
}

type GetMessagesService struct {
	repo Repository
}

func NewGetMessagesService(repo Repository) *GetMessagesService {
	return &GetMessagesService{repo: repo}
}

func (s *GetMessagesService) Execute(ctx context.Context, in GetMessagesInput) (GetMessagesOutput, error) {
	limit := in.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	messages, err := s.repo.GetMessagesByRoom(ctx, GetMessagesByRoomParams{
		RoomID: in.RoomID,
		Limit:  int32(limit),
	})
	if err != nil {
		return GetMessagesOutput{}, ErrGetMessages(err)
	}

	slices.Reverse(messages)

	if messages == nil {
		messages = []GetMessagesByRoomRow{}
	}

	return GetMessagesOutput{Messages: messages}, nil
}
