package mailjet

import "errors"

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
)
