package agent

import (
	"regexp"
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
	// Moves are the session moves this turn makes (noticing, listen_only), recorded in session_moves
	Moves []string
}

// Situation is everything Decide reads besides the analysis. it is gathered
// by Handle so Decide itself stays pure
type Situation struct {
	// Asked is whether each earlier assistant reply in the window asked a
	// question, oldest first. its length doubles as the reply count, which is
	// exact for the first replies of a session, the only place it matters
	Asked   []bool
	Pattern store.Pattern   // trajectory over turn_signals
	Moved   map[string]bool // moves this session has already made, listen_only included
	Variant string
}

// moveListen is recorded once a person says they want company rather than
// advice. it holds for the rest of the session: being asked to just listen
// and then coached two turns later is the failure it exists to prevent
const moveListen = "listen_only"

// Decide is a pure function on purpose. everything the agent does that is
// not generating language lives here, where a unit test can reach it
func Decide(an Analysis, analysed bool, message string, sit Situation) Plan {
	words := len(strings.Fields(message))
	voice := Variants[resolveVariant(sit.Variant)]
	asked := userAsked(message)

	// the analyser failed or timed out. behave like the original retrieval
	// system: search on the raw message, answer at a middling length, and
	// allow citations since nothing says they are out of place. the phrase
	// check and an earlier listen_only still hold, they need no model
	if !analysed {
		listening := sit.Moved[moveListen] || wantsListening(message)
		sh := shapeFor("reflect", words, true)
		sh.Cite = true
		sh.Ask = cadence("", words, asked, listening, sit.Asked, voice.quiet())
		p := Plan{Retrieve: true, Query: message, Shape: sh}
		listen(&p, listening, sit)
		return p
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

	// asking for advice outright lifts listen-only for this turn only; the
	// move stays recorded, so the next vent is listened to again
	listening := sit.Moved[moveListen] || an.WantsAdvice == "no" || wantsListening(message)
	listening = listening && an.Intent != "seek_advice"

	p.Shape = shapeFor(an.Intent, words, p.Retrieve)
	p.Shape.Ask = cadence(an.Intent, words, asked, listening, sit.Asked, voice.quiet())
	listen(&p, listening, sit)
	notice(&p, an.Intent, sit)
	return p
}

// listen applies listen-only to a plan and records the move the first time
func listen(p *Plan, listening bool, sit Situation) {
	if !listening {
		return
	}
	p.Shape.MayAdvise = false
	if !sit.Moved[moveListen] {
		p.Moves = append(p.Moves, moveListen)
	}
	p.Guidance = strings.TrimSpace(p.Guidance + "\n" + listenGuidance)
}

const listenGuidance = "They've said they want to be listened to, not advised. Stay with them: no fixing, " +
	"no suggestions, no silver linings."

// listenPhrases are ways people say they want company, not solutions. the
// analyser's wants_advice catches the rest; this check costs nothing and
// still works when the analyser is down
var listenPhrases = regexp.MustCompile(`(?i)\b(just (really )?(need|want(ed)?) to (vent|rant|moan|talk|get (this|it) off my chest)|` +
	`(don['’]?t|do not) (really )?(want|need) (any )?(advice|solutions|tips)|not (looking|asking) for (advice|solutions|tips)|` +
	`(just|please) listen|(don['’]?t|do not) (try to )?fix|let me (vent|rant)|need (a|to) (rant|vent))`)

func wantsListening(message string) bool { return listenPhrases.MatchString(message) }

// notice adds at most one noticing move per session, only when someone is
// venting or reflecting. an observation from the trajectory, never a verdict
func notice(p *Plan, intent string, sit Situation) {
	if noticed(sit.Moved) || (intent != "vent" && intent != "reflect") {
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

// noticed is true once any noticing move has been made this session.
// listen_only is a move too, but it is not an observation and must not use
// up the session's one noticing
func noticed(moved map[string]bool) bool {
	for m := range moved {
		if strings.HasPrefix(m, "notice_") {
			return true
		}
	}
	return false
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
	"vent": "They mainly want to be heard. React to the specific thing that happened and to how they seem, " +
		"in plain words. No advice unless they ask for it.",
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
