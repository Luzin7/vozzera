package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
)

type voiceFakePublisher struct {
	envelopes []realtime.Envelope
	topics    []realtime.Topic
}

func (f *voiceFakePublisher) Publish(_ context.Context, topic realtime.Topic, env realtime.Envelope) error {
	f.topics = append(f.topics, topic)
	f.envelopes = append(f.envelopes, env)
	return nil
}

var _ realtime.Publisher = (*voiceFakePublisher)(nil)

func TestParticipantFromEvent(t *testing.T) {
	p := participantFromEvent(&livekit.ParticipantInfo{
		Sid:      "PSID123",
		Identity: "user-uuid-abc",
		Name:     "Luan",
	})

	if p.SID != "PSID123" {
		t.Errorf("SID = %q, want %q", p.SID, "PSID123")
	}
	if p.UserID != "user-uuid-abc" {
		t.Errorf("UserID = %q, want %q", p.UserID, "user-uuid-abc")
	}
	if p.Username != "Luan" {
		t.Errorf("Username = %q, want %q", p.Username, "Luan")
	}
}

func TestNewWebhookHandler(t *testing.T) {
	presence := realtime.NewVoiceRoomPresence()
	publisher := &voiceFakePublisher{}
	handler := NewWebhookHandler("api-key", "api-secret", presence, publisher)

	if handler == nil {
		t.Fatal("NewWebhookHandler retornou nil")
	}
	if handler.presence != presence {
		t.Error("presence não foi injetado")
	}
	if handler.publisher != publisher {
		t.Error("publisher não foi injetado")
	}
}

func TestWebhookHandler_Handle_EventoDesconhecido(t *testing.T) {
	presence := realtime.NewVoiceRoomPresence()
	publisher := &voiceFakePublisher{}
	handler := NewWebhookHandler("test-key", "test-secret", presence, publisher)

	body := map[string]interface{}{
		"event": "some.random.event",
		"room": map[string]interface{}{
			"name": uuid.New().String(),
		},
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/voice/webhook", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Handle(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, esperava 200 ou 400 (webhook sem assinatura válida)", resp.StatusCode)
	}
}

func TestWebhookHandler_Handle_PayloadMalFormado(t *testing.T) {
	presence := realtime.NewVoiceRoomPresence()
	publisher := &voiceFakePublisher{}
	handler := NewWebhookHandler("test-key", "test-secret", presence, publisher)

	req := httptest.NewRequest(http.MethodPost, "/api/voice/webhook", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Handle(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}
