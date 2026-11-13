package auth

import "context"

type fakeMailer struct {
	send func(ctx context.Context, to, subject, html string) error
}

func (f *fakeMailer) Send(ctx context.Context, to, subject, html string) error {
	if f.send == nil {
		return nil
	}
	return f.send(ctx, to, subject, html)
}

var _ MailSender = (*fakeMailer)(nil)
