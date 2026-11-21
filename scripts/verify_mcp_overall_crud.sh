#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${QIANCHUAN_MCP_LOCAL_URL:-http://127.0.0.1:18081/qianchuan-app/mcp}"
KEY_FILE="${QIANCHUAN_MCP_KEY_FILE:-/opt/go-qianchuan/tenants/owner.key}"
ACCOUNT_ID="${QIANCHUAN_ACCOUNT_ID:-1855810750424170}"
PRODUCT_ID="${QIANCHUAN_OVERALL_TEST_PRODUCT_ID:-3828195166146134185}"
AWEME_ID="${QIANCHUAN_OVERALL_TEST_AWEME_ID:-2362231432675704}"
ROI2_GOAL="${QIANCHUAN_OVERALL_TEST_ROI2_GOAL:-1.90}"
START_DATE="${QIANCHUAN_OVERALL_TEST_START_DATE:-$(TZ=Asia/Shanghai date -d tomorrow +%F)}"
DEBUG_DIR="${QIANCHUAN_MCP_DEBUG_DIR:-}"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

[[ -f "$KEY_FILE" ]] || fail "missing MCP key file: $KEY_FILE"
key="$(tr -d '\r\n' <"$KEY_FILE")"
[[ -n "$key" ]] || fail "empty MCP key file: $KEY_FILE"

work="$(mktemp -d "${TMPDIR:-/tmp}/qianchuan-overall-crud.XXXXXX")"
cleanup() {
  rm -rf "$work"
}
trap cleanup EXIT

rpc() {
  local output="$1"
  local id="$2"
  local method="$3"
  local params="$4"
  curl --fail-with-body --silent --show-error \
    -X POST "$MCP_URL" \
    -H "Authorization: Bearer $key" \
    -H "Content-Type: application/json" \
    --data-binary "$(printf '{"jsonrpc":"2.0","id":%s,"method":"%s","params":%s}' \
      "$id" "$method" "$params")" >"$output"
}

call_tool() {
  local name="$1"
  local arguments="$2"
  local output="$work/$name.json"
  rpc "$output" 100 tools/call \
    "$(printf '{"name":"%s","arguments":%s}' "$name" "$arguments")"
  python3 - "$output" "$name" <<'PY'
import json
import pathlib
import sys

path, name = sys.argv[1:]
envelope = json.loads(pathlib.Path(path).read_text(encoding="utf-8"))
if envelope.get("error"):
    raise SystemExit(f"{name}: JSON-RPC error: {envelope['error']}")
result = envelope.get("result") or {}
content = result.get("content")
text = content[0].get("text", "") if isinstance(content, list) and content else ""
if result.get("isError") is True:
    raise SystemExit(f"{name}: MCP tool error: {text}")
try:
    payload = json.loads(text)
except json.JSONDecodeError as exc:
    raise SystemExit(f"{name}: invalid JSON response: {exc}: {text[:300]}")
print(json.dumps(payload, ensure_ascii=False, separators=(",", ":")))
PY
}

status="$(call_tool qianchuan_get_status "{\"account_id\":\"$ACCOUNT_ID\"}")"
python3 - "$status" <<'PY'
import json
import sys

status = json.loads(sys.argv[1])
if status.get("authenticated") is not True:
    raise SystemExit(f"account is not authenticated: {status}")
if status.get("plan_crud_test_write_enabled") is not True:
    raise SystemExit(
        "PLAN_CRUD_TEST_WRITE_ENABLED is false; enable it only for this guarded test"
    )
print(
    "OK MCP authenticated; "
    f"account={status.get('advertiser_id')}; plan CRUD test writes enabled"
)
PY

suffix="$(TZ=Asia/Shanghai date +%Y%m%d-%H%M%S)-$$"
name="Codex-CRUD-OVERALL-TEST-$suffix"
renamed="$name-UPDATED"

create_args="$(python3 - "$ACCOUNT_ID" "$name" "$PRODUCT_ID" "$AWEME_ID" \
  "$ROI2_GOAL" "$START_DATE" <<'PY'
import json
import sys

account_id, name, product_id, aweme_id, roi2_goal, start_date = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "name": name,
    "product_id": product_id,
    "aweme_id": aweme_id,
    "budget": 300,
    "roi2_goal": float(roi2_goal),
    "start_date": start_date,
    "end_date": start_date,
    "confirm": True,
}, ensure_ascii=False, separators=(",", ":")))
PY
)"
create="$(call_tool qianchuan_create_test_overall_product_plan "$create_args")"
ad_id="$(python3 - "$create" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
if result.get("created") is not True:
    raise SystemExit(f"create failed: {result}")
if result.get("verified") is not True or result.get("verified_test_identity") is not True:
    raise SystemExit(f"create readback did not verify the overall test plan: {result}")
if result.get("paused_immediately") is not True:
    raise SystemExit(f"create did not remain immediately paused: {result}")
plan = result.get("plan") or {}
if plan.get("status") != "DISABLE" or plan.get("opt_status") != "DISABLE":
    raise SystemExit(f"created plan is not paused: {result}")
if result.get("plan_type") != "OVERALL_PROJECT":
    raise SystemExit(f"unexpected plan type: {result.get('plan_type')!r}")
if result.get("budget") != 300:
    raise SystemExit(f"unexpected test budget: {result.get('budget')!r}")
print(result["ad_id"])
PY
)"
printf 'OK created, paused, and read back overall test plan %s\n' "$ad_id"

rename_args="$(python3 - "$ACCOUNT_ID" "$ad_id" "$renamed" <<'PY'
import json
import sys

account_id, ad_id, new_name = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "ad_id": ad_id,
    "new_name": new_name,
    "confirm": True,
}, separators=(",", ":")))
PY
)"
rename="$(call_tool qianchuan_rename_test_overall_product_plan "$rename_args")"
python3 - "$rename" "$renamed" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
if result.get("renamed") is not True:
    raise SystemExit(f"rename failed: {result}")
if result.get("verified") is not True:
    raise SystemExit(f"rename readback did not verify: {result}")
if result.get("actual_name") != sys.argv[2]:
    raise SystemExit(f"unexpected actual name: {result.get('actual_name')!r}")
PY
printf 'OK renamed and read back overall test plan %s\n' "$ad_id"

delete_args="$(python3 - "$ACCOUNT_ID" "$ad_id" <<'PY'
import json
import sys

account_id, ad_id = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "ad_id": ad_id,
    "confirm": True,
}, separators=(",", ":")))
PY
)"
delete="$(call_tool qianchuan_delete_test_overall_product_plan "$delete_args")"
python3 - "$delete" "$ad_id" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
if result.get("deleted") is not True:
    raise SystemExit(f"delete failed: {result}")
if result.get("verified") is not True:
    raise SystemExit(f"delete readback did not verify DELETE state: {result}")
evidence = result.get("verify_evidence") or {}
if evidence.get("detail_status") != "DELETE" and evidence.get("detail_opt_status") != "DELETE":
    if evidence.get("list_status") != "DELETE" and evidence.get("list_opt_status") != "DELETE":
        raise SystemExit(f"delete lacks explicit DELETE evidence: {result}")
print(f"OK deleted test plan {sys.argv[2]} with explicit DELETE readback")
PY

read_args="$(python3 - "$ACCOUNT_ID" "$ad_id" <<'PY'
import json
import sys

account_id, ad_id = sys.argv[1:]
print(json.dumps({"account_id": account_id, "ad_id": ad_id}, separators=(",", ":")))
PY
)"
residual="$(call_tool qianchuan_get_plan "$read_args")"
python3 - "$residual" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
plan = result.get("plan") or {}
status = str(plan.get("status") or "").upper()
opt_status = str(plan.get("opt_status") or "").upper()
if status not in {"DELETE", "DELETED"} and opt_status not in {"DELETE", "DELETED"}:
    raise SystemExit(f"residual plan is not in a deleted state: {result}")
print(f"OK residual plan check: status={status}, opt_status={opt_status}")
PY

if [[ -n "$DEBUG_DIR" ]]; then
  mkdir -p "$DEBUG_DIR"
  cp "$work/qianchuan_create_test_overall_product_plan.json" "$DEBUG_DIR/"
  cp "$work/qianchuan_rename_test_overall_product_plan.json" "$DEBUG_DIR/"
  cp "$work/qianchuan_delete_test_overall_product_plan.json" "$DEBUG_DIR/"
fi

printf 'PASS: Qianchuan MCP overall-plan CRUD round trip\n'
