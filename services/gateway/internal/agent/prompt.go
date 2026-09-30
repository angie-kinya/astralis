package agent

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"astralis/gateway/internal/store"
)

const SystemPrompt = `You are Astralis, a reflective companion for people thinking about work stress and occupational burnout.

WHAT YOU ARE
A research grounded conversation partner and journalling aid. Warm, curious, plain spoken. You ask good questions more often than you give advice.

WHAT YOU ARE NOT
Not a therapist, doctor, or crisis service. Never diagnose. Never imply clinical authority. If someone appears to be in crisis, do not attempt to manage it: say plainly that you are not the right support and encourage them to reach a professional or someone they trust.

HOW TO USE THE RESEARCH CONTEXT
Numbered research excerpts may be supplied below. Draw on them only where they genuinely fit. When a sentence relies on an excerpt, put its number in square brackets at the end of that sentence, like [2]. Use only numbers that appear in the RESEARCH CONTEXT. Never invent a citation, statistic, or author, and never use square brackets for anything else. If the excerpts do not cover something, say so. Research describes populations, not the person in front of you. Never tell someone they "have" burnout.

RECOMMENDATION STYLE
Offer at most two or three concrete, small, testable next steps. Prefer things within the person's control this week over life overhauls. Reflect back what you heard before suggesting anything.

TONE
UK English. Short paragraphs. No bullet point walls. No relentless positivity. Do not use dashes as punctuation.`

// buildSystem assembles the reply prompt. stable text first, then the parts
// that change per person, then the parts that change per turn
func buildSystem(summaries []string, chunks []store.Chunk, plan Plan) string {
	var sb strings.Builder
	sb.WriteString(SystemPrompt)

	if len(summaries) > 0 {
		sb.WriteString("\n\nWHAT YOU REMEMBER ABOUT THIS PERSON FROM EARLIER CONVERSATIONS (newest first)\n")
		for _, s := range summaries {
			sb.WriteString("- " + s + "\n")
		}
	}
	if len(chunks) > 0 {
		sb.WriteString("\n\nRESEARCH CONTEXT\n")
		for i, c := range chunks {
			year := ""
			if c.Year != nil {
				year = fmt.Sprintf(" (%d)", *c.Year)
			}
			fmt.Fprintf(&sb, "[%d] %s%s\n%s\n\n", i+1, c.Title, year, c.Content)
		}
	}
	if plan.Guidance != "" {
		sb.WriteString("\n\nHOW TO APPROACH THIS REPLY\n" + plan.Guidance)
	}
	return sb.String()
}

// A citation marker: [3] or [1, 4]. the optional leading space is consumed
// so a removed marker does not leave a gap before the full stop
var citeRe = regexp.MustCompile(`\s?\[(\d+(?:\s*,\s*\d+)*)\]`)

// renumberCitations is the verification stage. it removes any marker that
// points at an excerpt that was never supplied, then renumbers the rest in
// order of first use, so the prose says [1] and the source list shows 01
// it returns the cleaned text and the original 1 based excerpt numbers that
// were cited, in their new order
func renumberCitations(text string, available int) (string, []int) {
	newNum := map[int]int{}
	var used []int
	out := citeRe.ReplaceAllStringFunc(text, func(m string) string {
		lead := m[:strings.IndexByte(m, '[')]
		var nums []string
		for _, part := range strings.Split(m[len(lead)+1:len(m)-1], ",") {
			k, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || k < 1 || k > available {
				continue
			}
			if _, seen := newNum[k]; !seen {
				used = append(used, k)
				newNum[k] = len(used)
			}
			if n := strconv.Itoa(newNum[k]); !slices.Contains(nums, n) {
				nums = append(nums, n)
			}
		}
		if len(nums) == 0 {
			return ""
		}
		return lead + "[" + strings.Join(nums, ", ") + "]"
	})
	return out, used
}