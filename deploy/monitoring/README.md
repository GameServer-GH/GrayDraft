# GrayDraft Monitoring

## Prometheus（dev）

```bash
docker compose -f deploy/docker-compose/docker-compose.dev.yml \
  --profile monitoring \
  --env-file deploy/config/dev/.env.ports up -d prometheus grafana core
```

- Scrape: Core `/metrics`（含 `graydraft_dlq_messages`、`graydraft_chat_retrieval_gate_total`、`graydraft_chat_waterfall_total`、`graydraft_chat_llm_empty_total`、`graydraft_ugc_task_index_lag_seconds`）
- Rules: `prometheus/alerts.yml`（O-03 DLQ 积压告警）
- Dashboard: `grafana/dashboards/graydraft-rag.json` 覆盖 retrieval/cache、no-useful gate、4D waterfall、LLM empty、UGC index lag。

## DLQ 告警

| Alert | 条件 | 说明 |
|-------|------|------|
| `GrayDraftDLQMessagesPresent` | `graydraft_dlq_messages > 0` 持续 5m | 有死信需人工处理 |
| `GrayDraftDLQBacklogHigh` | `>= 10` 持续 2m | 严重积压 |

在 Grafana 中配置 Alertmanager / 通知渠道，或查看 Prometheus **Alerts** 页（规则已加载即会显示 pending/firing 状态）。
