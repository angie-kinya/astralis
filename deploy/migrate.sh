#!/usr/bin/env bash
# Apply every migration, in order. Safe to run on every deploy: each file is
# written to be idempotent (IF NOT EXISTS, guarded constraint creation), and CI
# proves it by applying them all twice to a clean database.
set -euo pipefail
cd "$(dirname "$0")"

for f in ../db/migrations/*.sql; do
  echo "migrate: $(basename "$f")"
  # Capture first, then filter. Piping psql straight into grep would make the
  # loop's success depend on grep's exit code, not on whether the SQL worked.
  if ! out=$(docker compose exec -T postgres sh -c \
        'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -q' < "$f" 2>&1); then
    echo "$out"
    echo "migration failed: $f" >&2
    exit 1
  fi
  echo "$out" | grep -v NOTICE || true
done
