# DLQ 运维 API

管理 JetStream 死信队列 `GRAYDRAFT_UGC_DLQ`（subject: `graydraft.ugc.dlq`）。Worker 在 UGC 消费超过 `max_deliver` 后将消息写入 DLQ。

## 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/admin/dlq/stats` | 流统计（消息数、seq 范围） |
| GET | `/api/v1/admin/dlq/messages?limit=50` | 列表（新→旧） |
| POST | `/api/v1/admin/dlq/replay` | 重放回 `graydraft.ugc.ingest` |
| POST | `/api/v1/admin/dlq/discard` | 从 DLQ 删除 |

需 Core 启用 `nats.jetstream.enabled` 且 NATS 可连。

## 重放

```json
POST /api/v1/admin/dlq/replay
{ "sequences": [12, 11] }
```

行为：

1. 读取 DLQ 消息体（`UGCQueueMessage` JSON）
2. 重新 `Publish` 到 ingest subject（带 `MsgId=task_id` 去重）
3. 从 DLQ 流 **DeleteMsg**
4. 若含 `task_id`，将 `ugc_ingest_tasks` 中 `failed`/`processing` 任务重置为 `pending`

响应：

```json
{
  "results": [
    { "sequence": 12, "task_id": "uuid", "ok": true },
    { "sequence": 11, "ok": false, "error": "..." }
  ]
}
```

## 丢弃

```json
POST /api/v1/admin/dlq/discard
{ "sequences": [10] }
```

仅从 DLQ 删除，不重新投递。

## Console

Console **DLQ** 页支持列表、多选重放/丢弃。见 [`console.md`](console.md)。

## 监控与告警（O-03）

Core 暴露 Prometheus 指标 `graydraft_dlq_messages`（JetStream `GRAYDRAFT_UGC_DLQ` 当前条数），每 30s 轮询更新。

告警规则见 `deploy/monitoring/prometheus/alerts.yml`：

- `GrayDraftDLQMessagesPresent` — 有死信持续 5 分钟
- `GrayDraftDLQBacklogHigh` — 积压 ≥ 10 条持续 2 分钟

启用方式见 [`deploy/monitoring/README.md`](../../deploy/monitoring/README.md)。

## 相关

- [`jetstream.md`](jetstream.md)
- [`ugc.md`](ugc.md)
