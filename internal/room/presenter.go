package room

import (
	"time"

	"github.com/google/uuid"
)

type RoomResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	CreatedBy *uuid.UUID `json:"created_by"`
	HasVoice  bool       `json:"has_voice"`
	StaffOnly bool       `json:"staff_only"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
}

func RoomPresenter(room Room) RoomResponse {
	var createdBy *uuid.UUID
	if room.CreatedBy.Valid {
		id := uuid.UUID(room.CreatedBy.Bytes)
		createdBy = &id
	}

	return RoomResponse{
		ID:        room.ID,
		Name:      room.Name,
		CreatedBy: createdBy,
		HasVoice:  room.HasVoice,
		StaffOnly: room.StaffOnly,
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
