#!/usr/bin/env bash
# Nightly logical backup. pg_dump's custom format (-Fc) is compressed and lets
# you restore selectively with pg_restore.
#
# This protects against "I dropped a table". It does NOT protect against losing
# the droplet — for that, enable DigitalOcean droplet backups, or copy these
# files off the machine. A backup that lives only next to the thing it backs up
# is half a backup.
set -euo pipefail
cd "$(dirname "$0")"

DEST="${BACKUP_DIR:-$HOME/backups}"
mkdir -p "$DEST"
chmod 700 "$DEST"   # these files contain every conversation in plain text

FILE="$DEST/astralis-$(date +%F).dump"
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$FILE"
chmod 600 "$FILE"

# Keep two weeks. Erasure requests are honoured in backups by expiry: a
# deleted person's data is gone from the last dump 14 days after deletion.
find "$DEST" -name 'astralis-*.dump' -mtime +14 -delete
echo "backup ok: $FILE ($(du -h "$FILE" | cut -f1))"
