package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSendMessageService_Execute(t *testing.T) {
	repo := newFakeRepo()
	repo.createMessage = func(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error) {
		return CreateMessageRow{
			ID:        uuid.New(),
			RoomID:    arg.RoomID,
			UserID:    arg.UserID,
			Content:   arg.Content,
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}, nil
	}

	events := &fakePublisher{}
	svc := NewSendMessageService(repo, events)

	t.Run("conteúdo em branco", func(t *testing.T) {
		_, err := svc.Execute(context.Background(), SendMessageInput{RoomID: uuid.New(), Content: "   "})
		if !errors.Is(err, ErrInvalidContent) {
			t.Errorf("erro = %v, want ErrInvalidContent", err)
		}
	})

	t.Run("sucesso emite broadcast", func(t *testing.T) {
		out, err := svc.Execute(context.Background(), SendMessageInput{
			RoomID:   uuid.New(),
			UserID:   uuid.New(),
			Username: "luand",
			Content:  " oi ",
		})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Content != "oi" {
			t.Errorf("content = %q, want %q", out.Content, "oi")
		}

		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}
		env := events.envelopes[0]
		if env.Type != EventMessageCreated {
			t.Errorf("type = %q, want %q", env.Type, EventMessageCreated)
		}
		var payload MessagePayload
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.Username != "luand" {
			t.Errorf("username = %q, want %q", payload.Username, "luand")
		}
	})
}

func TestSendMessageService_Execute_ReplyTo(t *testing.T) {
	target := GetReplyTargetRow{
		ID:       uuid.New(),
		Content:  pgtype.Text{String: "original", Valid: true},
		Username: "luand",
	}

	t.Run("resposta inclui snapshot", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getReplyTarget = func(ctx context.Context, arg GetReplyTargetParams) (GetReplyTargetRow, error) {
			if arg.ID != target.ID {
				t.Errorf("target = %v, want %v", arg.ID, target.ID)
			}
			return target, nil
		}
		repo.createMessage = func(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error) {
			if !arg.ReplyToMessageID.Valid || arg.ReplyToMessageID.Bytes != target.ID {
				t.Errorf("reply_to_message_id = %v, want %v", arg.ReplyToMessageID, target.ID)
			}
			return CreateMessageRow{
				ID:        uuid.New(),
				RoomID:    arg.RoomID,
				UserID:    arg.UserID,
				Content:   arg.Content,
				CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}, nil
		}
		events := &fakePublisher{}
		svc := NewSendMessageService(repo, events)

		out, err := svc.Execute(context.Background(), SendMessageInput{
			RoomID:           uuid.New(),
			UserID:           uuid.New(),
			Username:         "luand",
			Content:          "resposta",
			ReplyToMessageID: &target.ID,
		})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.ReplyTo == nil || out.ReplyTo.ID != target.ID {
			t.Fatalf("ReplyTo = %+v, want id %v", out.ReplyTo, target.ID)
		}

		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}
		var payload MessagePayload
		if err := json.Unmarshal(events.envelopes[0].Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.ReplyTo == nil {
			t.Fatal("ReplyTo = nil, want snapshot")
		}
		if payload.ReplyTo.ID != target.ID || payload.ReplyTo.Username != target.Username || payload.ReplyTo.Content != target.Content.String {
			t.Errorf("ReplyTo = %+v, want %+v", payload.ReplyTo, target)
		}
	})

	t.Run("referência inexistente", func(t *testing.T) {
		repo := newFakeRepo()
		created := false
		repo.getReplyTarget = func(ctx context.Context, arg GetReplyTargetParams) (GetReplyTargetRow, error) {
			return GetReplyTargetRow{}, pgx.ErrNoRows
		}
		repo.createMessage = func(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error) {
			created = true
			return CreateMessageRow{}, nil
		}
		svc := NewSendMessageService(repo, &fakePublisher{})

		_, err := svc.Execute(context.Background(), SendMessageInput{
			RoomID:           uuid.New(),
			Content:          "resposta",
			ReplyToMessageID: &target.ID,
		})
		if !errors.Is(err, ErrInvalidReplyTarget) {
			t.Errorf("erro = %v, want ErrInvalidReplyTarget", err)
		}
		if created {
			t.Error("mensagem não deveria ter sido gravada")
		}
	})

	t.Run("referência de outra sala", func(t *testing.T) {
		roomID := uuid.New()
		repo := newFakeRepo()
		repo.getReplyTarget = func(ctx context.Context, arg GetReplyTargetParams) (GetReplyTargetRow, error) {
			if arg.RoomID != roomID {
				t.Errorf("room = %v, want %v", arg.RoomID, roomID)
			}
			return GetReplyTargetRow{}, pgx.ErrNoRows
		}
		svc := NewSendMessageService(repo, &fakePublisher{})

		_, err := svc.Execute(context.Background(), SendMessageInput{
			RoomID:           roomID,
			Content:          "resposta",
			ReplyToMessageID: &target.ID,
		})
		if !errors.Is(err, ErrInvalidReplyTarget) {
			t.Errorf("erro = %v, want ErrInvalidReplyTarget", err)
		}
	})

	t.Run("sem resposta publica reply_to nulo", func(t *testing.T) {
		repo := newFakeRepo()
		repo.createMessage = func(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error) {
			if arg.ReplyToMessageID.Valid {
				t.Errorf("reply_to_message_id = %v, want null", arg.ReplyToMessageID)
			}
			return CreateMessageRow{
				ID:        uuid.New(),
				RoomID:    arg.RoomID,
				UserID:    arg.UserID,
				Content:   arg.Content,
				CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			}, nil
		}
		events := &fakePublisher{}
		svc := NewSendMessageService(repo, events)

		if _, err := svc.Execute(context.Background(), SendMessageInput{
			RoomID:  uuid.New(),
			Content: "sem resposta",
		}); err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(events.envelopes[0].Data, &raw); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if string(raw["reply_to"]) != "null" {
			t.Errorf("reply_to = %s, want null", raw["reply_to"])
		}
	})
}
