package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func noRowsRepo() *fakeRepo {
	repo := newFakeRepo()
	repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
		return GetReactionStateRow{}, pgx.ErrNoRows
	}
	repo.addReaction = func(context.Context, AddReactionParams) (int64, error) {
		return 0, pgx.ErrNoRows
	}
	return repo
}

func TestAddReactionService_Execute(t *testing.T) {
	t.Run("adiciona e devolve a contagem agregada", func(t *testing.T) {
		repo := newFakeRepo()
		messageID := uuid.New()
		userID := uuid.New()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{DistinctEmojiCount: 1, EmojiExists: false}, nil
		}
		repo.addReaction = func(ctx context.Context, arg AddReactionParams) (int64, error) {
			if arg.MessageID != messageID || arg.UserID != userID || arg.Emoji != "🔥" {
				t.Errorf("params = %+v, want mensagem %v usuário %v emoji 🔥", arg, messageID, userID)
			}
			return 1, nil
		}
		repo.getReactionsByIDs = func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
			return []GetReactionsByMessageIDsRow{
				{MessageID: messageID, Emoji: "🔥", Count: 1, UserIds: []uuid.UUID{userID}},
			}, nil
		}
		events := &fakePublisher{}

		out, err := NewAddReactionService(repo, events).Execute(context.Background(), ReactionInput{
			RoomID:    uuid.New(),
			MessageID: messageID,
			UserID:    userID,
			Username:  "alice",
			Emoji:     " 🔥 ",
		})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Reaction.Emoji != "🔥" || out.Reaction.Count != 1 {
			t.Errorf("reaction = %+v, want emoji 🔥 count 1", out.Reaction)
		}
	})

	t.Run("add duplicado não publica evento", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{DistinctEmojiCount: 1, EmojiExists: true}, nil
		}
		repo.addReaction = func(context.Context, AddReactionParams) (int64, error) {
			return 0, nil
		}
		repo.getReactionsByIDs = func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
			return []GetReactionsByMessageIDsRow{
				{Emoji: "🔥", Count: 1, UserIds: []uuid.UUID{uuid.New()}},
			}, nil
		}
		events := &fakePublisher{}

		out, err := NewAddReactionService(repo, events).Execute(context.Background(), ReactionInput{Emoji: "🔥"})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Reaction.Count != 1 {
			t.Errorf("count = %d, want 1", out.Reaction.Count)
		}
		if len(events.envelopes) != 0 {
			t.Errorf("envelopes = %d, want 0", len(events.envelopes))
		}
	})

	t.Run("emoji vazio", func(t *testing.T) {
		_, err := NewAddReactionService(newFakeRepo(), &fakePublisher{}).Execute(context.Background(), ReactionInput{Emoji: "   "})
		if !errors.Is(err, ErrEmptyEmoji) {
			t.Errorf("erro = %v, want ErrEmptyEmoji", err)
		}
	})

	t.Run("emoji longo demais", func(t *testing.T) {
		_, err := NewAddReactionService(newFakeRepo(), &fakePublisher{}).Execute(context.Background(), ReactionInput{Emoji: strings.Repeat("🔥", 11)})
		if !errors.Is(err, ErrEmojiTooLong) {
			t.Errorf("erro = %v, want ErrEmojiTooLong", err)
		}
	})

	t.Run("vigésimo primeiro emoji distinto", func(t *testing.T) {
		repo := newFakeRepo()
		added := false
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{DistinctEmojiCount: 20, EmojiExists: false}, nil
		}
		repo.addReaction = func(context.Context, AddReactionParams) (int64, error) {
			added = true
			return 1, nil
		}

		_, err := NewAddReactionService(repo, &fakePublisher{}).Execute(context.Background(), ReactionInput{Emoji: "🔥"})
		if !errors.Is(err, ErrTooManyReactions) {
			t.Errorf("erro = %v, want ErrTooManyReactions", err)
		}
		if added {
			t.Error("reação gravada acima do limite de emojis distintos")
		}
	})

	t.Run("emoji já existente é permitido no limite", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{DistinctEmojiCount: 20, EmojiExists: true}, nil
		}
		repo.addReaction = func(context.Context, AddReactionParams) (int64, error) {
			return 1, nil
		}
		repo.getReactionsByIDs = func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
			return nil, nil
		}

		_, err := NewAddReactionService(repo, &fakePublisher{}).Execute(context.Background(), ReactionInput{Emoji: "🔥"})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
	})

	t.Run("mensagem de outra sala", func(t *testing.T) {
		_, err := NewAddReactionService(noRowsRepo(), &fakePublisher{}).Execute(context.Background(), ReactionInput{Emoji: "🔥"})
		if !errors.Is(err, ErrMessageNotFound) {
			t.Errorf("erro = %v, want ErrMessageNotFound", err)
		}
	})

	t.Run("mensagem apagada", func(t *testing.T) {
		_, err := NewAddReactionService(noRowsRepo(), &fakePublisher{}).Execute(context.Background(), ReactionInput{Emoji: "🔥"})
		if !errors.Is(err, ErrMessageNotFound) {
			t.Errorf("erro = %v, want ErrMessageNotFound", err)
		}
	})

	t.Run("evento leva a contagem nova", func(t *testing.T) {
		repo := newFakeRepo()
		roomID := uuid.New()
		messageID := uuid.New()
		userID := uuid.New()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{DistinctEmojiCount: 1}, nil
		}
		repo.addReaction = func(context.Context, AddReactionParams) (int64, error) {
			return 1, nil
		}
		repo.getReactionsByIDs = func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
			return []GetReactionsByMessageIDsRow{
				{MessageID: messageID, Emoji: "🔥", Count: 3, UserIds: []uuid.UUID{userID}},
			}, nil
		}
		events := &fakePublisher{}

		_, err := NewAddReactionService(repo, events).Execute(context.Background(), ReactionInput{
			RoomID:    roomID,
			MessageID: messageID,
			UserID:    userID,
			Username:  "alice",
			Emoji:     "🔥",
		})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if len(events.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(events.envelopes))
		}

		env := events.envelopes[0]
		if env.Type != EventReactionAdded {
			t.Errorf("type = %q, want %q", env.Type, EventReactionAdded)
		}
		if events.topics[0] != realtime.RoomTopic(roomID) {
			t.Errorf("topic = %q, want %q", events.topics[0], realtime.RoomTopic(roomID))
		}

		var payload ReactionPayload
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.MessageID != messageID || payload.Emoji != "🔥" || payload.UserID != userID || payload.Username != "alice" || payload.Count != 3 {
			t.Errorf("payload = %+v, want mensagem %v emoji 🔥 usuário %v count 3", payload, messageID, userID)
		}
	})
}

func TestRemoveReactionService_Execute(t *testing.T) {
	t.Run("remove e devolve a contagem restante", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{DistinctEmojiCount: 1, EmojiExists: true}, nil
		}
		repo.removeReaction = func(context.Context, RemoveReactionParams) (int64, error) {
			return 1, nil
		}
		repo.getReactionsByIDs = func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
			return []GetReactionsByMessageIDsRow{{Emoji: "🔥", Count: 2, UserIds: []uuid.UUID{uuid.New()}}}, nil
		}
		events := &fakePublisher{}

		out, err := NewRemoveReactionService(repo, events).Execute(context.Background(), ReactionInput{Emoji: "🔥"})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Reaction.Count != 2 {
			t.Errorf("count = %d, want 2", out.Reaction.Count)
		}
		if len(events.envelopes) != 1 || events.envelopes[0].Type != EventReactionRemoved {
			t.Fatalf("envelopes = %+v, want um %s", events.envelopes, EventReactionRemoved)
		}
	})

	t.Run("remover reação inexistente é no-op", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{}, nil
		}
		repo.removeReaction = func(context.Context, RemoveReactionParams) (int64, error) {
			return 0, nil
		}
		repo.getReactionsByIDs = func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
			return nil, nil
		}
		events := &fakePublisher{}

		out, err := NewRemoveReactionService(repo, events).Execute(context.Background(), ReactionInput{Emoji: "🔥"})
		if err != nil {
			t.Fatalf("Execute() erro inesperado: %v", err)
		}
		if out.Reaction.Count != 0 {
			t.Errorf("count = %d, want 0", out.Reaction.Count)
		}
		if len(events.envelopes) != 0 {
			t.Errorf("envelopes = %d, want 0", len(events.envelopes))
		}
	})

	t.Run("mensagem apagada", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{}, pgx.ErrNoRows
		}

		_, err := NewRemoveReactionService(repo, &fakePublisher{}).Execute(context.Background(), ReactionInput{Emoji: "🔥"})
		if !errors.Is(err, ErrMessageNotFound) {
			t.Errorf("erro = %v, want ErrMessageNotFound", err)
		}
	})
}

func TestGetMessagesService_Reactions(t *testing.T) {
	messageID := uuid.New()
	otherID := uuid.New()
	repo := newFakeRepo()
	repo.getMessagesByRoom = func(context.Context, GetMessagesByRoomParams) ([]GetMessagesByRoomRow, error) {
		return []GetMessagesByRoomRow{
			{ID: otherID, Content: pgtype.Text{String: "sem reação", Valid: true}, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			{ID: messageID, Content: pgtype.Text{String: "com reação", Valid: true}, CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		}, nil
	}
	repo.getReactionsByIDs = func(_ context.Context, messageIDs []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
		if len(messageIDs) != 2 {
			t.Errorf("messageIDs = %v, want duas mensagens numa consulta", messageIDs)
		}
		return []GetReactionsByMessageIDsRow{
			{MessageID: messageID, Emoji: "🔥", Count: 1, UserIds: []uuid.UUID{uuid.New()}},
			{MessageID: messageID, Emoji: "😂", Count: 2, UserIds: []uuid.UUID{uuid.New(), uuid.New()}},
		}, nil
	}

	out, err := NewGetMessagesService(repo).Execute(context.Background(), GetMessagesInput{RoomID: uuid.New(), Limit: 50})
	if err != nil {
		t.Fatalf("Execute() erro inesperado: %v", err)
	}

	response := MessagesPresenter(out.Messages, out.Reactions)
	withReactions := response[0]
	if len(withReactions.Reactions) != 2 {
		t.Fatalf("reactions = %d, want 2", len(withReactions.Reactions))
	}
	if withReactions.Reactions[0].Emoji != "🔥" || withReactions.Reactions[1].Emoji != "😂" {
		t.Errorf("ordem = %q, %q, want 🔥, 😂", withReactions.Reactions[0].Emoji, withReactions.Reactions[1].Emoji)
	}
	if len(response[1].Reactions) != 0 {
		t.Errorf("mensagem sem reações = %+v, want vazio", response[1].Reactions)
	}
}

func TestMessagesPresenter_EmptyReactionsSerializesAsArray(t *testing.T) {
	body, err := json.Marshal(MessagesPresenter(
		[]GetMessagesByRoomRow{{ID: uuid.New(), Content: pgtype.Text{String: "oi", Valid: true}}},
		nil,
	))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), `"reactions":[]`) {
		t.Errorf("json = %s, want reactions como []", body)
	}
}

func TestMessagePayload_EmptyReactionsSerializesAsArray(t *testing.T) {
	body, err := json.Marshal(MessagePayload{Reactions: []ReactionSummary{}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), `"reactions":[]`) {
		t.Errorf("json = %s, want reactions como []", body)
	}
}

func TestHandleAddReaction(t *testing.T) {
	messageID := uuid.New()
	reactionPath := func() string {
		return "/api/rooms/" + uuid.NewString() + "/messages/" + messageID.String() + "/reactions"
	}
	reactableRepo := func() *fakeRepo {
		repo := newFakeRepo()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{DistinctEmojiCount: 1}, nil
		}
		repo.addReaction = func(context.Context, AddReactionParams) (int64, error) {
			return 1, nil
		}
		repo.getReactionsByIDs = func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
			return []GetReactionsByMessageIDsRow{{MessageID: messageID, Emoji: "🔥", Count: 1, UserIds: []uuid.UUID{uuid.New()}}}, nil
		}
		return repo
	}

	t.Run("sucesso devolve a reação agregada", func(t *testing.T) {
		w := httptest.NewRecorder()
		newChatMux(reactableRepo(), nil).ServeHTTP(w, authenticatedRequest(http.MethodPost, reactionPath(), `{"emoji":"🔥"}`))

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var body ReactionResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if body.Emoji != "🔥" || body.Count != 1 {
			t.Errorf("body = %+v, want emoji 🔥 count 1", body)
		}
	})

	t.Run("sala sem acesso", func(t *testing.T) {
		added := false
		repo := reactableRepo()
		repo.addReaction = func(context.Context, AddReactionParams) (int64, error) {
			added = true
			return 1, nil
		}

		w := httptest.NewRecorder()
		newChatMux(repo, fakeRoomAccess{allowed: false}).ServeHTTP(w, authenticatedRequest(http.MethodPost, reactionPath(), `{"emoji":"🔥"}`))

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		if added {
			t.Error("reação gravada numa sala sem acesso")
		}
	})

	t.Run("mensagem apagada", func(t *testing.T) {
		repo := newFakeRepo()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{}, pgx.ErrNoRows
		}

		w := httptest.NewRecorder()
		newChatMux(repo, nil).ServeHTTP(w, authenticatedRequest(http.MethodPost, reactionPath(), `{"emoji":"🔥"}`))

		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})

	t.Run("emoji vazio", func(t *testing.T) {
		w := httptest.NewRecorder()
		newChatMux(newFakeRepo(), nil).ServeHTTP(w, authenticatedRequest(http.MethodPost, reactionPath(), `{"emoji":""}`))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("emoji longo demais", func(t *testing.T) {
		w := httptest.NewRecorder()
		body := `{"emoji":"` + strings.Repeat("🔥", 11) + `"}`
		newChatMux(newFakeRepo(), nil).ServeHTTP(w, authenticatedRequest(http.MethodPost, reactionPath(), body))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("vigésimo primeiro emoji", func(t *testing.T) {
		repo := reactableRepo()
		repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
			return GetReactionStateRow{DistinctEmojiCount: 20}, nil
		}

		w := httptest.NewRecorder()
		newChatMux(repo, nil).ServeHTTP(w, authenticatedRequest(http.MethodPost, reactionPath(), `{"emoji":"🔥"}`))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestHandleRemoveReaction(t *testing.T) {
	messageID := uuid.New()
	repo := newFakeRepo()
	repo.getReactionState = func(context.Context, GetReactionStateParams) (GetReactionStateRow, error) {
		return GetReactionStateRow{DistinctEmojiCount: 1, EmojiExists: true}, nil
	}
	repo.removeReaction = func(context.Context, RemoveReactionParams) (int64, error) {
		return 1, nil
	}
	repo.getReactionsByIDs = func(context.Context, []uuid.UUID) ([]GetReactionsByMessageIDsRow, error) {
		return nil, nil
	}

	path := "/api/rooms/" + uuid.NewString() + "/messages/" + messageID.String() + "/reactions"
	w := httptest.NewRecorder()
	newChatMux(repo, nil).ServeHTTP(w, authenticatedRequest(http.MethodDelete, path, `{"emoji":"🔥"}`))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var body ReactionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if body.Emoji != "🔥" || body.Count != 0 {
		t.Errorf("body = %+v, want emoji 🔥 count 0", body)
	}
}
