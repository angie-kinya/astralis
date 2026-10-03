package agent

import "unicode/utf8"

// Plan is what the agent will do for this turn. it is decided by plain Go,
// not by the model: the same analysis always produces the same plan, which
// makes the routing testable, auditable and free
type Plan struct {
	Retrieve bool
	Query    string
	// Guidance is added to the reply prompt so the reply fits the intent
	Guidance string
}

// Decide is a pure function on purpose. everything the agent does that is
// not generating language lives here, where a unit test can reach it
func Decide(an Analysis, analysed bool, message string) Plan {
	// The analyser failed or timed out. behave exactly like the original
	// retrieval system did: search on the raw message and answer normally
	if !analysed {
		return Plan{Retrieve: true, Query: message}
	}

	query := an.SearchQuery
	if query == "" {
		query = message
	}

	switch an.Intent {
	case "greeting", "off_topic":
		return Plan{Guidance: guidance[an.Intent]}
	case "ask_evidence":
		// Asking what the research says is always a reason to retrieve
		return Plan{Retrieve: true, Query: query, Guidance: guidance[an.Intent]}
	}

	// A message too short to carry meaning ("yes", "ok thanks") retrieves
	// noise unless the analyser rewrote it into a real query
	retrieve := an.NeedsResearch && (an.SearchQuery != "" || utf8.RuneCountInString(message) >= 20)
	return Plan{Retrieve: retrieve, Query: query, Guidance: guidance[an.Intent]}
}

var guidance = map[string]string{
	"vent": "The person mainly wants to be heard. Reflect back what you heard in your own words. " +
		"Do not offer advice unless they ask for it. A question is optional; often it is better to end on acknowledgement.",
	"seek_advice": "The person is asking what to do. Reflect back briefly, then offer at most two small, " +
		"concrete steps they could try this week, each tied to what they told you.",
	"ask_evidence": "The person wants to know what the research says. Lead with the evidence and its limits. " +
		"Say plainly when the excerpts do not cover their question.",
	"report_progress": "The person is telling you how something they tried went. Acknowledge it specifically. " +
		"Reflect what seems to have made the difference. Ask about it only if you genuinely need to know, never as a reflex.",
	"reflect": "The person is thinking out loud. Help them notice patterns by reflecting what you notice; " +
		"use a question only if it opens something up. Do not rush to conclusions or advice.",
	"greeting":  "Reply briefly and warmly. A light invitation to say how work has been is fine; no interrogation.",
	"off_topic": "Reply briefly and kindly, and explain that Astralis is here for work stress and burnout.",
}
