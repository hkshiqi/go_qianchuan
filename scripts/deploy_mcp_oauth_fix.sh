#!/usr/bin/env bash
set -euo pipefail

expected="${EXPECTED_SHA256:?EXPECTED_SHA256 is required}"
source_binary="${SOURCE_BINARY:-/tmp/qianchuan-server-oauthfix-linux-amd64}"
service_root="${SERVICE_ROOT:-/opt/go-qianchuan}"
live_binary="$service_root/backend/bin/qianchuan-server"
backup_binary="$live_binary.backup-20261120-oauthfix"

actual="$(sha256sum "$source_binary" | awk '{print $1}')"
[[ "$actual" == "$expected" ]] || {
  printf 'FAIL: uploaded binary SHA-256 %s, want %s\n' "$actual" "$expected" >&2
  exit 1
}

cp -p "$live_binary" "$backup_binary"
install -m 0755 "$source_binary" "$live_binary"
systemctl restart qianchuan-backend

for _ in $(seq 1 30); do
  if curl -fsS http://127.0.0.1:18081/health >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

systemctl is-active qianchuan-backend
curl -fsS http://127.0.0.1:18081/health
printf '\n'
sha256sum "$live_binary" "$backup_binary"
