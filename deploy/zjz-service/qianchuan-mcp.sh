#!/bin/sh
set -eu

ENV_FILE="${QIANCHUAN_MCP_ENV_FILE:-/opt/go-qianchuan/backend/.env}"
MCP_BIN="${QIANCHUAN_MCP_BIN:-/opt/go-qianchuan/backend/bin/qianchuan-mcp}"

env_value() {
	key="$1"
	awk -v key="$key" '
		$0 ~ "^[[:space:]]*" key "[[:space:]]*=" {
			value = $0
			sub(/^[^=]*=/, "", value)
			sub(/\r$/, "", value)
			sub(/[[:space:]]+$/, "", value)
			if (value ~ /^".*"$/ || value ~ /^'\''.*'\''$/) {
				value = substr(value, 2, length(value) - 2)
			}
			print value
			exit
		}
	' "$ENV_FILE"
}

if [ ! -r "$ENV_FILE" ]; then
	echo "qianchuan-mcp: environment file is not readable: $ENV_FILE" >&2
	exit 1
fi

if [ ! -x "$MCP_BIN" ]; then
	echo "qianchuan-mcp: binary is not executable: $MCP_BIN" >&2
	exit 1
fi

api_key="$(env_value MCP_API_KEY)"
if [ -z "$api_key" ]; then
	echo "qianchuan-mcp: MCP_API_KEY is missing from $ENV_FILE" >&2
	exit 1
fi

base_url="$(env_value QIANCHUAN_MCP_BASE_URL)"
if [ -z "$base_url" ]; then
	base_url="http://127.0.0.1:18081/api/mcp"
fi

exec env \
	MCP_API_KEY="$api_key" \
	QIANCHUAN_MCP_BASE_URL="$base_url" \
	"$MCP_BIN"
