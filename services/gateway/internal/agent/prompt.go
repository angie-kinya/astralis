package agent

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"astralis/gateway/internal/store"
)

// buildSystem assembles the reply prompt. stable text first (the character),
// then what changes per person (memory), then what changes per turn
// (research, guidance, shape). the shape goes last, nearest the reply
func buildSystem(variant string, summaries []string, facts []store.Fact, chunks []store.Chunk, plan Plan) string {
	var sb strings.Builder
	sb.WriteString(Character)
	sb.WriteString(Variants[variant].Line)

	if len(summaries) > 0 {
		sb.WriteString("\n\nEARLIER CONVERSATIONS (your own notes, newest first)\n")
		for _, s := range summaries {
			sb.WriteString("- " + s + "\n")
		}
	}
	if len(facts) > 0 {
		sb.WriteString("\n\n" + renderFacts(facts))
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
	sb.WriteString("\n\n" + plan.Shape.Instructions())
	return sb.String()
}

// renderFacts dates every fact: "2 Nov, person ..." is what makes recall read
// as remembering rather than as a database lookup
func renderFacts(facts []store.Fact) string {
	var sb strings.Builder
	sb.WriteString("WHAT YOU KNOW ABOUT THEM (use only when it fits; never list these back)\n")
	for _, f := range facts {
		fmt.Fprintf(&sb, "- %s, %s %q: %s", f.LastSeen.Format("2 Jan"), f.Kind, f.Subject, f.Detail)
		if f.Kind == "commitment" {
			sb.WriteString(" (" + f.Status + ")")
		}
		sb.WriteString("\n")
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
