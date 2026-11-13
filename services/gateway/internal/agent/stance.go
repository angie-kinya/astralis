package agent

import (
	"regexp"
	"strings"
)

// Stance is the purpose of one reply. v2.1 steered replies with a handful of
// booleans spread across intents; v2.2 decides one stance per turn, and the
// stance carries the length band, the question cadence, whether advice is
// given or offered, and how research is used. Go chooses it from the analysis
// and the state of the conversation; the analyser's stance_hint is one input,
// never the decision
type Stance string

const (
	StanceGreet    Stance = "greet"
	StanceOffTopic Stance = "off_topic"
	StanceListen   Stance = "listen"  // stay with them, no fixing
	StanceExplore  Stance = "explore" // early in a topic: reflect, observe, open it up
	StanceAdvise   Stance = "advise"  // small concrete things grounded in what they said
	StanceInform   Stance = "inform"  // what the research says and where it stops
)

// Stances is the analyser's vocabulary for stance_hint
var Stances = []string{
	string(StanceListen), string(StanceExplore), string(StanceAdvise),
	string(StanceInform), string(StanceGreet), string(StanceOffTopic),
}

// Band is a reply length in words, lowest and highest. the model is never
// told a word count: Instructions turns the band into sentences
type Band struct{ Min, Max int }

// bands per stance. advice and evidence need room to be specific; a listening
// reply that runs long starts to sound like a speech
var bands = map[Stance]Band{
	StanceGreet:    {15, 60},
	StanceOffTopic: {20, 60},
	StanceListen:   {40, 120},
	StanceExplore:  {60, 150},
	StanceAdvise:   {120, 260},
	StanceInform:   {150, 320},
}

// mirror narrows a stance's band to the person: a one-line message gets the
// lower part of the band, a long thoughtful one the whole of it. the floor
// never moves, and the top never falls below a third of the way up, so even
// "ugh" leaves room for a real reply
func mirror(b Band, userWords int) Band {
	hi := min(b.Max, max(b.Min+(b.Max-b.Min)/3, b.Min+userWords*3/2))
	return Band{b.Min, hi}
}

// strongAsk is someone asking for help in so many words. it lifts listen-only
// for the turn, as asking outright always has
var strongAsk = regexp.MustCompile(`(?i)\b(i need (some )?help|need (some )?(help|advice|ideas|tips)|help me|` +
	`how (do|can|should|would) i|what (should|can|could|do) i (actually )?do|what would you (actually |really |honestly )?do|` +
	`any (advice|tips|ideas|suggestions)|(give|got) (me )?(some )?(advice|tips|ideas)|` +
	`what (should|can) i try|where do i (even )?start)\b`)

// softAsk is an implicit ask: naming a problem as a problem ("having trouble
// with", "struggling to"). it is a reason to advise once the topic has had a
// turn of exploring, not a reason to skip straight to fixing
var softAsk = regexp.MustCompile(`(?i)\b(having (a )?(hard )?(trouble|difficulty|a hard time)|trouble (with|finding|getting|starting)|` +
	`struggl(e|ing) (with|to)|can'?t (seem to|figure out|work out)|don'?t (even )?know (how|what|where) to)\b`)

// evidenceAsk is asking what research says, in words that cannot be about
// anything else ("I study at night" and "computer science" are not asks). it
// is what earns inline [n] markers outside the inform stance
var evidenceAsk = regexp.MustCompile(`(?i)(\bwhat (does|do) (the )?(research|studies|science|evidence)\b|` +
	`\b(any|is there( any)?|got any) (research|studies|evidence)\b|` +
	`\b(research|studies|evidence) (say|says|show|shows|suggest|suggests)\b|` +
	`\baccording to (the )?(research|studies)\b|\b(cite|citations?|sources?|references?)\s*\?|` +
	`\b(show|give|send) me (the )?(studies|research|sources|papers)\b)`)

// offerMade spots the explore offer in the previous reply, and yes spots a
// short acceptance of it ("yes please", "sure, go on")
var (
	offerMade = regexp.MustCompile(`(?i)\b(couple of|a few|some) (ideas|things to try|suggestions)\b`)
	yes       = regexp.MustCompile(`(?i)^\W*(yes|yeah|yep|yup|sure|ok(ay)?|please|go on|go for it|ideas?)\b`)
)

func acceptedOffer(lastReply, message string) bool {
	return offerMade.MatchString(lastReply) && strings.Contains(lastReply, "?") &&
		yes.MatchString(message) && len(strings.Fields(message)) <= 8
}

// turnState is what chooseStance reads: the analysis reduced to the facts the
// table needs, plus the state of the conversation. a plain struct so every
// row of the table is a one-line test
type turnState struct {
	intent     string
	hint       string
	depth      string
	asked      bool // a strong ask, or the analyser's wants_advice yes, this turn
	softAsked  bool // an implicit ask this turn or earlier on this topic
	listening  bool // listen_only this session, or they said no advice this turn
	evidence   bool // they asked about research in words
	topicTurns int  // turns on the active topic, this one included
}

// chooseStance is the decision table, first matching row wins:
//
//	greeting                                      -> greet
//	off_topic                                     -> off_topic
//	ask_evidence, hint inform, or a research ask  -> inform
//	a strong ask ("i need help", "how do i")      -> advise, lifting listen-only for the turn
//	listen-only or "no advice"                    -> listen
//	an implicit ask on a topic's 2nd+ turn        -> advise
//	vent, hint listen, or a heavy message         -> listen
//	a topic on its 3rd+ turn                      -> advise
//	anything else                                 -> explore
//
// an implicit ask on a topic's first turn explores rather than advises: "I'm
// having trouble finding motivation" is a problem named, not yet understood,
// and advice before one question reads as not listening. it still counts on
// the next turn, so the thread reaches advice a turn early
func chooseStance(s turnState) Stance {
	switch {
	case s.intent == "greeting":
		return StanceGreet
	case s.intent == "off_topic":
		return StanceOffTopic
	case s.intent == "ask_evidence" || s.hint == string(StanceInform) || s.evidence:
		return StanceInform
	case s.asked:
		return StanceAdvise
	case s.listening:
		return StanceListen
	case s.softAsked && s.topicTurns >= 2:
		return StanceAdvise
	case s.intent == "vent" || s.hint == string(StanceListen) || s.depth == "heavy":
		return StanceListen
	case s.topicTurns >= 3:
		return StanceAdvise
	default:
		return StanceExplore
	}
}
