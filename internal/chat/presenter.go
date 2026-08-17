package chat

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

type MessageResponse struct {
	ID        uuid.UUID `json:"id"`
	Content   string    `json:"content"`
	CreatedAt string    `json:"created_at"`
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
}

func MessagesPresenter(messages []GetMessagesByRoomRow) []MessageResponse {
	result := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		result = append(result, MessageResponse{
			ID:        m.ID,
			Content:   m.Content.String,
			CreatedAt: m.CreatedAt.Time.Format(time.RFC3339),
			UserID:    m.UserID,
			Username:  m.Username,
		})
	}
	return result
}

type UpdateMessageResponse struct {
	ID        uuid.UUID `json:"id"`
	RoomID    uuid.UUID `json:"room_id"`
	Content   string    `json:"content"`
	UpdatedAt string    `json:"updated_at"`
}

func UpdateMessagePresenter(msg UpdateMessageRow) UpdateMessageResponse {
	return UpdateMessageResponse{
		ID:        msg.ID,
		RoomID:    msg.RoomID,
		Content:   msg.Content.String,
		UpdatedAt: msg.UpdatedAt.Time.Format(time.RFC3339),
	}
}

type DeleteMessageResponse struct {
	ID        uuid.UUID `json:"id"`
	RoomID    uuid.UUID `json:"room_id"`
	UserID    uuid.UUID `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt string    `json:"created_at"`
}

func DeleteMessagePresenter(msg DeleteMessageRow) DeleteMessageResponse {
	return DeleteMessageResponse{
		ID:        msg.ID,
		RoomID:    msg.RoomID,
		UserID:    msg.UserID,
		Content:   msg.Content.String,
		CreatedAt: msg.CreatedAt.Time.Format(time.RFC3339),
	}
}
