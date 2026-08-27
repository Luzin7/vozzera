package chat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeRegisterer struct {
	subscribed   map[string][]realtime.Topic
	unsubscribed map[string][]realtime.Topic
}

func newFakeRegisterer() *fakeRegisterer {
	return &fakeRegisterer{
		subscribed:   make(map[string][]realtime.Topic),
		unsubscribed: make(map[string][]realtime.Topic),
	}
}

func (f *fakeRegisterer) Register(c *realtime.Client) {}
func (f *fakeRegisterer) Unregister(c *realtime.Client) {}
func (f *fakeRegisterer) Subscribe(c *realtime.Client, topic realtime.Topic) {
	key := c.UserID.String()
	f.subscribed[key] = append(f.subscribed[key], topic)
}
func (f *fakeRegisterer) Unsubscribe(c *realtime.Client, topic realtime.Topic) {
	key := c.UserID.String()
	f.unsubscribed[key] = append(f.unsubscribed[key], topic)
}

type fakeAuthorizer struct {
	err error
}

func (f *fakeAuthorizer) CanSubscribe(ctx context.Context, userID string, topic realtime.Topic) error {
	return f.err
}

func TestChatRouter_HandleMessage(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()
	roomID := uuid.New()
	username := "luand"

	t.Run("subscribe sala text", func(t *testing.T) {
		reg := newFakeRegisterer()
		client := realtime.NewClient(reg, nil, userID, username, sessionID, nil)
		pub := &fakePublisher{}
		router := &ChatRouter{
			registerer: reg,
			publisher:  pub,
			authorizer: &fakeAuthorizer{err: nil},
		}

		data, _ := json.Marshal(map[string]interface{}{"room_id": roomID.String()})
		env := realtime.Envelope{V: 1, Type: CmdSubscribe, Data: data}

		err := router.HandleMessage(client, env)
		if err != nil {
			t.Fatalf("HandleMessage() erro inesperado: %v", err)
		}

		got := reg.subscribed[userID.String()]
		if len(got) != 1 {
			t.Fatalf("subscribed = %d, want 1", len(got))
		}
		want := realtime.Topic("room:" + roomID.String())
		if got[0] != want {
			t.Errorf("topic = %q, want %q", got[0], want)
		}
	})

	t.Run("subscribe sala voice", func(t *testing.T) {
		reg := newFakeRegisterer()
		client := realtime.NewClient(reg, nil, userID, username, sessionID, nil)
		router := &ChatRouter{
			registerer: reg,
			authorizer: &fakeAuthorizer{err: ErrNotTextRoom},
		}

		data, _ := json.Marshal(map[string]interface{}{"room_id": roomID.String()})
		env := realtime.Envelope{V: 1, Type: CmdSubscribe, Data: data}

		err := router.HandleMessage(client, env)
		if err != nil {
			t.Fatalf("HandleMessage() erro inesperado: %v", err)
		}

		if len(reg.subscribed[userID.String()]) != 0 {
			t.Error("subscribe não deveria ter sido chamado")
		}
	})

	t.Run("unsubscribe", func(t *testing.T) {
		reg := newFakeRegisterer()
		client := realtime.NewClient(reg, nil, userID, username, sessionID, nil)
		router := &ChatRouter{registerer: reg}

		data, _ := json.Marshal(map[string]interface{}{"room_id": roomID.String()})
		env := realtime.Envelope{V: 1, Type: CmdUnsubscribe, Data: data}

		err := router.HandleMessage(client, env)
		if err != nil {
			t.Fatalf("HandleMessage() erro inesperado: %v", err)
		}

		got := reg.unsubscribed[userID.String()]
		if len(got) != 1 {
			t.Fatalf("unsubscribed = %d, want 1", len(got))
		}
		want := realtime.Topic("room:" + roomID.String())
		if got[0] != want {
			t.Errorf("topic = %q, want %q", got[0], want)
		}
	})

	t.Run("typing.start", func(t *testing.T) {
		reg := newFakeRegisterer()
		client := realtime.NewClient(reg, nil, userID, username, sessionID, nil)
		pub := &fakePublisher{}
		router := &ChatRouter{registerer: reg, publisher: pub}

		data, _ := json.Marshal(map[string]interface{}{"room_id": roomID.String()})
		env := realtime.Envelope{V: 1, Type: CmdTypingStart, Data: data}

		err := router.HandleMessage(client, env)
		if err != nil {
			t.Fatalf("HandleMessage() erro inesperado: %v", err)
		}

		if len(pub.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(pub.envelopes))
		}
		got := pub.envelopes[0]
		if got.Type != CmdTypingStart {
			t.Errorf("type = %q, want %q", got.Type, CmdTypingStart)
		}
		if got.Topic != realtime.Topic("room:"+roomID.String()) {
			t.Errorf("topic = %q, want %q", got.Topic, "room:"+roomID.String())
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(got.Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload["user_id"] != userID.String() {
			t.Errorf("user_id = %v, want %v", payload["user_id"], userID.String())
		}
		if payload["username"] != username {
			t.Errorf("username = %v, want %v", payload["username"], username)
		}
	})

	t.Run("typing.stop", func(t *testing.T) {
		reg := newFakeRegisterer()
		client := realtime.NewClient(reg, nil, userID, username, sessionID, nil)
		pub := &fakePublisher{}
		router := &ChatRouter{registerer: reg, publisher: pub}

		data, _ := json.Marshal(map[string]interface{}{"room_id": roomID.String()})
		env := realtime.Envelope{V: 1, Type: CmdTypingStop, Data: data}

		err := router.HandleMessage(client, env)
		if err != nil {
			t.Fatalf("HandleMessage() erro inesperado: %v", err)
		}

		if len(pub.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(pub.envelopes))
		}
		got := pub.envelopes[0]
		if got.Type != CmdTypingStop {
			t.Errorf("type = %q, want %q", got.Type, CmdTypingStop)
		}
	})

	t.Run("message", func(t *testing.T) {
		reg := newFakeRegisterer()
		client := realtime.NewClient(reg, nil, userID, username, sessionID, nil)
		pub := &fakePublisher{}
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
		sender := NewSendMessageService(repo, pub)
		router := &ChatRouter{sender: sender, registerer: reg, publisher: pub}

		data, _ := json.Marshal(map[string]interface{}{"room_id": roomID.String(), "content": "hello"})
		env := realtime.Envelope{V: 1, Type: CmdMessage, Data: data}

		err := router.HandleMessage(client, env)
		if err != nil {
			t.Fatalf("HandleMessage() erro inesperado: %v", err)
		}

		if len(pub.envelopes) != 1 {
			t.Fatalf("envelopes = %d, want 1", len(pub.envelopes))
		}
		got := pub.envelopes[0]
		if got.Type != EventMessageCreated {
			t.Errorf("type = %q, want %q", got.Type, EventMessageCreated)
		}

		var payload MessagePayload
		if err := json.Unmarshal(got.Data, &payload); err != nil {
			t.Fatalf("Unmarshal payload: %v", err)
		}
		if payload.Username != username {
			t.Errorf("username = %q, want %q", payload.Username, username)
		}
	})

	t.Run("command com room_id nulo", func(t *testing.T) {
		reg := newFakeRegisterer()
		client := realtime.NewClient(reg, nil, userID, username, sessionID, nil)
		router := &ChatRouter{registerer: reg}

		data, _ := json.Marshal(map[string]interface{}{"room_id": uuid.Nil.String()})
		env := realtime.Envelope{V: 1, Type: CmdSubscribe, Data: data}

		err := router.HandleMessage(client, env)
		if err != nil {
			t.Fatalf("HandleMessage() erro inesperado: %v", err)
		}

		if len(reg.subscribed[userID.String()]) != 0 {
			t.Error("subscribe não deveria ter sido chamado para room_id nulo")
		}
	})
}