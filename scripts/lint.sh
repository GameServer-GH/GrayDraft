#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if command -v golangci-lint >/dev/null; then
  golangci-lint run "$ROOT/internal/core/..." "$ROOT/internal/gateway/..." "$ROOT/pkg/gocommon/..." || true
fi

if command -v ruff >/dev/null; then
  ruff check "$ROOT/internal/worker" "$ROOT/internal/ops" || true
fi

echo "Lint complete."
