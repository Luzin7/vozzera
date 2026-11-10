package voice

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

func TestAwayEvents_FalhaAoPublicarPresencaRegistraErro(t *testing.T) {
	publisher := &lockedPublisher{err: errors.New("broker indisponível")}
	buf := captureLog(t)

	newAwayEvents(newTestState(), publisher).publishPresence(context.Background(), uuid.New(), realtime.PresenceJoined, nil)

	if !strings.Contains(buf.String(), "voice.publish level=error type="+realtime.PresenceJoined) {
		t.Errorf("log = %q, want o tipo da presença", buf.String())
	}
	if !strings.Contains(buf.String(), "broker indisponível") {
		t.Errorf("log = %q, want o erro do publisher", buf.String())
	}
}
