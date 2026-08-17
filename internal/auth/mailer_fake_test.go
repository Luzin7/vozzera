package auth

type fakeMailer struct {
	send func(to, subject, html string) error
}

func (f *fakeMailer) Send(to, subject, html string) error {
	if f.send == nil {
		return nil
	}
	return f.send(to, subject, html)
}

var _ MailSender = (*fakeMailer)(nil)
