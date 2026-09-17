// Package safety is the pre-filter that runs BEFORE the LLM and before any
// learning pipeline touches a message.
//
// Design principle: this is a router, not a therapist. Its only job is to
// decide "does this need a human rather than an agent?" and, if so, hand back
// fixed, human-written text. It never improvises, never calls the LLM, and
// nothing it flags enters retrieval or memory.
//
// Two layers, deliberately:
//  1. Rules -- deterministic, auditable, zero latency, no dependency.
//  2. Model -- a small trained classifier for phrasing the rules miss.
//
// Either firing is enough. False positives are cheap: a person sees a resource
// card. False negatives are not. Tuned accordingly, and never the other way.
package safety

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Level int

const (
	LevelOK Level = iota
	LevelCrisis
)

type Result struct {
	Level       Level
	MatchedRule string
	Score       float64
}

func (r Result) IsCrisis() bool { return r.Level == LevelCrisis }

// Each rule carries a name as well as a pattern so a flagged message is
// explainable in the audit table. "Why was I shown a helpline?" deserves a
// better answer than "a regex somewhere".
var rules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"self_harm_intent", regexp.MustCompile(`(?i)\b(kill|hurt|harm)\s+(myself|me)\b`)},
	{"suicidal_ideation", regexp.MustCompile(`(?i)\b(suicidal|suicide|end my life|take my (own )?life)\b`)},
	{"no_reason_to_live", regexp.MustCompile(`(?i)\b(no reason to (live|go on)|don'?t want to (live|be here|wake up)|better off (dead|without me))\b`)},
	{"self_injury", regexp.MustCompile(`(?i)\b(cutting myself|self[- ]harm(ing)?|overdose)\b`)},
	{"harm_to_others", regexp.MustCompile(`(?i)\b(kill|hurt)\s+(him|her|them|someone|everyone)\b`)},
}

// Classifier is the optional ML layer. The interface lives here, in the
// consumer, not next to the implementation -- that is idiomatic Go, and it is
// what lets the test file define a two-line stub with no imports.
type Classifier interface {
	Score(ctx context.Context, text string) (float64, error)
}

type Filter struct {
	clf       Classifier
	threshold float64
}

// A nil Classifier is valid and means "rules only". Making the zero value
// useful is worth more than a constructor that returns an error.
func NewFilter(clf Classifier, threshold float64) *Filter {
	return &Filter{clf: clf, threshold: threshold}
}

func (f *Filter) Check(ctx context.Context, text string) Result {
	for _, r := range rules {
		if r.re.MatchString(text) {
			return Result{Level: LevelCrisis, MatchedRule: r.name, Score: 1.0}
		}
	}
	if f.clf == nil {
		return Result{Level: LevelOK}
	}
	score, err := f.clf.Score(ctx, text)
	if err != nil {
		// Availability over strictness. If the classifier is down, the rules
		// still stand and chat keeps working. Failing closed here would take
		// the whole product offline every time a Python container restarts.
		return Result{Level: LevelOK, MatchedRule: "classifier_unavailable"}
	}
	if score >= f.threshold {
		return Result{Level: LevelCrisis, MatchedRule: "classifier", Score: score}
	}
	return Result{Level: LevelOK, Score: score}
}

// CrisisResponse is fixed text. Never generated, never varied, never A/B
// tested. Kenyan numbers first, because that is where the beta users are.
const CrisisResponse = `It sounds like you may be going through something very heavy right now, and I want to be honest with you: Astralis is a research and reflection tool, not a crisis service, and I am not the right support for this moment.

Please reach out to someone who can help properly:

**Kenya**
- Befrienders Kenya — +254 722 178 177
- Kenya Red Cross counselling — toll free 1199
- Emergency services — 999 or 112

**International**
- findahelpline.com lists free, confidential helplines by country

If you are in immediate danger, please contact emergency services or go to the nearest hospital.

If you would like, tell someone you trust today — a friend, family member, or your doctor. I will still be here afterwards to talk about work stress and burnout whenever you want.`

// ------------------------------------------------------------ HTTP classifier

type HTTPClassifier struct {
	url  string
	http *http.Client
}

// Returns nil -- not an error -- when unconfigured, so main can write:
//   var clf safety.Classifier
//   if h := safety.NewHTTPClassifier(url); h != nil { clf = h }
// and the filter degrades to rules-only with no branching anywhere else.
func NewHTTPClassifier(url string) *HTTPClassifier {
	if url == "" {
		return nil
	}
	return &HTTPClassifier{
		url:  strings.TrimRight(url, "/"),
		http: &http.Client{Timeout: 3 * time.Second}, // short: this is in the hot path
	}
}

func (h *HTTPClassifier) Score(ctx context.Context, text string) (float64, error) {
	body, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.url+"/score", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := h.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	var out struct {
		Score float64 `json:"score"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Score, nil
}