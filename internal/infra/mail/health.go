package mail

import (
	"net/http"
	"sync"
	"time"
)

const (
	healthMaxAge = time.Hour

	MailStatusOK      = "ok"
	MailStatusFailing = "failing"
	MailStatusUnknown = "unknown"
)

type Health struct {
	mu         sync.Mutex
	lastStatus int
	checkedAt  time.Time
}

func NewHealth() *Health {
	return &Health{}
}

func (h *Health) RecordSuccess() {
	h.record(http.StatusAccepted)
}

func (h *Health) RecordFailure(status int) {
	if !isProviderFailure(status) {
		status = http.StatusServiceUnavailable
	}
	h.record(status)
}

func (h *Health) Status() string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.checkedAt.IsZero() || time.Since(h.checkedAt) >= healthMaxAge {
		return MailStatusUnknown
	}
	if isProviderFailure(h.lastStatus) {
		return MailStatusFailing
	}
	return MailStatusOK
}

func (h *Health) record(status int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastStatus = status
	h.checkedAt = time.Now()
}

func isProviderFailure(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return true
	}
	return status >= http.StatusInternalServerError
}
