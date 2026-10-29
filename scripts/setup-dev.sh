#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

chmod +x deploy/scripts/*.sh scripts/*.sh 2>/dev/null || true

if [[ -z "$(grep -E '^DEEPSEEK_API_KEY=.+' deploy/config/dev/.env.gateway 2>/dev/null || true)" ]]; then
  echo "Tip: set DEEPSEEK_API_KEY in deploy/config/dev/.env.gateway and .env.core"
fi

echo "Dev environment ready. Run: make dev-up"
