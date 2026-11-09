package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SendMessageInput struct {
	RoomID           uuid.UUID
	UserID           uuid.UUID
	Username         string
	Content          string
	ReplyToMessageID *uuid.UUID
}

type SendMessageOutput struct {
	ID        uuid.UUID
	RoomID    uuid.UUID
	UserID    uuid.UUID
	Username  string
	Content   string
	CreatedAt time.Time
	ReplyTo   *ReplyToPayload
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

	replyTo, err := s.resolveReplyTo(ctx, in)
	if err != nil {
		return SendMessageOutput{}, err
	}

	replyToID, replyToUsername, replyToContent := replyToColumns(replyTo)
	msg, err := s.repo.CreateMessage(ctx, CreateMessageParams{
		RoomID:           in.RoomID,
		UserID:           in.UserID,
		Content:          pgtype.Text{String: content, Valid: true},
		ReplyToMessageID: replyToID,
		ReplyToUsername:  replyToUsername,
		ReplyToContent:   replyToContent,
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
		ReplyTo:   replyTo,
		Reactions: []ReactionSummary{},
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
		ReplyTo:   replyTo,
	}, nil
}

func (s *SendMessageService) resolveReplyTo(ctx context.Context, in SendMessageInput) (*ReplyToPayload, error) {
	if in.ReplyToMessageID == nil {
		return nil, nil
	}

	target, err := s.repo.GetReplyTarget(ctx, GetReplyTargetParams{
		ID:     *in.ReplyToMessageID,
		RoomID: in.RoomID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidReplyTarget
	}
	if err != nil {
		return nil, ErrGetReplyTarget(err)
	}

	return &ReplyToPayload{
		ID:       target.ID,
		Username: target.Username,
		Content:  target.Content.String,
	}, nil
}

func replyToColumns(replyTo *ReplyToPayload) (pgtype.UUID, pgtype.Text, pgtype.Text) {
	if replyTo == nil {
		return pgtype.UUID{}, pgtype.Text{}, pgtype.Text{}
	}
	return pgtype.UUID{Bytes: replyTo.ID, Valid: true},
		pgtype.Text{String: replyTo.Username, Valid: true},
		pgtype.Text{String: replyTo.Content, Valid: true}
}
