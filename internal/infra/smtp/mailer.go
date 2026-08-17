package smtp

import (
	"fmt"
	netsmtp "net/smtp"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type SmtpMailer struct {
	cfg Config
}

func NewSmtpMailer(cfg Config) *SmtpMailer {
	return &SmtpMailer{cfg: cfg}
}

func (m *SmtpMailer) Send(to, subject, html string) error {
	auth := netsmtp.PlainAuth("", m.cfg.User, m.cfg.Password, m.cfg.Host)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		m.cfg.From, to, subject, html,
	)
	return netsmtp.SendMail(
		fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port),
		auth,
		m.cfg.From,
		[]string{to},
		[]byte(msg),
	)
}
