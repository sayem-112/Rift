// Package mail sends transactional email (sign-in links).
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// LogMailer writes emails to the log instead of sending them. Used in
// development, so sign-in links appear in the server output.
type LogMailer struct{ Log *slog.Logger }

func (m LogMailer) Send(_ context.Context, msg Message) error {
	m.Log.Info("email (not sent: no RESEND_API_KEY)", "to", msg.To, "subject", msg.Subject, "text", msg.Text)
	return nil
}

// ResendMailer sends through the Resend HTTP API.
type ResendMailer struct {
	APIKey   string
	From     string
	Endpoint string // defaults to Resend's production API; overridden in tests
	Client   *http.Client
}

func NewResend(apiKey, from string) *ResendMailer {
	return &ResendMailer{
		APIKey:   apiKey,
		From:     from,
		Endpoint: "https://api.resend.com/emails",
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *ResendMailer) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(map[string]any{
		"from":    m.From,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"text":    msg.Text,
		"html":    msg.HTML,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.Client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("resend: status %d: %s", resp.StatusCode, detail)
	}
	return nil
}
