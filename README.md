# Astralis

A quiet place to think about work stress and burnout.

**Live:** https://astralis-lab.dev

> **Astralis is not therapy, diagnosis, or a crisis service.** It is an independent
> research project. Messages suggesting crisis are intercepted before they reach any
> model and answered with a standard response pointing to real support.

## What it does

- **Conversations** grounded in published research when it helps, with citations checked
  against what was actually retrieved
- **Memory you can see:** short dated facts (people, events, things you're trying) recalled
  when relevant, listed in Settings under *What Astralis remembers*, each deletable
- **Circle:** optional, anonymous peer messaging in real time; delete, block and report;
  only your chosen name is ever shown
- **Constellations:** a three-minute, turn-based game (dots and boxes on a 4×4 star field),
  behind `GAMES_ENABLED`. Play someone from the Circle, play Astralis (a deterministic Go
  opponent, no model), or play solo on one device. The leaderboard counts only games
  between people, is opt-in, resets every Monday and gives points for finishing as well as
  winning; no streaks, no loss counts, and the agent never mentions it
- **Practices, reading and helplines**, and a private **notepad** no model ever reads
- **Sign-in by email code** only; your email is never visible to other users

## How it works

Each message runs through an agent in the Go orchestrator (`services/gateway/internal/agent`):

```
guard → remember → analyse → plan → act → respond → verify → record
```

The rule for v2: **the prompt says who Astralis is; Go decides how it behaves.**

| Stage | What happens | Where |
|---|---|---|
| guard | rules + classifier; a crisis never reaches a model and is answered with fixed text | `safety` |
| remember | in parallel (errgroup): analysis, fact recall, 21-day trajectory, last 2 summaries | `agent.go` |
| analyse | small model: intent, JD-R burnout signals, search query | `analyse.go` |
| plan | pure Go: retrieval, a **shape** (word budget, may ask / advise / cite), at most one "noticing" move per kind per session | `policy.go`, `shape.go` |
| act | pgvector search over the research library, reusing the turn's embedding | `rag` |
| respond | larger model writes within the shape; `max_tokens` follows the word budget | `character.go` |
| verify | citations checked; a deterministic style linter (length, questions, stock phrases, repeated openers) triggers **one** repair pass | `lint.go` |
| record | `agent_runs` (per-stage ms, tokens, variant, violations, repaired), `turn_signals` | `store` |

- **Person model.** Every 20 minutes `cmd/summarise` compresses idle sessions and extracts
  durable facts with structured output into `person_facts` (one row per subject; repeats
  raise `mentions`). Flagged messages never reach the extractor.
- **Trajectory.** `turn_signals` is finally read back: 7- vs 21-day exhaustion and recurring
  demands let Astralis say "this is the third week the manager has come up", once.
- **Variants.** Two voices that differ in one dimension (terseness), assigned per session by
  Thompson sampling on ratings and the end-of-session card.
- **Evals.** `services/evals` replays 30 scripted scenarios and gates agent changes on
  safety routing, stylometric AI-tell metrics and a pairwise judge (`make eval`).

No model holds a key, calls a tool or touches the database, and none is trained on
anyone's words. Astralis improves because its research library grows weekly (OpenAlex),
ratings reweight what gets retrieved, and evals show whether a change to the voice helped.

**Real time.** `GET /api/v1/events` is one Server-Sent Events stream per tab, fed by an
in-memory hub (`internal/realtime`): Circle messages, deletions and read receipts, and the
agent's truthful progress ("reading 2 studies"). Replies are not token-streamed on purpose:
the verifier has to see the whole reply before anyone does. One API container makes the
in-memory hub enough; a second one would back it with Postgres `LISTEN/NOTIFY`.

**Constellations.** The rules are a pure, value-typed engine (`internal/game`) with table tests
and a fuzzer (`go test -fuzz=FuzzRandomGames ./internal/game`) proving every legal game ends
with all nine boxes owned. The server is authoritative: the client sends only an edge number,
and each move is saved under an optimistic lock on `games.version`, so two tabs or two fast
clicks can never both land (the loser gets `409` and refetches). Invites check opt-in and
blocks in the same statement as messages; one pending invite per pair is a unique index.

## Stack

| Layer | Choice | Why |
|---|---|---|
| API, agent + jobs | Go 1.26, Gin | one language for all orchestration; api, summarise and announce ship in one image |
| Models | Anthropic (analyser + reply), OpenAI embeddings | called only from Go; keys never leave the server |
| Sign-in | email + 6-digit code, owned by Go | no third-party account needed; sessions are revocable database rows |
| Data | PostgreSQL 16 + pgvector | relational and vector in one engine |
| Email | Resend (HTTPS API) | DigitalOcean blocks outbound SMTP on droplets |
| Ingestion | Python 3.12 (OpenAlex → chunk → embed) | Python where Python is strongest |
| Safety classifier | Python, scikit-learn, FastAPI | separate deploy cadence; Go degrades to rules if it is down |
| Frontend | Next.js 14 App Router, Tailwind | JetBrains Mono + Sora |
| Hosting | one DigitalOcean droplet, Docker Compose, Caddy | automatic TLS; one origin, so no CORS |
| CI/CD | GitHub Actions → ghcr.io → SSH | images built in CI, never on the droplet |

## Layout

```
db/migrations/               numbered, idempotent SQL
services/gateway/            Go: cmd/api, cmd/summarise, cmd/announce, cmd/worker
services/research/           Python: the self-research ingestion job
services/safety-classifier/  Python: crisis classifier behind FastAPI
services/evals/              Python: offline agent evals (scenarios, metrics, pairwise judge, gate)
web/                         Next.js frontend
deploy/                      production compose, Caddyfile, update/migrate/backup scripts, crontab
scripts/smoke.sh             end-to-end smoke test
```

## Quick start

```bash
cp .env.example .env                 # fill in ANTHROPIC_API_KEY and OPENAI_API_KEY
make up && make migrate
make api                             # :8080 — sign-in codes print here locally
make classifier                      # :8000 (run `python train.py` once first)
make research                        # populate the knowledge base
cd web && cp .env.local.example .env.local && npm install && npm run dev   # :3000
```

`make smoke-api` + `make smoke` runs the end-to-end test. `make check` runs what CI runs.

Deployment: `deploy/` (one droplet, Docker Compose, Caddy). CI deploys every green push to `main`.

## Safety and data

- A rules layer plus an optional classifier runs **before** the LLM. Either firing is
  enough. Flagged messages never reach a model, never enter retrieval, and never enter
  the session summary.
- Consent is enforced at the routing layer (`requireConsent` middleware) and recorded
  with its date; changing the terms (`TermsUpdatedAt`) asks everyone again.
- `DELETE /api/v1/me` is real erasure; `ON DELETE CASCADE` takes the email address, every
  sign-in session, message, summary, remembered fact, rating, note, block, report and
  crisis flag with the user row.
- Remembered facts are visible and deletable one by one in Settings. A Circle report keeps
  a snapshot of the reported message for moderation, so deleting it cannot erase evidence.
- A user's email is returned only by `/me`, to its owner. No peer-facing payload contains it.
- Crisis flags are stored for human review of filter accuracy. Review them.

## Licence

See [LICENSE](./LICENSE).
