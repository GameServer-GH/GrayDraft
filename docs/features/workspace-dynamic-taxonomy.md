# 工作区自适应 4D 语义蓝图

## 概述

除静态 `taxonomy_config.yaml`（全行业脚手架）外，每个 `kb_type` 可在 `workspace_4d_taxonomy` 中保存 **Blueprint**：

- **domain / scenes / styles**：由 Blueprint Agent 根据工作区背景 + 样例文档 LLM 生成
- **关键词进化**：导入 `mapping_delta` → Loop A 晋升 → Loop B 压缩
- **标签裂变**：高频无法对齐的候选 → 新标准 label
- **语义对齐**：规则 → 字符相似度 → Embedding 余弦（`embed_align_min`）

## 触发 Blueprint

- 保存 / 富化 `ContextProfile`（`PUT/POST .../context`）
- 智能导入成功提交后
- 首次检索 `Effective()` 且尚无 `blueprint_status=ready`

配置：`taxonomy_evolve.blueprint_enabled`（需 `taxonomy_evolve.enabled: true`）

## 导入 kb_type

- 教育语料使用 tutorial、paper、note；其他工作区使用 `gd_*` 分区
- 智能导入 **必须** 提供工作区分区 `hint_kb_type`（`gd_*`）
- 多主题提示仅来自 **骨架扫描**（`ScanSkeleton` 的 `doc_type`），多主题只看文档骨架

## 运维

- 修改工作区背景后自动排队重新生成 Blueprint
- Blueprint 版本 bump 后自动触发 Worker 影子回填（`ai_worker.url` 须可达）
- taxonomy `version` bump 后请对该 `kb_type` 执行 Redis purge
- 重启 Core 加载 YAML 脚手架变更；Blueprint 在 DB 中无需重编译

## 相关代码

- `internal/core/service/taxonomyevolve/blueprint.go`
- `internal/core/service/taxonomyevolve/aligner.go`
- `internal/core/service/taxonomyevolve/birth.go`
- `internal/core/model/taxonomy_blueprint.go`
