package sendgrid

type NoopMailer struct{}

func NewNoopMailer() *NoopMailer {
	return &NoopMailer{}
}

func (m *NoopMailer) Send(to, subject, html string) error {
	return ErrNotConfigured
}
