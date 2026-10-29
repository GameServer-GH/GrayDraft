 # Core 服务架构

 > **定位**：理解 GrayDraft Go 服务的启动、路由、中间件、作用域与配置体系
 > **前置**：[系统逻辑总览](../../architecture/系统逻辑总览.md)（§0.1–§0.3）
 > **代码根**：`internal/core/cmd/gateway/main.go`

 ---

 ## 0. 学习地图

 ```mermaid
 flowchart LR
   A[main.go 启动] --> B[配置加载]
   B --> C[基础设施初始化]
   C --> D[Service 装配]
   D --> E[Handler 装配]
   E --> F[路由注册]
   F --> G[中间件链]
   G --> H[信号监听 + 优雅关停]
 ```

 | 阶段 | 先读懂什么 | 验收标准 |
|------|-----------|----------|
| 1 | 启动流程 | 能画出 main.go 的初始化顺序图 |
| 2 | 路由注册 | 能找到任一 API 对应的 handler 和 service |
| 3 | 中间件链 | 能说出一个请求经过的中间件顺序 |
| 4 | 作用域解析 | 能解释 tenant_id + kb_id → storage_partition 的查表逻辑 |
| 5 | 配置体系 | 能区分 YAML 配置和环境变量覆盖的优先级 |

 ---

 ## 1. 启动流程

 `internal/core/cmd/gateway/main.go`（~280 行）是 Core 服务唯一入口。启动分 8 个阶段：

 ```
 1. 配置加载        config.Load(cfgPath)
 2. 日志初始化      logger.New(logLevel)
 3. 数据库连接      postgres.New(ctx, cfg.DSN())
 4. 自动迁移        postgres.RunMigrations(db, migrationsDir)
 5. Repository 初始化  ChunkRepo, DocRepo, ScopeRepo, UGCRepo, MMCache
 6. Service 装配     14 个 service（retriever, chat, ugc, smartimport...）
 7. Handler 装配     15 个 handler
 8. 路由注册         r.GET/POST + 分组路由
 ```

 **关键代码**：

 ```go
 cfg, err := config.Load(cfgPath)
 db, err := postgres.New(ctx, cfg.DSN())
 postgres.RunMigrations(db, migrationsDir)

 // Service 装配（有依赖顺序）
 hybrid := retriever.NewHybrid(chunkRepo, cfg, embClient, rrClient, cacheMgr, clf, llmClient)
 ugcSvc := ugc.NewService(cfg, ugcRepo, natsPub, ugc.NewWorkerClient(cfg))
 smartImportSvc := smartimport.NewService(ugcSvc, docRepo, chunkRepo, llmClient, embClient, mmProc, cfg)
 chatSvc := chat.NewService(cfg, hybrid, llmClient, answerCache, chunkRepo, docRepo)
 ```

 ### 优雅关停

 ```go
 quit := make(chan os.Signal, 1)
 signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
 <-quit
 // 10 秒超时关停
 shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
 srv.Shutdown(shutdownCtx)
 ```

 ---

 ## 2. HTTP 框架：Gin

 Core 使用 **Gin** 作为 HTTP 框架。Gin 的核心概念：

| 概念 | 说明 | GrayDraft 例子 |
|------|------|-------------|
| `gin.Engine` | 路由引擎 | `r := gin.New()` |
| `gin.Context` | 请求上下文 | 每个 handler 函数的 `*gin.Context` 参数 |
| `r.Use()` | 注册中间件 | `r.Use(middleware.RequestID())` |
| `r.GET/POST` | 注册路由 | `v2.POST("/agent-runs", agentH.CreateRun)` |
| `r.Group()` | 路由分组 | `v1 := r.Group("/api/v1")` |
| `gin.H` | JSON 响应快捷方式 | `gin.H{"status": "ok"}` |

 **为什么用 Gin 而不是标准库？**

 - 路由参数解析（`:task_id`）比 `http.ServeMux` 更灵活
 - 中间件链（`Use` + `Next/Abort`）比标准库 `http.Handler` 包装更清晰
 - 内置 JSON 绑定、参数校验、错误恢复
 - 性能接近 `httprouter`，在 Go Web 框架中属于第一梯队

 ---

 ## 3. 路由注册

 所有路由在 `main.go` 中集中注册，按功能域分组：

 ### 3.1 路由总览表

 | 方法 | 路径 | Handler | Service |
|------|------|---------|---------|
| GET | `/api/health` | `healthH.Health` | —（直查 DB/NATS） |
| POST | `/api/v1/retrieval/search` | `retrievalH.Search` | `retriever.HybridRetriever` |
| POST | `/api/v1/retrieval/diagnose` | `retrievalH.Diagnose` | `retriever.HybridRetriever` |
| POST | `/api/v1/classifier/preview` | `classifyH.Classify` | `classifier.Classifier` |
 | POST | `/api/v2/agent-runs` | `agentH.CreateRun` | `agent.Launcher` |
 | GET | `/api/v2/agent-runs/:run_id/events/stream` | `agentH.EventsStream` | `agent.RunRepository` |
 | POST | `/api/v1/chat/feedback` | `chatFeedbackH.Submit` | `chat.FeedbackService` |
| POST | `/api/v1/ugc/notify` | `ugcH.Notify` | `ugc.Service` |
| POST | `/api/v1/ugc/smart-import` | `smartImportH.Import` | `smartimport.Service` |
| GET | `/api/v1/ugc/tasks/:task_id` | `ugcH.TaskStatus` | `ugc.Service` |

 ### 3.2 路由分组

 ```go
 v1 := r.Group("/api/v1")       // 公开 API
 tenant := v1.Group("/tenants/:tenant_id")  // PaaS 多租户
 kb := v1.Group("/kb/:kb_type")             // Legacy 分区
 admin := v1.Group("/admin")                // 运维面（需认证）
 admin.Use(middleware.AdminAuth(cfg.Auth.JWTSecret))
 ```

 ### 3.3 Handler → Service → Repository 三层

 ```
 HTTP 请求
   → Handler（参数解析、HTTP 响应）
     → Service（业务逻辑、编排）
       → Repository（数据库/NATS/Redis 操作）
 ```

 Handler 不直接操作数据库——所有数据访问都通过 Service → Repository 链。

 ---

 ## 4. 中间件链

 请求进入 Gin 后经过的中间件顺序：

 ```
 1. gin.Recovery()         — panic 恢复 + 500
 2. middleware.RequestID() — 生成/提取 X-Request-ID
 3. middleware.OtelTrace() — OpenTelemetry span（可选）
 4. middleware.ChatBudget()— RAG 请求预算控制
 5. middleware.Prometheus()— HTTP 指标采集（可选）
 6. middleware.RateLimiter — 按工作区限流（仅 /ugc/* 路由）
 7. middleware.AdminAuth  — JWT 认证（仅 /admin/* 路由）
 8. Handler 函数
 ```

 ### 4.1 中间件实现位置

| 中间件 | 文件 | 作用 |
|--------|------|------|
| `RequestID` | `middleware/requestid.go` | 每请求分配 UUID，写入 Header `X-Request-ID` |
| `OtelTrace` | `middleware/`（Otel 相关） | 创建 Span，注入 trace_id 到 context |
| `ChatBudget` | `middleware/budget.go` | RAG 请求的 token/时间预算 |
| `Prometheus` | `middleware/prometheus.go` | HTTP 请求计数、延迟 Histogram |
| `RateLimiter` | `middleware/rate_limiter.go` | 按 `storage_partition` 限流（token bucket） |
| `AdminAuth` | `middleware/admin_auth.go` | JWT Bearer Token 验证 |

 ### 4.2 限流中间件详解

 仅对入库路由生效：

 ```go
 v1.POST("/ugc/notify", rateLimiter.IngestRateLimit(), ugcH.Notify)
 v1.POST("/ugc/smart-import", rateLimiter.IngestRateLimit(), smartImportH.Import)
 ```

 限流键 = `ingest:<storage_partition>`，使用 token bucket 算法，配置来自 `dynconfig`（支持运行时动态调整）。

 ---

 ## 5. 作用域解析

 代码路径：`internal/core/service/kbscope/resolver.go`

 GrayDraft 同时支持 PaaS 多租户和 Legacy `kb_type` 两种调用约定。

 ### 5.1 两种约定

 | 约定 | 请求字段 | 解析方式 |
|------|----------|----------|
| PaaS（推荐） | `tenant_id` + `kb_id` | 查 `knowledge_bases` 表 → `storage_partition` |
| Legacy | `kb_type` | 直接作为分区键 |

 ### 5.2 解析流程

 ```go
 func (r *Resolver) ResolveKBType(ctx, tenantID, kbID, legacyKBType string) (string, error) {
     // 1. Legacy kb_type 直接使用（gd_ 前缀的工作区分区）
     if legacyKBType != "" && isWorkspaceStoragePartition(legacyKBType) {
         return legacyKBType, nil
     }
     // 2. PaaS: 查 knowledge_bases 表
     if tenantID != "" && kbID != "" {
         return r.scope.ResolveStoragePartition(ctx, tenantID, kbID)
     }
     // 3. 兜底
     if legacyKBType != "" {
         return legacyKBType, nil
     }
     return "", fmt.Errorf("kb_type or (tenant_id, kb_id) required")
 }
 ```

 ### 5.3 GrayDraft 工作区映射

 ```
 workspace_key (用户可见)
   → BFF 计算 tenant_id = "tn_" + SHA256(workspace_key)[:16]
   → kb_id = "default"
   → storage_partition = "gd_" + SHA256(workspace_key)[:12]
 ```

 工作区打开时，BFF 调用 `tenants/ensure` + `kbs/ensure` 自动注册。

 ### 5.4 Scope 数据结构

 ```go
 type KBScope struct {
     TenantID         string  // "tn_abc123..."
     KBID             string  // "default"
     StoragePartition string  // "gd_def456..." → kb_types.name
 }
 ```

 后续所有 Service 操作都基于 `storage_partition` 做数据隔离。

 ---

 ## 6. 配置体系

 ### 6.1 加载链路

 ```
 YAML 文件（config/config.dev.yaml）
   → config.Load(path)
     → 环境变量 overlay（overlayEnv）
       → 运行时 dynconfig（动态热加载）
 ```

 **环境变量覆盖示例**：

 | 环境变量 | YAML 路径 | 说明 |
|----------|-----------|------|
| `CONFIG_PATH` | — | YAML 文件路径 |
| `DB_HOST` | `database.host` | PostgreSQL 地址 |
| `DB_PORT` | `database.port` | PostgreSQL 端口 |
| `REDIS_URL` | `redis.url` | Redis 连接串 |
| `NATS_URL` | `nats.url` | NATS 服务器地址 |
| `MODEL_GATEWAY_URL` | `model_gateway.url` | Gateway 基址 |
| `AI_WORKER_URL` | `ai_worker.url` | Worker 基址 |
| `LOG_LEVEL` | — | 日志级别 |

 ### 6.2 关键 YAML 配置段

 | 段 | 作用 |
|----|------|
| `app` | 应用名、环境（dev/prod）、端口 |
| `database` | PostgreSQL DSN |
| `redis` | Redis 连接与缓存 TTL |
| `nats` | JetStream stream/subject 配置 |
| `model_gateway` | Gateway URL、超时 |
| `retrieval` | top_k、RRF k 值、semantic_4d 开关与阈值 |
| `classifier` | 意图分类、LLM 开关 |
| `chat` | RAG 上下文条数、search_only 降级、LLM 超时 |
| `cache` | L1 Ristretto 大小、L2 Redis TTL |
| `answer_cache` | 答案缓存 TTL、语义相似阈值 |
| `smart_import` | min_confidence、标题校验规则 |
| `taxonomy_evolve` | Blueprint 开关、Aligner 阈值 |
| `observability` | Metrics/Otel 开关 |

 ### 6.3 配置使用模式

 ```go
 // 主配置加载
 cfg, err := config.Load(cfgPath)

 // 使用配置值
 hybrid := retriever.NewHybrid(chunkRepo, cfg, ...)
 chatSvc := chat.NewService(cfg, ...)

 // 动态配置（支持运行时热加载）
 dynCfg := dynconfig.New(cfg)
 rateLimiter := middleware.NewRateLimiter(dynCfg)
 ```

 ---

 ## 7. Service 依赖图

 ```mermaid
 flowchart TB
   subgraph repos[Repository 层]
     CR[ChunkRepo]
     DR[DocRepo]
     SR[ScopeRepo]
     UR[UGCRepo]
     MC[MMCache]
   end

   subgraph infra[基础设施 Service]
     EMB[embedder.Client]
     RR[reranker.Client]
     CM[cache.Manager]
     AC[answercache.Manager]
     LLC[llmclient.Client]
     NP[nats.Publisher]
   end

   subgraph biz[业务 Service]
     CLF[classifier.Classifier]
     HYB[retriever.HybridRetriever]
     UGC[ugc.Service]
     CHAT[chat.Service]
     SI[smartimport.Service]
     TE[taxonomyevolve.Service]
     MM[multimodal.Processor]
     RO[ragops.Healer]
   end

   CR --> HYB & SI
   DR --> TE & SI
   SR --> KB[KBResolver]
   EMB --> HYB & SI & TE
   RR --> HYB
   LLC --> CLF & HYB & CHAT & SI & TE
   NP --> UGC
   CM --> HYB & UGC & SI & TE
   AC --> CHAT & SI
   CLF --> HYB
   HYB --> CHAT
   UGC --> SI
   MM --> SI
 ```

 **依赖注入模式**：所有 Service 通过构造函数注入依赖（Repository、Client、Config），而非全局变量或单例。

 ---

 ## 8. 错误处理约定

 - **Handler 层**：捕获 Service 错误，转换为 HTTP 状态码 + JSON body
 - **Service 层**：返回 Go `error`，不关 HTTP 细节
 - **Repository 层**：返回 `error`，包含具体 DB/NATS 错误上下文

 常见错误码映射：

| 场景 | HTTP 状态码 |
|------|------------|
| 参数缺失/无效 | 400 |
| scope 不存在 | 404 |
| 重复入库（content_hash 命中） | 409 |
| 智能导入置信度不足 | 422（仍带 preview） |
| DB/NATS 错误 | 500 |
| LLM 超时 | 504 |

 ---

 ## 9. 学习练习

 1. 阅读 `main.go`，画出从 `config.Load` 到 `srv.ListenAndServe` 的完整调用链。
 2. 追踪 `POST /api/v2/agent-runs` 请求：它经过哪些中间件？哪个 handler？JetStream 之后由哪个 Launcher 执行？
 3. 配置加载：如果同时设置 `config.dev.yaml` 的 `http.port=18080` 和环境变量 `HTTP_PORT=9090`，哪个生效？为什么？
 4. 理解 `StoragePartition`：为什么 GrayDraft 工作区需要 `gd_` 前缀？如果去掉会怎样？
 5. 画出 Service 依赖图，标出哪些 Service 有循环依赖风险。

 ---

 ## 10. 相关文档

 - [系统逻辑总览 §2 §9 §10](../../architecture/系统逻辑总览.md)
 - [development.md](../../development.md)
 - [配置示例](../../../deploy/config/dev/)
 - 姊妹篇：[入库流水线](../02-pipeline/01-ingestion.md) — ugc/notify 到 index_ready
 - 姊妹篇：[检索链路](../02-pipeline/02-retrieval.md) — 混合检索全链路
 - 姊妹篇：[SSE RAG 对话](../02-pipeline/03-agent-v2-sse.md) — 流式对话协议
