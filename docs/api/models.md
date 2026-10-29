# 模型配置

GrayDraft 通过 Model Gateway 调用远程模型。理解和短任务、讲解、看图走书生。向量和重排走百炼。Core 与 Worker 只访问 Gateway。

## 默认模型

| 能力 | 配置键 | 默认值 | 说明 |
|------|--------|--------|------|
| 规划、导入、分类、关键词 | `models.llm_fast` | `intern-s1-mini` | 短 JSON。Gateway 对这一层关闭思考 |
| 讲解 | `models.llm` | `intern-s2` | 最终回答。失败时回退到 `llm_fast` |
| 课件页、截图 | `multimodal.vl_max_model` | `internvl-latest` | 抽出文字后进入章节，再做向量 |
| 向量 | `models.embedding` | `text-embedding-v4` | 1024 维，百炼。见 [`embedding.md`](embedding.md) |
| 重排 | `models.rerank` | `qwen3-rerank` | 百炼专用排序接口。失败时 Worker 回退词法打分 |

更换模型名只改 YAML 或环境变量。更换向量模型需要整库重新嵌入。

## Gateway 配置

文件：`internal/gateway/config/config.dev.yaml`

```yaml
models:
  embedding:
    provider: dashscope
    model: text-embedding-v4
  llm_fast:
    provider: intern
    model: intern-s1-mini
  llm:
    provider: intern
    model: intern-s2
  rerank:
    provider: dashscope
    model: qwen3-rerank

providers:
  intern:
    enabled: true
    base_url: https://chat.intern-ai.org.cn/api/v1
  dashscope:
    enabled: true
    base_url: https://dashscope.aliyuncs.com/compatible-mode

embedding:
  fallback_local: false
llm:
  fallback_local: false
```

环境变量（`deploy/config/dev/.env.gateway`）：

| 变量 | 说明 |
|------|------|
| `INTERN_API_KEY` / `INTERN_BASE_URL` | 书生对话与视觉 |
| `DASHSCOPE_API_KEY` | 向量与重排 |
| `DASHSCOPE_BASE_URL` | 向量兼容接口，默认 `https://dashscope.aliyuncs.com/compatible-mode`。重排走同一主机上的文本排序接口 |
| `EMBEDDING_MODEL` / `LLM_MODEL_FAST` / `LLM_MODEL`（或 `LLM_MODEL_COMPLEX`）/ `RERANK_MODEL` | 覆盖模型名 |
| `EMBEDDING_PROVIDER` / `LLM_PROVIDER` / `RERANK_PROVIDER` | 当前为 `dashscope` / `intern` / `dashscope` |

未配置对应 API Key 且 `fallback_local: false` 时，Gateway 返回错误。

## Core 配置

`internal/core/config/config.dev.yaml`：

```yaml
model_gateway:
  url: http://gateway:18002
  embedding_model: text-embedding-v4
  llm_model_fast: intern-s1-mini
  llm_model: intern-s2
  rerank_model: qwen3-rerank

multimodal:
  vl_plus_model: internvl-latest
  vl_max_model: internvl-latest
```

Core 调用 Gateway 时：规划、导入和分类走 `intern-s1-mini`；讲解走 `intern-s2`。`intern-s2` 失败时 Gateway 回退到快层模型。

`POST /v1/chat/completions` 的 `model` 字段支持别名：`fast` / `complex`（或 `pro`），以及具体模型 ID。

## Worker 配置

```yaml
embedding:
  model: text-embedding-v4
  gateway_url: http://gateway:18002

rerank:
  model: qwen3-rerank
  use_api: true
  fallback_lexical: true
```

- `use_api: true`：L1/L2 重排走 Gateway `POST /v1/rerank`
- `fallback_lexical: true`：API 失败时回退词法重排

## API 端点

| 端点 | 模型参数 |
|------|----------|
| `POST /v1/embeddings` | `model` 可选，默认配置 embedding |
| `POST /v1/chat/completions` | `model` 可选，默认配置 llm |
| `POST /v1/rerank` | `model` 可选，默认配置 rerank |

`GET /health` 返回当前绑定的 provider 与 model。

## 切换 OpenAI 示例

```yaml
models:
  embedding:
    provider: openai
    model: text-embedding-3-small
  llm:
    provider: openai
    model: gpt-4o-mini
```

并设置 `OPENAI_API_KEY`、`providers.openai.enabled: true`。

## 相关

- [`embedding.md`](embedding.md) — 百炼 DashScope Embedding（O-13）
- [`gateway.md`](gateway.md)
- [`agent-v2.md`](agent-v2.md) — Agent v2 对话 API
- [`chat.md`](chat.md) — 旧 Chat API（已下线）
