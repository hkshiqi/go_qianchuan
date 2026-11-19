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
test_port="${TEST_PORT:-18082}"
test_pid=""
cleanup() {
  if [[ -n "$test_pid" ]]; then
    kill "$test_pid" 2>/dev/null || true
    wait "$test_pid" 2>/dev/null || true
  fi
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
TENANT_STORE_PATH="$work/tenants.json" \
TENANT_ENCRYPTION_KEY="$encryption_key" \
  "$BACKEND_ROOT/bin/qianchuan-tenant" add-account \
    --id isolation-a \
    --advertiser-id 900000003

(
  cd "$BACKEND_ROOT"
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
  export PORT="$test_port"
  export TENANT_STORE_PATH="$work/tenants.json"
  export TOKEN_STORE_PATH="$work/tokens.json"
  exec "$BACKEND_ROOT/bin/qianchuan-server"
) >"$work/server.log" 2>&1 &
test_pid="$!"

for _ in $(seq 1 30); do
  if curl -fsS "http://127.0.0.1:$test_port/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done
curl -fsS "http://127.0.0.1:$test_port/healthz" >/dev/null ||
  fail "temporary isolation server failed to start; see $work/server.log"

call_status() {
  local key_file="$1"
  local arguments="${2:-{}}"
  local key
  key="$(tr -d '\r\n' < "$key_file")"
  curl -sS -X POST "http://127.0.0.1:$test_port/mcp" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $key" \
    --data "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"qianchuan_get_status\",\"arguments\":$arguments}}"
}

call_tool() {
  local key
  key="$(tr -d '\r\n' < "$1")"
  local tool="$2"
  local arguments="$3"
  curl -sS -X POST "http://127.0.0.1:$test_port/mcp" \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $key" \
    --data "{\"jsonrpc\":\"2.0\",\"id\":4,\"method\":\"tools/call\",\"params\":{\"name\":\"$tool\",\"arguments\":$arguments}}"
}

status_a="$(call_status "$work/isolation-a.key")"
status_b="$(call_status "$work/isolation-b.key")"
accounts_a="$(call_tool "$work/isolation-a.key" qianchuan_list_accounts '{}')"
foreign_status="$(call_status "$work/isolation-a.key" '{"account_id":"900000002"}')"
missing_selection="$(call_tool "$work/isolation-a.key" qianchuan_list_plans '{}')"
selected_status="$(call_status "$work/isolation-a.key" '{"account_id":"900000003"}')"
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
if a.get("advertiser_id") != "" or a.get("account_selection_required") is not True:
    raise SystemExit(f"multi-account key did not require an explicit selection: {a}")
if b.get("advertiser_id") != "900000002" or b.get("account_selection_required"):
    raise SystemExit(f"single-account key context failed: {b}")
print("OK multi-account key requires selection; single-account key resolves directly")
PY

python3 - "$accounts_a" "$foreign_status" "$missing_selection" "$selected_status" <<'PY'
import json
import sys

def parse(payload):
    envelope = json.loads(payload)
    if envelope.get("error"):
        raise SystemExit(f"RPC failed: {envelope['error']}")
    result = envelope["result"]
    text = result["content"][0]["text"]
    return result.get("isError", False), text

is_error, text = parse(sys.argv[1])
accounts = json.loads(text)
if is_error or sorted(item["account_id"] for item in accounts["accounts"]) != ["900000001", "900000003"]:
    raise SystemExit(f"account listing failed: {accounts}")
if not accounts["account_selection_required"]:
    raise SystemExit("multi-account listing did not require explicit selection")

is_error, text = parse(sys.argv[2])
if not is_error or "not bound to this tenant" not in text:
    raise SystemExit(f"foreign account was not rejected: {text}")

is_error, text = parse(sys.argv[3])
if not is_error or "account_id is required" not in text:
    raise SystemExit(f"missing selection was not rejected: {text}")

is_error, text = parse(sys.argv[4])
status = json.loads(text)
if is_error or status.get("advertiser_id") != "900000003" or status.get("account_selection_required"):
    raise SystemExit(f"selected account context failed: {status}")
print("OK one key selects two accounts; missing and foreign account_id are rejected")
PY

printf 'PASS: MCP tenant isolation verification\n'
