package mail

type MailSender interface {
	Send(to, subject, html string) error
}

type ProviderError interface {
	error
	HTTPStatus() int
	ResponseBody() string
}
