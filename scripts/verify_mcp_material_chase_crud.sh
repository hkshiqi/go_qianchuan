#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${QIANCHUAN_MCP_LOCAL_URL:-http://127.0.0.1:18081/qianchuan-app/mcp}"
KEY_FILE="${QIANCHUAN_MCP_KEY_FILE:-/opt/go-qianchuan/tenants/owner.key}"
ACCOUNT_ID="${QIANCHUAN_ACCOUNT_ID:-1855810750424170}"
PARENT_PLAN_ID="${QIANCHUAN_LIVE_PLAN_ID:-1856283025550586}"
METRIC_DATE="${QIANCHUAN_METRIC_DATE:-$(TZ=Asia/Shanghai date -d '1 day ago' +%F)}"
DEBUG_DIR="${QIANCHUAN_MCP_DEBUG_DIR:-}"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

[[ -f "$KEY_FILE" ]] || fail "missing MCP key file: $KEY_FILE"
key="$(tr -d '\r\n' <"$KEY_FILE")"
[[ -n "$key" ]] || fail "empty MCP key file: $KEY_FILE"

work="$(mktemp -d "${TMPDIR:-/tmp}/qianchuan-chase-crud.XXXXXX")"
created_task_id=""

call_tool() {
  local name="$1"
  local arguments="$2"
  local output="$work/$name.json"
  curl --fail-with-body --silent --show-error \
    -X POST "$MCP_URL" \
    -H "Authorization: Bearer $key" \
    -H "Content-Type: application/json" \
    --data-binary "$(printf '{"jsonrpc":"2.0","id":100,"method":"tools/call","params":{"name":"%s","arguments":%s}}' \
      "$name" "$arguments")" >"$output"
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

call_tool_retry() {
  local attempts="${QIANCHUAN_MCP_RETRY_ATTEMPTS:-6}"
  local delay="${QIANCHUAN_MCP_RETRY_DELAY_SECONDS:-12}"
  local attempt=1
  local output
  while true; do
    if output="$(call_tool "$@" 2>&1)"; then
      printf '%s\n' "$output"
      return 0
    fi
    if (( attempt >= attempts )) || [[ "$output" != *"40100"* ]]; then
      printf '%s\n' "$output" >&2
      return 1
    fi
    printf 'RETRY: request frequency limited, waiting %ss (attempt %d/%d)\n' \
      "$delay" "$attempt" "$attempts" >&2
    sleep "$delay"
    attempt=$((attempt + 1))
  done
}

cleanup() {
  local status=$?
  if [[ -n "$created_task_id" ]] && (( status != 0 )); then
    local args
    args="$(python3 - "$ACCOUNT_ID" "$PARENT_PLAN_ID" "$created_task_id" <<'PY'
import json
import sys

account_id, ad_id, task_id = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "ad_id": ad_id,
    "task_ids": [task_id],
    "opt_type": "DELETE",
    "confirm": True,
}, separators=(",", ":")))
PY
)"
    printf 'CLEANUP: deleting test material chase %s\n' "$created_task_id" >&2
    call_tool qianchuan_update_control_task_status "$args" >/dev/null || true
  fi
  rm -rf "$work"
  exit "$status"
}
trap cleanup EXIT

status="$(call_tool qianchuan_get_status "{\"account_id\":\"$ACCOUNT_ID\"}")"
python3 - "$status" <<'PY'
import json
import sys

status = json.loads(sys.argv[1])
if status.get("authenticated") is not True:
    raise SystemExit(f"account is not authenticated: {status}")
if status.get("plan_crud_test_write_enabled") is not True:
    raise SystemExit(
        "PLAN_CRUD_TEST_WRITE_ENABLED is false; enable it only for this guarded CRUD test"
    )
print("OK MCP authenticated; guarded CRUD test writes enabled")
PY

materials_args="$(python3 - "$ACCOUNT_ID" "$METRIC_DATE" <<'PY'
import json
import sys

account_id, metric_date = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "start_date": metric_date,
    "end_date": metric_date,
    "page": 1,
    "page_size": 200,
}, separators=(",", ":")))
PY
)"
plan_materials="$(call_tool_retry qianchuan_material_report "$materials_args")"
selected_ids="$(python3 - "$plan_materials" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
materials = result.get("materials") or []
protected = result.get("protected_top3") or []
if len(protected) != 3:
    raise SystemExit(f"protected_top3 must contain exactly 3 materials, got {len(protected)}")
top3_ids = [str(item.get("material_id") or "") for item in protected]
if any(not material_id for material_id in top3_ids):
    raise SystemExit(f"protected_top3 contains an empty material_id: {protected}")
selected = list(top3_ids)
for material in materials:
    material_id = str(material.get("material_id") or "")
    if not material_id or material_id in selected:
        continue
    ctr = material.get("ctr")
    conversion_rate = material.get("conversion_rate")
    if (
        isinstance(ctr, (int, float))
        and not isinstance(ctr, bool)
        and ctr >= 7
    ) or (
        isinstance(conversion_rate, (int, float))
        and not isinstance(conversion_rate, bool)
        and conversion_rate >= 8
    ):
        selected.append(material_id)
        break
print(json.dumps(selected, separators=(",", ":")))
PY
)"

suffix="$(TZ=Asia/Shanghai date +%Y%m%d-%H%M%S)"
name="Codex-CRUD-CHASE-TEST-$suffix"
renamed="Codex-CRUD-CHASE-TEST-$suffix-UPD"
create_args="$(python3 - "$ACCOUNT_ID" "$PARENT_PLAN_ID" "$name" "$selected_ids" "$METRIC_DATE" <<'PY'
import json
import sys

account_id, ad_id, name, material_ids_json, metric_date = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "ad_id": ad_id,
    "name": name,
    "material_ids": json.loads(material_ids_json),
    "budget": 200,
    "bid": 14.32,
    "metric_date": metric_date,
    "confirm": True,
}, separators=(",", ":")))
PY
)"
create="$(call_tool_retry qianchuan_create_test_material_chase "$create_args")"
created_task_id="$(python3 - "$create" "$selected_ids" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
expected_ids = json.loads(sys.argv[2])
if result.get("created") is not True:
    raise SystemExit(f"create failed: {result}")
if result.get("verified") is not True:
    raise SystemExit(f"create readback did not verify: {result}")
if result.get("paused_immediately") is not True:
    raise SystemExit(f"create was not immediately paused: {result}")
task = result.get("task") or {}
if task.get("task_status") != "DISABLE":
    raise SystemExit(f"created task is not paused: {result}")
actual_ids = {str(item.get("material_id")) for item in (task.get("material_list") or [])}
if not set(expected_ids) <= actual_ids:
    raise SystemExit(f"created task lost selected materials: {result}")
task_id = str(result.get("task_id") or "")
if not task_id:
    raise SystemExit(f"create returned no task_id: {result}")
print(task_id)
PY
)"
printf 'OK created, paused, and read back material chase %s\n' "$created_task_id"

update_args="$(python3 - "$ACCOUNT_ID" "$PARENT_PLAN_ID" "$created_task_id" "$renamed" <<'PY'
import json
import sys

account_id, ad_id, task_id, new_name = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "ad_id": ad_id,
    "task_id": task_id,
    "new_name": new_name,
    "budget": 210,
    "confirm": True,
}, separators=(",", ":")))
PY
)"
updated="$(call_tool_retry qianchuan_update_test_material_chase "$update_args")"
python3 - "$updated" "$renamed" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
if result.get("updated") is not True:
    raise SystemExit(f"update failed: {result}")
if result.get("verified") is not True:
    raise SystemExit(f"update readback did not verify: {result}")
if result.get("actual_name") != sys.argv[2]:
    raise SystemExit(f"unexpected updated name: {result}")
if abs(float(result.get("actual_budget") or 0) - 210) > 0.000001:
    raise SystemExit(f"unexpected updated budget: {result}")
if result.get("task_status") != "DISABLE":
    raise SystemExit(f"task is no longer paused after update: {result}")
PY
printf 'OK renamed and read back material chase %s\n' "$created_task_id"

pause_args="$(python3 - "$ACCOUNT_ID" "$PARENT_PLAN_ID" "$created_task_id" <<'PY'
import json
import sys

account_id, ad_id, task_id = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "ad_id": ad_id,
    "task_ids": [task_id],
    "opt_type": "PAUSE",
    "confirm": True,
}, separators=(",", ":")))
PY
)"
paused="$(call_tool_retry qianchuan_update_control_task_status "$pause_args")"
python3 - "$paused" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
if result.get("updated") is not True or result.get("verified") is not True:
    raise SystemExit(f"pause failed or was not verified: {result}")
readbacks = result.get("readbacks") or {}
if not readbacks or any(item.get("verified") is not True for item in readbacks.values()):
    raise SystemExit(f"pause readback is incomplete: {result}")
PY
printf 'OK pause and readback verified\n'

delete_args="$(python3 - "$ACCOUNT_ID" "$PARENT_PLAN_ID" "$created_task_id" <<'PY'
import json
import sys

account_id, ad_id, task_id = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "ad_id": ad_id,
    "task_ids": [task_id],
    "opt_type": "DELETE",
    "confirm": True,
}, separators=(",", ":")))
PY
)"
deleted="$(call_tool_retry qianchuan_update_control_task_status "$delete_args")"
python3 - "$deleted" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
if result.get("updated") is not True or result.get("verified") is not True:
    raise SystemExit(f"delete failed or was not verified: {result}")
readbacks = result.get("readbacks") or {}
if not readbacks:
    raise SystemExit(f"delete returned no readback evidence: {result}")
for item in readbacks.values():
    status = str(item.get("task_status") or "").upper()
    if status not in {"DELETE", "DELETED", "ABSENT_AFTER_DELETE"}:
        raise SystemExit(f"delete lacks explicit delete evidence: {result}")
PY
created_task_id=""
printf 'OK delete and readback verified\n'

if [[ -n "$DEBUG_DIR" ]]; then
  mkdir -p "$DEBUG_DIR"
  cp "$work/qianchuan_create_test_material_chase.json" "$DEBUG_DIR/"
  cp "$work/qianchuan_update_test_material_chase.json" "$DEBUG_DIR/"
  cp "$work/qianchuan_update_control_task_status.json" "$DEBUG_DIR/"
fi

printf 'PASS: Qianchuan MCP live material-chase CRUD round trip\n'
