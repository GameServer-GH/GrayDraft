 # 入库流水线

 > **定位**：理解一条 `ugc/notify` 如何变成可检索的 chunk（Core → NATS → Worker → index_ready）
 > **前置**：[Core 服务架构](../04-architecture/01-core-service.md)（启动、作用域）
 > **代码路径**：`internal/core/service/ugc/service.go` + `internal/worker/worker/pipeline/ingest.py`

 ---

 ## 0. 全景概览

 ```
 Client
   │ POST /api/v1/ugc/notify {kb_type, title, content}
   ▼
 Core (Go)
   │ 1. 校验 kb_type 存在
   │ 2. 创建 ugc_ingest_tasks 记录 (status=pending)
   │ 3. NATS JetStream publish → subject: graydraft.ugc.ingest
   │ 4. 返回 {task_id}
   ▼
 NATS JetStream
   │ Pull consumer → Worker
   ▼
 Worker (Python)
   │ 1. quality_gate     — 长度/敏感词校验
   │ 2. parse_content    — mime → 规范化正文
   │ 3. chunk_text       — 语义分块
   │ 4. upsert_document  — 写 kb_documents (index_ready=false)
   │ 5. embed_texts      — Gateway Embedding
   │ 6. mark_index_ready — 更新 kb_documents.index_ready=true
   │ 7. update task      — status=completed
   ▼
 PostgreSQL
   kb_documents (index_ready=true) + kb_chunks (embedding 就绪)
 ```

 ---

 ## 1. 入口：POST /api/v1/ugc/notify

 代码路径：`internal/core/service/ugc/service.go` → `Notify()`

 ### 1.1 请求结构

 ```json
 {
   "kb_type":     "product",
   "title":       "如何配置 index_ready",
   "content":     "index_ready 是 GrayDraft 中...",
   "mime_type":   "text/plain",
   "tags":        ["部署", "配置"],
   "doc_id":      "uuid-or-null",
   "generate_qa": true
 }
 ```

 | 字段 | 必填 | 说明 |
|------|------|------|
| `kb_type` | ✓ | 目标分区 |
| `title` | ✓ | 文档标题 |
| `content` | ✓ | 正文 |
| `mime_type` | ✗ | 内容类型，默认 text/plain |
| `tags` | ✗ | 标签列表 |
| `doc_id` | ✗ | 指定 doc_id（强制覆盖更新模式） |
| `generate_qa` | ✗ | 是否自动生成 QA 对，默认 true |

 ### 1.2 响应

 ```json
 {
   "task_id": "550e8400-e29b-41d4-a716-446655440000",
   "channel": "jetstream"
 }
 ```

 ### 1.3 Core 处理逻辑

 ```
 Notify(req):
   1. 校验 kb_type 在 kb_types 表中存在 → 不存在返回 400
   2. 生成 task_id (UUID)
   3. 写入 ugc_ingest_tasks: {task_id, status=pending, kb_type, title, content}
   4. NATS JetStream publish:
      subject: graydraft.ugc.ingest
      body: JSON {task_id, kb_type, title, content, mime_type, tags, generate_qa}
   5. NATS 失败 → 尝试 sync_fallback (HTTP 同步调用 Worker /ingest)
   6. 两者都失败 → task status=failed
   7. 返回 {task_id, channel}
 ```

 **发布失败回退**：

 ```go
 if err := s.nats.Publish(ctx, subject, msg); err != nil {
     if s.cfg.UGC.SyncFallback {
         // HTTP 同步调用 Worker
         s.worker.Ingest(ctx, req)
     } else {
         // 标记任务失败
         s.ugcRepo.UpdateTaskStatus(ctx, taskID, "failed")
     }
 }
 ```

 ---

 ## 2. NATS JetStream 消息队列

 ### 2.1 为什么用 NATS 而不是 HTTP 直调？

 | | NATS JetStream | HTTP 直调 |
 |------|---------------|-----------|
| 可靠性 | 持久化 + 重试 + DLQ | 网络中断即丢失 |
| 削峰 | Consumer 按速率消费 | Worker 需要自己队列 |
| 可观测 | stream/consumer 监控 | 需自建 |
| 运维 | DLQ 重放/丢弃 | 需人工处理 |

 ### 2.2 Stream 配置

 ```
 Stream: GRAYDRAFT_UGC
 Subject: graydraft.ugc.ingest
 DLQ Subject: graydraft.ugc.dlq
 Retry Policy: 最多重试 3 次，指数退避
 ```

 ### 2.3 DLQ 死信队列

 消息重试耗尽后进入 DLQ（`GRAYDRAFT_UGC_DLQ`，subject `graydraft.ugc.dlq`）。

 DLQ 管理接口（需要 admin 认证）：

 ```bash
 # 查看 DLQ 统计
 curl http://localhost:8443/api/v1/admin/dlq/stats

 # 列出死信消息
 curl http://localhost:8443/api/v1/admin/dlq/messages

 # 重放指定消息
 curl -X POST http://localhost:8443/api/v1/admin/dlq/replay \
   -d '{"message_ids": ["msg-1", "msg-2"]}'

 # 丢弃指定消息
 curl -X POST http://localhost:8443/api/v1/admin/dlq/discard \
   -d '{"message_ids": ["msg-1"]}'
 ```

 ---

 ## 3. Worker 流水线（Python）

 代码路径：`internal/worker/worker/pipeline/ingest.py` → `IngestPipeline`

 ### 3.1 流水线阶段

 ```python
 class IngestPipeline:
     async def _ingest(self, req: IngestRequest) -> IngestResponse:
         # 1. 质量门禁
         gate = check_content(req.content, title=req.title,
             min_length=settings.quality_gate_min_length,
             max_length=settings.quality_gate_max_length)
         if not gate.passed:
             raise ValueError(f"quality_gate_rejected: {gate.reason}")

         # 2. 解析正文
         text, title = parse_content(req.content, req.mime_type, req.title)

         # 3. 语义分块
         chunks = chunk_text(text, settings.chunk_size, settings.chunk_overlap)

         # 4. 写文档 + 切片（index_ready=false）
         doc_id = await repo.upsert_document(kb_type, title, text, ...)
         await repo.insert_chunks(doc_id, chunks)

         # 5. Embedding 向量化
         embeddings = await embed_texts([c.text for c in chunks])

         # 6. 写回向量 + 标记 index_ready
         await repo.update_chunk_embeddings(chunk_ids, embeddings)
         await repo.mark_index_ready(doc_id)
```

 ### 3.2 各阶段详解

 #### 阶段 1：质量门禁（quality_gate）

 检查项：

 | 规则 | 阈值 | 不通过动作 |
|------|------|-----------|
| 最小长度 | `quality_gate_min_length`（默认 10 字符） | reject |
| 最大长度 | `quality_gate_max_length` | reject（或截断） |
| 敏感词 | 黑名单正则 | reject |
| 低信息密度 | TF-IDF 类密度 < 0.06 | `needs_llm_cleaning` 标记 |

 **注意**：质量门禁在 Worker 侧执行，不在 Core 侧——因为某些检测需要解析后的纯文本。

 #### 阶段 2：解析（parse_content）

 - 按 `mime_type` 选择解析器
 - `text/plain` → 直接使用
 - `text/html` → 提取纯文本
 - `text/markdown` → 保留结构

 #### 阶段 3：语义分块（chunk_text）

 详见 [算法 - 标准与改良深度学习](../03-algorithms/01-standard-and-improved.md) §1.5 §1.6。

 三种策略（优先级从高到低）：

 1. **Markdown 表格优先**：检测 `| ... |` 表格 → 单独成 chunk
 2. **段落边界感知**：按 `##` / `\n\n` 切分
 3. **固定窗口 fallback**：chunk_size=512, overlap=64

 #### 阶段 4：写库（upsert_document + insert_chunks）

 ```
 upsert_document:
   - 按 title + kb_type 查找现有文档
   - 存在 → 更新 content_hash、kb_version++
   - 不存在 → 新建
   - 返回 doc_id

 insert_chunks:
   - 批量写入 kb_chunks 表
   - 每个 chunk 带 position、search_text
   - 此时 embedding 列为 NULL
   - index_ready = false
 ```

 **为什么先写库再向量化？**

 1. 即使 embedding 失败，文档和切片已持久化（可重试）
 2. chunk_id 在写库时生成，embedding 需要 chunk_id 做关联
 3. index_ready=false 保证「未完成」的切片不会被检索到

 #### 阶段 5：向量化（embed_texts）

 ```python
 texts = [c.text for c in chunks]
 # 可选：加 workspace 前缀
 prefix = build_embedding_prefix(profile)
 texts = [prefix + t for t in texts]
 embeddings = await embed_texts(texts)
 ```

 - 批量调用 Gateway Embedding API
 - 返回 `float32[]` 数组（维度取决于模型配置）
 - 写入 `kb_chunks.embedding`（pgvector 类型）

 #### 阶段 6：标记就绪（mark_index_ready）

 ```sql
 UPDATE kb_documents SET index_ready = true WHERE id = $1;
 ```

 此后该文档的所有切片可被检索。

 ---

 ## 4. index_ready 状态机

 ```
 文档写入 (index_ready=false)
   → Worker embedding 完成 → index_ready=true ✅ 可检索
   → Worker embedding 失败 → index_ready=false ❌ 不可检索（需重试）
 ```

 **检索 SQL 中的过滤**：

 ```sql
 SELECT * FROM kb_chunks
 WHERE kb_type = $1
   AND EXISTS (
     SELECT 1 FROM kb_documents
     WHERE kb_documents.id = kb_chunks.doc_id
       AND kb_documents.index_ready = true
   )
 ORDER BY embedding <=> $2
 LIMIT 20;
 ```

 **常见排障**：检索无结果 → 先检查 `index_ready` 是否为 true。

 ```bash
 # 检查文档状态
 curl http://localhost:8443/api/v1/ugc/tasks/:task_id
 # 返回 {status: "completed", doc_id: "..."}

 # 如果 status 是 pending，检查 Worker 日志
 ```

 ---

 ## 5. 任务状态轮询

 ```bash
 # 查任务状态
 curl http://localhost:8443/api/v1/ugc/tasks/550e8400-...

 # 返回
 {
   "task_id": "550e8400-...",
   "status": "completed",       # pending | processing | completed | failed
   "doc_id": "660e8400-...",
   "created_at": "...",
   "completed_at": "..."
 }
 ```

 状态流转：

 ```
 pending → (Worker 消费) → processing → completed
                                     → failed (→ DLQ)
 ```

 ### 阶段管道（Stage Pipeline, V1-07）

 可选功能：Worker 在每个阶段完成后发阶段事件：

 ```bash
 curl http://localhost:8443/api/v1/ugc/tasks/:task_id/events
 # SSE stream: stage=quality_gate, stage=chunk, stage=embed, ...
 ```

 ---

 ## 6. 缓存 purge

 入库完成后，相关缓存需要失效（避免查到过期的检索结果/答案）：

 ```
 入库成功
   → ugcSvc.WithCaches(cacheMgr, answerCache)
     → 按 storage_partition purge 检索缓存
     → 按 storage_partition purge 答案缓存
     → catalog_version 不做 bump（单文档更新不需全量失效）
 ```

 ---

 ## 7. 学习练习

 1. 追踪一次 `ugc/notify`：画出从 Core Notify → NATS publish → Worker consume → index_ready 的完整时序图。
 2. 如果 Worker 在 embed 阶段崩溃，重启后会怎样？chunk 和 embedding 的状态是否一致？
 3. DLQ：模拟一个必定失败的消息（如 kb_type 不存在），看它如何进入 DLQ，再 replay。
 4. index_ready：为什么检索 SQL 用 JOIN 而不是直接在 kb_chunks 上加 index_ready 列？
 5. 对比 sync_fallback：什么场景下 HTTP 同步调用比 NATS 更合适？

 ---

 ## 8. 相关文档

 - [Core 服务架构](../04-architecture/01-core-service.md)
 - [jetstream.md](../../api/jetstream.md)
 - [ugc.md](../../api/ugc.md)
 - [worker.md](../../api/worker.md)
 - [算法 - 标准与改良深度学习](../03-algorithms/01-standard-and-improved.md) §1.4–§1.7
