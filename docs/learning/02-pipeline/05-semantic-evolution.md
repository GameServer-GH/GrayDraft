 # 语义进化（Taxonomy Evolve）

 > **定位**：理解 Blueprint 自发现 → Aligner 对齐 → 影子回填的完整进化链路，侧重触发条件与运维视角
 > **前置**：[自研算法深度学习 §6](../03-algorithms/02-custom-algorithms.md)

 ---

 ## 0. 进化全景

 ```
 工作区有内容 + context_profile
   │
   ├── 1. Blueprint 自发现
   │     └─ LLM 分析文档摘要 → 生成 scene/style 字典
   │
   ├── 2. Embedding Aligner
   │     └─ 将自由文本 scene/style 映射到 Blueprint 标准标签
   │
   └── 3. 影子回填（Shadow Backfill）
         └─ Worker 批量更新存量文档 semantic_tags_4d
```

 ---

 ## 1. Blueprint 自发现

 代码：`internal/core/service/taxonomyevolve/blueprint.go`

 ### 触发条件

 `MaybeEnqueueBlueprint` 在以下情况触发：

 1. 工作区有 `context_profile`（工作区背景）或样例文档
 2. Blueprint 尚未 ready（状态不是 `ready`）
 3. 不在 pending 中（避免重复排队）

 ### 生成流程

 ```
 1. MaybeEnqueueBlueprint
 2. 异步 RunBlueprint
     ├─ LLM 读工作区背景 + 文档摘要
     ├─ 生成 {domain, scenes: [{label, keywords}, ...], styles: [...]}
     └─ blueprint_validator 校验（scene 数量、互斥性等）
 3. 写入 workspace_4d_taxonomy
 4. 版本 bump → 触发影子回填
 ```

 ### Delta / Merge / Birth

 Blueprint 不是一次性产物——后续内容变化时触发进化：

| 操作 | 触发条件 | 行为 |
|------|----------|------|
| Delta | 出现新的 scene/style 模式 | 追加到 Blueprint |
| Merge | 两个类似 scene 可合并 | LLM 判定→合并 |
| Birth | 全新的内容域 | 创建新的 Blueprint 条目 |

 ---

 ## 2. Embedding Aligner

 代码：`internal/core/service/taxonomyevolve/aligner.go`

 ### 对齐流程

 ```
 自由文本 scene/style（如 "配装备"、"装备选择"）
   │
   ├── 路径 1: 字符串相似度（label + keywords）
   │     └─ ratio ≥ 0.72 → 匹配
   │
   ├── 路径 2: Embedding 最近邻
   │     └─ 余弦 ≥ embed_align_min（默认 0.88）
   │
   └── 无法对齐
         └─ NoteUnaligned → 候选进化池
 ```

 ### 两种对齐源

| 源 | 对齐方式 |
|----|----------|
| Blueprint 工作区 | 优先 embedding 对齐 |
| YAML 脚手架 | `GenericNormalize`（规则对齐） |

 ---

 ## 3. 影子回填

 代码：`internal/core/service/taxonomyevolve/backfill.go`

 ### 回填流程

 ```
 Blueprint 版本 bump
   → 触发 backfill job
   → Worker 按 kb_type 分批读取存量文档
   → 每批 80 条：重新提取 scene/style → Aligner 对齐 → 写回 semantic_tags_4d
   → 全量完成后：缓存 purge
   → 在线主链路不阻塞
 ```

 ### batch 80 的选择

| batch 大小 | 影响 |
|-----------|------|
| 太小（<20） | Worker 调用频繁，Gateway 限流风险 |
| 太大（>200） | 单批耗时长，超时风险；失败面大 |
| 80（默认） | 平衡：每批 ~30s，可重试 |

 ### 回填后缓存 purge

 ```
 回填完成
   → catalog_version bump
   → 所有旧缓存 key 自动失效（key 含 version）
   → 回填期间的短期不一致可接受（新标签文档查新 key，旧文档暂走旧 key）
 ```

 ---

 ## 4. 学习练习

 1. 修改工作区背景（context_profile），观察 Blueprint 是否重新生成。
 2. 手动触发一次回填，观察 Worker 日志中的 batch 进度。
 3. 回填期间发起一次检索——检索结果用的是新标签还是旧标签？
 4. 如果 Blueprint 出错（LLM 输出格式不对），blueprint_validator 如何拒绝？

 ---

 ## 5. 相关文档

 - [自研算法深度学习 §6](../03-algorithms/02-custom-algorithms.md)
 - [workspace-dynamic-taxonomy.md](../../features/workspace-dynamic-taxonomy.md)
 - [workspace-background.md](../../features/workspace-background.md)
