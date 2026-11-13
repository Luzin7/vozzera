package mail

import (
	"net/http"
	"testing"
	"time"
)

func TestHealthWithoutSendsIsUnknown(t *testing.T) {
	if got := NewHealth().Status(); got != MailStatusUnknown {
		t.Errorf("Status() = %q, want %q", got, MailStatusUnknown)
	}
}

func TestHealthStatus(t *testing.T) {
	health := NewHealth()

	health.RecordSuccess()
	if got := health.Status(); got != MailStatusOK {
		t.Errorf("Status() = %q, want %q", got, MailStatusOK)
	}

	health.RecordFailure(http.StatusInternalServerError)
	if got := health.Status(); got != MailStatusFailing {
		t.Errorf("Status() = %q, want %q", got, MailStatusFailing)
	}

	expired := &Health{
		lastStatus: http.StatusInternalServerError,
		checkedAt:  time.Now().Add(-2 * healthMaxAge),
	}
	if got := expired.Status(); got != MailStatusUnknown {
		t.Errorf("Status() = %q, want %q depois do max age", got, MailStatusUnknown)
	}
}
