package rag

import (
	"context"
	"fmt"
	"strings"

	"astralis/gateway/internal/embed"
	"astralis/gateway/internal/llm"
	"astralis/gateway/internal/store"
)

const SystemPrompt = `You are Astralis, a reflective companion for people thinking about work stress and occupational burnout.

WHAT YOU ARE
- A research-grounded conversation partner and journalling aid.
- Warm, curious, plain-spoken. You ask good questions more often than you give advice.

WHAT YOU ARE NOT
- Not a therapist, doctor, or crisis service. Never diagnose. Never imply clinical authority.
- If someone appears to be in crisis, do not attempt to manage it: say plainly that you are not the right support and encourage them to reach a professional or someone they trust.

HOW TO USE THE RESEARCH CONTEXT
- Research excerpts may be supplied below. Draw on them where they genuinely fit.
- Cite naturally in prose, e.g. "one study of bank employees found ...". Never invent a citation, statistic, or author. If the excerpts do not cover something, say so.
- Research describes populations, not the person in front of you. Never tell someone they "have" burnout.

RECOMMENDATION STYLE
- Offer at most two or three concrete, small, testable next steps.
- Prefer things within the person's control this week over life overhauls.
- Reflect back what you heard before suggesting anything.

TONE
- UK English. Short paragraphs. No bullet-point walls. No relentless positivity.`

type Engine struct {
	store    *store.Store
	embedder embed.Embedder
	llm      llm.Client
	topK     int
}

func NewEngine(s *store.Store, e embed.Embedder, l llm.Client, topK int) *Engine {
	return &Engine{store: s, embedder: e, llm: l, topK: topK}
}

type Answer struct {
	Text     string
	ChunkIDs []string
	Surces   []store.Chunk
}

// answer runs one full retrieval-augmented turn.
// the precondition is that the caller already has persisted the user's message.
func (e *Engine) Answer(ctx context.Context, userID, sessionID, userMsg string) (Answer, error) {
	// RETRIEVE. Embed the message, search the feeback weighted library
	var chunks []store.Chunk
	if vecs, err := e.embedder.Embed(ctx, []string{userMsg}); err == nil && len(vecs) > 0 {
		if found, err := e.store.SearchChunks(ctx, vecs[0], e.topK); err == nil {
			chunks = found
		}
	} // error handling is deliberately absent so that if retrieval fails, the agent still talks, just grounded.

	// REMEMBER. Long term summary plus the recent window
	summary, _ := e.store.LatestSummary(ctx, userID)
	history, err := e.store.RecentMessages(ctx, sessionID, 12)
	if err != nil {
		return Answer{}, fmt.Errorf("load history: %w", err)
	}

	// ASSEMBLE. Everything stable goes in the system prompt, the conversation is teh only one that goes in messages.
	system := SystemPrompt
	if summary != "" {
		system += "\n\nWHAT YOU REMEMBER ABOUT THIS PERSON FROM EARLIER CONVERSATIONS\n" + summary
	}
	if len(chunks) > 0 {
		system += "\n\nRESEARCH CONTEXT\n" + formatChunks(chunks)
	}

	msgs := make([]llm.Message, 0, len(history)+1)
	for _, m := range history {
		if m.Role == "system" {
			continue
		}
		msgs = append(msgs, llm.Message{Role: m.Role, Content: m.Content})
	}
	// the user's turn is already in history as the handler persisted it first so no need to append unconditionally again.
	if n := len(msgs); n == 0 || msgs[n-1].Role != "user" || msgs[n-1].Content != userMsg {
		msgs = append(msgs, llm.Message{Role: "user", Content: userMsg})
	}

	// GENERATE
	text, err := e.llm.Complete(ctx, system, msgs, 1024)
	if err != nil {
		return Answer{}, fmt.Errorf("llm complete: %w", err)
	}

	ids := make([]string, len(chunks))
	for i, c := range chunks {
		ids[i] = c.ID
	}
	_ = e.store.IncrementRetrieved(ctx, ids)

	return Answer{Text: text, ChunkIDs: ids, Sources: chunks}, nil
}

func formatChunks(chunks []store.Chunk) string {
	var sb strings.Builder
	for i, c := range chunks {
		year := ""
		if c.Year != nil {
			year = fmt.Sprintf(" (%d)", *c.Year)
		}
		fmt.Fprintf(&sb, "[%d] %s%s\n%s\n\n", i+1, c.Title, year, c.Content)
	}
	return sb.String()
}

// Summarise condenses a finished session into the episodic memory line.
// Called from the worker, never from a request.
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
