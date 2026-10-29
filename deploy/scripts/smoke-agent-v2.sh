#!/usr/bin/env bash
# Create one education Agent run against local Core.
set -euo pipefail
CORE_URL="${CORE_URL:-http://localhost:18080}"
curl -sS -X POST "${CORE_URL}/api/v2/agent-runs" \
  -H 'Content-Type: application/json' \
  -H "X-Tenant-ID: ${TENANT_ID:-tenant_edu}" \
  -H "X-Actor-ID: ${ACTOR_ID:-dev}" \
  -H "X-Authorized-KBs: ${KB_TYPE:-tutorial}" \
  -d "{\"agent_id\":\"edu\",\"policy_version\":\"edu_workspace_v1\",\"idempotency_key\":\"smoke-$(date +%s)\",\"prompt\":\"${PROMPT:-InternVL 负责什么}\"}"
echo
