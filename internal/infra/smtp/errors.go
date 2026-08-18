package smtp

import "errors"

var (
	ErrInvalidConfig = errors.New("configuração SMTP incompleta: SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS e MAIL_FROM_ADDRESS são obrigatórios")
	ErrNotConfigured = errors.New("SMTP não configurado")
)
