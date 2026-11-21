#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${QIANCHUAN_MCP_LOCAL_URL:-http://127.0.0.1:18081/mcp}"
KEY_FILE="${QIANCHUAN_MCP_KEY_FILE:-/opt/go-qianchuan/tenants/owner.key}"
ACCOUNT_ID="${QIANCHUAN_ACCOUNT_ID:-1855810750424170}"
PARENT_PLAN_ID="${QIANCHUAN_LIVE_PLAN_ID:-1856283025550586}"
APPLY="${QIANCHUAN_CLEANUP_APPLY:-false}"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

[[ -f "$KEY_FILE" ]] || fail "missing MCP key file: $KEY_FILE"
key="$(tr -d '\r\n' <"$KEY_FILE")"
[[ -n "$key" ]] || fail "empty MCP key file: $KEY_FILE"

work="$(mktemp -d "${TMPDIR:-/tmp}/qianchuan-crud-cleanup.XXXXXX")"
cleanup() {
  rm -rf "$work"
}
trap cleanup EXIT

call_tool() {
  local name="$1"
  local arguments="$2"
  local output="$work/$name-$(date +%s%N).json"
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
  local delay="${QIANCHUAN_MCP_RETRY_DELAY_SECONDS:-15}"
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

collect_plans() {
  local scene="$1"
  local output_file="$2"
  local page=1
  : >"$output_file"
  while true; do
    local payload
    payload="$(call_tool_retry qianchuan_list_product_plans "{\"account_id\":\"$ACCOUNT_ID\",\"adlab_scene\":\"$scene\",\"status\":\"ALL_INCLUDE_DELETED\",\"page\":$page,\"page_size\":100}")"
    printf '%s\n' "$payload" >>"$output_file"
    local has_next
    has_next="$(python3 - "$payload" "$page" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
page_info = result.get("page_info") or {}
current = int(page_info.get("page") or sys.argv[2])
total = int(page_info.get("total_page") or 0)
if current != int(sys.argv[2]):
    raise SystemExit(f"unexpected product-plan page {current}")
print(1 if total > current else 0)
PY
)"
    if [[ "$has_next" != "1" ]]; then
      break
    fi
    page=$((page + 1))
  done
}

plans_file="$work/product-plans.jsonl"
collect_plans OVERALL_PROJECT "$plans_file"
collect_plans UNI_PROJECT "$plans_file"

python3 - "$plans_file" <<'PY'
import json
import pathlib
import sys

prefixes = (
    "Codex-CRUD-TEST-",
    "Codex-CRUD-OVERALL-TEST-",
    "Codex-CRUD-PRODUCT-CARD-TEST-",
)
seen = set()
rows = []
for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    for plan in json.loads(line).get("plans") or []:
        ad_id = str(plan.get("ad_id") or "")
        name = str(plan.get("name") or "")
        status = str(plan.get("status") or "").upper()
        opt_status = str(plan.get("opt_status") or "").upper()
        if not ad_id or ad_id in seen or not name.startswith(prefixes):
            continue
        seen.add(ad_id)
        rows.append({
            "ad_id": ad_id,
            "name": name,
            "status": status,
            "opt_status": opt_status,
        })
print(json.dumps(rows, ensure_ascii=False, indent=2))
PY

control_file="$work/control-tasks.jsonl"
: >"$control_file"
control_page=1
while true; do
  control_payload="$(call_tool_retry qianchuan_list_control_tasks "{\"account_id\":\"$ACCOUNT_ID\",\"ad_id\":\"$PARENT_PLAN_ID\",\"start_time\":\"2023-03-18 00:00:00\",\"end_time\":\"2099-12-31 23:59:59\",\"page\":$control_page,\"page_size\":100}")"
  printf '%s\n' "$control_payload" >>"$control_file"
  control_has_next="$(python3 - "$control_payload" "$control_page" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
page_info = result.get("page_info") or {}
current = int(page_info.get("page") or sys.argv[2])
total = int(page_info.get("total_page") or 0)
if current != int(sys.argv[2]):
    raise SystemExit(f"unexpected control-task page {current}")
print(1 if total > current else 0)
PY
)"
  if [[ "$control_has_next" != "1" ]]; then
    break
  fi
  control_page=$((control_page + 1))
done

printf '== live plan residue ==\n'
python3 - "$plans_file" <<'PY'
import json
import pathlib
import sys

prefix = "Codex-CRUD-TEST-"
rows = []
for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    for plan in json.loads(line).get("plans") or []:
        name = str(plan.get("name") or "")
        if not name.startswith(prefix) or name.startswith(
            ("Codex-CRUD-OVERALL-TEST-", "Codex-CRUD-PRODUCT-CARD-TEST-")
        ):
            continue
        rows.append({
            "ad_id": str(plan.get("ad_id") or ""),
            "name": name,
            "status": str(plan.get("status") or "").upper(),
            "opt_status": str(plan.get("opt_status") or "").upper(),
        })
print(json.dumps(rows, ensure_ascii=False, indent=2))
PY

live_plans_json="$(python3 - "$plans_file" <<'PY'
import json
import pathlib
import sys

prefix = "Codex-CRUD-TEST-"
seen = set()
rows = []
for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    for plan in json.loads(line).get("plans") or []:
        ad_id = str(plan.get("ad_id") or "")
        name = str(plan.get("name") or "")
        status = str(plan.get("status") or "").upper()
        opt_status = str(plan.get("opt_status") or "").upper()
        if not ad_id or ad_id in seen:
            continue
        if not name.startswith(prefix) or name.startswith(
            ("Codex-CRUD-OVERALL-TEST-", "Codex-CRUD-PRODUCT-CARD-TEST-")
        ):
            continue
        seen.add(ad_id)
        if status in {"DELETE", "DELETED"} or opt_status in {"DELETE", "DELETED"}:
            continue
        rows.append({"ad_id": ad_id, "name": name})
print(json.dumps(rows, ensure_ascii=False))
PY
)"

printf '== product/card/overall plan residue ==\n'
python3 - "$plans_file" <<'PY'
import json
import pathlib
import sys

prefixes = ("Codex-CRUD-OVERALL-TEST-", "Codex-CRUD-PRODUCT-CARD-TEST-")
rows = []
for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    for plan in json.loads(line).get("plans") or []:
        name = str(plan.get("name") or "")
        if not name.startswith(prefixes):
            continue
        rows.append({
            "ad_id": str(plan.get("ad_id") or ""),
            "name": name,
            "status": str(plan.get("status") or "").upper(),
            "opt_status": str(plan.get("opt_status") or "").upper(),
        })
print(json.dumps(rows, ensure_ascii=False, indent=2))
PY

product_plans_json="$(python3 - "$plans_file" <<'PY'
import json
import pathlib
import sys

prefixes = ("Codex-CRUD-OVERALL-TEST-", "Codex-CRUD-PRODUCT-CARD-TEST-")
seen = set()
rows = []
for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    for plan in json.loads(line).get("plans") or []:
        ad_id = str(plan.get("ad_id") or "")
        name = str(plan.get("name") or "")
        status = str(plan.get("status") or "").upper()
        opt_status = str(plan.get("opt_status") or "").upper()
        if not ad_id or ad_id in seen or not name.startswith(prefixes):
            continue
        seen.add(ad_id)
        if status in {"DELETE", "DELETED"} or opt_status in {"DELETE", "DELETED"}:
            continue
        rows.append({"ad_id": ad_id, "name": name})
print(json.dumps(rows, ensure_ascii=False))
PY
)"

printf '== material chase residue ==\n'
python3 - "$control_file" <<'PY'
import json
import pathlib
import sys

rows = []
for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    for task in json.loads(line).get("tasks") or []:
        name = str(task.get("name") or "")
        if not name.startswith("Codex-CRUD-CHASE-TEST-"):
            continue
        rows.append({
            "task_id": str(task.get("id") or ""),
            "name": name,
            "task_status": str(task.get("task_status") or "").upper(),
        })
print(json.dumps(rows, ensure_ascii=False, indent=2))
PY

if [[ "$APPLY" != "true" ]]; then
  printf 'DRY-RUN: rerun with QIANCHUAN_CLEANUP_APPLY=true to delete the listed active test objects\n'
  exit 0
fi

control_json="$(python3 - "$control_file" <<'PY'
import json
import pathlib
import sys

seen = set()
rows = []
for line in pathlib.Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    for task in json.loads(line).get("tasks") or []:
        task_id = str(task.get("task_id") or task.get("id") or "")
        name = str(task.get("name") or "")
        status = str(task.get("task_status") or task.get("status") or "").upper()
        if not task_id or task_id in seen or not name.startswith("Codex-CRUD-CHASE-TEST-"):
            continue
        seen.add(task_id)
        if status not in {"DELETE", "DELETED", "DISABLE"}:
            continue
        rows.append(task_id)
print(json.dumps(rows, separators=(",", ":")))
PY
)"
if [[ "$control_json" != "[]" ]]; then
  args="$(python3 - "$ACCOUNT_ID" "$PARENT_PLAN_ID" "$control_json" <<'PY'
import json
import sys

print(json.dumps({
    "account_id": sys.argv[1],
    "ad_id": sys.argv[2],
    "task_ids": json.loads(sys.argv[3]),
    "opt_type": "DELETE",
    "confirm": True,
}, separators=(",", ":")))
PY
)"
  printf 'DELETE control tasks %s\n' "$control_json"
  call_tool qianchuan_update_control_task_status "$args" >/dev/null
fi

if [[ "$control_json" != "[]" ]]; then
  fail "residual material-chase tasks remain after cleanup: $control_json"
fi
if [[ "$product_plans_json" != "[]" ]]; then
  printf 'WARN: product-plan test residue is outside this cleanup tool scope: %s\n' "$product_plans_json" >&2
fi
if [[ "$live_plans_json" != "[]" ]]; then
  printf 'WARN: live-plan test residue is outside this cleanup tool scope: %s\n' "$live_plans_json" >&2
fi

printf 'PASS: cleanup requests completed; rerun without apply to verify residue\n'
