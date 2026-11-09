package chat

import (
	"context"
	"slices"

	"github.com/google/uuid"
)

const (
	defaultMessagesLimit = 50
	maxMessagesLimit     = 200
)

type GetMessagesInput struct {
	RoomID uuid.UUID
	Limit  int
	Before string
}

type GetMessagesOutput struct {
	Messages   []GetMessagesByRoomRow
	Reactions  map[uuid.UUID][]ReactionSummary
	NextBefore string
}

type GetMessagesService struct {
	repo Repository
}

func NewGetMessagesService(repo Repository) *GetMessagesService {
	return &GetMessagesService{repo: repo}
}

func (s *GetMessagesService) Execute(ctx context.Context, in GetMessagesInput) (GetMessagesOutput, error) {
	limit := clampMessagesLimit(in.Limit)

	var before *messageCursor
	if in.Before != "" {
		cursor, err := decodeMessageCursor(in.Before)
		if err != nil {
			return GetMessagesOutput{}, err
		}
		before = &cursor
	}

	messages, err := s.fetchPage(ctx, in.RoomID, before, limit+1)
	if err != nil {
		return GetMessagesOutput{}, ErrGetMessages(err)
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	nextBefore := ""
	if hasMore {
		oldest := messages[len(messages)-1]
		nextBefore = messageCursor{CreatedAt: oldest.CreatedAt.Time, ID: oldest.ID}.encode()
	}

	slices.Reverse(messages)

	if messages == nil {
		messages = []GetMessagesByRoomRow{}
	}

	reactions, err := s.fetchReactions(ctx, messages)
	if err != nil {
		return GetMessagesOutput{}, ErrGetMessages(err)
	}

	return GetMessagesOutput{Messages: messages, Reactions: reactions, NextBefore: nextBefore}, nil
}

func (s *GetMessagesService) fetchReactions(ctx context.Context, messages []GetMessagesByRoomRow) (map[uuid.UUID][]ReactionSummary, error) {
	reactions := make(map[uuid.UUID][]ReactionSummary, len(messages))
	if len(messages) == 0 {
		return reactions, nil
	}

	messageIDs := make([]uuid.UUID, 0, len(messages))
	for _, message := range messages {
		messageIDs = append(messageIDs, message.ID)
	}

	rows, err := s.repo.GetReactionsByMessageIDs(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		reactions[row.MessageID] = append(reactions[row.MessageID], ReactionSummary{
			Emoji:   row.Emoji,
			Count:   row.Count,
			UserIDs: row.UserIds,
		})
	}

	return reactions, nil
}

func (s *GetMessagesService) fetchPage(ctx context.Context, roomID uuid.UUID, before *messageCursor, limit int) ([]GetMessagesByRoomRow, error) {
	if before == nil {
		return s.repo.GetMessagesByRoom(ctx, GetMessagesByRoomParams{RoomID: roomID, Limit: int32(limit)})
	}

	rows, err := s.repo.GetMessagesByRoomBefore(ctx, GetMessagesByRoomBeforeParams{
		RoomID:          roomID,
		BeforeCreatedAt: before.CreatedAt,
		BeforeID:        before.ID,
		Limit:           int32(limit),
	})
	if err != nil {
		return nil, err
	}

	messages := make([]GetMessagesByRoomRow, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, GetMessagesByRoomRow(row))
	}
	return messages, nil
}

func clampMessagesLimit(limit int) int {
	if limit <= 0 {
		return defaultMessagesLimit
	}
	return min(limit, maxMessagesLimit)
}
