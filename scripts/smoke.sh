#!/usr/bin/env bash
# Astralis end-to-end smoke test.
#
# Exercises every guarantee the system claims to make, against a running API.
# It mints its own tokens from BACKEND_JWT_SECRET, so it needs no browser and
# no GitHub login -- which is exactly what makes it runnable in CI later.
#
#   ./scripts/smoke.sh                       # against localhost:8080
#   API=https://api.astralis.example ./scripts/smoke.sh
#
# It creates two users with @smoke.local emails and deletes them at the end.
set -uo pipefail

API="${API:-http://localhost:8080}"
U="$API/api/v1"
PASS=0; FAIL=0

[ -f .env ] && { set -a; . ./.env; set +a; }
: "${BACKEND_JWT_SECRET:?set BACKEND_JWT_SECRET (or create .env)}"

mint() {
  python3 -c "
import base64,hmac,hashlib,json,time,os,sys
b=lambda x: base64.urlsafe_b64encode(x).rstrip(b'=').decode()
h=b(json.dumps({'alg':'HS256','typ':'JWT'},separators=(',',':')).encode())
p=b(json.dumps({'sub':sys.argv[1],'email':sys.argv[2],'name':sys.argv[3],
 'iat':int(time.time()),'exp':int(time.time())+3600},separators=(',',':')).encode())
s=b(hmac.new(os.environ['BACKEND_JWT_SECRET'].encode(),f'{h}.{p}'.encode(),hashlib.sha256).digest())
print(f'{h}.{p}.{s}')" "$1" "$2" "$3"
}

# check <name> <expected-substring> <actual>
check() {
  if [[ "$3" == *"$2"* ]]; then printf '  \033[32m✓\033[0m %s\n' "$1"; PASS=$((PASS+1))
  else printf '  \033[31m✗\033[0m %s\n      expected: %s\n      actual:   %s\n' "$1" "$2" "$3"; FAIL=$((FAIL+1)); fi
}
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
sec()  { printf '\n\033[1m%s\033[0m\n' "$1"; }

TA=$(mint smoke-a a@smoke.local "Smoke A"); TB=$(mint smoke-b b@smoke.local "Smoke B")
A="Authorization: Bearer $TA"; B="Authorization: Bearer $TB"; J="Content-Type: application/json"

sec "health and auth"
check "healthz responds"            '"status":"ok"'  "$(curl -s -m 5 "$API/healthz")"
check "no token is rejected"        "401"            "$(code "$U/me")"
check "garbage token is rejected"   "401"            "$(code -H 'Authorization: Bearer not.a.token' "$U/me")"

sec "consent gate"
check "new user is not consented"   '"consented":false' "$(curl -s -H "$A" "$U/me")"
check "sessions blocked pre-consent" "403"           "$(code -H "$A" "$U/sessions")"
curl -s -X POST -H "$A" "$U/consent" >/dev/null
curl -s -X POST -H "$B" "$U/consent" >/dev/null
check "sessions open post-consent"  "200"            "$(code -H "$A" "$U/sessions")"

sec "identity"
check "short handle rejected"       "400"            "$(code -X POST -H "$A" -H "$J" -d '{"handle":"ab"}' "$U/me/handle")"
check "email-shaped handle rejected" "400"           "$(code -X POST -H "$A" -H "$J" -d '{"handle":"me@work.com"}' "$U/me/handle")"
check "handle accepted"             '"handle"'       "$(curl -s -X POST -H "$A" -H "$J" -d '{"handle":"smokeharbour"}' "$U/me/handle")"
check "duplicate handle refused"    "handle_taken"   "$(curl -s -X POST -H "$B" -H "$J" -d '{"handle":"smokeharbour"}' "$U/me/handle")"
curl -s -X POST -H "$B" -H "$J" -d '{"handle":"smokesignal"}' "$U/me/handle" >/dev/null

sec "circle: opt-in and presence"
check "not listed before opt-in"    '"peers":[]'     "$(curl -s -H "$A" "$U/peers")"
curl -s -X POST -H "$A" -H "$J" -d '{"peerOptIn":true}' "$U/me/peer-opt-in" >/dev/null
curl -s -X POST -H "$B" -H "$J" -d '{"peerOptIn":true}' "$U/me/peer-opt-in" >/dev/null
PEERS=$(curl -s -H "$A" "$U/peers")
check "peer appears after opt-in"   "smokesignal"    "$PEERS"
check "peer shows as online"        '"online":true'  "$PEERS"
check "email never exposed to peer" ""               "$(echo "$PEERS" | grep -q '@smoke.local' && echo LEAKED || echo '')"

BID=$(echo "$PEERS" | python3 -c 'import sys,json;print(json.load(sys.stdin)["peers"][0]["id"])')
AID=$(curl -s -H "$B" "$U/peers" | python3 -c 'import sys,json;print(json.load(sys.stdin)["peers"][0]["id"])')

sec "circle: delivery"
check "message sends"               '"delivered":true' "$(curl -s -X POST -H "$A" -H "$J" -d '{"body":"Rough week, two people left."}' "$U/peers/$BID/messages")"
check "recipient has 1 unread"      '"unread":1'     "$(curl -s -H "$B" "$U/peers/unread")"
check "recipient reads it"          "Rough week"     "$(curl -s -H "$B" "$U/peers/$AID/messages")"
check "unread clears on open"       '"unread":0'     "$(curl -s -H "$B" "$U/peers/unread")"

sec "safety — the checks that matter most"
CRISIS=$(curl -s -X POST -H "$A" -H "$J" -d '{"body":"there is no reason to live"}' "$U/peers/$BID/messages")
check "crisis peer msg NOT delivered" '"delivered":false' "$CRISIS"
check "crisis peer msg returns card"  '"crisis":true'     "$CRISIS"
check "crisis text absent from thread" "" \
  "$(curl -s -H "$B" "$U/peers/$AID/messages" | grep -q 'no reason to live' && echo LEAKED || echo '')"
check "ordinary burnout idiom passes" '"delivered":true' \
  "$(curl -s -X POST -H "$A" -H "$J" -d '{"body":"this workload is killing my motivation"}' "$U/peers/$BID/messages")"

sec "notes: isolation"
NOTE=$(curl -s -X POST -H "$A" "$U/notes" | python3 -c 'import sys,json;print(json.load(sys.stdin)["id"])')
check "note created"                "-"              "$NOTE"
check "owner can save"              '"saved":true'   "$(curl -s -X PUT -H "$A" -H "$J" -d '{"title":"t","body":"b"}' "$U/notes/$NOTE")"
check "other user cannot save"      "404"            "$(code -X PUT -H "$B" -H "$J" -d '{"title":"x","body":"x"}' "$U/notes/$NOTE")"
check "other user sees no notes"    '"notes":[]'     "$(curl -s -H "$B" "$U/notes")"

sec "authorisation"
check "unknown session is 404"      "404"            "$(code -H "$A" "$U/sessions/00000000-0000-0000-0000-000000000000/messages")"

sec "erasure"
check "delete succeeds"             '"deleted":true' "$(curl -s -X DELETE -H "$A" "$U/me")"
check "deleted user is gone"        '"peers":[]'     "$(curl -s -H "$B" "$U/peers")"
curl -s -X DELETE -H "$B" "$U/me" >/dev/null

printf '\n\033[1m%d passed, %d failed\033[0m\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ] || exit 1
