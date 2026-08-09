package voice

import (
	"time"

	"github.com/livekit/protocol/auth"
)

type TokenIssuer struct {
	apiKey    string
	apiSecret string
}

func NewTokenIssuer(apiKey, apiSecret string) *TokenIssuer {
	return &TokenIssuer{
		apiKey:    apiKey,
		apiSecret: apiSecret,
	}
}

func (ti *TokenIssuer) IssueToken(identity, room, displayName string) (string, error) {
	at := auth.NewAccessToken(ti.apiKey, ti.apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     room,
	}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(displayName).
		SetValidFor(6 * time.Hour)

	return at.ToJWT()
}
