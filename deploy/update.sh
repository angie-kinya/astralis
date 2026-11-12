#!/usr/bin/env bash
# Roll the droplet forward to a new build. Called by CI over SSH, or by hand:
#   TAG=<git sha> ./deploy/update.sh      (TAG defaults to latest)
set -euo pipefail
cd "$(dirname "$0")/.."

git pull --ff-only            # compose file, Caddyfile, migrations, scripts
cd deploy
export TAG="${TAG:-latest}"

docker compose pull --quiet
# --wait blocks until the healthcheck passes. without it, a recreated postgres
# (new image pulled) is "started" but not yet accepting connections, and the
# first migration fails with a socket error
docker compose up -d --wait postgres
./migrate.sh                 # schema before code: new code may need new columns
docker compose up -d --remove-orphans
# the Caddyfile is a bind mount, so `up` never notices it changed. a reload is
# graceful (no dropped connections) and a no-op when nothing changed
docker compose exec -T caddy caddy reload --config /etc/caddy/Caddyfile >/dev/null 2>&1 \
  || echo "warn: caddy reload failed; the old config keeps serving"
docker image prune -f >/dev/null

# Wait for the API to come back rather than declaring success on faith.
for i in $(seq 1 30); do
  if docker compose exec -T api wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
    echo "deploy ok: TAG=$TAG"; exit 0
  fi
  sleep 2
done
echo "API did not become healthy"; docker compose logs --tail 50 api; exit 1
