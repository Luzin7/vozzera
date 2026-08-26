package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
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
	repo      Repository
	publisher realtime.Publisher
}

func NewSendMessageService(repo Repository, publisher realtime.Publisher) *SendMessageService {
	return &SendMessageService{repo: repo, publisher: publisher}
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

	payload := MessagePayload{
		ID:        msg.ID,
		RoomID:    msg.RoomID,
		UserID:    in.UserID,
		Username:  in.Username,
		Content:   msg.Content.String,
		CreatedAt: msg.CreatedAt.Time,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Erro ao serializar payload: %v", err)
	}

	topic := realtime.Topic(fmt.Sprintf("room:%s", msg.RoomID.String()))

	env := realtime.Envelope{
		V:     1,
		Type:  EventMessageCreated,
		Topic: topic,
		TS:    time.Now(),
		Data:  payloadBytes,
	}

	err = s.publisher.Publish(ctx, topic, env)
	if err != nil {
		log.Printf("Erro ao publicar mensagem: %v", err)
	}

	return SendMessageOutput{
		ID:        msg.ID,
		RoomID:    msg.RoomID,
		UserID:    in.UserID,
		Username:  in.Username,
		Content:   msg.Content.String,
		CreatedAt: msg.CreatedAt.Time,
	}, nil
}
