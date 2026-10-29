# NATS JetStream（UGC 持久化）

UGC 异步入库使用 **JetStream** 持久化消息，支持重启不丢、显式 Ack、重试与 DLQ。

## 流与消费者

| 资源 | 名称 | 说明 |
|------|------|------|
| 主流 | `GRAYDRAFT_UGC` | Subject: `graydraft.ugc.ingest`（勿用 `graydraft.ugc.>`，会与 DLQ 重叠） |
| 入库 Subject | `graydraft.ugc.ingest` | Core 发布、Worker 消费 |
| DLQ 流 | `GRAYDRAFT_UGC_DLQ` | Subject: `graydraft.ugc.dlq` |
| 消费者 | `graydraft-worker-ugc` | Pull、显式 Ack，`max_deliver=5` |

## 行为

1. **Core** 启动时 `AddStream`（幂等），`js.Publish` 带 `MsgId=task_id` 去重
2. **Worker** Pull 拉取，成功 `ack`；失败 `nak`（指数退避）；超过 `max_deliver` 写入 DLQ 并 `ack` 移除
3. **NATS 服务** 需 `-js` 与持久化目录 `-sd /data`（compose 已配置 volume）

## 配置（Core `config.dev.yaml`）

```yaml
nats:
  jetstream:
    enabled: true
    stream: GRAYDRAFT_UGC
    subjects: [graydraft.ugc.ingest]
    dlq_stream: GRAYDRAFT_UGC_DLQ
    dlq_subject: graydraft.ugc.dlq
```

环境变量：`NATS_JETSTREAM_ENABLED`、`NATS_STREAM`、`NATS_DLQ_SUBJECT`

## 验证

```bash
# 1. 启动基础设施 + 服务
docker compose -f deploy/docker-compose/docker-compose.dev.yml \
  --env-file deploy/config/dev/.env.ports up -d postgres nats redis worker core

# 2. 投递 UGC
curl -X POST http://localhost:18080/api/v1/ugc/notify \
  -H 'Content-Type: application/json' \
  -d '{"kb_type":"tutorial","title":"JS测试","content":"JetStream 持久化入库验证。"}'

# 预期 channel=jetstream，任务最终 completed

# 3. 查看 JetStream（需 NATS CLI 或监控 :28222）
curl -s http://localhost:28222/jsz?consumers=1 | head
```

## 关闭 JetStream（回退 Core NATS）

Worker/Core 配置 `jetstream.enabled: false`；Worker 将使用 Core NATS 订阅（无持久化，仅开发兜底）。

## DLQ 运维

死信列表、重放与丢弃见 [`dlq.md`](dlq.md)；Console **DLQ** 页提供可视化操作。

## 相关文档

- [`ugc.md`](ugc.md) — UGC API
- [`worker.md`](worker.md) — Worker 入库
- [`dlq.md`](dlq.md) — DLQ Admin API
