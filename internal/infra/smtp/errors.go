package smtp

import "errors"

var (
	ErrInvalidConfig = errors.New("configuração SMTP incompleta: SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS e SMTP_FROM são obrigatórios")
	ErrNotConfigured = errors.New("SMTP não configurado")
)
