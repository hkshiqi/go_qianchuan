#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${QIANCHUAN_MCP_LOCAL_URL:-https://dkl4ii67.zjz-service.cn/mcp}"
KEY_FILE="${QIANCHUAN_MCP_KEY_FILE:-/opt/go-qianchuan/tenants/owner.key}"
ACCOUNT_ID="${QIANCHUAN_ACCOUNT_ID:-1855810750424170}"
AD_ID="${QIANCHUAN_EXISTING_PRODUCT_PLAN_ID:-}"
IMAGE_ID="${QIANCHUAN_EXISTING_IMAGE_ID:-}"
TITLE_TEXT="${QIANCHUAN_EXISTING_TITLE:-红卫羊脂皂五盒装}"
TEST_BUDGET="${QIANCHUAN_EXISTING_PLAN_TEST_BUDGET:-31}"
BOOST_BUDGET="${QIANCHUAN_EXISTING_BOOST_BUDGET:-1}"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

[[ -f "$KEY_FILE" ]] || fail "missing MCP key file: $KEY_FILE"
[[ -n "$AD_ID" ]] || fail "QIANCHUAN_EXISTING_PRODUCT_PLAN_ID is required"
[[ -n "$IMAGE_ID" ]] || fail "QIANCHUAN_EXISTING_IMAGE_ID is required"
key="$(tr -d '\r\n' <"$KEY_FILE")"
[[ -n "$key" ]] || fail "empty MCP key file: $KEY_FILE"

RPC_ID=100
rpc() {
  local method="$1"
  local params="$2"
  RPC_ID=$((RPC_ID + 1))
  curl --fail-with-body --silent --show-error \
    -X POST "$MCP_URL" \
    -H "Authorization: Bearer $key" \
    -H "Content-Type: application/json" \
    --data-binary "{\"jsonrpc\":\"2.0\",\"id\":$RPC_ID,\"method\":\"$method\",\"params\":$params}"
}

call_tool() {
  local name="$1"
  local arguments="$2"
  rpc tools/call "$(python3 - "$name" "$arguments" <<'PY'
import json
import sys

print(json.dumps({
    "name": sys.argv[1],
    "arguments": json.loads(sys.argv[2]),
}, ensure_ascii=False, separators=(",", ":")))
PY
)" | python3 - "$name" <<'PY'
import json
import sys

name = sys.argv[1]
envelope = json.load(sys.stdin)
if envelope.get("error"):
    raise SystemExit(f"{name}: JSON-RPC error: {envelope['error']}")
result = envelope.get("result") or {}
content = result.get("content") or []
text = content[0].get("text", "") if content else ""
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
    raise SystemExit("PLAN_CRUD_TEST_WRITE_ENABLED is false; enable only for this guarded test")
print("OK MCP authenticated; guarded plan test writes enabled")
PY

before="$(call_tool qianchuan_get_plan "{\"account_id\":\"$ACCOUNT_ID\",\"ad_id\":\"$AD_ID\"}")"
python3 - "$before" <<'PY'
import json
import sys

plan = json.loads(sys.argv[1]).get("plan") or {}
if plan.get("marketing_goal") != "VIDEO_PROM_GOODS":
    raise SystemExit(f"wrong marketing_goal: {plan.get('marketing_goal')}")
if plan.get("status") != "DISABLE" or plan.get("opt_status") != "DISABLE":
    raise SystemExit(
        f"plan must be paused: status={plan.get('status')} opt_status={plan.get('opt_status')}"
    )
if not plan.get("product_infos"):
    raise SystemExit("plan detail does not expose product_infos")
has_image_material = any(
    creative.get("image_material")
    for creative in plan.get("multi_product_creative_list") or []
)
isolated_overall_test = (
    plan.get("adlab_scene") == "OVERALL_PROJECT"
    and str(plan.get("name") or "").startswith("Codex-CRUD-OVERALL-TEST-")
)
if not has_image_material and not isolated_overall_test:
    raise SystemExit(
        "plan has no image_material and is not an isolated paused overall test plan"
    )
print("OK paused product plan is eligible for image operations")
PY

suffix="$(TZ=Asia/Shanghai date +%Y%m%d-%H%M%S)-$$"
name="Codex-CRUD-EXISTING-PLAN-TEST-$suffix"
args="$(python3 - "$ACCOUNT_ID" "$AD_ID" "$name" "$TEST_BUDGET" "$BOOST_BUDGET" "$IMAGE_ID" "$TITLE_TEXT" <<'PY'
import json
import sys

account_id, ad_id, name, budget, boost_budget, image_id, title = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "ad_id": ad_id,
    "name": name,
    "budget": float(budget),
    "boost_budget": float(boost_budget),
    "add_image_ids": [image_id],
    "add_titles": [title],
    "confirm": True,
}, ensure_ascii=False, separators=(",", ":")))
PY
)"
result="$(call_tool qianchuan_test_existing_product_plan_operations "$args")"
python3 - "$result" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
steps = result.get("steps") or []
actions = [step.get("action") for step in steps]
required = [
    "update_budget",
    "add_materials",
    "delete_test_added_materials",
    "enable_isolated_test_plan",
    "create_and_stop_one_click_boost",
    "restore_plan_status",
]
if actions != required:
    raise SystemExit(f"unexpected operation sequence: {actions}")
if result.get("verified") is not True:
    raise SystemExit(f"operation verification failed: {result}")
restored = result.get("restored") or {}
if restored.get("budget") != result.get("original_budget"):
    raise SystemExit(f"budget not restored: {restored}")
if restored.get("material_ids") != result.get("original_material_ids"):
    raise SystemExit(f"materials not restored: {restored}")
print(json.dumps({
    "ok": True,
    "ad_id": result.get("ad_id"),
    "actions": actions,
    "restored": restored,
}, ensure_ascii=False, indent=2))
PY
