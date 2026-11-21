#!/usr/bin/env bash
set -euo pipefail

MCP_URL="${QIANCHUAN_MCP_LOCAL_URL:-http://127.0.0.1:18081/qianchuan-app/mcp}"
KEY_FILE="${QIANCHUAN_MCP_KEY_FILE:-/opt/go-qianchuan/tenants/owner.key}"
ACCOUNT_ID="${QIANCHUAN_ACCOUNT_ID:-1855810750424170}"
PRODUCT_ID="${QIANCHUAN_PRODUCT_CARD_PRODUCT_ID:-3826314764871664464}"
MAX_PRODUCT_ATTEMPTS="${QIANCHUAN_PRODUCT_CARD_MAX_ATTEMPTS:-12}"
TITLE_OVERRIDE="${QIANCHUAN_PRODUCT_CARD_TITLE:-}"
SELLING_POINT="${QIANCHUAN_PRODUCT_CARD_SELLING_POINT:-温和清洁不紧绷}"
DEBUG_DIR="${QIANCHUAN_MCP_DEBUG_DIR:-}"

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

[[ -f "$KEY_FILE" ]] || fail "missing MCP key file: $KEY_FILE"
key="$(tr -d '\r\n' <"$KEY_FILE")"
[[ -n "$key" ]] || fail "empty MCP key file: $KEY_FILE"

work="$(mktemp -d "${TMPDIR:-/tmp}/qianchuan-product-card-crud.XXXXXX")"
created_ad_id=""

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
  if [[ -n "$created_ad_id" ]] && (( status != 0 )); then
    local args
    args="$(python3 - "$ACCOUNT_ID" "$created_ad_id" <<'PY'
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
    printf 'CLEANUP: deleting product-card test plan %s\n' "$created_ad_id" >&2
    call_tool qianchuan_delete_test_product_card_plan "$args" >/dev/null || true
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

products_file="$work/available-products.jsonl"
: >"$products_file"
product_page=1
while true; do
  products="$(call_tool_retry qianchuan_list_available_products "{\"account_id\":\"$ACCOUNT_ID\",\"page\":$product_page,\"page_size\":30}")"
  printf '%s\n' "$products" >>"$products_file"
  has_next="$(
    python3 - "$products" "$product_page" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
page_info = result.get("page_info") or {}
requested = int(sys.argv[2])
current = int(page_info.get("page") or requested)
total = int(page_info.get("total_page") or 0)
if current != requested:
    raise SystemExit(
        f"official product list returned page {current} for requested page {requested}"
    )
print("1" if total > current else "0")
PY
  )"
  if [[ "$has_next" != "1" ]]; then
    break
  fi
  product_page=$((product_page + 1))
done

candidates_file="$work/product-candidates.json"
python3 - "$products_file" "$candidates_file" "$PRODUCT_ID" <<'PY'
import json
import pathlib
import sys

products_path = pathlib.Path(sys.argv[1])
candidates_path = pathlib.Path(sys.argv[2])
preferred_id = sys.argv[3]
products = []
product_count = 0
for line in products_path.read_text(encoding="utf-8").splitlines():
    product_page = json.loads(line)
    page_products = product_page.get("products") or []
    product_count += len(page_products)
    products.extend(page_products)
page_info = json.loads(
    products_path.read_text(encoding="utf-8").splitlines()[0]
).get("page_info") or {}
reported_total = int(page_info.get("total_number") or 0)
if reported_total and product_count != reported_total:
    raise SystemExit(
        f"official product list returned {product_count} of {reported_total} products; "
        "candidate rotation cannot be certified against a partial list"
    )
seen = set()
ordered = []
for product in products:
    product_id = str(product.get("product_id") or "")
    if not product_id or product_id in seen:
        continue
    name = str(product.get("name") or "").strip()
    if len(name) < 10:
        continue
    seen.add(product_id)
    ordered.append({
        "product_id": product_id,
        "name": name,
    })
ordered.sort(key=lambda item: item["product_id"] != preferred_id)
if not ordered:
    raise SystemExit("official available-product list returned no named products")
candidates_path.write_text(
    json.dumps(ordered, ensure_ascii=False, indent=2),
    encoding="utf-8",
)
print(f"OK collected {len(ordered)} product candidates; preferred={preferred_id}")
PY

images="$(call_tool qianchuan_list_image_library "{\"account_id\":\"$ACCOUNT_ID\",\"image_mode\":\"SQUARE\",\"page\":1,\"page_size\":100}")"
image_id="$(python3 - "$images" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
for image in result.get("images") or []:
    image_id = str(image.get("id") or "")
    if image_id and image.get("image_mode") == "SQUARE":
        print(image_id)
        break
else:
    raise SystemExit(f"no existing SQUARE image is available: {result}")
PY
)"
printf 'OK selected existing SQUARE image %s\n' "$image_id"

suffix="$(TZ=Asia/Shanghai date +%Y%m%d-%H%M%S)"
name="Codex-CRUD-PRODUCT-CARD-TEST-$suffix"
renamed="$name-UPD"
attempts_file="$work/attempts.jsonl"
: >"$attempts_file"
create=""
candidate_count="$(python3 - "$candidates_file" <<'PY'
import json
import sys

print(len(json.load(open(sys.argv[1], encoding="utf-8"))))
PY
)"
if (( candidate_count < 1 )); then
  fail "no official product candidates are available"
fi
if (( MAX_PRODUCT_ATTEMPTS < 1 )); then
  fail "QIANCHUAN_PRODUCT_CARD_MAX_ATTEMPTS must be positive"
fi
attempt_limit="$MAX_PRODUCT_ATTEMPTS"
if (( attempt_limit > candidate_count )); then
  attempt_limit="$candidate_count"
fi
for ((attempt_index = 0; attempt_index < attempt_limit; attempt_index++)); do
  selection="$(python3 - "$candidates_file" "$attempt_index" "$TITLE_OVERRIDE" <<'PY'
import json
import sys

candidates = json.load(open(sys.argv[1], encoding="utf-8"))
index = int(sys.argv[2])
title_override = sys.argv[3].strip()
candidate = candidates[index]
print(json.dumps({
    "product_id": candidate["product_id"],
    "title": title_override or candidate["name"],
}, ensure_ascii=False, separators=(",", ":")))
PY
)"
  PRODUCT_ID="$(python3 - "$selection" <<'PY'
import json
import sys

print(json.loads(sys.argv[1])["product_id"])
PY
)"
  TITLE="$(python3 - "$selection" <<'PY'
import json
import sys

print(json.loads(sys.argv[1])["title"])
PY
)"
  create_args="$(python3 - "$ACCOUNT_ID" "$name" "$PRODUCT_ID" "$image_id" "$TITLE" "$SELLING_POINT" <<'PY'
import json
import sys

account_id, name, product_id, image_id, title, selling_point = sys.argv[1:]
print(json.dumps({
    "account_id": account_id,
    "name": name,
    "product_id": product_id,
    "image_id": image_id,
    "title": title,
    "selling_points": [selling_point],
    "budget": 30,
    "confirm": True,
}, ensure_ascii=False, separators=(",", ":")))
PY
)"
  printf 'TRY %d/%d: product %s (%s)\n' \
    "$((attempt_index + 1))" "$attempt_limit" "$PRODUCT_ID" "$TITLE" >&2
  if create="$(call_tool_retry qianchuan_create_test_product_card_plan "$create_args")" 2>"$work/create-error.txt"; then
    python3 - "$attempts_file" "$PRODUCT_ID" "$TITLE" "success" "" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
record = {
    "product_id": sys.argv[2],
    "title": sys.argv[3],
    "result": sys.argv[4],
    "error": sys.argv[5],
}
with path.open("a", encoding="utf-8") as handle:
    handle.write(json.dumps(record, ensure_ascii=False) + "\n")
PY
    break
  fi
  create_error="$(cat "$work/create-error.txt")"
  python3 - "$attempts_file" "$PRODUCT_ID" "$TITLE" "failed" "$create_error" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
record = {
    "product_id": sys.argv[2],
    "title": sys.argv[3],
    "result": sys.argv[4],
    "error": sys.argv[5],
}
with path.open("a", encoding="utf-8") as handle:
    handle.write(json.dumps(record, ensure_ascii=False) + "\n")
PY
  printf 'SKIP: product %s is currently occupied by product-overall delivery\n' "$PRODUCT_ID" >&2
done
if [[ -z "$create" ]]; then
  fail "all $attempt_limit product-card candidates were rejected by product-overall delivery conflicts"
fi
created_ad_id="$(python3 - "$create" "$image_id" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
expected_image_id = sys.argv[2]
if result.get("created") is not True:
    raise SystemExit(f"create failed: {result}")
if result.get("verified") is not True:
    raise SystemExit(f"create readback did not verify: {result}")
if result.get("paused_immediately") is not True:
    raise SystemExit(f"create was not immediately paused: {result}")
if result.get("plan_type") != "PURE_PRODUCT_CARD":
    raise SystemExit(f"unexpected plan type: {result.get('plan_type')!r}")
plan = result.get("plan") or {}
if plan.get("status") != "DISABLE" or plan.get("opt_status") != "DISABLE":
    raise SystemExit(f"created plan is not paused: {result}")
evidence = result.get("creative_evidence") or {}
if evidence.get("expected_image_id") != expected_image_id:
    raise SystemExit(f"creative evidence has unexpected image ID: {result}")
if evidence.get("video_material_count") not in (0, None):
    raise SystemExit(f"pure product card contains video: {result}")
ad_id = str(result.get("ad_id") or "")
if not ad_id:
    raise SystemExit(f"create returned no ad_id: {result}")
print(ad_id)
PY
)"
printf 'OK created, paused, and structurally verified product-card plan %s\n' "$created_ad_id"

rename_args="$(python3 - "$ACCOUNT_ID" "$created_ad_id" "$renamed" <<'PY'
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
rename="$(call_tool qianchuan_rename_test_product_card_plan "$rename_args")"
python3 - "$rename" "$renamed" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
if result.get("renamed") is not True:
    raise SystemExit(f"rename failed: {result}")
if result.get("verified") is not True:
    raise SystemExit(f"rename readback did not verify: {result}")
if result.get("actual_name") != sys.argv[2]:
    raise SystemExit(f"unexpected renamed plan: {result}")
if result.get("status") != "DISABLE" or result.get("opt_status") != "DISABLE":
    raise SystemExit(f"plan is no longer paused after rename: {result}")
PY
printf 'OK renamed and read back product-card plan %s\n' "$created_ad_id"

delete_args="$(python3 - "$ACCOUNT_ID" "$created_ad_id" <<'PY'
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
deleted="$(call_tool qianchuan_delete_test_product_card_plan "$delete_args")"
python3 - "$deleted" <<'PY'
import json
import sys

result = json.loads(sys.argv[1])
if result.get("deleted") is not True:
    raise SystemExit(f"delete failed: {result}")
if result.get("verified") is not True:
    raise SystemExit(f"delete readback did not verify: {result}")
evidence = result.get("verify_evidence") or {}
statuses = {
    str(evidence.get("detail_status") or "").upper(),
    str(evidence.get("detail_opt_status") or "").upper(),
    str(evidence.get("list_status") or "").upper(),
    str(evidence.get("list_opt_status") or "").upper(),
}
if not statuses & {"DELETE", "DELETED"}:
    raise SystemExit(f"delete lacks explicit DELETE evidence: {result}")
PY
created_ad_id=""
printf 'OK delete and explicit DELETE readback verified\n'

if [[ -n "$DEBUG_DIR" ]]; then
  mkdir -p "$DEBUG_DIR"
  cp "$candidates_file" "$DEBUG_DIR/"
  cp "$attempts_file" "$DEBUG_DIR/"
  cp "$work/qianchuan_list_image_library.json" "$DEBUG_DIR/"
  cp "$work/qianchuan_create_test_product_card_plan.json" "$DEBUG_DIR/"
  cp "$work/qianchuan_rename_test_product_card_plan.json" "$DEBUG_DIR/"
  cp "$work/qianchuan_delete_test_product_card_plan.json" "$DEBUG_DIR/"
fi

printf 'PASS: Qianchuan MCP pure product-card CRUD round trip\n'
