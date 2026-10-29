# Gateway API

模型路由网关，默认端口 `18002`。通过 **OpenAI 兼容 HTTP API** 调用远程模型（默认 **DeepSeek**），配置见 [`models.md`](models.md)。

## `POST /v1/embeddings`

```json
{
  "texts": ["自适应混合检索"],
  "model": "text-embedding-v4"
}
```

`model` 省略时使用配置 `models.embedding.model`。选型与维度说明见 [`embedding.md`](embedding.md)。

## `POST /v1/chat/completions`

OpenAI 兼容流式对话（`stream: true` 必填）。

```json
{
  "model": "deepseek-v4-pro",
  "stream": true,
  "messages": [
    {"role": "system", "content": "你是 GrayDraft 助手…"},
    {"role": "user", "content": "用户问题"}
  ]
}
```

需配置 `DEEPSEEK_API_KEY`（或对应 provider 的 API Key）。

## `POST /v1/rerank`

使用配置的 Rerank 模型（embedding API）对候选做 cosine 重排：

```json
{
  "query": "混合检索",
  "model": "deepseek-rerank-v1",
  "top_n": 5,
  "candidates": [
    {"id": "1", "content": "…", "kb_type": "tutorial"}
  ]
}
```

## `GET /health`

返回 `mode: api` 与三类模型绑定信息。

## 环境变量

见 `deploy/config/dev/.env.gateway`。
