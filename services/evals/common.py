"""shared helpers for the eval harness. nothing here calls a model or the network."""
import json
import os
import re
import subprocess
from pathlib import Path

HERE = Path(__file__).resolve().parent
RUNS = HERE / "runs"
JUDGED = HERE / "judged"
RESULTS = Path(os.environ.get("EVAL_RESULTS", HERE / "results.csv"))  # override in tests

# keep in step with services/gateway/internal/agent/lint.go: same phrases, same dash rule,
# so the offline number and the production style_violations column mean the same thing
STOCK = re.compile(
    r"(?i)\b(it sounds like|i hear you|that'?s (completely |totally )?(valid|understandable)|"
    r"it'?s (important|okay) to|you'?re not alone|great question|i'?m here for you|"
    r"take care of yourself|be gentle with yourself|that must be (so )?(hard|difficult))\b"
)
DASH = re.compile(r" [–-] |—")
_SENT = re.compile(r"(?<=[.!?])\s+")

# lint.go's jargon list and plain_words rule, word for word
JARGON = re.compile(
    r"(?i)\b(mismatch(es)?|unsustainable|leverag(e|es|ed|ing)|paradigms?|"
    r"optimi[sz](e|es|ed|ing|ation)|frameworks?|dressed up as|cognitive|holistic(ally)?|actionable|"
    r"(set|setting|hold|holding|healthy|firm|clear) boundaries|hold(ing)? space)\b"
)
_LETTERS = re.compile(r"[a-z]+")
_VOWELS = re.compile(r"[aeiouy]+")

# shape.go's userAsked: a "?" or a leading question word. "when" and "where" are left
# out because "when am I supposed to..." in a vent is rhetorical
_QWORDS = {"what", "how", "why", "who", "are", "is", "do", "does", "can", "could", "would", "will", "should"}


def words(s: str) -> list[str]:
    return s.split()


def sentences(s: str) -> list[str]:
    return [x for x in _SENT.split(s.strip()) if x]


def opener(s: str) -> str:
    # first three lowercased words, exactly as lint.go's opener()
    return " ".join(s.lower().split()[:3])


def letter_words(s: str) -> list[str]:
    # the word list lint.go counts syllables over: runs of a-z in the lowercased text
    return _LETTERS.findall(s.lower())


def syllables(word: str) -> int:
    # vowel groups minus a silent final e, exactly as lint.go's syllables()
    n = len(_VOWELS.findall(word))
    if n > 1 and word.endswith("e") and not word.endswith("le"):
        n -= 1
    return max(1, n)


def long_words(s: str) -> tuple[int, int]:
    ws = letter_words(s)
    return sum(syllables(w) >= 4 for w in ws), len(ws)


def plain_words(s: str) -> bool:
    # false when a reply leans on long or technical words; one long word is never enough
    long, total = long_words(s)
    return not JARGON.search(s) and (long < 2 or long * 100 <= total * 8)


def user_asked(s: str) -> bool:
    if "?" in s:
        return True
    f = s.lower().split()
    return len(f) > 1 and f[0].strip(".,!'\"") in _QWORDS


def load_jsonl(path: str | Path) -> list[dict]:
    with open(path, encoding="utf-8") as f:
        return [json.loads(line) for line in f if line.strip()]


def git_sha() -> str:
    if sha := os.environ.get("GIT_SHA"):
        return sha[:12]
    try:
        out = subprocess.run(["git", "rev-parse", "--short=12", "HEAD"], cwd=HERE,
                             capture_output=True, text=True, check=True)
        return out.stdout.strip()
    except (OSError, subprocess.CalledProcessError):
        return "unknown"
