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
	// Topic is the session's active topic after this turn, stored on the run
	// so the next turn can tell whether the thread goes on
	Topic store.TopicState
	// Guidance is added to the reply prompt so the reply fits the stance
	Guidance string
	// Shape is the form of the reply: stance, length, questions, advice, citations
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
	// Topic is the active topic before this turn, from the session's last run
	Topic store.TopicState
	// Earlier are the person's earlier messages in the window, oldest first,
	// and LastReply is the reply just before this message ("" on a first turn)
	Earlier   []string
	LastReply string
}

// moveListen is recorded once a person says they want company rather than
// advice. it holds for the rest of the session: being asked to just listen
// and then coached two turns later is the failure it exists to prevent
const moveListen = "listen_only"

// Decide is a pure function on purpose. everything the agent does that is
// not generating language lives here, where a unit test can reach it.
// analysed is false when the analyser failed or timed out: the turn then
// runs on the phrase checks and the conversation state alone, which need no
// model, and searches on the raw message like the original retrieval system
func Decide(an Analysis, analysed bool, message string, sit Situation) Plan {
	if !analysed {
		an = Analysis{}
	}
	words := len(strings.Fields(message))
	voice := Variants[resolveVariant(sit.Variant)]
	userQ := userAsked(message)

	// a hello or a thank you keeps the thread where it was rather than
	// counting as a turn on it
	topic := sit.Topic
	if an.Intent != "greeting" && an.Intent != "off_topic" {
		topic = followTopic(sit.Topic, an.Topic)
	}

	phrase := wantsListening(message)
	declined := offerMade.MatchString(sit.LastReply) && declinedOffer(message)
	listening := sit.Moved[moveListen] || an.WantsAdvice == "no" || phrase || declined
	st := chooseStance(turnState{
		intent: an.Intent,
		hint:   an.StanceHint,
		depth:  an.Depth,
		asked: an.Intent == "seek_advice" || an.WantsAdvice == "yes" || acceptedOffer(sit.LastReply, message) ||
			(strongAsk.MatchString(message) && !phrase && !declined && an.WantsAdvice != "no"),
		softAsked:  an.StanceHint == string(StanceAdvise) || softAsk.MatchString(message) || askedBefore(sit.Earlier, topic.Turns-1),
		listening:  listening,
		evidence:   evidenceAsk.MatchString(message),
		topicTurns: topic.Turns,
	})

	p := Plan{Topic: topic, Guidance: guidance(st, an.Intent), Shape: shapeFor(st, words)}
	p.Shape.Ask = cadence(st, words, userQ, listening, sit.Asked, voice.quiet())
	switch st {
	case StanceAdvise:
		p.Shape.Advice = AdviceGive
	case StanceInform:
		p.Shape.Advice = AdviceAllow
	case StanceExplore:
		// the second turn on a topic is where a companion asks whether ideas
		// would help. the offer is the reply's one question, so it may follow
		// a reply that asked; it never answers a question with a question
		if topic.Turns == 2 && !userQ {
			p.Shape.Advice, p.Shape.Ask = AdviceOffer, AskInvite
		}
	}

	// research: evidence asks always search; exploring and advising search
	// whenever the topic is one the library covers. listening never does: a
	// study is not company
	p.Query = topic.Topic
	if p.Query == "" && utf8.RuneCountInString(message) >= 20 {
		p.Query = message // a message too short to carry meaning retrieves noise
	}
	researchable := !analysed || construct(topic.Topic) != "" || an.NeedsResearch ||
		len(an.Demands)+len(an.Resources) > 0
	switch st {
	case StanceInform:
		if p.Query == "" {
			p.Query = message
		}
		p.Retrieve = true
	case StanceExplore, StanceAdvise:
		p.Retrieve = researchable && p.Query != ""
	}
	// markers and the studies chip only when evidence is what they came for;
	// otherwise research is something Astralis has read, not something it cites
	switch {
	case p.Retrieve && (st == StanceInform || evidenceAsk.MatchString(message)):
		p.Shape.Cite = CiteMarkers
	case p.Retrieve:
		p.Shape.Cite = CiteWords
	}

	// listen-only overrides everything except an outright ask, which lifts
	// it for this turn only; the move stays recorded, so the next vent is
	// listened to again
	if listening && st != StanceAdvise {
		listen(&p, sit)
	}
	notice(&p, st, sit)
	return p
}

// askedBefore is true when one of the last n earlier messages asked for
// help, outright or by naming a problem. n is the topic's earlier turns, so
// an ask about something else entirely does not carry over
func askedBefore(earlier []string, n int) bool {
	for _, m := range earlier[max(0, len(earlier)-max(0, n)):] {
		if strongAsk.MatchString(m) || softAsk.MatchString(m) {
			return true
		}
	}
	return false
}

// declinedOffer is a no to "want a couple of ideas?": they would rather keep
// talking, which is the same as asking to be listened to. only a short no
// counts, and never one about ideas: "no idea what to do" is an ask
var (
	declineRe   = regexp.MustCompile(`(?i)^\W*(no|nah|nope|not really|not yet|not now)\b`)
	keepTalking = regexp.MustCompile(`(?i)\b(keep talking|rather (just )?(keep )?talk)`)
)

func declinedOffer(message string) bool {
	if keepTalking.MatchString(message) {
		return true
	}
	return declineRe.MatchString(message) && len(strings.Fields(message)) <= 6 &&
		!strings.Contains(strings.ToLower(message), "idea")
}

// listen applies listen-only to a plan and records the move the first time
func listen(p *Plan, sit Situation) {
	p.Shape.Advice = AdviceNone
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

// notice adds at most one noticing move per session, only while listening or
// exploring. an observation from the trajectory, never a verdict
func notice(p *Plan, st Stance, sit Situation) {
	if noticed(sit.Moved) || (st != StanceListen && st != StanceExplore) {
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

// stanceGuidance is how to approach each stance. length, questions, advice
// and citations are not here: Shape decides those in code
var stanceGuidance = map[Stance]string{
	StanceListen: "They mainly want to be heard. React to the specific thing that happened and to how they seem, " +
		"in plain words, and stay with it. No fixing and no silver linings.",
	StanceExplore: "This is early in the topic. Reflect back what is specific in what they said, offer one observation " +
		"of your own, and help them open it up. Don't rush to conclusions or advice.",
	StanceAdvise: "Start with a warm, specific acknowledgement of what they said. Then give the ideas, grounded in their " +
		"own words and, where it fits, in what the research says. Small and doable beats big and wise.",
	StanceInform: "They want to know what the research says. Lead with what it finds and where it stops, plainly. " +
		"Say so when the excerpts don't cover their question.",
	StanceGreet:    "Brief and easy. A light opening to say how work has been is fine; no interrogation.",
	StanceOffTopic: "Brief and kind. Say that Astralis is here for work stress and burnout.",
}

// intentNotes add what the stance alone does not say
var intentNotes = map[string]string{
	"report_progress": "They are telling you how something they tried went. Respond to the specifics of what changed " +
		"and what seems to have made the difference.",
}

func guidance(st Stance, intent string) string {
	return strings.TrimSpace(stanceGuidance[st] + "\n" + intentNotes[intent])
}
