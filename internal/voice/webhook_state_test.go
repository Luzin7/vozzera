package voice

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
)

const (
	webhookKey    = "test-key"
	webhookSecret = "test-secret-with-enough-length-for-hs256"
)

type webhookEventSpec struct {
	event          string
	eventID        string
	roomID         uuid.UUID
	userID         uuid.UUID
	sid            string
	createdAt      int64
	roomCreatedMs  int64
	roomCreatedSec int64
}

func signedWebhookRequest(t *testing.T, spec webhookEventSpec) *http.Request {
	t.Helper()

	body := fmt.Sprintf(`{"event":%q,"id":%q,"createdAt":"%d","room":{"name":%q%s},"participant":{"sid":%q,"identity":%q,"name":"alice","joinedAt":"%d"}}`,
		spec.event, spec.eventID, spec.createdAt, spec.roomID, roomCreationJSON(spec), spec.sid, spec.userID, spec.createdAt-60)
	sum := sha256.Sum256([]byte(body))
	token, err := auth.NewAccessToken(webhookKey, webhookSecret).
		SetSha256(base64.StdEncoding.EncodeToString(sum[:])).
		SetValidFor(time.Minute).
		ToJWT()
	if err != nil {
		t.Fatalf("ToJWT erro: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/voice/webhook", strings.NewReader(body))
	req.Header.Set("Authorization", token)
	return req
}

func roomCreationJSON(spec webhookEventSpec) string {
	fields := ""
	if spec.roomCreatedSec != 0 {
		fields += fmt.Sprintf(`,"creationTime":"%d"`, spec.roomCreatedSec)
	}
	if spec.roomCreatedMs != 0 {
		fields += fmt.Sprintf(`,"creationTimeMs":"%d"`, spec.roomCreatedMs)
	}
	return fields
}

func serveWebhook(handler *WebhookHandler, req *http.Request) int {
	recorder := httptest.NewRecorder()
	handler.Handle(recorder, req)
	return recorder.Code
}

func newWebhookFixture(state VoiceRoomState) (*WebhookHandler, *lockedPublisher) {
	publisher := &lockedPublisher{}
	return NewWebhookHandler(webhookKey, webhookSecret, state, publisher), publisher
}

func joinedSpec(roomID, userID uuid.UUID, sid string, createdAt int64) webhookEventSpec {
	return webhookEventSpec{event: "participant_joined", eventID: "EV_" + sid, roomID: roomID, userID: userID, sid: sid, createdAt: createdAt}
}

func TestWebhook_JoinedGravaNoEstadoEPublicaPresenca(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID, userID := uuid.New(), uuid.New()

	status := serveWebhook(handler, signedWebhookRequest(t, joinedSpec(roomID, userID, "sid-a", 1_700_000_000)))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if got := mustSnapshot(t, state, roomID).Participants; len(got) != 1 || got[0].SID != "sid-a" {
		t.Errorf("participantes = %+v, want sid-a", got)
	}
	if got := publisher.types(); len(got) != 2 || got[0] != "voice.presence.joined" || got[1] != EventSessionSnapshot {
		t.Errorf("publicado = %v, want [voice.presence.joined %s]", got, EventSessionSnapshot)
	}
}

func TestWebhook_JoinedUsaCreatedAtEmMilissegundosParaLimparAusente(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID, userID := joinedRoomWithAwayUser(t, state, 1_700_000_000_000)

	status := serveWebhook(handler, signedWebhookRequest(t, joinedSpec(roomID, userID, "sid-back", 1_700_000_000)))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	assertAwayUsers(t, mustSnapshot(t, state, roomID))
	want := []string{"voice.presence.joined", EventSessionSnapshot, EventAwaySnapshot, EventAwayEnded}
	if got := publisher.types(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("publicado = %v, want %v", got, want)
	}
	var payload awayEndedPayload
	json.Unmarshal(publisher.last().Data, &payload)
	if payload.Reason != AwayEndedReturned || payload.UserID != userID {
		t.Errorf("payload = %+v, want returned do usuário", payload)
	}
}

func joinedRoomWithAwayUser(t *testing.T, state VoiceRoomState, sinceMs int64) (roomID, awayUser uuid.UUID) {
	t.Helper()
	roomID, awayUser = uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(uuid.New(), "sid-active"), at(0))
	mustEnterAway(t, state, AwayEntry{RoomID: roomID, UserID: awayUser, Username: "alice", Since: time.UnixMilli(sinceMs), Owner: AwayOwner{InstanceID: uuid.New(), ConnID: uuid.New()}})
	return roomID, awayUser
}

func TestWebhook_LeftSempreRepublicaPresencaMesmoReenviado(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID, userID := uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(userID, "sid-a"), at(0))
	left := webhookEventSpec{event: "participant_left", eventID: "EV_left", roomID: roomID, userID: userID, sid: "sid-a", createdAt: 1_700_000_100}

	first := serveWebhook(handler, signedWebhookRequest(t, left))
	second := serveWebhook(handler, signedWebhookRequest(t, left))

	if first != http.StatusOK || second != http.StatusOK {
		t.Fatalf("status = %d e %d, want 200", first, second)
	}
	want := []string{"voice.presence.left", EventSessionSnapshot, EventAwaySnapshot, "voice.presence.left", EventSessionSnapshot, EventAwaySnapshot}
	if got := publisher.types(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("publicado = %v, want %v", got, want)
	}
}

func TestWebhook_LeftReenviadoDepoisDeAplicadoAindaPublicaSnapshotDeAusentesVazio(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID, _ := joinedRoomWithAwayUser(t, state, 1_700_000_000_000)
	left := webhookEventSpec{event: "participant_left", eventID: "EV_left", roomID: roomID, userID: uuid.New(), sid: "sid-active", createdAt: 1_700_000_100}
	serveWebhook(handler, signedWebhookRequest(t, left))
	publisher.envelopes = nil

	status := serveWebhook(handler, signedWebhookRequest(t, left))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	want := []string{"voice.presence.left", EventSessionSnapshot, EventAwaySnapshot}
	if got := publisher.types(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("publicado = %v, want %v", got, want)
	}
	if string(publisher.last().Data) != "[]" {
		t.Errorf("snapshot de ausentes = %s, want []", publisher.last().Data)
	}
}

func TestWebhook_LeftDoUltimoAtivoLimpaAusentes(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID, awayUser := joinedRoomWithAwayUser(t, state, 1_700_000_000_000)
	left := webhookEventSpec{event: "participant_left", eventID: "EV_left", roomID: roomID, userID: uuid.New(), sid: "sid-active", createdAt: 1_700_000_100}

	status := serveWebhook(handler, signedWebhookRequest(t, left))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	want := []string{"voice.presence.left", EventSessionSnapshot, EventAwaySnapshot, EventAwayEnded}
	if got := publisher.types(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("publicado = %v, want %v", got, want)
	}
	var payload awayEndedPayload
	json.Unmarshal(publisher.last().Data, &payload)
	if payload.Reason != AwayEndedRoomEmpty || payload.UserID != awayUser {
		t.Errorf("payload = %+v, want room_empty do ausente", payload)
	}
	if string(publisher.envelopes[2].Data) != "[]" {
		t.Errorf("snapshot de ausentes = %s, want []", publisher.envelopes[2].Data)
	}
}

func TestWebhook_RoomFinishedLimpaTudoEPublicaSnapshotsVazios(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID, _ := joinedRoomWithAwayUser(t, state, 1_700_000_000_000)
	finished := webhookEventSpec{event: "room_finished", eventID: "EV_fin", roomID: roomID, userID: uuid.New(), sid: "x", createdAt: 1_700_000_200}

	status := serveWebhook(handler, signedWebhookRequest(t, finished))

	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	want := []string{"voice.presence.snapshot", EventSessionSnapshot, EventAwaySnapshot, EventAwayEnded}
	if got := publisher.types(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("publicado = %v, want %v", got, want)
	}
	snapshot := mustSnapshot(t, state, roomID)
	if len(snapshot.Participants) != 0 || len(snapshot.Away) != 0 {
		t.Errorf("snapshot = %+v, want vazio", snapshot)
	}
}

func TestWebhook_ErroDoEstadoRetorna503ELogaSemPublicar(t *testing.T) {
	events := []string{"participant_joined", "participant_left", "room_finished"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			buf := captureLog(t)
			state := &erroringState{VoiceRoomState: newTestState(), webhookErr: errKVDown}
			handler, publisher := newWebhookFixture(state)
			spec := webhookEventSpec{event: event, eventID: "EV_503", roomID: uuid.New(), userID: uuid.New(), sid: "sid-a", createdAt: 1_700_000_000}

			status := serveWebhook(handler, signedWebhookRequest(t, spec))

			if status != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503", status)
			}
			if got := publisher.types(); len(got) != 0 {
				t.Errorf("publicado = %v, want nada", got)
			}
			wantLog := fmt.Sprintf("voice.webhook level=error event_id=EV_503 event=%s room_id=%s status=503", event, spec.roomID)
			if !strings.Contains(buf.String(), wantLog) {
				t.Errorf("log = %q, want %q", buf.String(), wantLog)
			}
		})
	}
}

func TestWebhook_LeftMantemLogDeSessaoComEventID(t *testing.T) {
	buf := captureLog(t)
	handler, _ := newWebhookFixture(newTestState())
	left := webhookEventSpec{event: "participant_left", eventID: "EV_session", roomID: uuid.New(), userID: uuid.New(), sid: "sid-a", createdAt: 1_700_000_100}

	serveWebhook(handler, signedWebhookRequest(t, left))

	if !strings.Contains(buf.String(), "voice.session_ended event_id=EV_session") {
		t.Errorf("log = %q, want voice.session_ended com event_id", buf.String())
	}
}

const sessionCreatedMs = 1_700_000_000_123

func sessionEnvelopes(publisher *lockedPublisher) []string {
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	var payloads []string
	for _, env := range publisher.envelopes {
		if env.Type == EventSessionSnapshot {
			payloads = append(payloads, string(env.Data))
		}
	}
	return payloads
}

func TestWebhook_JoinedPublicaSessaoEmUTCComMilissegundos(t *testing.T) {
	handler, publisher := newWebhookFixture(newTestState())
	roomID := uuid.New()
	spec := joinedSpec(roomID, uuid.New(), "sid-a", 1_700_000_005)
	spec.roomCreatedMs = sessionCreatedMs

	serveWebhook(handler, signedWebhookRequest(t, spec))

	want := []string{`{"started_at":"2023-11-14T22:13:20.123Z"}`}
	if got := sessionEnvelopes(publisher); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sessão publicada = %v, want %v", got, want)
	}
	if topic := publisher.envelopes[1].Topic; topic != realtime.RoomTopic(roomID) {
		t.Errorf("tópico = %s, want o da sala", topic)
	}
}

func TestWebhook_JoinedRepetidoPublicaSessaoIdentica(t *testing.T) {
	handler, publisher := newWebhookFixture(newTestState())
	spec := joinedSpec(uuid.New(), uuid.New(), "sid-a", 1_700_000_005)
	spec.roomCreatedMs = sessionCreatedMs

	serveWebhook(handler, signedWebhookRequest(t, spec))
	serveWebhook(handler, signedWebhookRequest(t, spec))

	got := sessionEnvelopes(publisher)
	if len(got) != 2 || got[0] != got[1] {
		t.Fatalf("sessões publicadas = %v, want duas idênticas", got)
	}
}

func TestWebhook_JoinedSemCriacaoDaSalaUsaHorarioDoEventoELoga(t *testing.T) {
	buf := captureLog(t)
	handler, publisher := newWebhookFixture(newTestState())
	spec := joinedSpec(uuid.New(), uuid.New(), "sid-a", 1_700_000_005)

	serveWebhook(handler, signedWebhookRequest(t, spec))

	want := []string{`{"started_at":"2023-11-14T22:13:25.000Z"}`}
	if got := sessionEnvelopes(publisher); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sessão publicada = %v, want %v", got, want)
	}
	wantLog := "voice.session level=warn cause=missing_room_creation_time event_id=EV_sid-a"
	if !strings.Contains(buf.String(), wantLog) {
		t.Errorf("log = %q, want %q", buf.String(), wantLog)
	}
}

func TestWebhook_JoinedComSegundosUsaCreationTime(t *testing.T) {
	handler, publisher := newWebhookFixture(newTestState())
	spec := joinedSpec(uuid.New(), uuid.New(), "sid-a", 1_700_000_005)
	spec.roomCreatedSec = 1_700_000_000

	serveWebhook(handler, signedWebhookRequest(t, spec))

	want := []string{`{"started_at":"2023-11-14T22:13:20.000Z"}`}
	if got := sessionEnvelopes(publisher); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sessão publicada = %v, want %v", got, want)
	}
}

func TestWebhook_LeftDoUltimoAtivoPublicaSessaoNula(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID, userID := uuid.New(), uuid.New()
	mustJoin(t, state, roomID, participantOf(userID, "sid-a"), at(0))
	left := webhookEventSpec{event: "participant_left", eventID: "EV_left", roomID: roomID, userID: userID, sid: "sid-a", createdAt: 1_700_000_100, roomCreatedMs: sessionCreatedMs}

	serveWebhook(handler, signedWebhookRequest(t, left))

	if got := sessionEnvelopes(publisher); len(got) != 1 || got[0] != `{"started_at":null}` {
		t.Fatalf("sessão publicada = %v, want [null]", got)
	}
}

func TestWebhook_LeftComAtivoRestantePublicaSessaoVigente(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID := uuid.New()
	leaving, staying := uuid.New(), uuid.New()
	spec := joinedSpec(roomID, leaving, "sid-a", 1_700_000_005)
	spec.roomCreatedMs = sessionCreatedMs
	serveWebhook(handler, signedWebhookRequest(t, spec))
	other := joinedSpec(roomID, staying, "sid-b", 1_700_000_006)
	other.roomCreatedMs = sessionCreatedMs
	serveWebhook(handler, signedWebhookRequest(t, other))
	publisher.envelopes = nil
	left := webhookEventSpec{event: "participant_left", eventID: "EV_left", roomID: roomID, userID: leaving, sid: "sid-a", createdAt: 1_700_000_100, roomCreatedMs: sessionCreatedMs}

	serveWebhook(handler, signedWebhookRequest(t, left))

	if got := sessionEnvelopes(publisher); len(got) != 1 || got[0] != `{"started_at":"2023-11-14T22:13:20.123Z"}` {
		t.Fatalf("sessão publicada = %v, want a vigente", got)
	}
}

func TestWebhook_RoomFinishedPublicaSessaoNula(t *testing.T) {
	state := newTestState()
	handler, publisher := newWebhookFixture(state)
	roomID, _ := joinedRoomWithAwayUser(t, state, 1_700_000_000_000)
	finished := webhookEventSpec{event: "room_finished", eventID: "EV_fin", roomID: roomID, userID: uuid.New(), sid: "x", createdAt: 1_700_000_200}

	serveWebhook(handler, signedWebhookRequest(t, finished))

	if got := sessionEnvelopes(publisher); len(got) != 1 || got[0] != `{"started_at":null}` {
		t.Fatalf("sessão publicada = %v, want [null]", got)
	}
}
