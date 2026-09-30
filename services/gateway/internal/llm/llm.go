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

// request is everything one call can ask for. 0 values mean use the default
// a caller only sets what it cares about
type Request struct {
	// model overrides the client's default model for this call
	// the agent uses it to sedn analysis to a small fast model and replies to a larger one
	Model string
	System string
	Messages []Message
	MaxTokens int
	// when schema is set, it constrains the reply to json that matches it
	Schema map[string]any
	Effort string
}

type Usage struct {
	InputTokens int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type Response struct {
	Text string
	StopReason string
	Usage Usage
}

type Anthropic struct {
	apiKey string
	model  string
	http   *http.Client
	// baseURL is a field rather than a literal so tests can point the client
	// at a local fake server and check the exact request it sends.
	baseURL string
	//wait duration time before each retry
	backoff []time.Duration
}

const anthropicBaseURL = "https://api.anthropic.com"

func NewAnthropic(apiKey, model string) *Anthropic {
	return &Anthropic{
		apiKey:  apiKey,
		model:   model,
		baseURL: anthropicBaseURL,
		http:    &http.Client{Timeout: 90 * time.Second}, //use this instead of http.DefaultClient since it has no timeout and a hung connection blocks a goroutine forever
		backoff: []time.Duration{time.Second, 3 * time.Second},
	}
}

type outputConfig struct {
	Format *outputFormat `json:"format,omitempty"`
	Effort string `json:"effort,omitempty"`
}

type outputFormat struct {
	Type string `json:"type"`
	Schema map[string]any `json:"schema"`
}

type reqBody struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	outputConfig *outputConfig `json:"output_config,omitempty"`
}

type respBody struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (a *Anthropic) Complete(ctx context.Context, system string, msgs []Message, maxTokens int) (string, error) {
	res, err := a.Generate(ctx, Request{System: system, Messages: msgs, MaxTokens: maxTokens})
	return res.Text, err
}

func (a *Anthropic) Generate(ctx context.Context, r Request) (Response, error) {
	body := reqBody{Model: r.Model, MaxTokens: r.MaxTokens, System: r.System, Messages: r.Messages}
	if body.Model == "" {
		body.Model = a.model
	}
	if r.Schema != nil || r.Effort != "" {
		body.OutputConfig = &outputConfig{Effort: r.Effort}
		if r.Schema != nil {
			body.OutputConfig.Format = &outputFormat{Type: "json_schema", Schema: r.Schema}
		}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}

	for attempt := 0; ; attempt++ {
		res, retry, err := a.send(ctx, payload)
		if err == nil || !retry || attempt >= len(a.backoff) {
			return res, err
		}
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-time.After(a.backoff[attempt]):
		}
	}
}

func (a *Anthropic) send(ctx context.Context, payload []byte) (Response, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return Response{}, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	res, err := a.http.Do(req)
	if err != nil {
		// A cancelled context is the caller giving up, not a network blip.
		return Response{}, ctx.Err() == nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return Response{}, retryable(res.StatusCode), fmt.Errorf("anthropic status %d: %s", res.StatusCode, string(raw))
	}

	var out respBody
	if err := json.Unmarshal(raw, &out); err != nil {
		return Response{}, false, fmt.Errorf("decode anthropic response: %w", err)
	}
	if out.Error != nil {
		return Response{}, false, fmt.Errorf("anthropic error: %s", out.Error.Message)
	}

	var sb bytes.Buffer
	for _, blk := range out.Content {
		if blk.Type == "text" {
			sb.WriteString(blk.Text)
		}
	}
	return Response{Text: sb.String(), StopReason: out.StopReason, Usage: out.Usage}, false, nil
}

// 429 is a rate limit, 529 is Anthropic overloaded, 5xx is a server fault.
func retryable(status int) bool {
	return status == http.StatusTooManyRequests || status == 529 || status >= 500
}