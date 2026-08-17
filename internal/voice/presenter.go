package voice

import (
	"time"

	"github.com/google/uuid"
)

type RoomResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}

func RoomPresenter(room Room) RoomResponse {
	return RoomResponse{
		ID:        room.ID,
		Name:      room.Name,
		Type:      room.Type,
		CreatedAt: room.CreatedAt.Time.Format(time.RFC3339),
		UpdatedAt: room.UpdatedAt.Time.Format(time.RFC3339),
	}
}

func RoomsPresenter(rooms []Room) []RoomResponse {
	result := make([]RoomResponse, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, RoomPresenter(room))
	}
	return result
}

type TokenResponse struct {
	Token    string `json:"token"`
	URL      string `json:"url"`
	RoomName string `json:"room_name"`
}

func TokenPresenter(out TokenOutput) TokenResponse {
	return TokenResponse{
		Token:    out.Token,
		URL:      out.URL,
		RoomName: out.RoomName,
	}
}
