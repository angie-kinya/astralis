# Astralis

A research-grounded reflection companion for occupational burnout.

> **Astralis is not therapy, diagnosis, or a crisis service.** It is an experimental
> personal project in closed beta. Messages suggesting crisis are intercepted before
> they reach the language model and answered with fixed signposting to real support.

## What it actually does

- Talks about work stress, grounding what it says in published research it retrieves itself
- Runs a scheduled job that pulls papers from OpenAlex, chunks and embeds them into pgvector
- Keeps a short summary of each session so it has continuity across conversations
- Learns from thumbs up/down by **reweighting which research gets retrieved** — no model
  is ever fine-tuned on anyone's words

That last point is the architectural thesis of Phase 1. "Self-improving" here means a
growing, auditable, deletable **library** — not growing weights. See `docs/` and the
build guide for why.

## Stack

| Layer | Choice | Why |
|---|---|---|
| API + worker | Go 1.26, Gin, Asynq | one language for all orchestration; two binaries, one image |
| Data | PostgreSQL 16 + pgvector | relational and vector in one engine — one thing to operate |
| Queue | Redis | Asynq's backing store; retries and a dead-letter queue for free |
| Ingestion | Python 3.12 (OpenAlex → chunk → embed) | Python where Python is strongest |
| Safety classifier | Python, scikit-learn, FastAPI | separate deploy cadence; the Go side degrades if it is down |
| Frontend | Next.js 14 App Router, Tailwind, Auth.js | Auth.js owns login, Go owns the API |
| CI/CD | GitHub Actions → Render | blueprint in `render.yaml`, nothing configured by hand |

## Layout

```
db/migrations/        numbered, forward-only SQL
services/gateway/     Go: HTTP API (cmd/api) + background worker (cmd/worker)
services/research/    Python: the self-research ingestion job
services/safety-classifier/  Python: crisis classifier behind FastAPI
web/                  Next.js frontend
.github/workflows/    test on every PR, deploy on green main
```

## Quick start

```bash
cp .env.example .env                 # fill in your keys
make up && make migrate              # Postgres + Redis, then schema
make classifier &                    # :8000  (needs `python train.py` once)
make api &                           # :8080
make worker &
make research                        # populate the knowledge base
cd web && cp .env.local.example .env.local && npm install && npm run dev   # :3000
```

`make check` runs exactly what CI runs. Run it before you push.

## Safety and data

- A rules layer plus an optional classifier runs **before** the LLM. Either firing is
  enough. Flagged messages never reach Claude, never enter retrieval, and never enter
  the session summary.
- Consent is enforced at the routing layer (`requireConsent` middleware), not by handler
  discipline.
- `DELETE /api/v1/me` is real erasure; `ON DELETE CASCADE` takes every session, message,
  summary, rating and crisis flag with the user row.
- Crisis flags are stored for human review of filter accuracy. Review them.

## Licence

See [LICENSE](./LICENSE).
