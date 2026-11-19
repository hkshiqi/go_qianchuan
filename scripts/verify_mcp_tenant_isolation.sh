#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:18081}"
SERVICE_ROOT="${SERVICE_ROOT:-/opt/go-qianchuan}"
BACKEND_ROOT="${BACKEND_ROOT:-$SERVICE_ROOT/backend}"
OWNER_KEY_FILE="${OWNER_KEY_FILE:-$SERVICE_ROOT/tenants/owner.key}"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

status_code() {
  local method="$1"
  local path="$2"
  shift 2
  curl -sS -o /dev/null -w '%{http_code}' -X "$method" "$@" "$BASE_URL$path"
}

expect_code() {
  local want="$1"
  local method="$2"
  local path="$3"
  shift 3
  local got
  got="$(status_code "$method" "$path" "$@")"
  [[ "$got" == "$want" ]] || fail "$method $path returned $got, want $want"
  printf 'OK %s %s -> %s\n' "$method" "$path" "$got"
}

expect_code 200 GET /healthz
expect_code 401 POST /mcp \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
expect_code 404 POST /api/mcp -H 'Content-Type: application/json' --data '{}'
expect_code 404 POST /api/oauth/exchange -H 'Content-Type: application/json' --data '{}'

[[ -f "$OWNER_KEY_FILE" ]] || fail "owner key file is missing: $OWNER_KEY_FILE"
owner_key="$(tr -d '\r\n' < "$OWNER_KEY_FILE")"
[[ -n "$owner_key" ]] || fail "owner key file is empty"

owner_status="$(
  curl -sS -X POST "$BASE_URL/mcp" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $owner_key" \
    --data '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"qianchuan_get_status","arguments":{}}}'
)"
python3 - "$owner_status" <<'PY'
import json
import sys

envelope = json.loads(sys.argv[1])
if envelope.get("error"):
    raise SystemExit(f"owner status RPC failed: {envelope['error']}")
status = json.loads(envelope["result"]["content"][0]["text"])
expected = {
    "tenant_id": "owner",
    "advertiser_id": "1855810750424170",
    "aweme_id": "63307887035",
    "live_room_id": "7709635958206991154",
}
for field, want in expected.items():
    got = status.get(field)
    if got != want:
        raise SystemExit(f"owner {field} = {got!r}, want {want!r}")
if status.get("write_enabled") is not False:
    raise SystemExit("write operations are unexpectedly enabled")
print("OK owner key resolves only its bound account; writes disabled")
PY

encryption_key="$(sed -n 's/^TENANT_ENCRYPTION_KEY=//p' "$BACKEND_ROOT/.env" | tail -n 1)"
[[ -n "$encryption_key" ]] || fail "TENANT_ENCRYPTION_KEY is missing"

work="$(mktemp -d "${TMPDIR:-/tmp}/qianchuan-isolation.XXXXXX")"
cleanup() {
  rm -rf "$work"
}
trap cleanup EXIT

create_tenant() {
  local id="$1"
  local advertiser_id="$2"
  TENANT_STORE_PATH="$work/tenants.json" \
  TENANT_ENCRYPTION_KEY="$encryption_key" \
    "$BACKEND_ROOT/bin/qianchuan-tenant" create \
      --id "$id" \
      --name "$id" \
      --advertiser-id "$advertiser_id" \
      --key-file "$work/$id.key"
}

create_tenant isolation-a 900000001
create_tenant isolation-b 900000002

call_status() {
  local key
  key="$(tr -d '\r\n' < "$1")"
  curl -sS -X POST "$BASE_URL/mcp" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $key" \
    --data '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"qianchuan_get_status","arguments":{}}}'
}

status_a="$(call_status "$work/isolation-a.key")"
status_b="$(call_status "$work/isolation-b.key")"
python3 - "$status_a" "$status_b" <<'PY'
import json
import sys

def parse(payload):
    envelope = json.loads(payload)
    if envelope.get("error"):
        raise SystemExit(f"RPC failed: {envelope['error']}")
    return json.loads(envelope["result"]["content"][0]["text"])

a = parse(sys.argv[1])
b = parse(sys.argv[2])
if a.get("advertiser_id") != "900000001" or b.get("advertiser_id") != "900000002":
    raise SystemExit(f"tenant accounts crossed: A={a}, B={b}")
print("OK two isolated tenant keys resolve to different accounts")
PY

printf 'PASS: MCP tenant isolation verification\n'
