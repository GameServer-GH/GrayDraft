 # 部署运维

 > **定位**：理解 Docker Compose 五服务编排、Nginx 反代、配置体系与 Admin 运维面

 ---

 ## 0. 服务拓扑

 ```
 Nginx (:8443)              # TLS 终止 + 反代 + SSE 无缓冲
   ├── /api/* → Core (:18080)
   └── /console/* → Console BFF

 Core Go (:18080)           # HTTP API、Agent v2、检索编排
 Worker Python              # NATS 消费、入库流水线
 Gateway                    # Embedding / LLM / Rerank

 PostgreSQL (:5432)         # 主存储 + pgvector
 Redis (:6379)              # 缓存
 NATS (:4222)               # 消息队列 + JetStream
 ```

 ---

 ## 1. Docker Compose

 ```yaml
 services:
   postgres:    # PostgreSQL 15 + pgvector
   redis:       # Redis 7
   nats:        # NATS 2.x + JetStream
   core:        # Core Go（build: internal/core）
   worker:      # Worker Python（build: internal/worker）
   gateway:     # Model Gateway（外部或内置）
   nginx:       # Nginx 反代
   console:     # Console BFF + React UI（可选）
 ```

 ### 常用命令

 ```bash
 make dev-up        # 启动所有服务
 make dev-down      # 停止
 make dev-logs      # 查看日志
 make dev-build     # 重新构建镜像（--no-cache 首次或结构变更后）
 ```

 ---

 ## 2. Nginx 配置

 ### 关键配置项

 ```nginx
 server {
     listen 8443 ssl;

     location /api/ {
         proxy_pass http://core:18080;
         proxy_buffering off;           # SSE 必须关闭缓冲
         proxy_cache off;
         proxy_read_timeout 300s;       # LLM 流式可能长连接
     }

     location /console/ {
         proxy_pass http://console:3000;
     }
 }
 ```

 **为什么 SSE 需要关闭缓冲？**

 Nginx 默认会缓冲 upstream 响应，等收集到一定量再发给客户端。这对 SSE 是致命的——客户端期望收到增量 `data:` 帧，而不是等到流结束一次性收到。`proxy_buffering off` 关掉缓冲，每个 chunk 立即透传。

 ---

 ## 3. 环境变量体系

 ```
 deploy/config/dev/
 ├── .env.core        # Core 环境变量
 ├── .env.worker      # Worker 环境变量
 ├── .env.gateway     # Gateway 环境变量
 └── .env.ports       # 端口映射
 ```

 ### Core 关键环境变量

 | 变量 | 说明 |
|------|------|
| `CONFIG_PATH` | YAML 配置路径 |
| `DB_HOST/PORT/USER/PASSWORD/NAME` | PostgreSQL |
| `REDIS_URL` | Redis 连接 |
| `NATS_URL` | NATS 地址 |
| `MODEL_GATEWAY_URL` | Gateway 基址 |
| `AI_WORKER_URL` | Worker 基址 |
| `LOG_LEVEL` | 日志级别 |
| `MIGRATIONS_DIR` | 迁移脚本目录 |

 ---

 ## 4. Admin 运维面

 需要 JWT 认证（`Authorization: Bearer <token>`，token 通过 `/api/v1/auth/login` 获取）。

 ### 运维 API

 | 接口 | 说明 |
|------|------|
| `GET /admin/overview` | 概览：文档数、chunk 数、任务数 |
| `GET /admin/catalog` | 知识库目录 |
| `GET /admin/documents/stats` | 文档统计 |
| `GET /admin/ugc/tasks` | UGC 任务列表 |
| `GET /admin/chunks` | Chunk 抽样列表 |
| `POST /admin/global-index/refresh` | 刷新全局索引 |
| `POST /admin/tier/apply` | 冷热分层 |
| `POST /admin/ragops/scan` | RAG 自愈扫描 |
| `GET /admin/dlq/stats` | DLQ 统计 |
| `GET /admin/dlq/messages` | DLQ 消息列表 |
| `POST /admin/dlq/replay` | 重放 DLQ 消息 |
| `POST /admin/dlq/discard` | 丢弃 DLQ 消息 |

 ### DLQ 管理流程

 ```
 1. GET /admin/dlq/stats → 了解 DLQ 积压量
 2. GET /admin/dlq/messages → 查看具体失败原因
 3. 修复根因（如补上缺失的 kb_type）
 4. POST /admin/dlq/replay → 重放
 5. 确认重放成功 → POST /admin/dlq/discard
 ```

 ### 工作区数据管理

 ```bash
 # 查看分区统计
 curl http://localhost:8443/api/v1/kb/tutorial/workspace-data/stats

 # 清空分区数据 + 缓存 purge
 curl -X POST http://localhost:8443/api/v1/kb/tutorial/workspace-data/purge
 ```

 ---

 ## 5. 优雅关停

 ```
 SIGTERM → Core
   ├─ 停止接受新请求
   ├─ 等待现有请求完成（10s 超时）
   ├─ 关闭 DB/Redis/NATS 连接
   └─ 退出

 Worker
   ├─ 停止消费新消息（drain）
   ├─ 等待当前任务完成
   └─ 退出
 ```

 ---

 ## 6. 学习练习

 1. 修改 Docker Compose 的端口映射，将 Core 暴露在 9090 而不是 18080。
 2. 制造一条必定失败的 UGC 消息，观察 DLQ 中如何出现，然后 replay 和 discard。
 3. 测试 SSE 缓冲：将 Nginx `proxy_buffering on`，观察 curl -N 的行为有何不同。
 4. 模拟 Core 崩溃：`docker kill core`，观察 SWARM/Compose 是否自动重启。

 ---

 ## 7. 相关文档

 - [development.md](../../development.md)
 - [deployment.md](../../deployment.md)
 - [dlq.md](../../api/dlq.md)
 - [ragops.md](../../api/ragops.md)
 - [Core 服务架构](../04-architecture/01-core-service.md)
