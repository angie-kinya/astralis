package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dim() int
}

type OpenAI struct {
	apiKey string
	model string
	dim int
	http *http.Client
}

func NewOpenAI(apiKey, model string, dim int) *OpenAI {
	return &OpenAI{apiKey: apiKey, model: model, dim: dim, http: &http.Client{Timeout: 60 * time.Second}}
}

func (o *OpenAI) Dim() int { return o.dim }

func (o *OpenAI) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, errors.New("embed: no input")
	}
	payload, _ := json.Marshal(map[string]any{"model": o.model, "input": texts})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	res, err := o.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings status %d: %s", res.StatusCode, string(raw))
	}

	var out struct {
		Data []struct {
			Index int `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}

	vecs := make([][]float32, len(texts))
	for _, := range out.Data {
		if d.Index >= 0 && d.Index < len(vecs) {
			vecs[d.Index] = d.Embedding
		}
	}
	return vecs, nil
}
