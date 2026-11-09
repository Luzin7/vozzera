package chat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	maxEmojiRunes     = 10
	maxDistinctEmojis = 20
)

type ReactionSummary struct {
	Emoji   string      `json:"emoji"`
	Count   int32       `json:"count"`
	UserIDs []uuid.UUID `json:"user_ids"`
}

type ReactionInput struct {
	RoomID    uuid.UUID
	MessageID uuid.UUID
	UserID    uuid.UUID
	Username  string
	Emoji     string
}

type ReactionOutput struct {
	Reaction ReactionSummary
}

type reactionEvent struct {
	Type  string
	Input ReactionInput
	Count int32
}

type reactionService struct {
	repo      Repository
	publisher realtime.Publisher
}

func (s reactionService) summaryAfterChange(ctx context.Context, messageID uuid.UUID, emoji string) (ReactionSummary, error) {
	rows, err := s.repo.GetReactionsByMessageIDs(ctx, []uuid.UUID{messageID})
	if err != nil {
		return ReactionSummary{}, err
	}

	for _, row := range rows {
		if row.Emoji != emoji {
			continue
		}
		return ReactionSummary{Emoji: row.Emoji, Count: row.Count, UserIDs: row.UserIds}, nil
	}

	return ReactionSummary{Emoji: emoji, UserIDs: []uuid.UUID{}}, nil
}

func (s reactionService) publish(ctx context.Context, event reactionEvent) error {
	payload, err := json.Marshal(ReactionPayload{
		MessageID: event.Input.MessageID,
		Emoji:     event.Input.Emoji,
		UserID:    event.Input.UserID,
		Username:  event.Input.Username,
		Count:     event.Count,
	})
	if err != nil {
		return err
	}

	topic := realtime.RoomTopic(event.Input.RoomID)
	return s.publisher.Publish(ctx, topic, realtime.Envelope{
		V:     1,
		Type:  event.Type,
		Topic: topic,
		TS:    time.Now(),
		Data:  payload,
	})
}

func (s reactionService) ensureReactable(ctx context.Context, in ReactionInput) (GetReactionStateRow, error) {
	return s.repo.GetReactionState(ctx, GetReactionStateParams{
		ReactionEmoji: in.Emoji,
		MessageID:     in.MessageID,
		RoomID:        in.RoomID,
	})
}

type AddReactionService struct {
	reactionService
}

func NewAddReactionService(repo Repository, publisher realtime.Publisher) *AddReactionService {
	return &AddReactionService{reactionService{repo: repo, publisher: publisher}}
}

func (s *AddReactionService) Execute(ctx context.Context, in ReactionInput) (ReactionOutput, error) {
	emoji, err := normalizeEmoji(in.Emoji)
	if err != nil {
		return ReactionOutput{}, err
	}
	in.Emoji = emoji

	state, err := s.ensureReactable(ctx, in)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReactionOutput{}, ErrMessageNotFound
	}
	if err != nil {
		return ReactionOutput{}, ErrAddReaction(err)
	}
	if !state.EmojiExists && state.DistinctEmojiCount >= maxDistinctEmojis {
		return ReactionOutput{}, ErrTooManyReactions
	}

	inserted, err := s.repo.AddReaction(ctx, AddReactionParams{
		MessageID: in.MessageID,
		UserID:    in.UserID,
		Emoji:     emoji,
	})
	if err != nil {
		return ReactionOutput{}, ErrAddReaction(err)
	}

	reaction, err := s.summaryAfterChange(ctx, in.MessageID, emoji)
	if err != nil {
		return ReactionOutput{}, ErrAddReaction(err)
	}

	if inserted == 0 {
		return ReactionOutput{Reaction: reaction}, nil
	}

	if err := s.publish(ctx, reactionEvent{Type: EventReactionAdded, Input: in, Count: reaction.Count}); err != nil {
		return ReactionOutput{}, ErrAddReaction(err)
	}

	return ReactionOutput{Reaction: reaction}, nil
}

type RemoveReactionService struct {
	reactionService
}

func NewRemoveReactionService(repo Repository, publisher realtime.Publisher) *RemoveReactionService {
	return &RemoveReactionService{reactionService{repo: repo, publisher: publisher}}
}

func (s *RemoveReactionService) Execute(ctx context.Context, in ReactionInput) (ReactionOutput, error) {
	emoji, err := normalizeEmoji(in.Emoji)
	if err != nil {
		return ReactionOutput{}, err
	}
	in.Emoji = emoji

	_, err = s.ensureReactable(ctx, in)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReactionOutput{}, ErrMessageNotFound
	}
	if err != nil {
		return ReactionOutput{}, ErrRemoveReaction(err)
	}

	removed, err := s.repo.RemoveReaction(ctx, RemoveReactionParams{
		MessageID: in.MessageID,
		UserID:    in.UserID,
		Emoji:     emoji,
	})
	if err != nil {
		return ReactionOutput{}, ErrRemoveReaction(err)
	}

	reaction, err := s.summaryAfterChange(ctx, in.MessageID, emoji)
	if err != nil {
		return ReactionOutput{}, ErrRemoveReaction(err)
	}

	if removed == 0 {
		return ReactionOutput{Reaction: reaction}, nil
	}

	if err := s.publish(ctx, reactionEvent{Type: EventReactionRemoved, Input: in, Count: reaction.Count}); err != nil {
		return ReactionOutput{}, ErrRemoveReaction(err)
	}

	return ReactionOutput{Reaction: reaction}, nil
}

func normalizeEmoji(raw string) (string, error) {
	emoji := strings.TrimSpace(raw)
	if emoji == "" {
		return "", ErrEmptyEmoji
	}
	if utf8.RuneCountInString(emoji) > maxEmojiRunes {
		return "", ErrEmojiTooLong
	}
	return emoji, nil
}
