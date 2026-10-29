# GrayDraft 测试资源

教育场景评测与检索回归。端到端逻辑见 [docs/architecture/系统逻辑总览.md](../docs/architecture/系统逻辑总览.md)。

## 场景样例

路径：[`contest/edu/`](contest/edu/)

教程与论文材料上的 Agent 用例，覆盖词法检索、语义检索、相邻章节、实体扩展和笔记写入。语料说明见 `contest/edu/corpus/README.md`。

## Agent 黄金集

路径：[`agent-eval/golden.json`](agent-eval/golden.json)

`edu_workspace_v1` 的结构回放用例，租户 `tenant_edu`。

## 检索回归

路径：[`regression/`](regression/)

固定 UUID 的教程文档，验证专名、别名、概念题、多实体和无答案。导入与运行见 `regression/README.md`。

## 向量基准

路径：[`bench/`](bench/)

本地 pgvector 召回延迟，不依赖业务语料。
