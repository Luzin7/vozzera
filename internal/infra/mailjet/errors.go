package mailjet

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	ErrInvalidConfig      = errors.New("configuração do Mailjet incompleta: MAILJET_API_KEY, MAILJET_API_SECRET e MAIL_FROM_ADDRESS são obrigatórios")
	ErrNotConfigured      = errors.New("Mailjet não configurado")
	ErrRequestFailed      = errors.New("falha ao enviar email via Mailjet")
	ErrBadRequest         = errors.New("requisição rejeitada pelo Mailjet")
	ErrUnauthorized       = errors.New("credenciais do Mailjet inválidas")
	ErrForbidden          = errors.New("envio não autorizado pelo Mailjet")
	ErrRateLimited        = errors.New("limite de requisições do Mailjet excedido")
	ErrServerFailure      = errors.New("Mailjet indisponível")
	ErrMessageRejected    = errors.New("mensagem rejeitada pelo Mailjet")
	ErrUnexpectedResponse = errors.New("resposta inesperada do Mailjet")

	providerSecretPattern = regexp.MustCompile(`(?i)(Bearer|Basic)\s+[A-Za-z0-9_\-\.=+/]{8,}`)
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
	return fmt.Sprintf("mailjet respondeu %d: %s", e.status, e.body)
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
