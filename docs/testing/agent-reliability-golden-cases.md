# Agent 可靠性黄金用例

这些用例覆盖导入、检索、问答、缓存和 Markdown 的高频回归点。

| 场景 | 用例 | 覆盖测试 |
|---|---|---|
| 重复/连续 query 不串缓存 | 先问“番茄有哪些吃法”，再问“番茄周几吃，一次吃多少” | `internal/core/service/cache/manager_test.go` |
| Query4D 幻觉收口 | LLM 输出长背景 domain、非 query 子串 entity | `internal/core/service/retriever/query4d_test.go` |
| 有直接证据不误杀 | 文档含“周一 小番茄100g / 周二 番茄100g” | `internal/core/service/confidence/gate_test.go` |
| 表格 Markdown 标准化 | LLM 将表格输出粘成一行 | `internal/core/service/textutil/markdown_test.go` |
| 导入 ready barrier | task 等待到 queryable 或终态 | `internal/core/service/ugc/service_test.go` |
| Worker 完成后清缓存 | Worker 按 `kb_type` 删除 retrieval/answer Redis key | `internal/worker/tests/test_cache_purge.py` |

CI 至少应运行：

```bash
GOCACHE=/path/to/.gocache go test ./service/cache ./service/confidence ./service/retriever ./service/textutil ./service/ugc
python -m pytest internal/worker/tests/test_cache_purge.py
```
