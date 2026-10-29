# 部署配置（随仓库提交）

本目录下的 **`dev/`、`prod/` 内 `.env.*` 与各服务 `config.*.yaml` 均纳入版本库**，便于在新机器上：

```bash
git clone <repo>
cd GrayDraft
make dev-up    # 或 make prod-up
```

无需从 `.example` 复制或手工新建配置文件。

## 目录说明

| 路径 | 用途 |
|------|------|
| `dev/.env.ports` | Compose 端口映射（`make dev-up` 的 `--env-file`） |
| `dev/.env.core` | Core：DB、Redis、NATS、Gateway、缓存、API Key |
| `dev/.env.worker` | Worker：DB、NATS、Gateway |
| `dev/.env.gateway` | 模型网关：百炼 DashScope（Embedding）+ DeepSeek（LLM/Rerank） |
| `dev/.env.console` | Console BFF |
| `dev/.env.ops` | RAGOps |
| `dev/.env.postgres` | Postgres 容器（prod compose 使用） |
| `dev/.env.redis` | Redis 容器（prod compose 使用） |
| `prod/*` | 生产 compose 对应项（与 dev 同结构；Embedding 默认百炼 `text-embedding-v4`） |

各服务 YAML：`internal/{core,gateway,worker,console,ops}/config/config.{dev,prod}.yaml`。

## 本地覆盖（可选）

若有**仅本机**的密钥或路径，写入 `deploy/config/**/.env.local`（已在 `.gitignore` 中忽略），Compose 不会自动加载；可在 `docker compose` 中额外 `--env-file` 或 export 环境变量覆盖。

## 提交约定

- **允许并鼓励**提交 `deploy/config/**/*.env.*` 与 `internal/*/config/*.yaml`（clone 即部署）。
- **不要**把 `.env.local`、真实生产 SSL 证书提交进库。
- Agent / CI 提交代码时勿因「含 API Key」而拒绝或剥离上述路径下的配置文件。
