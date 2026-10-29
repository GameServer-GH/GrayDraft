 # Gateway 模型网关

 > **定位**：理解 Model Gateway 的多 Provider 路由、三种模型调用与流式代理
 > **代码路径**：`internal/gateway/`

 ---

 ## 1. Gateway 定位

 Gateway 是 GrayDraft 与外部 AI 模型之间的**统一代理层**。Core/Worker 不直接调用 OpenAI/其他 API，而是通过 Gateway 统一路由。

 ```
 Core / Worker
   │ POST /v1/embeddings
   │ POST /v1/chat/completions
   │ POST /v1/rerank
   ▼
 Gateway
   ├── Provider A（默认）
   ├── Provider B（fallback）
   └── Provider C（特定模型专用）
   ▼
 外部 AI 服务
```

 ---

 ## 2. 三种模型调用

 | 类型 | API 路径 | 输入 | 输出 | 是否流式 |
|------|----------|------|------|----------|
| Embedding | `POST /v1/embeddings` | `{input: text[], model: "..."}` | `{data: [{embedding: float[]}]}` | 否 |
| LLM Chat | `POST /v1/chat/completions` | `{messages: [...], stream: true}` | SSE delta 流 | 是 |
| Rerank | `POST /v1/rerank` | `{query, documents: [...]}` | `{results: [{index, score}]}` | 否 |

 ### 调用方映射

| 调用方 | Embedding | LLM | Rerank |
|--------|-----------|-----|--------|
| Core 检索 | ✓（query 向量化） | — | ✓（L2 精排） |
| Core 对话 | — | ✓（流式生成） | — |
| Core 智能导入 | — | ✓（标题推断） | — |
| Worker 入库 | ✓（chunk 向量化） | — | ✓（L1 轻排） |
| Worker QA 生成 | — | ✓（自动 QA） | — |

 ---

 ## 3. 多 Provider 路由

 ```yaml
 gateway:
   providers:
     - name: openai
       url: https://api.openai.com
       api_key: ${OPENAI_API_KEY}
       models: [gpt-4o, text-embedding-3-small]
     - name: local
       url: http://localhost:8080
       models: [bge-large-zh, qwen2.5-7b]
   routing:
     embedding: local        # embedding 走本地模型
     llm: openai             # LLM 走 OpenAI
     rerank: local           # rerank 走本地模型
     fallback: openai        # 主 Provider 不可用时的兜底
 ```

 ### 路由策略

| 策略 | 行为 |
|------|------|
| 固定路由 | 某类模型固定走某 Provider |
| Fallback | 主 Provider 超时/错误 → 自动切换到备选 |
| 模型匹配 | 根据请求中的 model 参数选择 Provider |

 ---

 ## 4. 流式 SSE 代理

 LLM 流式调用时，Gateway 不只是转发——它还做了以下处理：

 ```
 Client → Core → Gateway → OpenAI
                        ← SSE stream
              Gateway:
                1. 透传 delta 帧
                2. 监控超时（llm_total_timeout_ms）
                3. 流中断时发送 [DONE]
              Core:
                4. StripEmojis（去 emoji）
                5. NormalizeKBMarkdown（kb_only 模式）
                6. 写 SSE chunk 事件
```

 ---

 ## 5. 超时与重试

 | 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `request_timeout` | 单次请求超时 | 30s |
| `llm_total_timeout_ms` | LLM 流式总超时 | 60000ms |
| `max_retries` | 最大重试次数 | 2 |
| `retry_backoff` | 重试退避策略 | 指数退避 |

 ---

 ## 6. 学习练习

 1. 配置两个 Embedding Provider，模拟主 Provider 超时，观察是否自动 fallback。
 2. 对比同一个 query 在不同 LLM model 下的首个 token 延迟（观察 `first_token_seconds` 指标）。
 3. 如果 Gateway 完全不可用，Core 的检索和对话会怎样？（观察 fallback 行为）

 ---

 ## 7. 相关文档

 - [gateway.md](../../api/gateway.md)
 - [models.md](../../api/models.md)
 - [Core 服务架构](../04-architecture/01-core-service.md) §7
