package mailjet

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
	mailjetAPIURL   = "https://api.mailjet.com/v3.1/send"
	clientTimeout   = 15 * time.Second
	maxResponseSize = 64 << 10
	statusSuccess   = "success"
)

type Config struct {
	APIKey      string
	APISecret   string
	FromAddress string
	FromName    string
}

type Mailer struct {
	cfg    Config
	client *http.Client
	apiURL string
}

type sendRequest struct {
	Messages []message `json:"Messages"`
}

type message struct {
	From     sender      `json:"From"`
	To       []recipient `json:"To"`
	Subject  string      `json:"Subject"`
	HTMLPart string      `json:"HTMLPart"`
}

type sender struct {
	Email string `json:"Email"`
	Name  string `json:"Name,omitempty"`
}

type recipient struct {
	Email string `json:"Email"`
}

type sendResponse struct {
	Messages []messageResult `json:"Messages"`
}

type messageResult struct {
	Status string        `json:"Status"`
	Errors []resultError `json:"Errors"`
}

type resultError struct {
	ErrorCode    string `json:"ErrorCode"`
	ErrorMessage string `json:"ErrorMessage"`
}

func NewMailer(cfg Config) (*Mailer, error) {
	return newMailer(cfg, &http.Client{Timeout: clientTimeout}, mailjetAPIURL)
}

func newMailer(cfg Config, client *http.Client, apiURL string) (*Mailer, error) {
	if cfg.APIKey == "" || cfg.APISecret == "" || cfg.FromAddress == "" {
		return nil, ErrInvalidConfig
	}

	return &Mailer{
		cfg:    cfg,
		client: client,
		apiURL: apiURL,
	}, nil
}

func (m *Mailer) Send(to, subject, html string) error {
	req, err := m.buildRequest(to, subject, html)
	if err != nil {
		return err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("%w: ler resposta: %v", ErrRequestFailed, err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %s: %s", statusError(resp.StatusCode), resp.Status, strings.TrimSpace(string(body)))
	}

	return parseResult(body)
}

func (m *Mailer) buildRequest(to, subject, html string) (*http.Request, error) {
	payload, err := json.Marshal(sendRequest{
		Messages: []message{{
			From:     sender{Email: m.cfg.FromAddress, Name: m.cfg.FromName},
			To:       []recipient{{Email: to}},
			Subject:  subject,
			HTMLPart: html,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: serializar email: %v", ErrRequestFailed, err)
	}

	req, err := http.NewRequest(http.MethodPost, m.apiURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: montar requisição: %v", ErrRequestFailed, err)
	}
	req.SetBasicAuth(m.cfg.APIKey, m.cfg.APISecret)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func parseResult(body []byte) error {
	var result sendResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("%w: %v", ErrUnexpectedResponse, err)
	}
	if len(result.Messages) == 0 {
		return fmt.Errorf("%w: resposta sem Messages", ErrUnexpectedResponse)
	}

	for _, msg := range result.Messages {
		if msg.Status != statusSuccess {
			return fmt.Errorf("%w: %s", ErrMessageRejected, describeErrors(msg.Errors))
		}
	}

	return nil
}

func describeErrors(errs []resultError) string {
	if len(errs) == 0 {
		return "sem detalhe"
	}

	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, fmt.Sprintf("%s %s", e.ErrorCode, e.ErrorMessage))
	}

	return strings.TrimSpace(strings.Join(parts, "; "))
}

func statusError(status int) error {
	switch {
	case status == http.StatusBadRequest:
		return ErrBadRequest
	case status == http.StatusUnauthorized:
		return ErrUnauthorized
	case status == http.StatusForbidden:
		return ErrForbidden
	case status == http.StatusTooManyRequests:
		return ErrRateLimited
	case status >= http.StatusInternalServerError:
		return ErrServerFailure
	default:
		return ErrUnexpectedResponse
	}
}
