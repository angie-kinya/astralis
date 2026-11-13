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

// jargon is words that make a reply sound like a report or a therapist.
// short on purpose: every entry has an everyday way to say the same thing.
// "boundaries" only counts in its therapy-speak pairings, not on its own
var jargon = regexp.MustCompile(`(?i)\b(mismatch(es)?|unsustainable|leverag(e|es|ed|ing)|paradigms?|` +
	`optimi[sz](e|es|ed|ing|ation)|frameworks?|dressed up as|cognitive|holistic(ally)?|actionable|` +
	`(set|setting|hold|holding|healthy|firm|clear) boundaries|hold(ing)? space)\b`)

var (
	letters    = regexp.MustCompile(`[a-z]+`)
	vowelGroup = regexp.MustCompile(`[aeiouy]+`)
)

// syllables is the usual vowel-group estimate: count runs of vowels and drop
// a silent final e. it is wrong on some words, but evenly wrong, which is
// all a rate needs. common.py carries the same function
func syllables(word string) int {
	n := len(vowelGroup.FindAllString(word, -1))
	if n > 1 && strings.HasSuffix(word, "e") && !strings.HasSuffix(word, "le") {
		n--
	}
	return max(1, n)
}

// longWords counts words of four or more syllables, out of all words
func longWords(s string) (long, total int) {
	for _, w := range letters.FindAllString(strings.ToLower(s), -1) {
		total++
		if syllables(w) >= 4 {
			long++
		}
	}
	return long, total
}

// plainWords is false when a reply leans on long or technical words. one
// long word is never enough, so "everything okay?" in a short reply passes
func plainWords(s string) bool {
	long, total := longWords(s)
	return !jargon.MatchString(s) && (long < 2 || long*100 <= total*8)
}

func opener(s string) string {
	w := strings.Fields(strings.ToLower(s))
	return strings.Join(w[:min(3, len(w))], " ")
}

// Lint returns the rule names a reply broke, in a stable order.
// prev = earlier assistant replies in this session, oldest first
func Lint(reply string, s Shape, prev []string) []string {
	v := []string{}
	// a missing question on invite is never a violation: we invite, we
	// never force, and a forced question is the interview feel to avoid
	q := strings.Count(reply, "?")
	if q > 1 || (q == 1 && s.Ask == AskForbid) {
		v = append(v, "question")
	}
	// the band's top with a quarter of slack: over it is long, and a reply
	// under the band is never a violation, since a short complete reply beats
	// a padded one. a hand-built shape with no band has no length rule
	if n := len(strings.Fields(reply)); s.Band.Max > 0 && n > s.Band.Max*5/4 {
		v = append(v, "length")
	}
	if stock.MatchString(reply) {
		v = append(v, "stock_phrase")
	}
	if dash.MatchString(reply) {
		v = append(v, "dash")
	}
	if !plainWords(reply) {
		v = append(v, "plain_words")
	}
	if !complete(reply) {
		v = append(v, "truncated")
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
	"length":          "make it shorter, keeping what matters most",
	"truncated":       "finish the reply: it stops mid-sentence, so end on a complete sentence",
	"stock_phrase":    "replace stock reassurance with something specific to what they said",
	"dash":            "use commas or full stops instead of dashes",
	"plain_words":     "use everyday words, the way you'd say it out loud",
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
