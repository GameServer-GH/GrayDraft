# Smart Import 导入阻断点全景分析

> 分析范围：`/api/AgentSmartImport` → Core `/api/v1/ugc/smart-import` 完整调用链路  
> 更新日期：2026-10-28  
> 基于源码：`internal/core/service/smartimport/`, `internal/core/service/ugc/`, `internal/graydraft/internal/server/nexus.go`

---

## 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    AgentSmartImport 请求                          │
│  handleAgentSmartImport (graydraft/nexus.go:209)                │
│  ├─ 媒体识别（图片/视频 VLM）→ raw_text                           │
│  └─ Nexus.SmartImport() → Core POST /api/v1/ugc/smart-import    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              Service.Run() (service.go:75)                       │
│  ├─ validateSmartImportInput()                                   │
│  ├─ preprocessRequest()          ← 多模态附件 VLM/OCR 提取        │
│  ├─ runUnifiedImportLLM()        ← Fast LLM：标题+分类+分割      │
│  ├─ runSingle() / runSplit()                                    │
│  │   ├─ analyzeOne()             ← 置信度检查 + 去重             │
│  │   │   ├─ Analyze()            ← 标题提取 + 规范化             │
│  │   │   ├─ EvaluateConfidence() ← 5维规则检查                   │
│  │   │   └─ ResolveDedup()       ← 3层去重                      │
│  │   └─ submitPreview()          ← 提交到 UGC 队列              │
│  └─ ugc.Notify()                 ← NATS / sync_fallback          │
└─────────────────────────────────────────────────────────────────┘
```

---

## K1 — 基础设施层阻断

### K1.1 NATS JetStream 不可用

**位置**：`ugc/service.go:130-145`

**触发条件**：
- NATS 容器未启动或网络不通
- JetStream `replicas` > 集群节点数（NAS 单节点配了 `replicas:3`）
- JetStream stream 创建失败（subject 冲突等）

**表现**：
```json
"skipped": [{"reason": "unable to dispatch ugc: nats unavailable"}]
"readiness": {"status": "no_task", "message": "submitted request created no ingest task"}
```

**修复**：
- `config.prod.yaml` 中 `nats.jetstream.replicas` 必须 ≤ NATS 集群节点数
- 开启 `ugc.sync_fallback: true` 作为兜底

### K1.2 sync_fallback 未配置

**位置**：`ugc/service.go:120,144`

**触发条件**：
- `ugc.sync_fallback` 未配置或为 `false`（生产配置默认不包含此字段）
- 同时 NATS 不可用

**修复**：
```yaml
ugc:
  sync_fallback: true
```

### K1.3 Worker 地址不可达

**位置**：`ugc/worker_client.go:44-74`

**触发条件**：
- `AI_WORKER_URL` 指向的 Worker 未启动
- 网络隔离导致 HTTP 超时
- Worker 返回 >=400 状态码

**表现**：sync_fallback 降级也失败，UGC task 状态变为 `failed`

---

## K2 — 多模态预处理层阻断

### K2.1 VLM 未配置

**位置**：`preprocess.go:33-35`

**触发条件**：
- `multimodal.enabled: false`
- `MULTIMODAL_API_KEY` 未设置
- 有附件但 multimodal processor 不可用

**表现**：
```json
{"error": "multimodal processing is not configured (set DASHSCOPE_API_KEY)"}
```

### K2.2 VLM 提取返回空

**位置**：`preprocess.go:105`

**触发条件**：
- 图片完全无文字（纯风景、表情包等）
- VLM 调用超时 / 返回异常
- 所有提取结果为空

**表现**：
```json
{"error": "no text after multimodal extraction"}
```

### K2.3 跨模态验证标记

**位置**：`preprocess.go:125-137`

**触发条件**：
- VLM 提取的实体与工作区已有的实体白名单不匹配
- CrossModalVerify 发现矛盾信息

**表现**：不直接阻断，但会标记 `"multimodal_uncertain": true`，影响下游置信度

---

## K3 — 置信度检查层阻断（核心瓶颈）

### K3.1 五维检查机制

**位置**：`confidence_label.go:46-96` → `service.go:468-548`

| 维度 | 检查内容 | 失败阈值 | 多模态豁免 |
|------|---------|---------|-----------|
| **StructureOk** | Markdown 标题检测 | `hasStructureHeadings(sk)` 为 false | ✅ Adapt 2 |
| **EntityOk** | 信息密度 ≥ 0.06 | `IsNoiseByInfoDensity()` 为 true | ✅ Adapt 1 |
| **NoiseOk** | Ngram 重复率 ≤ 0.5 | `ngramRepeatRate > 0.5` | ✅ Adapt 4 (2026-10-28 修复) |
| **TitleOk** | 标题可在正文中验证 | `titlePassesValidation()` 为 false | ❌ 无豁免 |
| **EntityTraceOk** | 实体溯源率 ≥ 50% | `entityTraceRatio < 0.5` | ✅ Adapt 3 |

**判断逻辑**：
- 全部通过 → `LabelPass` → 允许导入
- 1-2 个非关键失败 (entity/noise 都 OK) → `LabelWarning` → 允许（前端警告）
- ≥3 个失败 或 noise 失败 → **`LabelReject` → 阻断导入**

### K3.2 多模态 NoiseOk 误判（已修复）

**位置**：`service.go:505-508`

**根因**：VLM 从图片提取的教程资料内容天然含有大量重复术语（角色名、装备名、方案标题），导致 ngram 重复率超过 0.5 阈值，被误判为噪音。

**修复**（已应用）：Adapt 4 — 当 `StructureOk && EntityOk` 时豁免 `NoiseOk`

### K3.3 标题验证失败

**位置**：`confidence_label.go:109-119` → `analyzer.go:196-224`

**触发条件**：
- 标题不在正文任何一行中出现
- 编辑距离 > 3
- LCS 相似度 < 70%
- 标题为 markdown 列表标记前缀泄露（如 `"- 角色名称：绯雪"`）

**表现**：`TitleOk = false`，可能触发 LabelReject

**修复**（部分已应用）：`isInvalidTitle()` 增加列表前缀检测

### K3.4 信息密度过低

**位置**：`info_density.go:406-408`

**触发条件**：
- 纯文本内容缺少领域关键词（CJK bigram 匹配率低）
- 内容太短（< 10 runes）
- 无数字、实体、结构标记

**表现**：`EntityOk = false`，密度 < 0.06 → Reject；0.06-0.10 → Warning + LLM 清洗标记

---

## K4 — 去重层阻断

### K4.1 三层层去重

**位置**：`dedup.go:27-135`

| 层级 | 检测方式 | 阻断行为 |
|------|---------|---------|
| Layer 1 | 精确 content_hash 匹配 | `unchanged` — 跳过导入 |
| Layer 2 | 标题完全匹配 | `update` — 更新已有文档 |
| Layer 3 | Embedding 语义相似度 | `blocked_similar` — 阻断（疑似重复） |

### K4.2 Layer 3 误拦截

**位置**：`dedup.go:121-132`

**触发条件**：
- 相似度 ≥ 阈值（单体文档 0.80 / 综合合集 0.85）
- 无实体信息时阈值提升到 0.92

**表现**：
```json
"action": "blocked_similar"
"import_allowed": false
```

**说明**：Layer 3 需要 caller 主动传入 embedding 相似度。当前 `analyzeOne` 调用链中未传入相似度（传 0），所以 Layer 3 实际上**不会触发**。这意味着只有 Layer 1+2 在生效。

### K4.3 目标文档不存在

**位置**：`dedup.go:44-48`

**触发条件**：
- 前端传了 `target_doc_id` 但在数据库中找不到

**表现**：
```json
{"error": "target document xxx not found in kb_type xxx"}
```

---

## K5 — KB 类型 / 工作区分区层阻断

### K5.1 缺少工作区分区

**位置**：`kb_type.go:23-30`

**触发条件**：
- `hint_kb_type` 为空
- `hint_kb_type` 不是 `gd_*` 格式

**表现**：
```json
{"error": "hint_kb_type required: workspace partition (gd_*)"}
```

### K5.2 LLM 返回非工作区 kb_type

**位置**：`unified_import_llm.go:210-216`

**触发条件**：
- LLM 返回了与工作区分区不一致的 kb_type
- LLM 编造了新的 kb_type

**行为**：静默使用 `hint_kb_type` 覆盖，不阻断但会产生 `"workspace kb_type locked"` 的 reason

---

## K6 — 标题提取层阻断

### K6.1 标题为空或无效

**位置**：`title_validate.go:22-53` → `analyzer.go:116-146`

**触发条件**：
- `extractTitle()` 找不到有效的首行 / markdown 标题
- `inferSubjectTitle()` 正则 `"…是…"` 模式无匹配
- 所有候选标题被 `isInvalidTitle()` 过滤
- 最终 fallback 到 `fallbackTitle()` — 截取正文前 80 字符

**表现**：标题为 `"InternVLV3.3绯雪概念说明\n\nInternVLV3.3绯雪…"` 之类的截断文本

### K6.2 标题含列表标记（已修复）

**位置**：`title_validate.go:43-48`

**根因**：VLM 提取的 markdown 中 `- 角色名称：绯雪` 没有被 `isInvalidTitle()` 拦截。

**修复**（已应用）：增加 `- ` / `* ` 前缀和 `reListMarker` 正则检测

### K6.3 标题内容验证失败

**位置**：`merge_llm.go:18-20`

**触发条件**：LLM 返回的标题 `titleAppearsInContent()` 返回 false

**行为**：LLM 标题被丢弃，使用 rules 标题。reason 附加 `"; llm title rejected (not in source)"`

---

## K7 — 拆分策略层阻断

### K7.1 拆分模式但无 importable 段

**位置**：`service.go:380-390`

**触发条件**：split=true 时，所有段落的 `ImportAllowed` 都为 false

**表现**：
```json
"message": "no segment submitted (all below threshold or unchanged)"
"error": "导入暂未成功：no segment submitted (all below threshold or unchanged)"
```

### K7.2 LLM 分割失败降级

**位置**：`plan_segments.go:13-30`

**触发条件**：
- Unified LLM 两次调用都失败
- 无缓存的 split plan

**行为**：降级为 `singleSkeletonPlan`（全文一段），但该段仍需通过 K3 置信度检查

### K7.3 分割覆盖被覆盖

**位置**：`service.go:281-283`

**触发条件**：
- 前端传入 `segments_override` 数组
- 覆盖了 LLM 的拆分结果

**说明**：如果覆盖的段数 > 3，会额外触发 MinHash 聚类去重合并

---

## K8 — LLM 调用层阻断

### K8.1 LLM 不可用

**位置**：`service.go:145-175` → `unified_import_service.go:18-26`

**触发条件**：
- `use_llm: false`
- LLM API key 未配置
- LLM endpoint 不可达

**行为**：跳过 Unified LLM，走纯规则路径（`method: "rules"`）

### K8.2 LLM 两次调用均失败

**位置**：`unified_import_service.go:74-84`

**触发条件**：Fast LLM 返回格式错误的 JSON，retry 也失败

**行为**：
- 设置 `import_llm_unified_fail_count = 2`
- Analyzer 中的 `useLegacyImportLLM()` 返回 true，走旧版标题 LLM
- 分割降级为 skeleton-based

### K8.3 LLM 超时

**位置**：`llmclient/client.go` (下游依赖)

**触发条件**：Fast LLM 响应时间超过配置的 `timeout_seconds`

**表现**：整个导入请求在 BFF 层感知为超时，可能返回 504

---

## K9 — 导入提交后阻断（Worker 层）

### K9.1 Worker 处理失败

**位置**：`ugc/worker_client.go:44-74`

**触发条件**：
- chunk embedding 失败
- 数据库写入异常
- 段落拆分后索引构建超时

**表现**：
- nat 通道：task 状态为 `failed`，可通过 DLQ 重放
- sync_fallback 通道：直接返回 HTTP 错误

### K9.2 DLQ 死信

**位置**：`nats/dlq.go`

**触发条件**：Worker 连续消费失败 + 消息过期

**行为**：消息进入 `GRAYDRAFT_UGC_DLQ` stream，需管理员手动重放

---

## K10 — BFF 层阻断（GrayDraft）

### K10.1 Nexus Core 不可达

**位置**：`nexus.go:218-222`

**触发条件**：`GRAYDRAFT_CORE_URL` 未配置

**表现**：
```json
{"error": "GRAYDRAFT_CORE_URL not configured"}
HTTP 503
```

### K10.2 媒体识别超时后取消

**位置**：`nexus.go:357-368`

**触发条件**：
- VLM 描述图片/视频超过 BFF 超时
- 用了 `context.Background()` 避免 client disconnect 取消，但仍有内部超时

**表现**：
```json
{"error": "media description failed: ..."}
HTTP 502
```

### K10.3 媒体格式不支持

**位置**：`nexus.go:274-280`

**触发条件**：
- 视频非 mp4/webm/mov
- 图片非 png/jpeg/webp/gif
- 文件过大（图片 > 20MB，视频 > 100MB）

**表现**：
```json
{"error": "unsupported video type: ..."}
{"error": "video too large (xxx bytes, max xxx)"}
```

---

## 故障排查决策树

```
导入失败
│
├─ error 包含 "GRAYDRAFT_CORE_URL not configured"
│  → K10.1: GrayDraft 环境变量缺失
│
├─ error 包含 "nats unavailable" / readiness.status = "no_task"
│  ├─ 检查 NATS 是否运行 → K1.1
│  ├─ 检查 replicas 配置 → K1.1
│  └─ 检查 sync_fallback → K1.2
│
├─ error 包含 "multimodal"
│  ├─ "not configured" → K2.1
│  └─ "no text after" → K2.2
│
├─ error 包含 "below minimum" / confidence check 相关
│  ├─ 检查 preview.reason 中的 Label → K3
│  ├─ LabelReject → 检查 cr.FailedChecks → K3.1-K3.4
│  └─ 标题异常 → K6
│
├─ preview.action = "unchanged" / "blocked_similar"
│  → K4.1-K4.3（去重拦截）
│
├─ error 包含 "hint_kb_type" / "workspace partition"
│  → K5.1（工作区分区缺失）
│
└─ 提交成功但 status = "failed"
   → K9.1-K9.2 (Worker 层故障), 检查 Worker 日志 / DLQ
```

---

## 关键配置检查清单

| 配置项 | 文件 | 推荐值 | 风险 |
|--------|------|--------|------|
| `nats.jetstream.replicas` | `config.prod.yaml` | ≤ NATS节点数 | K1.1 |
| `ugc.sync_fallback` | `config.prod.yaml` | `true` | K1.2 |
| `ai_worker.url` | `config.prod.yaml` | 可达的Worker地址 | K1.3 |
| `multimodal.enabled` | `config.prod.yaml` | `true`（图片导入必须） | K2.1 |
| `multimodal.api_key` | `.env.core` | 有效的火山方舟Key | K2.1 |
| `smart_import.min_confidence` | `config.prod.yaml` | `0.55`（默认即可） | K3 |
| `model_gateway.url` | `config.prod.yaml` | 可达的Gateway | K8 |
| `GRAYDRAFT_CORE_URL` | `.env.graydraft` | `http://core:18080` | K10.1 |
