#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8443}"

check() {
  local name=$1 url=$2
  if curl -sf "$url" >/dev/null; then
    echo "OK  $name"
  else
    echo "FAIL $name ($url)"
    return 1
  fi
}

check "nginx" "${BASE_URL}/~status" || true
check "core"  "${BASE_URL}/api/health" || true
