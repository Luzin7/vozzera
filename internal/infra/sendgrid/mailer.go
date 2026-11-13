package sendgrid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	sendgridAPIURL = "https://api.sendgrid.com/v3/mail/send"
	clientTimeout  = 15 * time.Second
)

type Config struct {
	APIKey      string
	FromAddress string
	FromName    string
}

type SendGridMailer struct {
	cfg    Config
	client *http.Client
	apiURL string
}

type sendRequest struct {
	Personalizations []personalization `json:"personalizations"`
	From             from              `json:"from"`
	Subject          string            `json:"subject"`
	Content          []content         `json:"content"`
}

type personalization struct {
	To []recipient `json:"to"`
}

type recipient struct {
	Email string `json:"email"`
}

type from struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type content struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

func NewSendGridMailer(cfg Config) (*SendGridMailer, error) {
	if cfg.APIKey == "" || cfg.FromAddress == "" {
		return nil, ErrInvalidConfig
	}

	return &SendGridMailer{
		cfg:    cfg,
		client: &http.Client{Timeout: clientTimeout},
		apiURL: sendgridAPIURL,
	}, nil
}

func (m *SendGridMailer) Send(to, subject, html string) error {
	payload, err := json.Marshal(sendRequest{
		Personalizations: []personalization{{To: []recipient{{Email: to}}}},
		From:             from{Email: m.cfg.FromAddress, Name: m.cfg.FromName},
		Subject:          subject,
		Content:          []content{{Type: "text/html", Value: html}},
	})
	if err != nil {
		return fmt.Errorf("serializar email: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, m.apiURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("montar requisição: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("enviar email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return m.providerError(resp)
	}

	return nil
}

func (m *SendGridMailer) providerError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	return NewProviderError(resp.StatusCode, m.redactSecrets(string(body)))
}

func (m *SendGridMailer) redactSecrets(body string) string {
	if m.cfg.APIKey == "" {
		return body
	}
	return strings.ReplaceAll(body, m.cfg.APIKey, redactedValue)
}
