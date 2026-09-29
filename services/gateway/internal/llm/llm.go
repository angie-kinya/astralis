// package llm wraps the anthropic messages api behing an interface
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Client interface {
	Complete(ctx context.Context, system string, msgs []Message, maxTokens int) (string, error)
}

type Anthropic struct {
	apiKey string
	model  string
	http   *http.Client
	// baseURL is a field rather than a literal so tests can point the client
	// at a local fake server and check the exact request it sends.
	baseURL string
}

const anthropicBaseURL = "https://api.anthropic.com"

func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{
		apiKey:  apiKey,
		model:   model,
		baseURL: anthropicBaseURL,
		http:    &http.Client{Timeout: 90 * time.Second}, //use this instead of http.DefaultClient since it has no timeout and a hung connection blocks a goroutine forever
	}
}

type reqBody struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

type respBody struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *Anthropic) Complete(ctx context.Context, system string, msgs []Message, maxTokens int) (string, error) {
	payload, err := json.Marshal(reqBody{
		Model: a.model, MaxTokens: maxTokens, System: system, Messages: msgs,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	res, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic request: %w", err)
	}

	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic status %d: %s", res.StatusCode, string(raw))
	}

	var out respBody
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode anthropic response: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("anthropic error: %s", out.Error.Message)
	}

	var sb bytes.Buffer
	for _, blk := range out.Content {
		if blk.Type == "text" {
			sb.WriteString(blk.Text)
		}
	}
	return sb.String(), nil
}
