package agent

import (
	"strconv"
	"strings"
)

// Shape is the form of one reply, decided in Go before the model writes. the
// model gets one instruction line per field. nothing here limits tokens: a
// reply that runs long is caught by the lint and revised, never cut off
type Shape struct {
	Stance Stance
	Band   Band // words, already mirrored to the person
	Ask    Ask  // question cadence for this reply
	Advice Advice
	Cite   CiteMode
}

// Ask is the question cadence. the zero value forbids, so a Shape built by
// hand never asks by accident
type Ask int

const (
	AskForbid Ask = iota // no question at all
	AskAllow             // a question is optional, at most one
	AskInvite            // end with one easy question, if it feels natural
)

func (a Ask) String() string {
	switch a {
	case AskAllow:
		return "allow"
	case AskInvite:
		return "invite"
	default:
		return "forbid"
	}
}

// Advice is how far a reply may go towards suggesting things. the zero value
// is none, for the same reason Ask's is forbid
type Advice int

const (
	AdviceNone  Advice = iota // no suggestions or next steps
	AdviceOffer               // none yet; ask whether they would like a couple of ideas
	AdviceAllow               // fine when it follows from the research, not required
	AdviceGive                // one to three small, concrete things
)

func (a Advice) String() string {
	return [...]string{"none", "offer", "allow", "give"}[a]
}

// CiteMode is how research reaches the reply. markers put [n] in the prose and
// the studies chip under it; words lets the model use what the excerpts say
// in plain language with no markers, which is how a person who has read the
// research talks when nobody asked for sources
type CiteMode int

const (
	CiteNone    CiteMode = iota // no excerpts, or none wanted
	CiteWords                   // draw on the excerpts in plain words, no markers
	CiteMarkers                 // inline [n] markers, verified and renumbered
)

func (c CiteMode) String() string {
	return [...]string{"none", "words", "markers"}[c]
}

// shapeFor is the stance's band mirrored to the person. cadence, advice and
// citations need the session and the research, so Decide and Handle set them
func shapeFor(st Stance, userWords int) Shape {
	b, ok := bands[st]
	if !ok {
		b = bands[StanceExplore]
	}
	return Shape{Stance: st, Band: mirror(b, userWords)}
}

// wordsPerSentence turns a band into the sentence counts the model is given.
// models count sentences far better than words, and "about 60 words" was read
// as a limit to stop at rather than a size to aim for
const wordsPerSentence = 15

// Sentences is the band as "usually lo to hi sentences"
func (b Band) Sentences() (lo, hi int) {
	lo = max(1, (b.Min+wordsPerSentence/2)/wordsPerSentence)
	hi = max(lo+1, (b.Max+wordsPerSentence/2)/wordsPerSentence)
	return lo, hi
}

func (s Shape) Instructions() string {
	var b strings.Builder
	b.WriteString("THIS REPLY\n")
	lo, hi := s.Band.Sentences()
	b.WriteString("Length: usually " + strconv.Itoa(lo) + " to " + strconv.Itoa(hi) + " sentences. Always finish your last sentence.\n")
	if s.Band.Max > 150 {
		b.WriteString("Break it into two or three short paragraphs.\n")
	}
	switch {
	case s.Advice == AdviceOffer:
		b.WriteString("End by asking, in your own words, whether they'd like a couple of ideas or would rather keep talking it through. That is your only question.\n")
	case s.Ask == AskInvite:
		b.WriteString("End with one easy, open question about them, if it feels natural.\n")
	case s.Ask == AskAllow:
		b.WriteString("A question is optional; at most one.\n")
	default:
		b.WriteString("Do not ask a question.\n")
	}
	switch s.Advice {
	case AdviceGive:
		b.WriteString("Offer one to three small, concrete things they could try this week, each tied to something they said and within their control. Offer them, don't prescribe, and invite them to say which fits, if any.\n")
	case AdviceAllow:
		b.WriteString("A practical point is fine if it follows from the research; keep it small.\n")
	default:
		b.WriteString("Do not give advice or next steps.\n")
	}
	switch s.Cite {
	case CiteMarkers:
		b.WriteString("When a sentence relies on a research excerpt, end it with the excerpt's number in square brackets, like [2]. Use only numbers from RESEARCH CONTEXT.\n")
	case CiteWords:
		b.WriteString("Where RESEARCH CONTEXT helps, use it in plain words, the way a friend who has read it would (\"there's decent research that...\"). No [n] markers, no titles or authors.\n")
	default:
		b.WriteString("Do not use [n] markers.\n")
	}
	return b.String()
}

// askedQuestion reads an earlier reply for the cadence window
func askedQuestion(reply string) bool { return strings.Contains(reply, "?") }

// inviteStances are where a question helps someone keep talking. advice and
// evidence replies already carry the substance; a question there is an
// interview
var inviteStances = map[Stance]bool{StanceListen: true, StanceExplore: true, StanceGreet: true}

// cadence decides the question rule for one reply. asked is whether each
// earlier assistant reply in the window asked, oldest first
func cadence(st Stance, userWords int, userAsked, listening bool, asked []bool, quiet int) Ask {
	n := len(asked)
	if (n > 0 && asked[n-1]) || userAsked || st == StanceInform || st == StanceOffTopic {
		return AskForbid
	}
	if !inviteStances[st] || listening {
		return AskAllow
	}
	for _, a := range asked[max(0, n-quiet):] {
		if a {
			return AskAllow
		}
	}
	// short messages are where a person most needs a hand to keep talking,
	// and early in a session a question is how a companion gets to know them
	if userWords < 15 || n < 3 {
		return AskInvite
	}
	return AskAllow
}

// questionWords open a message that asks us something even without a "?"
var questionWords = map[string]bool{
	"what": true, "how": true, "why": true, "who": true, "are": true, "is": true,
	"do": true, "does": true, "can": true, "could": true, "would": true, "will": true, "should": true,
}

// userAsked is true when the person put a question to us. we answer it
// rather than handing back one of our own. "when am I supposed to..." in a
// vent is rhetorical, so "when" and "where" are left out on purpose
func userAsked(message string) bool {
	if strings.Contains(message, "?") {
		return true
	}
	f := strings.Fields(strings.ToLower(message))
	return len(f) > 1 && questionWords[strings.Trim(f[0], ".,!'\"")]
}
