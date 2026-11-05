package voice

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"google.golang.org/protobuf/proto"
)

func TestLiveKitHTTPBaseURL(t *testing.T) {
	cases := map[string]string{
		"wss://projeto.livekit.cloud":   "https://projeto.livekit.cloud",
		"ws://localhost:7880":           "http://localhost:7880",
		"https://projeto.livekit.cloud": "https://projeto.livekit.cloud",
	}
	for input, want := range cases {
		if got := liveKitHTTPBaseURL(input); got != want {
			t.Errorf("liveKitHTTPBaseURL(%q) = %q, want %q", input, got, want)
		}
	}
}

type recordedRequest struct {
	path  string
	grant *auth.VideoGrant
}

func newFakeLiveKitServer(t *testing.T, rooms []*livekit.Room, participants []*livekit.ParticipantInfo) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	var mu sync.Mutex
	var requests []recordedRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		verifier, err := auth.ParseAPIToken(token)
		if err != nil {
			http.Error(w, "sem token", http.StatusUnauthorized)
			return
		}
		_, claims, err := verifier.Verify(webhookSecret)
		if err != nil {
			http.Error(w, "token inválido", http.StatusUnauthorized)
			return
		}

		mu.Lock()
		requests = append(requests, recordedRequest{path: r.URL.Path, grant: claims.Video})
		mu.Unlock()

		var message proto.Message = &livekit.ListRoomsResponse{Rooms: rooms}
		if strings.HasSuffix(r.URL.Path, "/ListParticipants") {
			message = &livekit.ListParticipantsResponse{Participants: participants}
		}
		body, _ := proto.Marshal(message)
		w.Header().Set("Content-Type", "application/protobuf")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	return server, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), requests...)
	}
}

func TestLiveKitRoomLister_ListaSalasComTokenRoomList(t *testing.T) {
	roomID := uuid.NewString()
	server, requests := newFakeLiveKitServer(t, []*livekit.Room{{Name: roomID, CreationTimeMs: 1_700_000_000_123}}, nil)
	lister := newLiveKitRoomListerWithClient(server.URL, webhookKey, webhookSecret, server.Client())

	rooms, err := lister.ListRooms(context.Background())

	if err != nil {
		t.Fatalf("ListRooms erro: %v", err)
	}
	if len(rooms) != 1 || rooms[0].Name != roomID || rooms[0].CreatedAt.UnixMilli() != 1_700_000_000_123 {
		t.Errorf("rooms = %v, want [%s] criada em 1700000000123", rooms, roomID)
	}
	got := requests()
	if len(got) != 1 || !strings.HasSuffix(got[0].path, "/ListRooms") || !got[0].grant.RoomList {
		t.Errorf("requests = %+v, want ListRooms com grant roomList", got)
	}
}

func TestLiveKitRoomLister_ListaParticipantesComTokenRoomAdminDaSala(t *testing.T) {
	roomID, userID := uuid.NewString(), uuid.NewString()
	server, requests := newFakeLiveKitServer(t, nil, []*livekit.ParticipantInfo{{Sid: "PA_1", Identity: userID, Name: "alice"}})
	lister := newLiveKitRoomListerWithClient(server.URL, webhookKey, webhookSecret, server.Client())

	participants, err := lister.ListParticipants(context.Background(), roomID)

	if err != nil {
		t.Fatalf("ListParticipants erro: %v", err)
	}
	if len(participants) != 1 || participants[0] != (Participant{SID: "PA_1", UserID: userID, Username: "alice"}) {
		t.Errorf("participants = %+v", participants)
	}
	got := requests()
	if len(got) != 1 || !got[0].grant.RoomAdmin || got[0].grant.Room != roomID {
		t.Errorf("requests = %+v, want grant roomAdmin da sala", got)
	}
}

func TestLiveKitRoomLister_ErroHTTPViraErro(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()
	lister := newLiveKitRoomListerWithClient(server.URL, webhookKey, webhookSecret, server.Client())

	if _, err := lister.ListRooms(context.Background()); err == nil {
		t.Fatal("ListRooms deveria devolver erro")
	}
}

func TestRoomCreationTime(t *testing.T) {
	cases := map[string]struct {
		room *livekit.Room
		want time.Time
	}{
		"milissegundos preferidos": {&livekit.Room{CreationTime: 1_700_000_000, CreationTimeMs: 1_700_000_000_123}, time.UnixMilli(1_700_000_000_123).UTC()},
		"segundos como fallback":   {&livekit.Room{CreationTime: 1_700_000_000}, time.Unix(1_700_000_000, 0).UTC()},
		"zero quando ausente":      {&livekit.Room{Name: "x"}, time.Time{}},
		"zero para sala nula":      {nil, time.Time{}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := roomCreationTime(tc.room); !got.Equal(tc.want) {
				t.Errorf("roomCreationTime = %v, want %v", got, tc.want)
			}
		})
	}
}
