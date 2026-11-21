#!/usr/bin/env python3
import json
import os
import pathlib
import sys
import urllib.request


MCP_URL = os.environ.get("QIANCHUAN_MCP_LOCAL_URL", "http://127.0.0.1:18081/mcp")
KEY_FILE = pathlib.Path(
    os.environ.get("QIANCHUAN_MCP_KEY_FILE", "/opt/go-qianchuan/tenants/owner.key")
)
ACCOUNT_ID = os.environ.get("QIANCHUAN_ACCOUNT_ID", "1855810750424170")
AD_ID = os.environ["QIANCHUAN_EXISTING_PRODUCT_PLAN_ID"]
IMAGE_ID = os.environ["QIANCHUAN_EXISTING_IMAGE_ID"]
TITLE_TEXT = os.environ.get("QIANCHUAN_EXISTING_TITLE", "红卫羊脂皂五盒装")
TEST_BUDGET = float(os.environ.get("QIANCHUAN_EXISTING_PLAN_TEST_BUDGET", "301"))
BOOST_BUDGET = float(os.environ.get("QIANCHUAN_EXISTING_BOOST_BUDGET", "1"))


def rpc(method, params, request_id):
    body = json.dumps(
        {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params},
        ensure_ascii=False,
    ).encode()
    request = urllib.request.Request(
        MCP_URL,
        data=body,
        method="POST",
        headers={
            "Authorization": "Bearer " + KEY_FILE.read_text(encoding="utf-8").strip(),
            "Content-Type": "application/json",
        },
    )
    with urllib.request.urlopen(request, timeout=90) as response:
        envelope = json.load(response)
    if envelope.get("error"):
        raise RuntimeError(f"{method}: {envelope['error']}")
    return envelope["result"]


REQUEST_ID = 1000


def call_tool(name, arguments):
    global REQUEST_ID
    REQUEST_ID += 1
    result = rpc("tools/call", {"name": name, "arguments": arguments}, REQUEST_ID)
    content = result.get("content") or []
    text = content[0].get("text", "") if content else ""
    if result.get("isError") is True:
        raise RuntimeError(f"{name}: {text}")
    try:
        payload = json.loads(text)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"{name}: invalid JSON response: {exc}: {text[:500]}") from exc
    if payload.get("error") is not None:
        raise RuntimeError(f"{name}: {payload}")
    return payload


def main():
    status = call_tool("qianchuan_get_status", {"account_id": ACCOUNT_ID})
    if not status.get("authenticated") or status.get("advertiser_id") != ACCOUNT_ID:
        raise RuntimeError(f"account is not authenticated: {status}")
    if not status.get("plan_crud_test_write_enabled"):
        raise RuntimeError("PLAN_CRUD_TEST_WRITE_ENABLED is false")
    print("OK MCP authenticated; guarded plan test writes enabled")

    before = call_tool(
        "qianchuan_get_plan", {"account_id": ACCOUNT_ID, "ad_id": AD_ID}
    ).get("plan") or {}
    if before.get("marketing_goal") != "VIDEO_PROM_GOODS":
        raise RuntimeError(f"wrong marketing_goal: {before.get('marketing_goal')}")
    if (
        str(before.get("status") or "").upper() != "DISABLE"
        or str(before.get("opt_status") or "").upper() != "DISABLE"
    ):
        raise RuntimeError(
            "plan must be paused: "
            f"status={before.get('status')} opt_status={before.get('opt_status')}"
        )
    if not before.get("product_infos"):
        raise RuntimeError("plan detail does not expose product_infos")
    has_image = any(
        creative.get("image_material")
        for creative in before.get("multi_product_creative_list") or []
    )
    isolated = (
        before.get("adlab_scene") == "OVERALL_PROJECT"
        and str(before.get("name") or "").startswith("Codex-CRUD-OVERALL-TEST-")
    )
    if not has_image and not isolated:
        raise RuntimeError("plan is neither image-based nor an isolated overall test")
    print(
        "OK paused image test target: "
        + json.dumps(
            {
                "ad_id": before.get("ad_id"),
                "name": before.get("name"),
                "adlab_scene": before.get("adlab_scene"),
                "budget": before.get("budget"),
                "materials": before.get("multi_product_creative_list"),
            },
            ensure_ascii=False,
        )
    )

    requested_name = (
        "Codex-CRUD-EXISTING-PLAN-TEST-"
        + __import__("datetime").datetime.now(__import__("datetime").timezone(__import__("datetime").timedelta(hours=8))).strftime("%Y%m%d-%H%M%S")
    )
    result = call_tool(
        "qianchuan_test_existing_product_plan_operations",
        {
            "account_id": ACCOUNT_ID,
            "ad_id": AD_ID,
            "name": requested_name,
            "budget": TEST_BUDGET,
            "boost_budget": BOOST_BUDGET,
            "add_image_ids": [IMAGE_ID],
            "add_titles": [TITLE_TEXT],
            "confirm": True,
        },
    )
    actions = [step.get("action") for step in result.get("steps") or []]
    required = [
        "update_budget",
        "add_materials",
        "delete_test_added_materials",
        "enable_isolated_test_plan",
        "create_and_stop_one_click_boost",
        "restore_plan_status",
    ]
    if actions != required:
        raise RuntimeError(f"unexpected operation sequence: {actions}")
    if result.get("verified") is not True:
        raise RuntimeError(f"operation verification failed: {result}")
    restored = result.get("restored") or {}
    if restored.get("budget") != result.get("original_budget"):
        raise RuntimeError(f"budget not restored: {restored}")
    if restored.get("material_ids") != result.get("original_material_ids"):
        raise RuntimeError(f"materials not restored: {restored}")
    print(
        json.dumps(
            {
                "ok": True,
                "ad_id": result.get("ad_id"),
                "steps": result.get("steps"),
                "restored": restored,
            },
            ensure_ascii=False,
            indent=2,
        )
    )


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        raise
