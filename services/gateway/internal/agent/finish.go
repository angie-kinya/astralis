package agent

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// v2.1 derived max_tokens from the word budget, so a short message capped the
// reply at 160 tokens and the model stopped mid-sentence. the ceiling is now a
// safety limit only: length is asked for in the prompt and checked by the
// lint, and nothing about a reply's length is enforced by cutting it off

// replyCeiling is the max_tokens for a reply call. effort lets the model think
// before it writes, and thinking spends from the same allowance, so a set
// effort gets more room
func replyCeiling(effort string) int {
	switch effort {
	case "":
		return 1024
	case "low":
		return 2048
	default:
		return 4096
	}
}

// stopMaxTokens is the stop_reason for a reply the ceiling cut short
const stopMaxTokens = "max_tokens"

// a citation marker at the very end, so "sleep matters [2]" is judged on the
// word before the marker rather than on the bracket
var trailingCite = regexp.MustCompile(`(\s*\[\d+(?:\s*,\s*\d+)*\])+\s*$`)

// closers may follow the full stop that ends a sentence: quotes, brackets and
// markdown emphasis
const closers = `"'”’)]*_`

// complete is true when a reply ends a sentence: terminal punctuation, or an
// emoji, which ends a casual line just as well. services/evals/common.py
// carries the same check
func complete(s string) bool {
	s = strings.TrimRight(trailingCite.ReplaceAllString(strings.TrimSpace(s), ""), closers)
	r, n := utf8.DecodeLastRuneInString(s)
	return n > 0 && (terminal(r) || unicode.Is(unicode.So, r))
}

func terminal(r rune) bool { return r == '.' || r == '!' || r == '?' || r == '…' }

// trimToSentence drops whatever follows the last complete sentence. a sentence
// ends at terminal punctuation (plus any closers and a citation marker) that
// is followed by whitespace or the end of the text, so "3.5" or "e.g" inside a
// sentence is not mistaken for an end. "" when no sentence ever finished
func trimToSentence(s string) string {
	s = strings.TrimSpace(s)
	end := 0
	for i, r := range s {
		if !terminal(r) {
			continue
		}
		j := i + utf8.RuneLen(r)
		for j < len(s) {
			c, n := utf8.DecodeRuneInString(s[j:])
			if !strings.ContainsRune(closers, c) {
				break
			}
			j += n
		}
		if m := leadingCite.FindStringIndex(s[j:]); m != nil {
			j += m[1]
		}
		if next, _ := utf8.DecodeRuneInString(s[j:]); j == len(s) || unicode.IsSpace(next) {
			end = j
		}
	}
	return strings.TrimSpace(s[:end])
}

var leadingCite = regexp.MustCompile(`^\s?\[\d+(?:\s*,\s*\d+)*\][.!?]?`)

// finish is the last check before a reply is stored. a reply the ceiling cut
// loses its unfinished sentence; one that simply ended without a full stop
// gets one. ok is false when a cut reply had no complete sentence at all,
// which the caller must treat as a failed reply, never as text to show
func finish(text string, cut bool) (out string, ok bool) {
	text = strings.TrimSpace(text)
	if cut {
		out = trimToSentence(text)
		return out, out != ""
	}
	if text == "" || complete(text) {
		return text, text != ""
	}
	// the model stopped on its own, so the words are all there; only the
	// mark is missing. a dangling comma or colon becomes the full stop
	return strings.TrimRight(text, ",;:–- ") + ".", true
}
