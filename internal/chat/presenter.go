package chat

import (
	"time"

	"github.com/google/uuid"
)

type MessageResponse struct {
	ID        uuid.UUID          `json:"id"`
	Content   string             `json:"content"`
	CreatedAt string             `json:"created_at"`
	UserID    uuid.UUID          `json:"user_id"`
	Username  string             `json:"username"`
	Reactions []ReactionResponse `json:"reactions"`
}

type ReactionResponse struct {
	Emoji   string      `json:"emoji"`
	Count   int32       `json:"count"`
	UserIDs []uuid.UUID `json:"user_ids"`
}

func ReactionPresenter(reaction ReactionSummary) ReactionResponse {
	userIDs := reaction.UserIDs
	if userIDs == nil {
		userIDs = []uuid.UUID{}
	}

	return ReactionResponse{
		Emoji:   reaction.Emoji,
		Count:   reaction.Count,
		UserIDs: userIDs,
	}
}

func ReactionsPresenter(reactions []ReactionSummary) []ReactionResponse {
	result := make([]ReactionResponse, 0, len(reactions))
	for _, reaction := range reactions {
		result = append(result, ReactionPresenter(reaction))
	}
	return result
}

func MessagesPresenter(messages []GetMessagesByRoomRow, reactions map[uuid.UUID][]ReactionSummary) []MessageResponse {
	result := make([]MessageResponse, 0, len(messages))
	for _, m := range messages {
		result = append(result, MessageResponse{
			ID:        m.ID,
			Content:   m.Content.String,
			CreatedAt: m.CreatedAt.Time.Format(time.RFC3339),
			UserID:    m.UserID,
			Username:  m.Username,
			Reactions: ReactionsPresenter(reactions[m.ID]),
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
