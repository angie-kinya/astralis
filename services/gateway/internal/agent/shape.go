package agent

import (
	"strconv"
	"strings"
)

// Shape is the form of one reply, decided in Go before the model writes.
// the model gets one instruction line per field and MaxTokens follows the
// budget, so a long reply is impossible rather than discouraged
type Shape struct {
	Words     int // target length
	Ask       Ask // question cadence for this reply
	MayAdvise bool
	Cite      bool // inline [n] markers allowed
}

// Ask is the question cadence. the zero value forbids, so a Shape built by
// hand never asks by accident
type Ask int

const (
	AskForbid Ask = iota // no question at all
	AskAllow             // a question is optional, at most one
	AskInvite            // end with one easy question, if it feels natural
)

func (a Ask) String() string {
	switch a {
	case AskAllow:
		return "allow"
	case AskInvite:
		return "invite"
	default:
		return "forbid"
	}
}

// word budgets per intent: {floor, ceiling}. the floors sit where a reply
// still has room for a reaction and a question; below them replies read curt
var budgets = map[string][2]int{
	"greeting": {15, 45}, "off_topic": {20, 50}, "vent": {35, 140},
	"report_progress": {25, 110}, "reflect": {45, 150},
	"seek_advice": {70, 180}, "ask_evidence": {90, 220},
}

// shapeFor mirrors the person: a one-line message gets a short reply, never
// shorter than the intent's floor. the question cadence is not here: it needs
// the session's history, so Decide sets it
func shapeFor(intent string, userWords int, retrieve bool) Shape {
	b, ok := budgets[intent]
	if !ok {
		b = budgets["reflect"]
	}
	words := max(b[0], min(b[1], int(float64(userWords)*1.4)+b[0]/2))
	return Shape{
		Words:     words,
		MayAdvise: intent == "seek_advice" || intent == "ask_evidence",
		Cite:      retrieve && (intent == "ask_evidence" || intent == "seek_advice"),
	}
}

// MaxTokens is ~1.3 tokens a word plus slack. the floor leaves room for a
// cited reply's markers, so a short budget truncates nothing mid-sentence
func (s Shape) MaxTokens() int { return max(160, s.Words*2+64) }

func (s Shape) Instructions() string {
	var b strings.Builder
	b.WriteString("THIS REPLY\n")
	b.WriteString("Length: about " + strconv.Itoa(s.Words) + " words. Shorter is fine.\n")
	switch s.Ask {
	case AskInvite:
		b.WriteString("End with one easy, open question about them, if it feels natural.\n")
	case AskAllow:
		b.WriteString("A question is optional; at most one.\n")
	default:
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

// askedQuestion reads an earlier reply for the cadence window
func askedQuestion(reply string) bool { return strings.Contains(reply, "?") }

// inviteIntents are where a question helps someone keep talking. advice and
// evidence replies already carry the substance; a question there is an
// interview
var inviteIntents = map[string]bool{"vent": true, "reflect": true, "report_progress": true, "greeting": true}

// cadence decides the question rule for one reply. asked is whether each
// earlier assistant reply in the window asked, oldest first
func cadence(intent string, userWords int, userAsked, listening bool, asked []bool, quiet int) Ask {
	n := len(asked)
	if (n > 0 && asked[n-1]) || userAsked || intent == "ask_evidence" || intent == "off_topic" {
		return AskForbid
	}
	if !inviteIntents[intent] || listening {
		return AskAllow
	}
	for _, a := range asked[max(0, n-quiet):] {
		if a {
			return AskAllow
		}
	}
	// short messages are where a person most needs a hand to keep talking,
	// and early in a session a question is how a companion gets to know them
	if userWords < 15 || n < 3 {
		return AskInvite
	}
	return AskAllow
}

// questionWords open a message that asks us something even without a "?"
var questionWords = map[string]bool{
	"what": true, "how": true, "why": true, "who": true, "are": true, "is": true,
	"do": true, "does": true, "can": true, "could": true, "would": true, "will": true, "should": true,
}

// userAsked is true when the person put a question to us. we answer it
// rather than handing back one of our own. "when am I supposed to..." in a
// vent is rhetorical, so "when" and "where" are left out on purpose
func userAsked(message string) bool {
	if strings.Contains(message, "?") {
		return true
	}
	f := strings.Fields(strings.ToLower(message))
	return len(f) > 1 && questionWords[strings.Trim(f[0], ".,!'\"")]
}
