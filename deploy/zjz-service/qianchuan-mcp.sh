#!/bin/sh
set -eu

MCP_BIN="${QIANCHUAN_MCP_BIN:-/opt/go-qianchuan/backend/bin/qianchuan-mcp}"

if [ "$#" -ne 0 ]; then
	echo "qianchuan-mcp: tenant arguments are not accepted; provide the caller's key through QIANCHUAN_TENANT_KEY" >&2
	exit 2
fi

if [ ! -x "$MCP_BIN" ]; then
	echo "qianchuan-mcp: binary is not executable: $MCP_BIN" >&2
	exit 1
fi

api_key="${QIANCHUAN_TENANT_KEY:-}"
if [ -z "$api_key" ]; then
	echo "qianchuan-mcp: QIANCHUAN_TENANT_KEY is required in the caller environment" >&2
	exit 1
fi

base_url="${QIANCHUAN_MCP_BASE_URL:-http://127.0.0.1:18081/qianchuan-app/mcp}"

exec env \
	QIANCHUAN_TENANT_KEY="$api_key" \
	QIANCHUAN_MCP_BASE_URL="$base_url" \
	"$MCP_BIN"
