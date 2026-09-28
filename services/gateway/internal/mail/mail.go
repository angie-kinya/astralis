// Package mail sends the two kinds of email Astralis ever sends: a sign-in
// code, and — only to people who opted in — a note that a new version exists.
//
// Why an HTTP API rather than SMTP: DigitalOcean blocks outbound SMTP ports
// (25, 465, 587) on droplets. An HTTPS provider sidesteps that entirely, and
// it is also what gives you delivery logs and a sender reputation you did not
// have to build yourself.
//
// Same adapter shape as llm and embed: one interface, swappable at the
// composition root.
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

type Sender interface {
	Send(ctx context.Context, m Message) error
}

// ------------------------------------------------------------------ Resend

type Resend struct {
	apiKey string
	from   string
	http   *http.Client
}

func NewResend(apiKey, from string) *Resend {
	return &Resend{apiKey: apiKey, from: from, http: &http.Client{Timeout: 15 * time.Second}}
}

func (r *Resend) Send(ctx context.Context, m Message) error {
	body, _ := json.Marshal(map[string]any{
		"from": r.from, "to": []string{m.To},
		"subject": m.Subject, "text": m.Text, "html": m.HTML,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("resend status %d: %s", res.StatusCode, raw)
	}
	return nil
}

// ------------------------------------------------------------------ Log

// Log prints messages instead of sending them. Development only — config
// refuses to start the API in production without a real provider, because a
// sign-in code in a production log is a sign-in code anyone with log access
// can use.
type Log struct{}

func (Log) Send(_ context.Context, m Message) error {
	slog.Warn("DEV MAIL — not sent, printed instead", "subject", m.Subject, "body", m.Text)
	return nil
}

// ------------------------------------------------------------------ templates

// SignInCode builds the code email. Deliberately plain: no tracking pixel, no
// images, no links to click. A code you type is harder to phish than a link.
func SignInCode(to, code string) Message {
	text := fmt.Sprintf(`Your Astralis sign-in code is:

    %s

It expires in 10 minutes and works once.

If you did not ask for this, ignore it — nobody can sign in without the code,
and nothing has been created.

Astralis never shows your email address to anyone else.`, code)

	html := fmt.Sprintf(`<div style="font-family:ui-monospace,Menlo,monospace;max-width:28rem;color:#16131F">
<p style="margin:0 0 16px">Your Astralis sign-in code is:</p>
<p style="font-size:28px;letter-spacing:.3em;font-weight:600;margin:0 0 16px;color:#6D3FE0">%s</p>
<p style="margin:0 0 8px;color:#575269">It expires in 10 minutes and works once.</p>
<p style="margin:0 0 8px;color:#575269">If you did not ask for this, ignore it — nobody can sign in without the code.</p>
<p style="margin:16px 0 0;font-size:12px;color:#8A849E">Astralis never shows your email address to anyone else.</p>
</div>`, code)

	return Message{To: to, Subject: "Your Astralis code: " + code, Text: text, HTML: html}
}
