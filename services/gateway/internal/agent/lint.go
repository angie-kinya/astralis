package agent

import (
	"regexp"
	"slices"
	"strings"
)

// stock phrases that read as assistant boilerplate. this list is a metric,
// not the strategy: the character spec is the strategy
var stock = regexp.MustCompile(`(?i)\b(it sounds like|i hear you|that'?s (completely |totally )?(valid|understandable)|` +
	`it'?s (important|okay) to|you'?re not alone|great question|i'?m here for you|` +
	`take care of yourself|be gentle with yourself|that must be (so )?(hard|difficult))\b`)

// a spaced hyphen or en dash, or an em dash anywhere
var dash = regexp.MustCompile(` [–-] |—`)

func opener(s string) string {
	w := strings.Fields(strings.ToLower(s))
	return strings.Join(w[:min(3, len(w))], " ")
}

// Lint returns the rule names a reply broke, in a stable order.
// prev = earlier assistant replies in this session, oldest first
func Lint(reply string, s Shape, prev []string) []string {
	v := []string{}
	q := strings.Count(reply, "?")
	if q > 1 || (q == 1 && !s.MayAsk) {
		v = append(v, "question")
	}
	if n := len(strings.Fields(reply)); n > s.Words*16/10 {
		v = append(v, "length")
	}
	if stock.MatchString(reply) {
		v = append(v, "stock_phrase")
	}
	if dash.MatchString(reply) {
		v = append(v, "dash")
	}
	if op := opener(reply); op != "" {
		for _, p := range prev[max(0, len(prev)-5):] {
			if opener(p) == op {
				v = append(v, "repeated_opener")
				break
			}
		}
	}
	return v
}

// repairHints turn violations into editor instructions for the one second pass
var repairHints = map[string]string{
	"question":        "remove the question, or keep only one if a question is allowed",
	"length":          "cut it to about the target length",
	"stock_phrase":    "replace stock reassurance with something specific to what they said",
	"dash":            "use commas or full stops instead of dashes",
	"repeated_opener": "start differently from your recent replies",
}

// repairNote is the revision brief appended to the system prompt
func repairNote(violations []string, draft string) string {
	var hints []string
	for _, k := range violations {
		if h, ok := repairHints[k]; ok && !slices.Contains(hints, h) {
			hints = append(hints, h)
		}
	}
	return "\n\nREVISION\nYou already drafted a reply to their latest message. Revise it: " +
		strings.Join(hints, "; ") + ". Keep what it says and any [n] markers that still fit. " +
		"Return only the revised reply.\n\nDRAFT\n" + draft
}
