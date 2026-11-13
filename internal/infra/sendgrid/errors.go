package sendgrid

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
)

var (
	ErrInvalidConfig = errors.New("configuração do SendGrid incompleta: SENDGRID_API_KEY e MAIL_FROM_ADDRESS são obrigatórios")
	ErrNotConfigured = errors.New("SendGrid não configurado")

	providerSecretPattern = regexp.MustCompile(`(SG\.[A-Za-z0-9_\-\.]{8,}|Bearer\s+[A-Za-z0-9_\-\.]{8,})`)
)

const (
	maxProviderErrorBodyBytes = 1024
	redactedValue             = "[REDACTED]"
)

type ProviderError struct {
	status int
	body   string
}

func NewProviderError(status int, body string) *ProviderError {
	return &ProviderError{
		status: status,
		body:   truncateBody(maskProviderSecrets(body)),
	}
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("sendgrid respondeu %d: %s", e.status, e.body)
}

func (e *ProviderError) HTTPStatus() int {
	return e.status
}

func (e *ProviderError) ResponseBody() string {
	return e.body
}

func maskProviderSecrets(body string) string {
	return providerSecretPattern.ReplaceAllString(body, redactedValue)
}

func truncateBody(body string) string {
	if len(body) <= maxProviderErrorBodyBytes {
		return body
	}
	return body[:maxProviderErrorBodyBytes] + "…"
}

func isProviderFailure(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return true
	}
	return status >= http.StatusInternalServerError
}
