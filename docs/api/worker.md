# Worker API

AI Worker 服务默认端口：`18001`。

## 健康检查

`GET /health`

## 文档解析

`POST /v1/parsing/parse` — HTML/Markdown/纯文本 → 规范化文本

`POST /v1/parsing/chunk` — 语义分块预览

## 重排

`POST /v1/rerank/l1` — 轻量重排（默认 Top 15）

`POST /v1/rerank/l2` — 精准重排（默认 Top 5）

开发环境使用词法重排；生产可替换为 BGE 等模型。

## 质量门禁

`POST /v1/quality-gate/check`

## QA 生成

`POST /v1/qa/generate`

## 入库流水线

`POST /v1/ingest` — 完整流水线：门禁 → 解析 → 分块 → 写库 → 向量 → `index_ready=true`

`POST /v1/ingest/ugc` — 社区 UGC 同步入库（等同 NATS 消息体）

### Ingest 示例

```bash
curl -X POST http://localhost:18001/v1/ingest \
  -H 'Content-Type: application/json' \
  -d '{
    "kb_type": "tutorial",
    "title": "新功能介绍",
    "content": "GrayDraft 支持自适应混合检索与 index_ready 索引就绪信号。",
    "generate_qa": true,
    "auto_index": true
  }'
```

## NATS / JetStream

- Subject: `graydraft.ugc.ingest`（流 `GRAYDRAFT_UGC`）
- 消费者: `graydraft-worker-ugc`（Pull + 显式 Ack）
- Payload: 与 `UGCMessage` 相同 JSON（含 `trace_id`、`task_id`）
- Headers: `X-Trace-Id` / `X-Request-ID`（与 body `trace_id` 一致）、`X-Task-Id`

Core 经 JetStream 持久化发布后，Worker 拉取消费并完成索引；失败重试，超限进入 `graydraft.ugc.dlq`。详见 [`jetstream.md`](jetstream.md)。

## 链路 trace（O-02）

| 环节 | 行为 |
|------|------|
| GrayDraft BFF | 入站无 `X-Trace-Id` 时生成 UUID；响应头回写；转发 GrayDraft 时带同一 id |
| Core Gateway | `middleware.RequestID` 写入 `context` 与响应头 |
| UGC notify | 队列 JSON + NATS header 携带 `trace_id`；`task_id` 在创建任务后写入 |
| Worker | NATS 消费与 `POST /v1/ingest/ugc` 日志字段 `trace_id`、`task_id` |

排障示例：在 Core / Worker 日志中按同一 `trace_id` 过滤，从 BFF 请求到入库完成一条线串联。

## QA 生成与上下文锚点（O-01）

入库时若 `generate_qa=true`，Worker 从 `kb_documents.extensions` 读取 smart-import 写入的字段：

| 字段 | 用途 |
|------|------|
| `subject` | 问句优先包含资料主题 |
| `parent_title` | 文档级标题语境 |
| `context_enhancement` | 综合增强语境（缺省时由 game + title 拼接） |
| `semantic_summary` | 可选，参与实体抽取 |

规则层会跳过含「以上就是」「全文完」等结语套话的 chunk，避免生成「请介绍以上就是」类低质 QA。`gd_*` 分区使用教程向问句模板。
