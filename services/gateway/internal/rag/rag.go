// Package rag now holds only episodic memory: the summary written when a
// session goes quiet. answering a message moved to package agent, which
// treats retrieval as one tool among the stages of a turn
package rag

import (
	"context"
	"fmt"
	"strings"

	"astralis/gateway/internal/embed"
	"astralis/gateway/internal/llm"
	"astralis/gateway/internal/store"
)

type Engine struct {
	store    *store.Store
	embedder embed.Embedder
	llm      llm.Client
	topK     int
}

func NewEngine(s *store.Store, e embed.Embedder, l llm.Client, topK int) *Engine {
	return &Engine{store: s, embedder: e, llm: l, topK: topK}
}

// summarise condenses a finished session into the episodic memory line
// called from the worker, never from a request
func (e *Engine) Summarise(ctx context.Context, msgs []store.Message) (string, error) {
	var sb strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&sb, "%s: %s\n", m.Role, m.Content)
	}
	const instruction = `Summarise this conversation in under 120 words for your own future reference.
  Capture: the person's work situation, recurring stressors, what they have already tried, and what they responded well to.
  Write neutral factual notes. No diagnosis, no clinical language, no advice.`

	return e.llm.Complete(ctx, instruction,
		[]llm.Message{{Role: "user", Content: sb.String()}}, 300)
}