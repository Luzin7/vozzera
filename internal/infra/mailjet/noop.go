package mailjet

import "context"

type NoopMailer struct{}

func NewNoopMailer() *NoopMailer {
	return &NoopMailer{}
}

func (m *NoopMailer) Send(ctx context.Context, to, subject, html string) error {
	return ErrNotConfigured
}
