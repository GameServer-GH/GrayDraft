# Agent Evaluation & Cost APIs

Admin APIs for golden dataset replay, quality metrics, and model cost attribution.

## Authentication

All endpoints require `Authorization: Bearer <admin_jwt>` from `POST /api/v1/auth/login`.

## Cost summary

`GET /api/v1/admin/cost/summary`

| Query | Description |
|---|---|
| `tenant_id` | Optional tenant filter |
| `hours` | Window size (default 24, max 720) |

Response highlights:

- `daily_tokens` — UTC day token total
- `daily_token_limit` — configured budget (`budget.daily_token_limit`)
- `budget_exceeded` — true when daily total exceeds limit
- `rows[]` — grouped by `operation`, `provider`, `model`, `tenant_id`, `agent_id`

Cost events are written asynchronously from LLM / Embedding / Rerank calls into `model_cost_events`.

## Run golden evaluation

`POST /api/v1/admin/agent-evals/run`

```json
{
  "dataset_path": "",
  "tenant_id": "tenant_edu",
  "agent_id": "eval",
  "policy_version": "edu_workspace_v1",
  "use_oracle": false
}
```

- `use_oracle: true` — structural smoke test only (no real Agent Loop)
- `use_oracle: false` — replay each case through production `Launcher.Run` (default in gateway)

Results persist to `agent_eval_runs` with per-case and per-category scores.

## List evaluation runs

`GET /api/v1/admin/agent-evals?tenant_id=&limit=20`

## Quality dashboard

`GET /api/v1/admin/agent-evals/quality?tenant_id=&days=7&limit=10`

Returns aggregated pass rate, category breakdown, tool/refusal metrics, and recent runs.

## Scheduling

Gateway honors:

| Env | Purpose |
|---|---|
| `AGENT_EVAL_INTERVAL_HOURS` | Periodic golden replay |
| `AGENT_EVAL_USE_ORACLE=1` | Scheduled runs use oracle only |
| `AGENT_EVAL_DATASET_ROOT` | Path to `golden.json` directory |
| `AGENT_EVAL_AUTHORIZED_KBS` | Comma-separated KB ids for replay |
| `BUDGET_DAILY_TOKEN_LIMIT` | Daily token budget alert threshold |

## Ops proxy

`internal/ops` mirrors these under `/v1/cost/summary`, `/v1/agent-evals`, `/v1/agent-evals/run`, `/v1/agent-evals/quality`.
