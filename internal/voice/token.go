package voice

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type TokenInput struct {
	UserID   uuid.UUID
	Username string
	RoomID   uuid.UUID
}

type TokenOutput struct {
	Token    string
	URL      string
	RoomName string
}

type TokenService struct {
	repo       Repository
	issuer     *TokenIssuer
	liveKitURL string
}

func NewTokenService(repo Repository, issuer *TokenIssuer, liveKitURL string) *TokenService {
	return &TokenService{repo: repo, issuer: issuer, liveKitURL: liveKitURL}
}

func (s *TokenService) Execute(ctx context.Context, in TokenInput) (TokenOutput, error) {
	room, err := s.repo.GetRoomByID(ctx, in.RoomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenOutput{}, ErrRoomNotFound
	}
	if err != nil {
		return TokenOutput{}, ErrGetRoom(err)
	}

	if room.Type != "voice" {
		return TokenOutput{}, ErrNotVoiceRoom
	}

	token, err := s.issuer.IssueToken(in.UserID.String(), room.ID.String(), in.Username)
	if err != nil {
		return TokenOutput{}, ErrIssueToken(err)
	}

	return TokenOutput{Token: token, URL: s.liveKitURL, RoomName: room.Name}, nil
}
