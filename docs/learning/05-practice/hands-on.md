# 动手跑通 GrayDraft 主链路

上一篇：[部署运维](../04-architecture/08-deployment-ops.md) · 下一篇：[测试即规格](tests-as-specs.md)

---

## 实验 1：启动开发环境

在仓库根目录执行：

```bash
make dev-up
make health
```

或直接检查 Core：

```bash
curl http://localhost:18080/api/health
```

观察 Core、Worker、Gateway、PostgreSQL、Redis、NATS 是否就绪。端口以 `deploy/config/dev/.env.ports` 为准。

---

## 实验 2：完成一次入库

按 [`docs/development.md`](../../development.md) 的 UGC 示例调用 `POST /api/v1/ugc/notify`，记录：

1. 返回的任务或文档标识；
2. 任务状态如何变化；
3. Core 在哪里投递消息；
4. Worker 依次执行哪些阶段；
5. 最终何时进入 `index_ready`。

把观察结果画成一条时序线，并在每一步标注代码入口。

---

## 实验 3：完成一次混合检索

按 [`docs/development.md`](../../development.md) 调用 `POST /api/v1/retrieval/search`，使用刚入库知识能回答的问题。

重点观察：

- 作用域是否命中正确 tenant / KB；
- 返回候选来自哪些召回路径；
- 是否出现缓存命中；
- RRF、Rerank 和分级分别在哪一步体现；
- 修改问题表述后，结果顺序与 Grade 如何变化。

---

## 实验 4：创建 Agent v2 Run 并消费 SSE

参考 [`docs/development.md`](../../development.md) 和 [`docs/api/agent-v2.md`](../../api/agent-v2.md)：

1. `POST /api/v2/agent-runs` 创建 Run；
2. 保存返回的 `run_id`；
3. 请求 `GET /api/v2/agent-runs/{run_id}/events/stream?after_seq=0`；
4. 记录 `run.*`、`step.*`、`tool.*`、`answer.*` 事件顺序；
5. 断开后使用最后一个序号重新连接，验证回放。

完成后应该能解释：为什么“创建 Run”和“消费事件”是两个请求，以及持久化事件如何支持断线恢复。

---

## 实验 5：跟踪一次 `knowledge.semantic`

从 Agent Run 的工具事件出发，沿以下路径定位：

```text
Runtime / Planner
  → Tool Registry
  → knowledge.semantic
  → Hybrid Retriever
  → 召回 / RRF / Rerank
  → Tool Result
  → Answer Gate
```

每一层只回答四件事：输入、输出、失败处理、对应测试。

---

## 实验 6：制造一个可解释的失败

选择一个安全且可恢复的场景，例如：

- 查询一个未入库的知识点；
- 使用错误 KB 作用域；
- 让检索结果低于门控要求；
- 在本地环境停止一个非数据服务后观察降级，再恢复服务。

记录用户可见结果、日志 / Trace、内部 Grade 或错误状态，以及系统是否降级、拒答或重试。不要删除数据库卷来制造故障。

---

## 本阶段产出

- [ ] 一张入库 → 检索 → Agent 回答时序图
- [ ] 一份包含关键事件序号的 Agent Run 记录
- [ ] 一个从请求定位到测试的读码笔记
- [ ] 一个失败场景及其降级 / 拒答解释

继续：[测试即规格](tests-as-specs.md)。
