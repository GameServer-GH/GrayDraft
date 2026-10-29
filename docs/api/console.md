# Console 管理面

GrayDraft 可视化管理后台（React + Go BFF），用于运维与 RAG 调试。规划中的完整运营能力见 [roadmap.md § P1](../roadmap.md#p1--管理面板与产品化)。

## 架构

```
Browser → Console BFF :13000 → Core :18080
                └─ 静态 SPA（dist）
```

BFF 将 `/api/*`、`/health` 反向代理到 Core，不直连数据库。

## 功能模块（MVP）

| 页面 | 能力 |
|------|------|
| **概览** | Core 健康、目录版本、文档/chunk 统计、UGC 任务分布 |
| **UGC 任务** | 列表筛选、投递 `notify`、查看状态 |
| **RAG 调试** | 分类预览、检索、Agent v2 Run SSE（含 cache_hit / gate JSON） |
| **DLQ** | 死信列表、批量重放/丢弃（联动 UGC 任务重置） |

## 登录（JWT，O-03）

配置 Core `JWT_SECRET` 后，Admin 路由需 Bearer Token。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/login` | `{"username","password"}` → `token` |
| GET | `/api/v1/auth/me` | 需 Authorization（校验 token） |

Console 前端登录页调用上述接口；未配置 `JWT_SECRET` 时 Admin 仍开放（仅 dev）。

环境变量（`deploy/config/dev/.env.core`）：

| 变量 | 说明 |
|------|------|
| `JWT_SECRET` | HS256 密钥；非空即启用鉴权 |
| `ADMIN_USER` | 默认 `admin` |
| `ADMIN_PASSWORD` | Console 登录密码 |
| `JWT_TOKEN_TTL_HOURS` | Token 有效期（默认 24） |

## Core Admin API

Admin 路由需 `Authorization: Bearer <token>`（当 `JWT_SECRET` 已配置）。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/admin/overview` | 目录 + 文档统计 + UGC 计数 |
| GET | `/api/v1/admin/catalog` | 目录版本列表 |
| GET | `/api/v1/admin/documents/stats` | 各 kb_type 文档/chunk 数 |
| GET | `/api/v1/admin/ugc/tasks` | `?limit=50&status=pending` |
| GET | `/api/v1/admin/dlq/stats` | DLQ 流统计 |
| GET | `/api/v1/admin/dlq/messages` | DLQ 消息列表 |
| POST | `/api/v1/admin/dlq/replay` | 重放死信 |
| POST | `/api/v1/admin/dlq/discard` | 丢弃死信 |

DLQ API 详见 [`dlq.md`](dlq.md)。

## 本地开发

```bash
# 终端 1: Core
cd internal/core && CONFIG_PATH=config/config.dev.yaml go run ./cmd/gateway

# 终端 2: 前端（直连 Core）
cd internal/console/frontend && npm install && npm run dev
# 打开 http://localhost:5173

# 或 BFF + 构建前端
cd internal/console/frontend && npm run build
cd internal/console/backend && CONFIG_PATH=config/config.dev.yaml go run ./cmd
# 打开 http://localhost:13000
```

## Docker

```bash
docker compose -f deploy/docker-compose/docker-compose.dev.yml \
  --env-file deploy/config/dev/.env.ports up -d core console

# Nginx 统一入口（若启用）
# https://localhost:8443/console
```

环境变量见 `deploy/config/dev/.env.console`：`CORE_URL`、`HTTP_PORT`。

## 安全说明

- **已启用 JWT**（`JWT_SECRET` 非空）：仅 `/api/v1/admin/*` 与 `/api/v1/auth/me` 需 Token；检索、Chat、UGC notify 等对外 API 仍匿名（由网关/网络层限流）。
- **未配置 JWT_SECRET**：Admin 开放，仅限本地调试。
- 生产请轮换 `JWT_SECRET` / `ADMIN_PASSWORD`，并通过 Nginx 将 Console 限制在内网。
