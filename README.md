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
| API + jobs | Go 1.26, Gin | one language for all orchestration; api, summarise and announce ship in one image |
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
web/                         Next.js frontend
deploy/                      production compose, Caddyfile, update/migrate/backup scripts, crontab
scripts/smoke.sh             41-check end-to-end test
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

Deployment: see the runbook, and `deploy/`.

## Safety and data

- A rules layer plus an optional classifier runs **before** the LLM. Either firing is
  enough. Flagged messages never reach Claude, never enter retrieval, and never enter
  the session summary.
- Consent is enforced at the routing layer (`requireConsent` middleware), not by handler
  discipline.
- `DELETE /api/v1/me` is real erasure; `ON DELETE CASCADE` takes the email address, every
  sign-in session, message, summary, rating, note and crisis flag with the user row.
- A user's email is returned only by `/me`, to its owner. No peer-facing payload contains it.
- Crisis flags are stored for human review of filter accuracy. Review them.

## Licence

See [LICENSE](./LICENSE).
