# UGC 回调 API

社区 / 第三方内容入库入口，由 Core 接收后通过 **NATS** 异步投递给 Worker。流水线说明见 [系统逻辑总览 · 流程 A](../architecture/系统逻辑总览.md#4-流程-augc-标准入库)。

**Agent 快速导入**（粘贴/文件、自动识别类型与标题、去重更新）见 [`smart-import.md`](smart-import.md)。

## `POST /api/v1/ugc/notify`

### 请求

`extensions` 为可选自定义 JSON（文章 ID、地理坐标等），见 [`extensions.md`](extensions.md)。

```json
{
  "kb_type": "tutorial",
  "title": "新功能介绍",
  "content": "GrayDraft 支持 UGC 异步入库与 index_ready 索引就绪。",
  "extensions": {
    "article_id": "post_1001",
    "article_type": "announcement"
  },
  "source_url": "https://docs.example.edu/internvl",
  "mime_type": "text/plain",
  "generate_qa": true,
  "tags": ["公告"]
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `kb_type` | 是 | `tutorial` / `paper` / `note` |
| `title` | 是 | 文档标题 |
| `content` | 是 | 正文 |
| `source_url` | 否 | 来源链接 |
| `doc_id` | 否 | 预分配文档 UUID |
| `generate_qa` | 否 | 默认 `true` |

### 响应 `202 Accepted`

```json
{
  "task_id": "uuid",
  "accepted": true,
  "channel": "nats",
  "message": "queued for async ingestion"
}
```

| `channel` | 含义 |
|-----------|------|
| `jetstream` | 已持久化发布到 `GRAYDRAFT_UGC` / `graydraft.ugc.ingest` |
| `nats` | Core NATS 发布（未启用 JetStream 时） |
| `sync_fallback` | NATS 不可用，已同步调用 Worker |

## `GET /api/v1/ugc/tasks/:task_id`

查询异步任务状态：`pending` → `processing` → `completed` / `failed`

可选参数：

| 参数 | 说明 |
|---|---|
| `wait=true` | 阻塞等待任务达到 `queryable=true` 或终态，适合 Agent 导入后立即问答前的 ready barrier |
| `timeout_ms` | 等待超时，默认 `10000`，最大 `30000` |

响应会包含 `queryable`、`readiness`、`readiness_validation`，用于区分「仍在索引」和「知识库确实没有命中」。

## `GET /api/v1/ugc/tasks/:task_id/events`

异步任务 ready barrier 的 SSE 订阅接口。适合 Agent / 前端在 `smart-import` 返回 `readiness.status=queued` 后订阅，不必自行高频轮询。

可选参数：

| 参数 | 说明 |
|---|---|
| `timeout_ms` | 订阅超时，默认 `10000`，最大 `30000` |
| `interval_ms` | 状态检查间隔，默认 `500`，范围 `100`–`5000` |

事件：

| event | data |
|---|---|
| `status` | 当前 task 状态，包含 `queryable`、`readiness`、`readiness_validation` |
| `done` | 已达到 `queryable=true` 或终态，data 同 `status` |
| `timeout` | 超时仍未达到 ready/终态，data 会带 `wait_timeout=true` |
| `error` | task 不存在或读取失败 |

## NATS / JetStream

Subject: `graydraft.ugc.ingest`（流 `GRAYDRAFT_UGC`）

与 Worker `UGCMessage` 字段一致，含 `task_id` 供状态回写；发布使用 `MsgId=task_id` 去重。

JetStream 持久化、Ack/Nak、DLQ 说明见 [`jetstream.md`](jetstream.md)。

## 流水线

```
社区 POST /api/v1/ugc/notify
  → Core 校验 kb_type、创建 ugc_ingest_tasks
  → JetStream publish（持久化）
  → Worker Pull 消费：门禁 → 分块 → 索引 → index_ready=true
  → 更新 ugc_ingest_tasks.status=completed
  （失败重试，超限 → graydraft.ugc.dlq）
```
