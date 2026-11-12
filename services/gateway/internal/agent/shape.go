package agent

import (
	"strconv"
	"strings"
)

// Shape is the form of one reply, decided in Go before the model writes.
// the model gets one instruction line per field and MaxTokens follows the
// budget, so a long reply is impossible rather than discouraged
type Shape struct {
	Words     int  // target length
	MayAsk    bool // at most one question
	MayAdvise bool
	Cite      bool // inline [n] markers allowed
}

// word budgets per intent: {floor, ceiling}
var budgets = map[string][2]int{
	"greeting": {8, 30}, "off_topic": {15, 40}, "vent": {20, 110},
	"report_progress": {15, 90}, "reflect": {35, 130},
	"seek_advice": {60, 170}, "ask_evidence": {90, 220},
}

// shapeFor mirrors the person: a one-line message gets a short reply.
// lastReplyAsked enforces the question rule in code, not in the prompt
func shapeFor(intent string, userWords int, lastReplyAsked, retrieve bool) Shape {
	b, ok := budgets[intent]
	if !ok {
		b = budgets["reflect"]
	}
	words := max(b[0], min(b[1], int(float64(userWords)*1.4)+b[0]/2))
	return Shape{
		Words:     words,
		MayAsk:    !lastReplyAsked && intent != "ask_evidence" && intent != "off_topic",
		MayAdvise: intent == "seek_advice" || intent == "ask_evidence",
		Cite:      retrieve && (intent == "ask_evidence" || intent == "seek_advice"),
	}
}

// scaled applies a variant's terseness without going below a usable floor
func (s Shape) scaled(f float64) Shape {
	if f > 0 && f != 1 {
		s.Words = max(8, int(float64(s.Words)*f))
	}
	return s
}

// MaxTokens is ~1.3 tokens a word plus slack. the floor leaves room for a
// cited reply's markers, so a short budget truncates nothing mid-sentence
func (s Shape) MaxTokens() int { return max(160, s.Words*2+64) }

func (s Shape) Instructions() string {
	var b strings.Builder
	b.WriteString("THIS REPLY\n")
	b.WriteString("Length: about " + strconv.Itoa(s.Words) + " words. Shorter is fine.\n")
	if !s.MayAsk {
		b.WriteString("Do not ask a question.\n")
	}
	if !s.MayAdvise {
		b.WriteString("Do not give advice or next steps unless they asked for them.\n")
	}
	if s.Cite {
		b.WriteString("When a sentence relies on a research excerpt, end it with the excerpt's number in square brackets, like [2]. Use only numbers from RESEARCH CONTEXT.\n")
	} else {
		b.WriteString("Do not use [n] markers; mention a study in words only if it truly matters.\n")
	}
	return b.String()
}

// askedQuestion feeds lastReplyAsked from the previous assistant turn
func askedQuestion(reply string) bool { return strings.Contains(reply, "?") }
