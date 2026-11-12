package agent

import (
	"strings"
	"unicode/utf8"

	"astralis/gateway/internal/store"
)

// Plan is what the agent will do for this turn. it is decided by plain Go,
// not by the model: the same inputs always produce the same plan, which
// makes the routing testable, auditable and free
type Plan struct {
	Retrieve bool
	Query    string
	// Guidance is added to the reply prompt so the reply fits the intent
	Guidance string
	// Shape is the form of the reply: length, questions, advice, citations
	Shape Shape
	// Moves are the noticing moves this turn makes, recorded in session_moves
	Moves []string
}

// Situation is everything Decide reads besides the analysis. it is gathered
// by Handle so Decide itself stays pure
type Situation struct {
	LastAsked bool            // the previous assistant reply asked a question
	Pattern   store.Pattern   // trajectory over turn_signals
	Moved     map[string]bool // noticing moves this session has already made
	Variant   string
}

// Decide is a pure function on purpose. everything the agent does that is
// not generating language lives here, where a unit test can reach it
func Decide(an Analysis, analysed bool, message string, sit Situation) Plan {
	words := len(strings.Fields(message))
	scale := Variants[sit.Variant].Scale

	// the analyser failed or timed out. behave like the original retrieval
	// system: search on the raw message, answer at a middling length, and
	// allow citations since nothing says they are out of place
	if !analysed {
		sh := shapeFor("reflect", words, sit.LastAsked, true).scaled(scale)
		sh.Cite = true
		return Plan{Retrieve: true, Query: message, Shape: sh}
	}

	query := an.SearchQuery
	if query == "" {
		query = message
	}

	var p Plan
	switch an.Intent {
	case "greeting", "off_topic":
		p = Plan{Guidance: guidance[an.Intent]}
	case "ask_evidence":
		// asking what the research says is always a reason to retrieve
		p = Plan{Retrieve: true, Query: query, Guidance: guidance[an.Intent]}
	default:
		// a message too short to carry meaning ("yes", "ok thanks") retrieves
		// noise unless the analyser rewrote it into a real query
		retrieve := an.NeedsResearch && (an.SearchQuery != "" || utf8.RuneCountInString(message) >= 20)
		p = Plan{Retrieve: retrieve, Query: query, Guidance: guidance[an.Intent]}
	}
	p.Shape = shapeFor(an.Intent, words, sit.LastAsked, p.Retrieve).scaled(scale)
	notice(&p, an.Intent, sit)
	return p
}

// notice adds at most one noticing move per session, only when someone is
// venting or reflecting. an observation from the trajectory, never a verdict
func notice(p *Plan, intent string, sit Situation) {
	if len(sit.Moved) > 0 || (intent != "vent" && intent != "reflect") {
		return
	}
	pt := sit.Pattern
	switch {
	case pt.RecurringDemand != "":
		p.Moves = append(p.Moves, "notice_demand")
		p.Guidance += "\nIf it fits, say once and plainly that " + humanise(pt.RecurringDemand) +
			" has come up in several of your recent conversations. An observation, not a conclusion."
	case pt.NRecent >= 4 && pt.ExhRecent-pt.ExhBefore >= 0.7:
		p.Moves = append(p.Moves, "notice_exhaustion")
		p.Guidance += "\nThe last week has sounded more tiring than the weeks before. You may say so gently, once."
	}
}

// humanise turns a closed-vocabulary demand into words a person would say
func humanise(demand string) string {
	if s, ok := demandWords[demand]; ok {
		return s
	}
	return strings.ReplaceAll(demand, "_", " ")
}

var demandWords = map[string]string{
	"manager_relationship":  "things with their manager",
	"colleague_conflict":    "friction with colleagues",
	"lack_of_recognition":   "not being recognised",
	"job_insecurity":        "worry about the job itself",
	"emotional_labour":      "the emotional side of the work",
	"organisational_change": "changes at work",
	"work_life_conflict":    "work spilling into the rest of life",
	"role_ambiguity":        "not being clear what the role is",
	"role_conflict":         "being pulled in opposite directions",
}

// guidance shapes the reply to the intent. length, questions and citations
// are not here: Shape decides those in code
var guidance = map[string]string{
	"vent": "They mainly want to be heard. Respond to the specific thing that happened, not to feelings in general. " +
		"No advice unless they ask for it.",
	"seek_advice": "They are asking what to do. Offer at most two small, concrete things they could try this week, " +
		"each tied to what they told you.",
	"ask_evidence": "They want to know what the research says. Lead with the evidence and where it stops. " +
		"Say plainly when the excerpts do not cover their question.",
	"report_progress": "They are telling you how something they tried went. Respond to the specifics of what changed " +
		"and what seems to have made the difference.",
	"reflect":   "They are thinking out loud. Offer what you notice, plainly. Do not rush to conclusions or advice.",
	"greeting":  "Brief and easy. A light opening to say how work has been is fine; no interrogation.",
	"off_topic": "Brief and kind. Say that Astralis is here for work stress and burnout.",
}
