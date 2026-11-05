package chat

import (
	"context"
	"encoding/base64"
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
