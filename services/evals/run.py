"""replay every scenario through the real agent and write one jsonl row per turn.

    API_URL=http://localhost:8080 python run.py [--variant curious] [--only vent_monday,...]

needs the api in ENV=development (it registers POST /api/v1/dev/turn only there).
each scenario gets a fresh scratch user, so facts and trajectory never leak between runs.
"""
import argparse
import json
import os
import sys
from concurrent.futures import ThreadPoolExecutor

import httpx
import yaml

from common import HERE, RUNS, git_sha

API = os.environ.get("API_URL", "http://localhost:8080").rstrip("/")


def play(client: httpx.Client, sc: dict, variant: str | None) -> list[dict]:
    rows, session = [], ""
    subjects = [f["subject"] for f in sc.get("facts", [])]
    for i, msg in enumerate(sc["turns"], start=1):
        body = {"persona": sc["id"], "message": msg, "sessionId": session}
        if not session:  # only the first turn seeds; later turns continue the stored session
            body["facts"] = sc.get("facts", [])
            if variant:
                body["variant"] = variant
        try:
            r = client.post(f"{API}/api/v1/dev/turn", json=body)
            r.raise_for_status()
            out = r.json()
        except httpx.HTTPError as e:
            # a failed turn is data, not a crash: the gate counts it
            out = {"error": str(e), "reply": "", "crisis": False, "sources": [], "run": None}
        session = out.get("sessionId", session)
        rows.append({
            "scenario": sc["id"], "kind": sc["kind"], "expect": sc["expect"],
            "crisis_from": sc.get("crisis_from"), "listen_from": sc.get("listen_from"), "turn": i, "user": msg,
            "users_so_far": sc["turns"][:i], "subjects": subjects,
            "reply": out.get("reply", ""), "crisis": bool(out.get("crisis")),
            "cited": len(out.get("sources") or []), "run": out.get("run"),
            "error": out.get("error"),
        })
        if out.get("error") and not session:
            break  # no session to continue
    return rows


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--variant", help="pin a voice variant for every scenario")
    ap.add_argument("--only", help="comma separated scenario ids")
    ap.add_argument("--workers", type=int, default=4)  # parallel scenarios; turns within one stay ordered
    ap.add_argument("--out", help="override output path")
    a = ap.parse_args()

    scenarios = yaml.safe_load((HERE / "scenarios.yaml").read_text())["scenarios"]
    if a.only:
        keep = set(a.only.split(","))
        scenarios = [s for s in scenarios if s["id"] in keep]

    RUNS.mkdir(exist_ok=True)
    tag = git_sha() + (f"-{a.variant}" if a.variant else "")
    out = a.out or RUNS / f"{tag}.jsonl"

    with httpx.Client(timeout=90) as client, ThreadPoolExecutor(a.workers) as pool:
        results = list(pool.map(lambda s: play(client, s, a.variant), scenarios))

    rows = [r for rs in results for r in rs]
    with open(out, "w", encoding="utf-8") as f:
        for r in rows:
            f.write(json.dumps(r, ensure_ascii=False) + "\n")
    errors = sum(1 for r in rows if r["error"])
    print(f"{len(rows)} turns from {len(scenarios)} scenarios -> {out} ({errors} errors)")
    return 1 if errors == len(rows) else 0


if __name__ == "__main__":
    sys.exit(main())
