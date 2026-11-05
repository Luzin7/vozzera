package voice

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/twitchtv/twirp"
)

const (
	liveKitCallTimeout = 2 * time.Second
	liveKitTokenTTL    = time.Minute
)

type RoomLister interface {
	ListRooms(ctx context.Context) ([]LiveKitRoom, error)
	ListParticipants(ctx context.Context, roomName string) ([]Participant, error)
}

type LiveKitRoom struct {
	Name      string
	CreatedAt time.Time
}

type LiveKitRoomLister struct {
	client    livekit.RoomService
	apiKey    string
	apiSecret string
}

var _ RoomLister = (*LiveKitRoomLister)(nil)

func NewLiveKitRoomLister(liveKitURL, apiKey, apiSecret string) *LiveKitRoomLister {
	return newLiveKitRoomListerWithClient(liveKitHTTPBaseURL(liveKitURL), apiKey, apiSecret, &http.Client{Timeout: liveKitCallTimeout})
}

func newLiveKitRoomListerWithClient(baseURL, apiKey, apiSecret string, httpClient *http.Client) *LiveKitRoomLister {
	return &LiveKitRoomLister{
		client:    livekit.NewRoomServiceProtobufClient(baseURL, httpClient),
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

func liveKitHTTPBaseURL(liveKitURL string) string {
	if rest, ok := strings.CutPrefix(liveKitURL, "wss://"); ok {
		return "https://" + rest
	}
	if rest, ok := strings.CutPrefix(liveKitURL, "ws://"); ok {
		return "http://" + rest
	}
	return liveKitURL
}

func (l *LiveKitRoomLister) ListRooms(ctx context.Context) ([]LiveKitRoom, error) {
	authCtx, err := l.authorize(ctx, &auth.VideoGrant{RoomList: true})
	if err != nil {
		return nil, err
	}

	response, err := l.client.ListRooms(authCtx, &livekit.ListRoomsRequest{})
	if err != nil {
		return nil, err
	}

	rooms := make([]LiveKitRoom, 0, len(response.GetRooms()))
	for _, room := range response.GetRooms() {
		rooms = append(rooms, LiveKitRoom{Name: room.GetName(), CreatedAt: roomCreationTime(room)})
	}
	return rooms, nil
}

func (l *LiveKitRoomLister) ListParticipants(ctx context.Context, roomName string) ([]Participant, error) {
	authCtx, err := l.authorize(ctx, &auth.VideoGrant{RoomAdmin: true, Room: roomName})
	if err != nil {
		return nil, err
	}

	response, err := l.client.ListParticipants(authCtx, &livekit.ListParticipantsRequest{Room: roomName})
	if err != nil {
		return nil, err
	}

	participants := make([]Participant, 0, len(response.GetParticipants()))
	for _, info := range response.GetParticipants() {
		participants = append(participants, participantFromEvent(info))
	}
	return participants, nil
}

func (l *LiveKitRoomLister) authorize(ctx context.Context, grant *auth.VideoGrant) (context.Context, error) {
	token, err := auth.NewAccessToken(l.apiKey, l.apiSecret).SetVideoGrant(grant).SetValidFor(liveKitTokenTTL).ToJWT()
	if err != nil {
		return nil, err
	}

	header := make(http.Header)
	header.Set("Authorization", "Bearer "+token)
	return twirp.WithHTTPRequestHeaders(ctx, header)
}
