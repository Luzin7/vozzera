package chat

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type SendMessageInput struct {
	RoomID   uuid.UUID
	UserID   uuid.UUID
	Username string
	Content  string
}

type SendMessageOutput struct {
	ID        uuid.UUID
	RoomID    uuid.UUID
	UserID    uuid.UUID
	Username  string
	Content   string
	CreatedAt time.Time
}

type SendMessageService struct {
	repo   Repository
	events RoomBroadcaster
}

func NewSendMessageService(repo Repository, events RoomBroadcaster) *SendMessageService {
	return &SendMessageService{repo: repo, events: events}
}

func (s *SendMessageService) Execute(ctx context.Context, in SendMessageInput) (SendMessageOutput, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" || len(content) > 4000 {
		return SendMessageOutput{}, ErrInvalidContent
	}

	msg, err := s.repo.CreateMessage(ctx, CreateMessageParams{
		RoomID:  in.RoomID,
		UserID:  in.UserID,
		Content: pgtype.Text{String: content, Valid: true},
	})
	if err != nil {
		return SendMessageOutput{}, ErrCreateMessage(err)
	}

	s.events.Broadcast(OutboundEvent{
		Type:      EventMessage,
		Action:    MessageCreated,
		ID:        msg.ID,
		RoomID:    msg.RoomID,
		UserID:    in.UserID,
		Username:  in.Username,
		Content:   msg.Content.String,
		CreatedAt: msg.CreatedAt.Time,
	})

	return SendMessageOutput{
		ID:        msg.ID,
		RoomID:    msg.RoomID,
		UserID:    in.UserID,
		Username:  in.Username,
		Content:   msg.Content.String,
		CreatedAt: msg.CreatedAt.Time,
	}, nil
}
