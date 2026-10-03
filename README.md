# Astralis

A quiet place to think about work stress and burnout.

**Live:** https://astralis-lab.dev

> **Astralis is not therapy, diagnosis, or a crisis service.** It is an independent
> research project. Messages suggesting crisis are intercepted before they reach any
> model and answered with a standard response pointing to real support.

## What it does

- **Conversations** grounded in published research when it helps, with citations checked
  against what was actually retrieved
- **Continuity:** a short summary of each session, so you don't start from zero
- **Circle:** optional, anonymous peer messaging; only your chosen name is ever shown
- **Practices, reading and helplines**, and a private **notepad** no model ever reads
- **Sign-in by email code** only; your email is never visible to other users

## How it works

Each message runs through an agent in the Go orchestrator (`services/gateway/internal/agent`):

```
guard → remember → analyse → plan → act → respond → verify → record
```

A rules-plus-classifier filter runs first. A small model analyses the message (intent,
burnout signals), plain Go decides the plan (including whether to search the research),
a larger model writes the reply, and invalid citations are stripped. Every turn is traced
in `agent_runs`; structured signals go to `turn_signals`.

No model holds a key, calls a tool or touches the database, and none is trained on
anyone's words. Astralis improves because its research library grows weekly (OpenAlex)
and ratings reweight what gets retrieved.

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
  sign-in session, message, summary, rating, note and crisis flag with the user row.
- A user's email is returned only by `/me`, to its owner. No peer-facing payload contains it.
- Crisis flags are stored for human review of filter accuracy. Review them.

## Licence

See [LICENSE](./LICENSE).
