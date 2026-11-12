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


def words(s: str) -> list[str]:
    return s.split()


def sentences(s: str) -> list[str]:
    return [x for x in _SENT.split(s.strip()) if x]


def opener(s: str) -> str:
    # first three lowercased words, exactly as lint.go's opener()
    return " ".join(s.lower().split()[:3])


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
