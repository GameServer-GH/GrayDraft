# Agent 导入/问答日志记录方案

> **文档类型：设计方案与计划。** 正文含设计目标或待办，实际交付以架构、接口文档和代码为准。 [当前文档入口](../README.md)

> 目标：完整记录 smart-import 和 Agent v2 问答的流转流程，支撑准确率与召回率验证。  
> 日志输出至 `logs/` 目录；Core v2 事件与 BFF legacy SSE 均可按 `trace_id` 关联。  
> 状态：**已实施**（2026-10-28），Agent v2 Trace 见 `api/agent-v2.md`。

---

## 1. 基础设施

项目已有两套 agentlog 基础设施，本方案复用不改：

| 位置 | 用途 | 输出路径 |
|---|---|---|
| `internal/core/observability/agentlog/` | Core 服务端（smart-import, chat-stream） | `{AGENT_LOG_DIR}/logs/agent/{day}/{ts}_{traceID}_{op}_core.log` |
| `internal/graydraft/pkg/agentlog/` | GrayDraft BFF 端（smart-import, chat-stream） | `{baseDir}/logs/agent/{wsKey}/{day}/...log` |

共用能力：`Session.Start/Step/Finish` 按时间戳写结构化步骤；`SanitizeRequest/SanitizeJSON` 自动裁 base64 为 `[base64 N bytes redacted]`；单条 JSON blob 上限 96 KiB。

环境变量：`AGENT_LOG_DIR` 或 `AGENT_LOG_ROOT` 指定日志根目录；默认 `.`（项目根）。

项目根 `.gitignore` 已添加 `logs/` 忽略规则。

---

## 2. 导入流日志（Smart Import）

### 2.1 完整步骤

每条 smart-import 请求产生一个 session 文件，包含以下步骤：

```
=== GrayDraft Core Agent Session ===
operation: smart-import
trace_id: abc123def456
started_utc: 2026-10-28T06:30:00.000Z

--- [ts] request_in ---
{ 请求元数据（raw_text 长度、附件数、split、submit；base64 已裁） }

--- [ts] multimodal_preprocess ---
{
  "extractions": [ { "filename", "route", "cached", "md5" } ],
  "raw_len": 1234,
  "extraction_texts": [                            ← 新增
    {
      "filename": "screenshot.png",
      "route": "vlm",
      "md5": "abc123...",
      "text": "页面显示了用户设置面板，包含...",   ← VLM/OCR 识别全文
      "cached": false
    }
  ]
}

--- [ts] llm_call_start ---
{ "call": 1, "prompt_len": 3421, "prompt_head": "..." }

--- [ts] llm_call_done ---
{ "call": 1, "response_len": 567, "response_head": "..." }

--- [ts] analyze_result ---                         ← 新增
{
  "kb_type": "gd_myworkspace",
  "title": "更新日志 v2.3",
  "confidence": 0.85,
  "action": "create",
  "method": "rules",
  "content_head": "导入内容头 500 字符..."
}

--- [ts] submit_result ---                          ← 新增（submit=true 时）
{
  "channel": "sync_fallback",
  "doc_id": "abc-def-123",
  "task_id": "task-456"
}

--- [ts] response_out ---
{ 最终 SmartImportResponse }

--- [ts] session_end ---
{ "status": "ok" }
```

split 模式下 `analyze_result` 和 `submit_result` 额外包含 `segment`（序号）和 `total`（总数）字段。

### 2.2 改动的文件

| 文件 | 改动 |
|---|---|
| `internal/core/service/smartimport/preprocess.go:21` | 函数签名扩展：返回值增加 `[]multimodal.ExtractionResult`，携带每张图的 `Text` 全文 |
| `internal/core/service/smartimport/service.go:94` | 调用处接收 `mmResults`，`multimodal_preprocess` 步骤新增 `extraction_texts` 数组，每项含 `filename`/`route`/`md5`/`text`/`cached` |
| `internal/core/service/smartimport/service.go:210` | `runSingle` 中 `analyzeOne` 返回后新增 `analyze_result` 步骤，记录 `kb_type`/`title`/`confidence`/`action`/`method`/`content_head`（500 runes 截断） |
| `internal/core/service/smartimport/service.go:253` | `runSingle` 中 `submitPreview` 返回后新增 `submit_result` 步骤，记录 `channel`/`doc_id`/`task_id` |
| `internal/core/service/smartimport/service.go:298` | `runSplit` 中每个 `analyzeSegment` 成功后新增 `analyze_result` 步骤（含 `segment`/`total`） |
| `internal/core/service/smartimport/service.go:365` | `runSplit` 中每个 `submitPreview` 成功后新增 `submit_result` 步骤（含 `segment`） |

---

## 3. 问答流日志（Chat Stream）

### 3.1 完整步骤

```
=== GrayDraft Core Agent Session ===
operation: chat-stream
trace_id: xyz789abc012
started_utc: 2026-10-28T06:31:00.000Z

--- [ts] request_in ---
{ "query": "...", "kb_type": "...", "chat_mode": "hybrid", "top_k": 5, "session_id": "..." }

--- [ts] retrieval_chunks ---                       ← 新增
{
  "previews": [
    {
      "rank": 1,
      "doc_title": "用户手册 v2.1",
      "chunk_id": "ck-abc123",
      "score": 0.92,
      "snippet": "chunk 正文头 500 字符..."
    },
    ...
  ]
}

--- [ts] sse_event ---
{ 每个 SSE 事件（现有逻辑不变） }

--- [ts] final_answer ---                           ← 新增
{
  "answer": "根据知识库中的信息，该功能支持...",
  "answer_len": 342
}

--- [ts] session_end ---
{ "status": "ok" }
```

### 3.2 改动的文件

| 文件 | 改动 |
|---|---|
| `internal/core/model/chat.go:28` | `ChatStreamEvent` 新增 `RetrievalChunkPreviews` 字段（`json:"retrieval_chunk_previews,omitempty"`） |
| `internal/core/model/chat.go:60` | 新增 `RetrievalChunkPreview` 结构体：`Rank`/`DocTitle`/`ChunkID`/`Score`/`Snippet` |
| `internal/core/service/chat/service.go:452` | 新增 `chunkPreviews()` 辅助函数：将 `[]Chunk` 转为 `[]RetrievalChunkPreview`，snippet 截取 500 runes |
| `internal/core/service/chat/service.go:93,103` | `metaEvent` 和 `doneEvent` 中填充 `RetrievalChunkPreviews` |
| `internal/core/handler/chat.go:90` | 新增 `answerBuilder strings.Builder` 累积流式答案 |
| `internal/core/handler/chat.go:118` | meta 事件时写入 `retrieval_chunks` 步骤（含 chunk previews 数组） |
| `internal/core/handler/chat.go:125` | 新增 `case "chunk"` 分支累积答案文本 |
| `internal/core/handler/chat.go:158` | 流结束后写入 `final_answer` 步骤（完整答案文本 + rune 长度） |

---

## 4. 字段截断规范

| 字段 | 截断策略 | 原因 |
|---|---|---|
| base64 附件内容 | `[base64 N bytes redacted]` | 无意义长文本，仅保留字节数标记 |
| `extraction_texts[].text` | 不截断，完整保留 | VLM/OCR 输出通常 < 8KB |
| `analyze_result.content_head` | 500 runes 截断 | 导入内容可能很长，保留头部分析即可 |
| `retrieval_chunks[].snippet` | 500 runes 截断 + `...` | chunk 正文，保留验证召回所需的关键信息 |
| `final_answer.answer` | 不截断，完整保留 | LLM 回答全文，准确率验证必需 |
| `raw_text` | 32 KiB 截断（现有逻辑） | 过长原始文本裁剪 |
| `llm_call_*.prompt_head` / `response_head` | 1500 runes 截断（现有逻辑） | prompt/response 调试用预览 |

---

## 5. 性能考量

所有新增日志步骤均受 `if sess != nil` 门控——只有当环境变量配置了 `AGENT_LOG_DIR`/`AGENT_LOG_ROOT`（即显式启用 agent 日志）时才会执行。未启用时零开销：

- 不创建 session 文件
- 不分配额外的 map/slice
- 不执行 chunk 内容拷贝或字符串截断
- 不影响 SSE 流式输出延迟

`chunkPreviews()` 在构建 `ChatStreamEvent` 时调用，但 `RetrievalChunkPreviews` 字段带有 `omitempty`，序列化为 JSON 时才展开。客户端可通过 SSE 收到的 `retrieval_chunk_previews` 忽略该字段（JSON 反序列化时未知字段默认丢弃）。

---

## 6. 日志消费示例

### 6.1 快速查询

```bash
# 查看某天所有 import 日志
ls logs/agent/2026-10-28/*smart-import*

# 按 trace_id 查找对应 chat 日志
grep -l "xyz789" logs/agent/2026-10-28/*

# 提取某条日志中的识别文字
grep -A 20 'multimodal_preprocess' logs/agent/2026-10-28/xxx.log | grep '"text"'

# 查看某次问答的检索结果和最终答案
grep -A 30 'retrieval_chunks' logs/agent/2026-10-28/yyy.log
grep -A 5 'final_answer' logs/agent/2026-10-28/yyy.log
```

### 6.2 评估脚本骨架

```python
import json, glob, re

def parse_session(path):
    """Parse a session log into list of (step_name, payload_dict)."""
    steps = []
    with open(path) as f:
        content = f.read()
    for m in re.finditer(r'--- \[.*?\] (\w+) ---\n(.*?)(?=\n--- \[|$)', content, re.DOTALL):
        name = m.group(1)
        body = m.group(2).strip()
        try:
            payload = json.loads(body) if body.startswith('{') else None
        except json.JSONDecodeError:
            payload = None
        steps.append((name, payload))
    return steps

# Import 日志 -> 提取导入内容
for log in glob("logs/agent/*/*smart-import*"):
    steps = parse_session(log)
    for name, payload in steps:
        if name == "multimodal_preprocess" and payload:
            for item in payload.get("extraction_texts", []):
                print(f"  [IMG] {item['filename']}: {item['text'][:100]}...")

# Chat 日志 -> 提取检索 chunks 和答案
for log in glob("logs/agent/*/*chat-stream*"):
    steps = parse_session(log)
    for name, payload in steps:
        if name == "retrieval_chunks" and payload:
            for c in payload.get("previews", []):
                print(f"  [CHUNK {c['rank']}] {c['doc_title']} score={c['score']}")
        if name == "final_answer" and payload:
            print(f"  [ANSWER] ({payload['answer_len']} chars)")
```

---

## 7. 实施清单

| 优先级 | 改动 | 文件 | 状态 |
|---|---|---|---|
| P0 | 导入流：记录多模态识别文字 | `smartimport/preprocess.go`, `smartimport/service.go` | ✅ |
| P0 | 问答流：记录检索 chunk 内容 | `model/chat.go`, `chat/service.go`, `chat/handler.go` | ✅ |
| P0 | 问答流：记录最终 LLM 答案 | `chat/handler.go` | ✅ |
| P1 | 导入流：增加 analyze_result 步骤 | `smartimport/service.go` | ✅ |
| P1 | 导入流：增加 submit_result 步骤 | `smartimport/service.go` | ✅ |
| P2 | `logs/` 目录 + `.gitignore` 规则 | 项目根 | ✅ |
