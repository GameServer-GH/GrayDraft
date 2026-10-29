# 测试即规格

上一篇：[动手实践](hands-on.md) · 下一篇：[进阶学习路线](learning-roadmap-ai-agent.md)

---

GrayDraft 的学习不能只读实现。测试通常更直接地表达输入、边界条件和预期行为；先读测试名称和断言，再回到实现，能减少被细节淹没。

## 1. 基础命令

```bash
# Core 全量 Go 测试
cd internal/core && go test ./...

# 指定包
cd internal/core && go test ./service/agent
cd internal/core && go test ./service/retriever

# 指定测试并显示日志
cd internal/core && go test ./service/agent -run TestName -v

# Worker 测试
cd internal/worker && python -m pytest -q
```

仓库快捷入口是 `make test-unit`。注意当前 Make 目标对 Worker 测试使用了容错执行；学习和验证 Worker 时，应单独运行 `pytest` 并确认真实退出码。

---

## 2. 按学习阶段选择测试

| 学习主题 | 优先阅读 / 运行 |
|----------|-----------------|
| Agent Loop | `service/agent/runtime_test.go`、`planner_test.go`、`runner_test.go` |
| Tool | `tools_test.go`、`knowledge_tool_test.go`、`governance_test.go` |
| Memory | `memory_test.go`、`long_memory_test.go`、`context_test.go` |
| 回答可靠性 | `answer_gate_test.go`、`groundedness_test.go`、`answer_validator_test.go` |
| SSE / 流式输出 | `answer_stream_test.go`、`handler/agent_test.go` |
| 混合检索 | `retriever/hybrid_test.go`、`query4d_test.go`、`entity_pool_test.go` |
| 分级与门控 | `confidence/gate_test.go`、`threshold_test.go`、`grounding_test.go` |
| 缓存 | `cache/manager_test.go`、`cache/key_test.go`、`answercache/key_test.go` |
| 智能导入 | `smartimport/*_test.go` |
| 语义进化 | `taxonomyevolve/*_test.go`、`model/semantic4d*_test.go` |
| Worker 流水线 | `internal/worker/tests/test_quality_gate.py`、`test_chunker.py`、`test_search_text.py` |
| BFF | `internal/graydraft/pkg/nexus/*_test.go` |

---

## 3. 每篇测试的阅读模板

```text
Given：测试构造了什么状态或输入？
When：调用哪个公开方法？
Then：哪些断言代表业务契约？
Boundary：空值、超时、低分、重复请求、断线等边界如何处理？
Regression：这个测试可能防止哪类历史问题复发？
```

如果文档描述与断言冲突，先确认测试是否仍在当前主流程中，再以当前代码和可执行测试为准更新理解。

---

## 4. 最小学习测试集

```bash
cd internal/core
go test ./service/agent ./service/retriever ./service/confidence ./service/cache
```

然后单独验证 Worker：

```bash
cd internal/worker
python -m pytest tests/test_quality_gate.py tests/test_chunker.py tests/test_search_text.py -q
```

这不是完整 CI，只是覆盖学习主线的烟感集合。

---

## 本阶段自检

- [ ] 能从文档中的机制找到对应测试文件
- [ ] 能解释至少一个 Agent Loop 测试的 Given / When / Then
- [ ] 能独立运行核心学习测试集
- [ ] 能区分“命令执行完成”和“测试因容错被忽略”

继续：[复习清单](../06-closed-loop/checklist.md)。
