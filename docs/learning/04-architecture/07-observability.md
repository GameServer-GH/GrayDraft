 # 可观测性

 > **定位**：理解 Prometheus 指标、OpenTelemetry 追踪、Agent 日志与 RAG 自愈扫描
 > **代码路径**：`internal/core/observability/`、`internal/core/middleware/`

 ---

 ## 1. Prometheus 指标

 配置：`observability.metrics_enabled: true`

 ```bash
 curl http://localhost:18080/metrics  # Core 直连
 curl http://localhost:8443/metrics    # 通过 Nginx
 ```

 ### 关键指标

| 指标 | 类型 | 说明 |
|------|------|------|
| `http_requests_total` | Counter | HTTP 请求总数（按 method, path, status） |
| `http_request_duration_seconds` | Histogram | 请求延迟分布 |
| `retrieval_total` | Counter | 检索次数（按 kb_type, cache_hit, strategy） |
| `retrieval_duration_seconds` | Histogram | 检索延迟 |
| `ingest_total` | Counter | 入库次数（按 kb_type, status） |
| `ingest_duration_seconds` | Histogram | 入库延迟（Worker 侧） |
| `gate_score` | Gauge | 门控分数 |
| `waterfall_tier` | Gauge | 瀑布 tier |
| `first_token_seconds` | Histogram | 首个 token 延迟 |

 ---

 ## 2. OpenTelemetry 追踪

 配置：`observability.otel_enabled: true`

 ### Span 结构

 ```
 HTTP Request (root span)
   ├── middleware.OtelTrace（注入 trace_id）
   ├── Hybrid.Search
   │   ├── classifier.Classify
   │   ├── pgvector query
   │   ├── fulltext query
   │   └── Gateway rerank
   └── chat.Stream
       ├── answer cache get
       ├── Gateway LLM stream
       └── answer cache set
 ```

 ### trace_id 透传

 ```
 Client → BFF (X-Trace-Id) → Nginx → Core (middleware.OtelTrace) → Gateway
 ```

 ---

 ## 3. Agent 日志

 环境变量：`AGENT_LOG_ROOT`（默认 `/var/log/graydraft`）

 ### 日志结构

 ```
 AGENT_LOG_ROOT/
 ├── 2026-10-28_core.log     # Core 日志（按日轮转）
 ├── 2026-10-28_worker.log   # Worker 日志
 └── 2026-10-28_gateway.log  # Gateway 日志
 ```

 ### 日志内容示例

 ```
 [2026-10-28T14:30:00Z] INFO retrieval query="InternVL 怎么用" kb_type=tutorial tier=1 duration=234ms
 [2026-10-28T14:30:01Z] INFO chat grade=A chunks=5 confidence=0.82 answer_confidence=0.85
 ```

 **与 trace_id 关联**：每条日志携带 `trace_id`，可通过 trace_id 串联一次完整请求的所有日志。

 ---

 ## 4. RAG 自愈扫描

 代码：`internal/core/service/ragops/`

 ```bash
 # 触发扫描
 curl -X POST http://localhost:8443/api/v1/admin/ragops/scan

 # 查看扫描任务
 curl http://localhost:8443/api/v1/admin/ragops/jobs

 # 手动执行自愈任务
 curl -X POST http://localhost:8443/api/v1/admin/ragops/jobs/:id/run
 ```

 自愈流程：

 ```
 scan → find defects（低质量 chunk、缺失 embedding、过期标签）
   → auto-fix（重新 embed、更新标签、淘汰低质文档）
   → report（修复数量、失败项）
 ```

 ---

 ## 5. 学习练习

 1. 启动 Prometheus + Grafana，导入 HTTP 延迟面板。
 2. 用 trace_id 追踪一次对话请求从 BFF → Core → Gateway 的完整路径。
 3. 触发一次 RAG 自愈扫描，观察修复了什么问题。

 ---

 ## 6. 相关文档

 - [ragops.md](../../api/ragops.md)
 - [部署运维](../04-architecture/08-deployment-ops.md)
