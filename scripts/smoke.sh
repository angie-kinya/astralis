#!/usr/bin/env bash
# Astralis end-to-end smoke test.
#
# Exercises every guarantee the system claims to make, against a running API.
# It signs in through the real email-code flow, reading the code from the
# `devCode` field the API returns ONLY when started with AUTH_DEV_ECHO_CODE=true
# — which config refuses in production. So run it against a local API:
#
#   AUTH_DEV_ECHO_CODE=true AUTH_RATE_LIMIT_PER_MIN=500 make api     # terminal 1
#   ./scripts/smoke.sh                                               # terminal 2
#
# Against production, use ./scripts/smoke.sh --public (unauthenticated checks only).
#
# Creates users with @smoke.local addresses and deletes them at the end.
set -uo pipefail

API="${API:-http://localhost:8080}"
U="$API/api/v1"
PASS=0; FAIL=0
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
J="Content-Type: application/json"

check() {
  if [[ "$3" == *"$2"* ]]; then printf '  \033[32m✓\033[0m %s\n' "$1"; PASS=$((PASS+1))
  else printf '  \033[31m✗\033[0m %s\n      expected: %s\n      actual:   %s\n' "$1" "$2" "${3:0:300}"; FAIL=$((FAIL+1)); fi
}
absent() {  # check that $2 does NOT occur in $3
  if [[ "$3" != *"$2"* ]]; then printf '  \033[32m✓\033[0m %s\n' "$1"; PASS=$((PASS+1))
  else printf '  \033[31m✗\033[0m %s\n      leaked: %s\n' "$1" "$2"; FAIL=$((FAIL+1)); fi
}
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
sec()  { printf '\n\033[1m%s\033[0m\n' "$1"; }
field(){ python3 -c "import sys,json;print(json.load(sys.stdin).get('$1',''))"; }

# signin <jar> <email> [notify] -> signs in through the real two-step flow
signin() {
  local dev
  dev=$(curl -s -X POST -H "$J" -d "{\"email\":\"$2\"}" "$U/auth/start" | field devCode)
  if [ -z "$dev" ]; then
    echo "  no devCode returned — start the API with AUTH_DEV_ECHO_CODE=true (development only)"; exit 1
  fi
  curl -s -c "$1" -X POST -H "$J" \
    -d "{\"email\":\"$2\",\"code\":\"$dev\",\"notifyReleases\":${3:-false}}" "$U/auth/verify"
}

sec "health"
check "healthz responds" '"status":"ok"' "$(curl -s -m 5 "$API/healthz")"
check "no cookie is rejected"      "401" "$(code "$U/me")"
check "forged cookie is rejected"  "401" "$(code -b 'astralis_session=forged' "$U/me")"
check "forged cookie is cleared"   "Max-Age=0" "$(curl -s -D - -o /dev/null -b 'astralis_session=forged' "$U/me")"
[ "${1:-}" = "--public" ] && { printf '\n\033[1m%d passed, %d failed\033[0m\n' "$PASS" "$FAIL"; [ "$FAIL" -eq 0 ]; exit; }

sec "email validation"
check "garbage email refused"      "400" "$(code -X POST -H "$J" -d '{"email":"not-an-email"}' "$U/auth/start")"
check "display-name form refused"  "400" "$(code -X POST -H "$J" -d '{"email":"Eve <eve@smoke.local>"}' "$U/auth/start")"

sec "one-time codes"
curl -s -X POST -H "$J" -d '{"email":"guess@smoke.local"}' "$U/auth/start" > "$TMP/g"
REAL=$(field devCode < "$TMP/g")
check "code issued" "sent" "$(cat "$TMP/g")"
WRONG=$(printf '%06d' $(( (10#$REAL + 1) % 1000000 )))
check "wrong code refused" "invalid_code" \
  "$(curl -s -X POST -H "$J" -d "{\"email\":\"guess@smoke.local\",\"code\":\"$WRONG\"}" "$U/auth/verify")"
for _ in 1 2 3 4; do curl -s -o /dev/null -X POST -H "$J" -d "{\"email\":\"guess@smoke.local\",\"code\":\"$WRONG\"}" "$U/auth/verify"; done
check "5th wrong guess burns the code, so the real one fails" "invalid_code" \
  "$(curl -s -X POST -H "$J" -d "{\"email\":\"guess@smoke.local\",\"code\":\"$REAL\"}" "$U/auth/verify")"

OLD=$(curl -s -X POST -H "$J" -d '{"email":"old@smoke.local"}' "$U/auth/start" | field devCode)
curl -s -o /dev/null -X POST -H "$J" -d '{"email":"old@smoke.local"}' "$U/auth/start"
check "requesting a new code burns the old one" "invalid_code" \
  "$(curl -s -X POST -H "$J" -d "{\"email\":\"old@smoke.local\",\"code\":\"$OLD\"}" "$U/auth/verify")"

for _ in 1 2 3; do curl -s -o /dev/null -X POST -H "$J" -d '{"email":"old@smoke.local"}' "$U/auth/start"; done
check "per-inbox hourly cap" "too_many_codes" \
  "$(curl -s -X POST -H "$J" -d '{"email":"old@smoke.local"}' "$U/auth/start")"

sec "sign-in and the cookie"
A="$TMP/a"; B="$TMP/b"
check "A signs in (new account)" "signedIn" "$(signin "$A" "Smoke.A@smoke.local" true)"
check "cookie is HttpOnly"       "#HttpOnly_" "$(cat "$A")"
signin "$B" "b@smoke.local" false > /dev/null
ME=$(curl -s -b "$A" "$U/me")
check "email normalised to lowercase" '"email":"smoke.a@smoke.local"' "$ME"
check "release-notes opt-in recorded"  '"notifyReleases":true' "$ME"
check "B left release notes off"       '"notifyReleases":false' "$(curl -s -b "$B" "$U/me")"
check "new account has no name yet"    '"handle":""' "$ME"

sec "onboarding order: name, then consent"
check "name can be chosen before consent" '"handle"' \
  "$(curl -s -b "$A" -X POST -H "$J" -d '{"handle":"smokeharbour"}' "$U/me/handle")"
check "conversations blocked before consent" "403" "$(code -b "$A" "$U/sessions")"
check "email-shaped name refused"  "400" "$(code -b "$B" -X POST -H "$J" -d '{"handle":"me@work.com"}' "$U/me/handle")"
check "duplicate name refused"     "handle_taken" \
  "$(curl -s -b "$B" -X POST -H "$J" -d '{"handle":"smokeharbour"}' "$U/me/handle")"
curl -s -o /dev/null -b "$B" -X POST -H "$J" -d '{"handle":"smokesignal"}' "$U/me/handle"
curl -s -o /dev/null -b "$A" -X POST "$U/consent"; curl -s -o /dev/null -b "$B" -X POST "$U/consent"
check "conversations open after consent" "200" "$(code -b "$A" "$U/sessions")"

sec "the email is never visible to another user"
curl -s -o /dev/null -b "$A" -X POST -H "$J" -d '{"peerOptIn":true}' "$U/me/peer-opt-in"
curl -s -o /dev/null -b "$B" -X POST -H "$J" -d '{"peerOptIn":true}' "$U/me/peer-opt-in"
PEERS_A=$(curl -s -b "$A" "$U/peers"); PEERS_B=$(curl -s -b "$B" "$U/peers")
check  "B appears to A by name"          "smokesignal" "$PEERS_A"
absent "B's email absent from A's view"  "b@smoke.local" "$PEERS_A"
absent "A's email absent from B's view"  "smoke.a@smoke.local" "$PEERS_B"
absent "no email field at all in peers"  '"email"' "$PEERS_A$PEERS_B"
BID=$(echo "$PEERS_A" | python3 -c 'import sys,json;print(json.load(sys.stdin)["peers"][0]["id"])')
AID=$(echo "$PEERS_B" | python3 -c 'import sys,json;print(json.load(sys.stdin)["peers"][0]["id"])')
curl -s -o /dev/null -b "$A" -X POST -H "$J" -d '{"body":"Rough week, two people left."}' "$U/peers/$BID/messages"
absent "no email in a peer thread" "@smoke.local" "$(curl -s -b "$B" "$U/peers/$AID/messages")"

sec "safety"
CRISIS=$(curl -s -b "$A" -X POST -H "$J" -d '{"body":"there is no reason to live"}' "$U/peers/$BID/messages")
check  "crisis peer message NOT delivered" '"delivered":false' "$CRISIS"
check  "sender gets the resource card"     '"crisis":true' "$CRISIS"
absent "crisis text absent from thread"    "no reason to live" "$(curl -s -b "$B" "$U/peers/$AID/messages")"
check  "ordinary burnout idiom passes"     '"delivered":true' \
  "$(curl -s -b "$A" -X POST -H "$J" -d '{"body":"this workload is killing my motivation"}' "$U/peers/$BID/messages")"

sec "notes isolation and authorisation"
NOTE=$(curl -s -b "$A" -X POST "$U/notes" | field id)
check "owner can save"         '"saved":true' "$(curl -s -b "$A" -X PUT -H "$J" -d '{"title":"t","body":"b"}' "$U/notes/$NOTE")"
check "other user cannot save" "404" "$(code -b "$B" -X PUT -H "$J" -d '{"title":"x","body":"x"}' "$U/notes/$NOTE")"
check "unknown session is 404" "404" "$(code -b "$A" "$U/sessions/00000000-0000-0000-0000-000000000000/messages")"

sec "sign-out really ends the session"
cp "$A" "$TMP/a.stolen"   # simulate a copied cookie
check "sign out"                        '"signedOut":true' "$(curl -s -b "$A" -X POST "$U/auth/signout")"
check "the old cookie no longer works"  "401" "$(code -b "$TMP/a.stolen" "$U/me")"
signin "$A" "smoke.a@smoke.local" false > /dev/null
check "returning sign-in reaches the same account" '"handle":"smokeharbour"' "$(curl -s -b "$A" "$U/me")"
check "returning sign-in kept release-notes setting" '"notifyReleases":true' "$(curl -s -b "$A" "$U/me")"

sec "erasure"
cp "$A" "$TMP/a.after"
check "delete succeeds"                   '"deleted":true' "$(curl -s -b "$A" -X DELETE "$U/me")"
check "deleted user's cookie is dead"     "401" "$(code -b "$TMP/a.after" "$U/me")"
# Assert A specifically is gone — other real users may legitimately be listed.
absent "deleted user vanished from circle" "$AID" "$(curl -s -b "$B" "$U/peers")"
curl -s -o /dev/null -b "$B" -X DELETE "$U/me"

printf '\n\033[1m%d passed, %d failed\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
