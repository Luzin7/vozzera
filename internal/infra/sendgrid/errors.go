package sendgrid

import "errors"

var (
	ErrInvalidConfig = errors.New("configuração do SendGrid incompleta: SENDGRID_API_KEY e MAIL_FROM_ADDRESS são obrigatórios")
	ErrNotConfigured = errors.New("SendGrid não configurado")
)
