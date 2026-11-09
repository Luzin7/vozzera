package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func messageRowAt(createdAt time.Time, content string) GetMessagesByRoomRow {
	return GetMessagesByRoomRow{
		ID:        uuid.New(),
		Content:   pgtype.Text{String: content, Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true},
	}
}

func encodeRaw(raw string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func rowsDescending(n int, newest time.Time) []GetMessagesByRoomRow {
	rows := make([]GetMessagesByRoomRow, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, messageRowAt(newest.Add(-time.Duration(i)*time.Second), "m"))
	}
	return rows
}

func toBeforeRows(rows []GetMessagesByRoomRow) []GetMessagesByRoomBeforeRow {
	out := make([]GetMessagesByRoomBeforeRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, GetMessagesByRoomBeforeRow(r))
	}
	return out
}

func TestGetMessagesService_Execute(t *testing.T) {
	t.Run("limite padrao busca uma linha a mais", func(t *testing.T) {
		repo := newFakeRepo()
		var calledLimit int32
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			calledLimit = arg.Limit
			return nil, nil
		}
		svc := NewGetMessagesService(repo)

		_, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 0})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if calledLimit != 51 {
			t.Errorf("limit = %d, want 51", calledLimit)
		}
	})

	t.Run("limite maximo busca uma linha a mais", func(t *testing.T) {
		repo := newFakeRepo()
		var calledLimit int32
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			calledLimit = arg.Limit
			return nil, nil
		}
		svc := NewGetMessagesService(repo)

		_, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 999})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if calledLimit != 201 {
			t.Errorf("limit = %d, want 201", calledLimit)
		}
	})

	t.Run("ordem reversa", func(t *testing.T) {
		repo := newFakeRepo()
		msg1 := GetMessagesByRoomRow{ID: uuid.New(), Content: pgtype.Text{String: "primeira", Valid: true}}
		msg2 := GetMessagesByRoomRow{ID: uuid.New(), Content: pgtype.Text{String: "segunda", Valid: true}}
		msg3 := GetMessagesByRoomRow{ID: uuid.New(), Content: pgtype.Text{String: "terceira", Valid: true}}
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return []GetMessagesByRoomRow{msg3, msg2, msg1}, nil
		}
		svc := NewGetMessagesService(repo)

		out, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Messages) != 3 {
			t.Fatalf("len(messages) = %d, want 3", len(out.Messages))
		}
		if out.Messages[0].ID != msg1.ID {
			t.Errorf("messages[0].id = %v, want %v", out.Messages[0].ID, msg1.ID)
		}
		if out.Messages[1].ID != msg2.ID {
			t.Errorf("messages[1].id = %v, want %v", out.Messages[1].ID, msg2.ID)
		}
		if out.Messages[2].ID != msg3.ID {
			t.Errorf("messages[2].id = %v, want %v", out.Messages[2].ID, msg3.ID)
		}
	})

	t.Run("sem mensagens", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return nil, nil
		}
		svc := NewGetMessagesService(repo)

		out, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Messages == nil {
			t.Error("Messages = nil, want empty slice")
		}
		if len(out.Messages) != 0 {
			t.Errorf("len(Messages) = %d, want 0", len(out.Messages))
		}
		if out.NextBefore != "" {
			t.Errorf("NextBefore = %q, want empty", out.NextBefore)
		}
	})

	t.Run("erro do repositorio", func(t *testing.T) {
		repo := newFakeRepo()
		sentinelErr := errors.New("db error")
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return nil, sentinelErr
		}
		svc := NewGetMessagesService(repo)

		_, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("pagina exata nao tem mais", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return rowsDescending(3, time.Now()), nil
		}
		svc := NewGetMessagesService(repo)

		out, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 3})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Messages) != 3 {
			t.Errorf("len(Messages) = %d, want 3", len(out.Messages))
		}
		if out.NextBefore != "" {
			t.Errorf("NextBefore = %q, want empty", out.NextBefore)
		}
	})

	t.Run("linha extra vira has_more e cursor da mais antiga", func(t *testing.T) {
		rows := rowsDescending(4, time.Now().UTC())
		wantOldest := rows[2]
		repo := newFakeRepo()
		repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
			return rows, nil
		}
		svc := NewGetMessagesService(repo)

		out, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 3})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(out.Messages) != 3 {
			t.Fatalf("len(Messages) = %d, want 3", len(out.Messages))
		}
		oldest := wantOldest
		if out.Messages[0].ID != oldest.ID {
			t.Errorf("messages[0].id = %v, want %v", out.Messages[0].ID, oldest.ID)
		}
		cursor, err := decodeMessageCursor(out.NextBefore)
		if err != nil {
			t.Fatalf("decodeMessageCursor() erro: %v", err)
		}
		if cursor.ID != oldest.ID || !cursor.CreatedAt.Equal(oldest.CreatedAt.Time) {
			t.Errorf("cursor = %+v, want id %v at %v", cursor, oldest.ID, oldest.CreatedAt.Time)
		}
	})

	t.Run("before usa a query com cursor composto", func(t *testing.T) {
		cursor := messageCursor{CreatedAt: time.Now().UTC(), ID: uuid.New()}
		roomID := uuid.New()
		repo := newFakeRepo()
		var got GetMessagesByRoomBeforeParams
		repo.getMessagesBefore = func(ctx context.Context, arg GetMessagesByRoomBeforeParams) ([]GetMessagesByRoomBeforeRow, error) {
			got = arg
			return toBeforeRows(rowsDescending(4, time.Now())), nil
		}
		svc := NewGetMessagesService(repo)

		out, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: roomID, Limit: 3, Before: cursor.encode()})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if got.RoomID != roomID || got.BeforeID != cursor.ID || !got.BeforeCreatedAt.Equal(cursor.CreatedAt) || got.Limit != 4 {
			t.Errorf("params = %+v, want room %v cursor %+v limit 4", got, roomID, cursor)
		}
		if len(out.Messages) != 3 || out.NextBefore == "" {
			t.Errorf("len = %d next = %q, want 3 and non-empty", len(out.Messages), out.NextBefore)
		}
	})

	t.Run("cursor invalido", func(t *testing.T) {
		invalid := map[string]string{
			"nao base64":    "***",
			"sem separador": encodeRaw("2026-01-01T00:00:00Z"),
			"data invalida": encodeRaw("ontem|" + uuid.NewString()),
			"uuid invalido": encodeRaw("2026-01-01T00:00:00Z|xyz"),
			"partes demais": encodeRaw("2026-01-01T00:00:00Z|" + uuid.NewString() + "|x"),
		}
		for name, before := range invalid {
			t.Run(name, func(t *testing.T) {
				repo := newFakeRepo()
				svc := NewGetMessagesService(repo)

				_, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50, Before: before})
				if !errors.Is(err, ErrInvalidCursor) {
					t.Errorf("err = %v, want ErrInvalidCursor", err)
				}
			})
		}
	})

	t.Run("erro do repositorio com cursor", func(t *testing.T) {
		repo := newFakeRepo()
		svc := NewGetMessagesService(repo)
		before := messageCursor{CreatedAt: time.Now(), ID: uuid.New()}.encode()

		_, err := svc.Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50, Before: before})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestMessagesPresenter_ReplyTo(t *testing.T) {
	t.Run("histórico traz snapshot da resposta", func(t *testing.T) {
		targetID := uuid.New()
		rows := []GetMessagesByRoomRow{{
			ID:              uuid.New(),
			Content:         pgtype.Text{String: "resposta", Valid: true},
			CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
			UserID:          uuid.New(),
			Username:        "luand",
			ReplyToID:       pgtype.UUID{Bytes: targetID, Valid: true},
			ReplyToUsername: pgtype.Text{String: "original", Valid: true},
			ReplyToContent:  pgtype.Text{String: "mensagem original", Valid: true},
		}}

		out := MessagesPresenter(rows, nil)
		if len(out) != 1 {
			t.Fatalf("len(out) = %d, want 1", len(out))
		}
		if out[0].ReplyTo == nil {
			t.Fatal("ReplyTo = nil, want snapshot")
		}
		if out[0].ReplyTo.ID != targetID || out[0].ReplyTo.Username != "original" || out[0].ReplyTo.Content != "mensagem original" {
			t.Errorf("ReplyTo = %+v, want id %v", out[0].ReplyTo, targetID)
		}
	})

	t.Run("id nulo com snapshot preenchido vira null", func(t *testing.T) {
		rows := []GetMessagesByRoomRow{{
			ID:              uuid.New(),
			Content:         pgtype.Text{String: "resposta", Valid: true},
			CreatedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
			UserID:          uuid.New(),
			Username:        "luand",
			ReplyToUsername: pgtype.Text{String: "original", Valid: true},
			ReplyToContent:  pgtype.Text{String: "mensagem original", Valid: true},
		}}

		out := MessagesPresenter(rows, nil)
		if out[0].ReplyTo != nil {
			t.Errorf("ReplyTo = %+v, want nil", out[0].ReplyTo)
		}
		raw, err := json.Marshal(out[0])
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if string(payload["reply_to"]) != "null" {
			t.Errorf("reply_to = %s, want null", payload["reply_to"])
		}
	})

	t.Run("sem resposta traz null", func(t *testing.T) {
		rows := []GetMessagesByRoomRow{{
			ID:        uuid.New(),
			Content:   pgtype.Text{String: "sem resposta", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			UserID:    uuid.New(),
			Username:  "luand",
		}}

		out := MessagesPresenter(rows, nil)
		if out[0].ReplyTo != nil {
			t.Errorf("ReplyTo = %+v, want nil", out[0].ReplyTo)
		}
		raw, err := json.Marshal(out[0])
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if string(payload["reply_to"]) != "null" {
			t.Errorf("reply_to = %s, want null", payload["reply_to"])
		}
	})
}

type replySnapshotFixture struct {
	repo    *fakeRepo
	events  *fakePublisher
	target  GetReplyTargetRow
	roomID  uuid.UUID
	userID  uuid.UUID
	history func() []MessageResponse
}

func newReplySnapshotFixture(t *testing.T, original string) *replySnapshotFixture {
	t.Helper()

	f := &replySnapshotFixture{
		repo:   newFakeRepo(),
		events: &fakePublisher{},
		target: GetReplyTargetRow{
			ID:       uuid.New(),
			Content:  pgtype.Text{String: original, Valid: true},
			Username: "luand",
		},
		roomID: uuid.New(),
		userID: uuid.New(),
	}

	var rows []GetMessagesByRoomRow
	f.repo.getReplyTarget = func(ctx context.Context, arg GetReplyTargetParams) (GetReplyTargetRow, error) {
		return f.target, nil
	}
	f.repo.createMessage = func(ctx context.Context, arg CreateMessageParams) (CreateMessageRow, error) {
		createdAt := time.Now()
		rows = append(rows, GetMessagesByRoomRow{
			ID:              uuid.New(),
			Content:         arg.Content,
			CreatedAt:       pgtype.Timestamptz{Time: createdAt, Valid: true},
			UserID:          arg.UserID,
			Username:        "responder",
			ReplyToID:       arg.ReplyToMessageID,
			ReplyToUsername: arg.ReplyToUsername,
			ReplyToContent:  arg.ReplyToContent,
		})
		return CreateMessageRow{
			ID:        rows[len(rows)-1].ID,
			RoomID:    arg.RoomID,
			UserID:    arg.UserID,
			Content:   arg.Content,
			CreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true},
		}, nil
	}
	f.repo.updateMessage = func(ctx context.Context, arg UpdateMessageParams) (UpdateMessageRow, error) {
		f.target.Content = arg.Content
		return UpdateMessageRow{
			ID:        arg.ID,
			RoomID:    arg.RoomID,
			Content:   arg.Content,
			UpdatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}, nil
	}
	f.repo.deleteMessage = func(ctx context.Context, arg DeleteMessageParams) (DeleteMessageRow, error) {
		f.target.Content = pgtype.Text{}
		return DeleteMessageRow{
			ID:        arg.ID,
			RoomID:    arg.RoomID,
			UserID:    arg.UserID,
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}, nil
	}
	f.repo.getMessagesByRoom = func(ctx context.Context, arg GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
		return rows, nil
	}
	f.history = func() []MessageResponse {
		out, err := NewGetMessagesService(f.repo).Execute(context.Background(), GetMessagesInput{RoomID: f.roomID, Limit: 50})
		if err != nil {
			t.Fatalf("GetMessagesService.Execute() erro inesperado: %v", err)
		}
		return MessagesPresenter(out.Messages, out.Reactions)
	}
	return f
}

func (f *replySnapshotFixture) sendReply(t *testing.T, content string) MessagePayload {
	t.Helper()

	if _, err := NewSendMessageService(f.repo, f.events).Execute(context.Background(), SendMessageInput{
		RoomID:           f.roomID,
		UserID:           f.userID,
		Username:         "responder",
		Content:          content,
		ReplyToMessageID: &f.target.ID,
	}); err != nil {
		t.Fatalf("SendMessageService.Execute() erro inesperado: %v", err)
	}

	var payload MessagePayload
	if err := json.Unmarshal(f.events.envelopes[len(f.events.envelopes)-1].Data, &payload); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	return payload
}

func TestReplyToSnapshot(t *testing.T) {
	t.Run("edição da original não altera a resposta", func(t *testing.T) {
		f := newReplySnapshotFixture(t, "original")
		f.sendReply(t, "resposta")

		if _, err := NewUpdateMessageService(f.repo, f.events).Execute(context.Background(), UpdateMessageInput{
			RoomID:    f.roomID,
			ContentID: f.target.ID,
			UserID:    uuid.New(),
			Content:   "editada",
		}); err != nil {
			t.Fatalf("UpdateMessageService.Execute() erro inesperado: %v", err)
		}

		history := f.history()
		if len(history) != 1 || history[0].ReplyTo == nil {
			t.Fatalf("history = %+v, want um reply_to", history)
		}
		if history[0].ReplyTo.Content != "original" {
			t.Errorf("reply_to.content = %q, want %q", history[0].ReplyTo.Content, "original")
		}
	})

	t.Run("exclusão da original mantém o snapshot", func(t *testing.T) {
		f := newReplySnapshotFixture(t, "original")
		f.sendReply(t, "resposta")

		if _, err := NewDeleteMessageService(f.repo, f.events).Execute(context.Background(), DeleteMessageInput{
			RoomID:    f.roomID,
			ContentID: f.target.ID,
			UserID:    uuid.New(),
		}); err != nil {
			t.Fatalf("DeleteMessageService.Execute() erro inesperado: %v", err)
		}

		history := f.history()
		if len(history) != 1 || history[0].ReplyTo == nil {
			t.Fatalf("history = %+v, want um reply_to", history)
		}
		if history[0].ReplyTo.Username != "luand" || history[0].ReplyTo.Content != "original" {
			t.Errorf("reply_to = %+v, want %q/%q", history[0].ReplyTo, "luand", "original")
		}
	})

	t.Run("payload WS e histórico têm o mesmo reply_to", func(t *testing.T) {
		f := newReplySnapshotFixture(t, "original")
		payload := f.sendReply(t, "resposta")

		history := f.history()
		if len(history) != 1 || history[0].ReplyTo == nil {
			t.Fatalf("history = %+v, want um reply_to", history)
		}

		payloadReply, err := json.Marshal(payload.ReplyTo)
		if err != nil {
			t.Fatalf("Marshal payload.ReplyTo: %v", err)
		}
		historyReply, err := json.Marshal(history[0].ReplyTo)
		if err != nil {
			t.Fatalf("Marshal history.ReplyTo: %v", err)
		}
		if string(payloadReply) != string(historyReply) {
			t.Errorf("reply_to = %s no WS e %s no histórico", payloadReply, historyReply)
		}
	})
}
