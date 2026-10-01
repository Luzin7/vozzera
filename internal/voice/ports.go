package voice

import (
	"context"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

type VoiceRoomAccess interface {
	AuthorizeVoice(ctx context.Context, roomID uuid.UUID, claims httpx.UserClaims) (roomName string, err error)
}
