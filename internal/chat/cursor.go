package chat

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
)

const cursorSeparator = "|"

type messageCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (c messageCursor) encode() string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + cursorSeparator + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeMessageCursor(encoded string) (messageCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return messageCursor{}, ErrInvalidCursor
	}

	parts := strings.Split(string(raw), cursorSeparator)
	if len(parts) != 2 {
		return messageCursor{}, ErrInvalidCursor
	}

	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return messageCursor{}, ErrInvalidCursor
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return messageCursor{}, ErrInvalidCursor
	}

	return messageCursor{CreatedAt: createdAt, ID: id}, nil
}
