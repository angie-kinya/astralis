package agent

import (
	"regexp"
	"strings"

	"astralis/gateway/internal/store"
)

// constructs are the burnout and work-stress ideas the research library is
// organised around. a topic that names one is worth a search whatever the
// analyser said about needs_research: the person deserves the evidence even
// when they did not ask for it by name. first match wins, so the specific
// constructs sit above the broad ones
var constructs = []struct {
	name string
	re   *regexp.Regexp
}{
	{"motivation", regexp.MustCompile(`motivat|procrastinat|getting started|worth starting|can'?t start|disengag|engagement`)},
	{"meaning", regexp.MustCompile(`meaning|purpose|passion|fulfil|worthwhile|calling`)},
	{"exhaustion", regexp.MustCompile(`exhaust|fatigue|tired|drained|burn ?out|burnt out|worn out|energy`)},
	{"detachment", regexp.MustCompile(`detach|switch(ing)? off|unplug|after hours|out of hours|evenings?|weekends?`)},
	{"cynicism", regexp.MustCompile(`cynic|don'?t care|pointless|numb|resent`)},
	{"efficacy", regexp.MustCompile(`efficacy|confidence|imposter|impostor|competen|self.doubt|not good enough`)},
	{"recovery", regexp.MustCompile(`recover|rest\b|sleep|breaks?\b|holiday|annual leave|time off`)},
	{"workload", regexp.MustCompile(`workload|overload|too much|deadline|overtime|long hours|meetings?|emails?|time pressure`)},
	{"autonomy", regexp.MustCompile(`autonom|control|micromanag|flexib`)},
	{"recognition", regexp.MustCompile(`recogni|apprecia|credit|reward|promotion|review`)},
	{"support", regexp.MustCompile(`support|manager|colleague|team|isolat|lonel|conflict|bully`)},
	{"insecurity", regexp.MustCompile(`insecur|redundan|layoff|restructur|merger|job loss`)},
	{"work_life", regexp.MustCompile(`work.life|family|partner|commut|childcare`)},
	{"role", regexp.MustCompile(`role|expectations|responsibilit`)},
}

// construct names the research construct a topic maps to, or ""
func construct(topic string) string {
	t := strings.ToLower(topic)
	for _, c := range constructs {
		if c.re.MatchString(t) {
			return c.name
		}
	}
	return ""
}

// topicStop are words too common in topics to say two of them match
var topicStop = map[string]bool{
	"work": true, "with": true, "about": true, "their": true, "from": true, "feeling": true,
	"feels": true, "lately": true, "being": true, "job": true, "at": true, "the": true,
}

func topicWords(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range letters.FindAllString(strings.ToLower(s), -1) {
		if len(w) >= 4 && !topicStop[w] {
			out[w] = true
		}
	}
	return out
}

// sameTopic is the continuity heuristic: two topics are one thread when they
// map to the same construct, or share at least a third of their words. the
// analyser rephrases a topic every turn ("motivation at work", then "nothing
// feels worth starting"), so exact equality would restart every thread
func sameTopic(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if ca := construct(a); ca != "" && ca == construct(b) {
		return true
	}
	wa, wb := topicWords(a), topicWords(b)
	shared := 0
	for w := range wa {
		if wb[w] {
			shared++
		}
	}
	union := len(wa) + len(wb) - shared
	return union > 0 && shared*3 >= union
}

// followTopic carries the session's active topic into this turn and counts
// the turns spent on it. a message with no topic of its own ("yeah", "ok",
// or the analyser down) continues the thread, so short replies still move a
// topic from exploring to advising
func followTopic(prev store.TopicState, topic string) store.TopicState {
	switch {
	case topic == "" && prev.Topic == "":
		return store.TopicState{}
	case topic == "":
		return store.TopicState{Topic: prev.Topic, Turns: prev.Turns + 1}
	case sameTopic(prev.Topic, topic):
		return store.TopicState{Topic: topic, Turns: prev.Turns + 1}
	default:
		return store.TopicState{Topic: topic, Turns: 1}
	}
}

// researchUsed is the heuristic behind agent_runs.used_in_words: did a reply
// draw on the excerpts without citing them? it counts distinctive words the
// reply shares with the excerpts that the person never said, or a plain
// mention of research. crude, but evenly crude, which is all a rate needs
var researchMention = regexp.MustCompile(`(?i)\b(research|studies|study|evidence|researchers)\b`)

func researchUsed(reply string, chunks []store.Chunk, said string) bool {
	if len(chunks) == 0 {
		return false
	}
	if researchMention.MatchString(reply) {
		return true
	}
	theirs := topicWords(said)
	var excerpts strings.Builder
	for _, c := range chunks {
		excerpts.WriteString(c.Content + " ")
	}
	from := topicWords(excerpts.String())
	n := 0
	for w := range topicWords(reply) {
		if len(w) >= 5 && from[w] && !theirs[w] && !commonWords[w] {
			n++
		}
	}
	return n >= 3
}

// commonWords would match any excerpt and say nothing about using one
var commonWords = map[string]bool{
	"there": true, "these": true, "things": true, "might": true, "could": true, "would": true,
	"people": true, "which": true, "where": true, "about": true, "other": true, "really": true,
	"something": true, "often": true, "because": true, "every": true, "first": true, "small": true,
	"while": true, "still": true, "start": true, "think": true, "makes": true, "sense": true,
}
