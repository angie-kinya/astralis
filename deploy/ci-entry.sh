#!/usr/bin/env bash
# Forced command for the CI deploy key.
#
# In /home/deploy/.ssh/authorized_keys the CI key is installed as:
#   restrict,command="/opt/astralis/deploy/ci-entry.sh" ssh-ed25519 AAAA... github-actions
#
# `restrict` removes port forwarding, agent forwarding and a TTY; `command=`
# means that whatever CI asks to run, THIS script runs instead. The only
# request it honours is "deploy this exact 40-character commit". A leaked CI
# key can therefore redeploy a commit you already built — and nothing else.
set -euo pipefail

cmd="${SSH_ORIGINAL_COMMAND:-}"
if [[ "$cmd" =~ ^TAG=([0-9a-f]{40})\ /opt/astralis/deploy/update\.sh$ ]]; then
  TAG="${BASH_REMATCH[1]}" exec /opt/astralis/deploy/update.sh
fi
echo "refused: only 'TAG=<sha> /opt/astralis/deploy/update.sh' is allowed" >&2
exit 1
