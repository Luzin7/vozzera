package voice

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

type TokenInput struct {
	Claims httpx.UserClaims
	RoomID uuid.UUID
}

type TokenOutput struct {
	Token    string
	URL      string
	RoomName string
}

type TokenService struct {
	access     VoiceRoomAccess
	issuer     *TokenIssuer
	liveKitURL string
}

func NewTokenService(access VoiceRoomAccess, issuer *TokenIssuer, liveKitURL string) *TokenService {
	return &TokenService{access: access, issuer: issuer, liveKitURL: liveKitURL}
}

func (s *TokenService) Execute(ctx context.Context, in TokenInput) (TokenOutput, error) {
	roomName, err := s.access.AuthorizeVoice(ctx, in.RoomID, in.Claims)
	if err != nil {
		return TokenOutput{}, err
	}

	token, err := s.issuer.IssueToken(in.Claims.UserID.String(), in.RoomID.String(), in.Claims.Username)
	if err != nil {
		return TokenOutput{}, ErrIssueToken(err)
	}

	return TokenOutput{Token: token, URL: s.liveKitURL, RoomName: roomName}, nil
}
