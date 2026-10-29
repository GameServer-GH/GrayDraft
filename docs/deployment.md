# 部署手册

架构与数据流见 [系统逻辑总览](architecture/系统逻辑总览.md)；文档索引见 [README.md](README.md)。

## Mac 本地 Docker 全栈

### 前置条件

- [Docker Desktop for Mac](https://www.docker.com/products/docker-desktop/)（Compose V2 已内置）
- 仓库根目录执行命令
- 首次构建会编译 Go / Node / Python 镜像，耗时约数分钟

### 配置说明

**`deploy/config/` 下 dev/prod 的 `.env.*` 均提交进 Git**（详见 [`deploy/config/README.md`](../deploy/config/README.md)）。换机 `git pull` 后可直接 `make dev-up`，无需从 `.example` 复制或新建配置。

| 文件 | 说明 |
|------|------|
| `deploy/config/dev/.env.ports` | 端口映射 |
| `deploy/config/dev/.env.core` | Core（DB / Redis / NATS / Gateway） |
| `deploy/config/dev/.env.gateway` | 模型网关（含 API Key） |
| `deploy/config/dev/.env.worker` | Worker |
| `deploy/config/dev/.env.console` / `.env.ops` | Console / Ops |
| `deploy/config/prod/*` | `make prod-up` 全套（含 postgres/redis） |

仅当更换 Key 或本机私密覆盖时编辑上述文件，或使用 `deploy/config/**/.env.local`（不提交）。Postgres / Redis / NATS 密码已在各 `.env.*` 中对齐；迁移由 Postgres 首次启动自动执行。

### 启动命令（Mac / zsh）

```bash
cd /path/to/GrayDraft

# 1. 配置已随仓库提交；可选：改 deploy/config/dev/.env.gateway 中的 API Key

# 2. 构建并后台启动（基础设施 + core + worker + gateway + console + ops + nginx）
make dev-up

# 等价于：
docker compose -f deploy/docker-compose/docker-compose.dev.yml \
  --env-file deploy/config/dev/.env.ports up -d --build

# 3. 健康检查
make health
# 或
curl -s http://localhost:8443/api/health
curl -s http://localhost:18080/health
```

### 可选 profile

```bash
# 监控：Prometheus + Grafana
make dev-up-mon

# 本机 GPU + vLLM（需模型目录 internal/worker/models 与 NVIDIA 运行时）
make dev-up-gpu
```

### 停止与清理

```bash
make dev-down          # 停止容器
make clean             # 停止并删除卷（会清空 Postgres 数据）
```

### 启动后访问地址

| 用途 | URL |
|------|-----|
| 统一入口（Nginx → Core API） | http://localhost:8443/api/ |
| Core 直连 | http://localhost:18080 |
| Gateway（Embedding / LLM / Rerank） | http://localhost:18002 |
| Worker | http://localhost:18001 |
| Console 管理面 | http://localhost:13000 或 http://localhost:8443/console |
| Postgres | `localhost:25432` |
| Redis | `localhost:26379` |
| NATS | `localhost:24222` |

### 教育场景回归

场景样例见 `tests/contest/edu/`。检索回归见 `tests/regression/README.md`。

### 常见问题

- **`env_file ... no such file`**：检查 `deploy/config/dev/.env.*` 是否存在。
- **Gateway / Chat 报 401 或模型错误**：检查 `INTERN_API_KEY`、`DASHSCOPE_API_KEY` 与 `docs/api/models.md`。
- **Postgres 迁移未执行**：删卷重建 `make clean && make dev-up`（仅开发环境）。
- **端口占用**：改 `deploy/config/dev/.env.ports` 中对应端口后重新 `make dev-up`。

## Docker Compose

| 环境 | 文件 |
|------|------|
| 开发 | `deploy/docker-compose/docker-compose.dev.yml` |
| 生产 | `deploy/docker-compose/docker-compose.prod.yml` |
| 测试 | `deploy/docker-compose/docker-compose.test.yml` |

```bash
# 开发
make dev-up

# 生产（需先配置 deploy/config/prod/ 下各 .env.*）
make prod-up
```

## Nginx

- 主配置：`deploy/nginx/nginx.conf`
- 开发站点：`deploy/nginx/dev/conf.d/`
- 生产站点：`deploy/nginx/prod/conf.d/`（含 SSE 流式优化）

对外入口默认：`8443`（HTTP）、`9443`（HTTPS，生产）。

## 数据库初始化

```bash
make db-init    # 分区表、pgvector、全局索引物化视图
make db-migrate
```

## 监控（可选 profile）

```bash
make dev-up-mon
```

- Prometheus：`19091`
- Grafana：`13030`
