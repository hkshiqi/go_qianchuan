#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${QIANCHUAN_MCP_LOCAL_URL:-http://127.0.0.1:18081/qianchuan-app/mcp}"
MCP_URL="${MCP_URL%/}"
BASE_URL="${MCP_URL%/mcp}"
KEY_FILE="${QIANCHUAN_MCP_KEY_FILE:-/opt/go-qianchuan/tenants/owner.key}"
ACCOUNT_ID="${QIANCHUAN_ACCOUNT_ID:-1855810750424170}"
AWEME_ID="${QIANCHUAN_AWEME_ID:-63307887035}"
LIVE_ROOM_ID="${QIANCHUAN_LIVE_ROOM_ID:-7710006340290513690}"
PLAN_ID="${QIANCHUAN_PLAN_ID:-1856283025550586}"
EXPECTED_BUDGET="${QIANCHUAN_EXPECTED_BUDGET:-20000}"
EXPECTED_ROI2_GOAL="${QIANCHUAN_EXPECTED_ROI2_GOAL:-1.95}"
DATE="${QIANCHUAN_DATE:-$(TZ=Asia/Shanghai date +%F)}"
REQUEST_INTERVAL_SECONDS="${QIANCHUAN_REQUEST_INTERVAL_SECONDS:-4}"
RETRY_DELAY_SECONDS="${QIANCHUAN_RETRY_DELAY_SECONDS:-15}"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

work="$(mktemp -d "${TMPDIR:-/tmp}/qianchuan-mcp-reads.XXXXXX")"
cleanup() {
  rm -rf "$work"
}
trap cleanup EXIT

http_code() {
  local method="$1"
  local url="$2"
  shift 2
  curl -sS -o /dev/null -w '%{http_code}' -X "$method" "$@" "$url"
}

expect_code() {
  local want="$1"
  local method="$2"
  local url="$3"
  shift 3
  local got
  got="$(http_code "$method" "$url" "$@")"
  [[ "$got" == "$want" ]] || fail "$method $url returned HTTP $got, want $want"
  printf 'OK HTTP %s %s\n' "$got" "$url"
}

rpc() {
  local output="$1"
  local id="$2"
  local method="$3"
  local params="$4"
  local payload

  payload=$(printf '{"jsonrpc":"2.0","id":%s,"method":"%s","params":%s}' \
    "$id" "$method" "$params")
  sleep "$REQUEST_INTERVAL_SECONDS"
  curl --fail-with-body --silent --show-error \
    -X POST "$MCP_URL" \
    -H "Authorization: Bearer $key" \
    -H "Content-Type: application/json" \
    --data-binary "$payload" >"$output"
}

call_tool() {
  local name="$1"
  local arguments="$2"
  local output="$work/$name.json"
  local attempt
  for attempt in 1 2; do
    rpc "$output" 100 tools/call \
      "$(printf '{"name":"%s","arguments":%s}' "$name" "$arguments")"
    if ! python3 - "$output" <<'PY'
import json
import pathlib
import sys

try:
    envelope = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
except Exception:
    raise SystemExit(0)
result = envelope.get("result") or {}
text = ""
content = result.get("content")
if isinstance(content, list) and content:
    text = str(content[0].get("text") or "")
transient = result.get("isError") is True and (
    "[40100]" in text
    or "GetUniPromMaterial fail" in text
    or "系统请求频率超限" in text
)
raise SystemExit(0 if transient else 1)
PY
    then
      break
    fi
    if (( attempt < 2 )); then
      printf 'RETRY %s after transient upstream error\n' "$name" >&2
      sleep "$RETRY_DELAY_SECONDS"
    fi
  done
  printf '%s' "$output"
}

expect_code 200 GET "$BASE_URL/health"
expect_code 401 POST "$MCP_URL" \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'

[[ -f "$KEY_FILE" ]] || fail "missing key file: $KEY_FILE"
key="$(tr -d '\r\n' <"$KEY_FILE")"
[[ -n "$key" ]] || fail "empty key file: $KEY_FILE"

rpc "$work/initialize.json" 1 initialize '{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"qianchuan-readonly-selfcheck","version":"1.0.0"}}'
rpc "$work/tools-list.json" 2 tools/list '{}'

material_report_json="$(call_tool qianchuan_material_report "{\"account_id\":\"$ACCOUNT_ID\",\"start_date\":\"$DATE\",\"end_date\":\"$DATE\",\"page_size\":100}")"
if [[ "${QIANCHUAN_MATERIAL_REPORT_ONLY:-false}" == "true" ]]; then
  status_json=""
  accounts_json=""
  plan_json=""
  plan_materials_json=""
  live_json_payload=""
else
  status_json="$(call_tool qianchuan_get_status "{\"account_id\":\"$ACCOUNT_ID\"}")"
  accounts_json="$(call_tool qianchuan_list_accounts '{}')"
  plan_json="$(call_tool qianchuan_get_plan "{\"account_id\":\"$ACCOUNT_ID\",\"ad_id\":\"$PLAN_ID\"}")"
  plan_materials_json="$(call_tool qianchuan_get_plan_materials "{\"account_id\":\"$ACCOUNT_ID\",\"ad_id\":\"$PLAN_ID\",\"start_date\":\"$DATE\",\"end_date\":\"$DATE\",\"page_size\":100}")"
  live_json_payload="$(call_tool qianchuan_live_overview "{\"account_id\":\"$ACCOUNT_ID\"}")"
fi
if [[ "${QIANCHUAN_REQUIRE_PLAN_IN_LIST:-false}" == "true" ]]; then
  plans_json="$(call_tool qianchuan_list_plans "{\"account_id\":\"$ACCOUNT_ID\",\"start_time\":\"$DATE 00:00:00\",\"end_time\":\"$DATE 23:59:59\",\"page_size\":100}")"
else
  plans_json="$work/skipped-plans.json"
  printf '%s' '{"jsonrpc":"2.0","id":0,"result":{"content":[{"type":"text","text":"{\"plans\":[]}"}]}}' >"$plans_json"
fi

python3 - \
  "$work/initialize.json" \
  "$work/tools-list.json" \
  "$status_json" \
  "$accounts_json" \
  "$plans_json" \
  "$plan_json" \
  "$plan_materials_json" \
  "$material_report_json" \
  "$live_json_payload" \
  "$ACCOUNT_ID" \
  "$AWEME_ID" \
  "$LIVE_ROOM_ID" \
  "$PLAN_ID" \
  "$EXPECTED_BUDGET" \
  "$EXPECTED_ROI2_GOAL" \
  "${QIANCHUAN_REQUIRE_PLAN_IN_LIST:-false}" <<'PY'
import json
import math
import pathlib
import sys

(
    initialize_path,
    tools_list_path,
    status_path,
    accounts_path,
    plans_path,
    plan_path,
    plan_materials_path,
    material_report_path,
    live_path,
    account_id,
    aweme_id,
    live_room_id,
    plan_id,
    expected_budget,
    expected_roi2_goal,
    expect_plan_in_list,
) = sys.argv[1:]


def load(path):
    try:
        return json.loads(pathlib.Path(path).read_text(encoding="utf-8"))
    except Exception as exc:
        raise SystemExit(f"invalid JSON response {path}: {exc}")


def rpc_result(envelope, label, require_success=True):
    if envelope.get("jsonrpc") != "2.0":
        raise SystemExit(f"{label}: invalid jsonrpc version: {envelope.get('jsonrpc')!r}")
    if envelope.get("error"):
        raise SystemExit(f"{label}: JSON-RPC error: {envelope['error']}")
    if "result" not in envelope:
        raise SystemExit(f"{label}: missing result")
    result = envelope["result"]
    if require_success:
        if result.get("isError") is True:
            detail = result.get("content", [{}])[0].get("text", "")
            raise SystemExit(f"{label}: MCP tool error: {detail}")
        content = result.get("content")
        if not isinstance(content, list) or not content:
            raise SystemExit(f"{label}: missing MCP content")
        text = content[0].get("text")
        if not isinstance(text, str) or not text.strip():
            raise SystemExit(f"{label}: empty MCP tool content")
        try:
            return json.loads(text)
        except json.JSONDecodeError as exc:
            raise SystemExit(f"{label}: invalid tool JSON: {exc}: {text[:300]}")
    return result


def require(condition, message):
    if not condition:
        raise SystemExit(message)


def close(actual, expected):
    return isinstance(actual, (int, float)) and not isinstance(actual, bool) and math.isclose(
        float(actual), float(expected), rel_tol=0, abs_tol=0.000001
    )


initialize = rpc_result(load(initialize_path), "initialize", require_success=False)
require(initialize.get("protocolVersion"), "initialize: missing protocolVersion")
require(
    initialize.get("serverInfo", {}).get("name") == "qianchuan-live-operations",
    f"initialize: unexpected serverInfo {initialize.get('serverInfo')}",
)

tools_result = rpc_result(load(tools_list_path), "tools/list", require_success=False)
tools = {tool.get("name"): tool for tool in tools_result.get("tools", [])}
required_read_tools = {
    "qianchuan_get_status",
    "qianchuan_list_accounts",
    "qianchuan_list_plans",
    "qianchuan_get_plan",
    "qianchuan_get_plan_materials",
    "qianchuan_material_report",
    "qianchuan_live_overview",
}
require(required_read_tools <= set(tools), f"tools/list: missing tools {sorted(required_read_tools - set(tools))}")
for name in required_read_tools:
    require(
        tools[name].get("annotations", {}).get("readOnlyHint") is True,
        f"tools/list: {name} is not marked read-only",
    )

material_report_only = not status_path
if material_report_only:
    plan = {}
    protected_top3 = []
    plan_materials = {"material_count": 0}
else:
    status = rpc_result(load(status_path), "qianchuan_get_status")
    require(status.get("authenticated") is True, f"status: account is not authenticated: {status}")
    require(status.get("read_enabled") is True, f"status: reads are disabled: {status}")
    require(status.get("write_enabled") is False, f"status: writes are unexpectedly enabled: {status}")
    require(status.get("advertiser_id") == account_id, f"status: advertiser_id={status.get('advertiser_id')!r}, want {account_id}")
    require(status.get("aweme_id") == aweme_id, f"status: aweme_id={status.get('aweme_id')!r}, want {aweme_id}")
    require(status.get("live_room_id") == live_room_id, f"status: live_room_id={status.get('live_room_id')!r}, want {live_room_id}")

    accounts = rpc_result(load(accounts_path), "qianchuan_list_accounts")
    matched = [item for item in accounts.get("accounts", []) if item.get("account_id") == account_id]
    require(len(matched) == 1, f"accounts: expected exactly one {account_id}, got matches={len(matched)}")
    matched_account = matched[0]
    require(matched_account.get("authenticated") is True, f"accounts: {account_id} is not authenticated")
    require(matched_account.get("aweme_id") == aweme_id, f"accounts: aweme_id={matched_account.get('aweme_id')!r}, want {aweme_id}")
    require(matched_account.get("live_room_id") == live_room_id, f"accounts: live_room_id={matched_account.get('live_room_id')!r}, want {live_room_id}")

    plans = rpc_result(load(plans_path), "qianchuan_list_plans")
    matched_plans = [item for item in plans.get("plans", []) if item.get("ad_id") == plan_id]
    if expect_plan_in_list == "true":
        require(len(matched_plans) == 1, f"plans: expected exactly one {plan_id}, got matches={len(matched_plans)}")
        plan_row = matched_plans[0]
    else:
        require(len(matched_plans) <= 1, f"plans: duplicate plan {plan_id}")

    plan = rpc_result(load(plan_path), "qianchuan_get_plan").get("plan") or {}
    require(str(plan.get("ad_id")) == plan_id, f"plan: ad_id={plan.get('ad_id')!r}, want {plan_id}")
    require(close(plan.get("budget"), expected_budget), f"plan: budget={plan.get('budget')!r}, want {expected_budget}")
    require(close(plan.get("roi2_goal"), expected_roi2_goal), f"plan: roi2_goal={plan.get('roi2_goal')!r}, want {expected_roi2_goal}")
    require(plan.get("delivery_setting", {}).get("budget") is not None, "plan: missing delivery_setting.budget")

    plan_materials = rpc_result(load(plan_materials_path), "qianchuan_get_plan_materials")
    require(plan_materials.get("material_count", 0) > 0, f"plan materials: empty result: {plan_materials}")
    require(plan_materials.get("materials"), "plan materials: current page is empty")
    protected_top3 = plan_materials.get("protected_top3") or []
    if plan_materials.get("material_count", 0) >= 3:
        require(len(protected_top3) == 3, f"plan materials: protected_top3 has {len(protected_top3)} items, want 3")
    for rank, material in enumerate(protected_top3, start=1):
        require(material.get("cost_rank") == rank, f"plan materials: protected item {rank} has rank {material.get('cost_rank')}")
        require(material.get("protected_top3") is True, f"plan materials: protected item {rank} is not flagged: {material}")
        require(material.get("eligible_for_replacement") is False, f"plan materials: protected item {rank} is replaceable: {material}")

material_report = rpc_result(load(material_report_path), "qianchuan_material_report")
require(material_report.get("data_topic") == "OVERALL_ROI_LIVE_MATERIAL_VIDEO", f"material report: unexpected data_topic {material_report.get('data_topic')!r}")
require(material_report.get("materials"), f"material report: empty result: {material_report}")
report_materials = material_report.get("materials") or []
report_anchor_id = material_report.get("anchor_id")
require(report_anchor_id, f"material report: missing resolved anchor_id: {material_report}")
if not material_report_only:
    require(
        str(plan.get("aweme_id")) == str(report_anchor_id),
        f"plan: aweme_id={plan.get('aweme_id')!r}, want resolved anchor_id {report_anchor_id!r} for show id {aweme_id}",
    )
    if expect_plan_in_list == "true":
        require(
            str(plan_row.get("aweme_id")) == str(report_anchor_id),
            f"plans: aweme_id={plan_row.get('aweme_id')!r}, want resolved anchor_id {report_anchor_id!r}",
        )
for material in report_materials:
    require(material.get("material_id"), f"material report: material without material_id: {material}")
    require(material.get("metric_source") == "qianchuan_report_uni_promotion", f"material report: unexpected metric source: {material}")
    require(
        material.get("base_cost") is not None and material.get("total_cost") is not None,
        f"material report: missing base_cost/total_cost: {material}",
    )
    require(
        close(material.get("cost"), material.get("base_cost")),
        f"material report: cost must default to base_cost: {material}",
    )
    require(
        "basic_stat_cost" in str(material.get("metric_basis") or ""),
        f"material report: missing metric basis: {material}",
    )
require(
    all(
        report_materials[index].get("base_cost") >= report_materials[index + 1].get("base_cost")
        for index in range(len(report_materials) - 1)
    ),
    "material report: rows are not sorted by base_cost desc",
)

if not material_report_only:
    live = rpc_result(load(live_path), "qianchuan_live_overview")
    require(live.get("live_room_id") == live_room_id, f"live overview: live_room_id={live.get('live_room_id')!r}, want {live_room_id}")
    require(live.get("account", {}).get("advertiser_id") == account_id, f"live overview: advertiser mismatch: {live.get('account')}")
    require("live" in live, "live overview: missing live payload")
    if live.get("live"):
        print("OK live overview returned current-room metrics")
    elif live.get("live") == {} and live.get("warnings"):
        print("OK live overview API succeeded; current room is offline or metrics are not yet populated")
    else:
        raise SystemExit(f"live overview: unexpected live payload: {live.get('live')!r}")

print(f"OK initialize + {len(required_read_tools)} read-only tools")
if not material_report_only:
    print(f"OK account {account_id} / aweme {aweme_id} / room {live_room_id}; writes disabled")
    print(f"OK plan {plan_id}: budget={plan.get('budget')}, roi2_goal={plan.get('roi2_goal')}, anchor={report_anchor_id}")
    print(f"OK plan materials={plan_materials.get('material_count')}; protected_top3={len(protected_top3)}")
print(f"OK material report rows={len(report_materials)}")
PY

printf 'PASS: Qianchuan MCP read-only self-check\n'
