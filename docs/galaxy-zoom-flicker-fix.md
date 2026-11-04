# 星系图缩放闪烁问题修复记录

> **文档类型：历史记录。** 正文保留当时的上下文与结论。 [当前文档入口](README.md)

## 问题现象

GrayDraft 知识库星系图面板中，使用鼠标滚轮或触控板进行放大/缩小操作时，画面出现明显的闪烁抖动，同时棋盘背景格子也伴随重绘闪烁。

## 根因分析

### 成因链路

```
wheel 事件 (120Hz MacBook 触控板)
  → canvas.zoomViewByWheelAt() 更新 Pinia store (panX, panY, viewScale)
  → worldStyle computed 每次生成新对象引用
  → Vue 对比发现引用不同 → patch DOM style 属性
  → 浏览器根据新 transform 重绘合成层
  → 下一帧 wheel 事件到达 → 重复上述过程
```

### 关键发现

1. **每次滚轮事件的 scale 值都不同**：MacBook 触控板的增量滚轮产生连续变化的 deltaY，导致 `zoomViewByWheelAt` 每次计算出的 `viewScale` 都是新值（如 0.055 → 0.063 → 0.071 → ...）

2. **panX/panY 同步变化**：缩放过程中锚点补偿算法使 pan 值也在每一帧改变，意味着 `translate3d` 字符串每次都不同

3. **Vue 响应式成为瓶颈**：Pinia store 值变化 → Vue computed 重算 → 新对象 → DOM patch → 浏览器 repaint，这个链路在 120Hz 触控板下每帧都触发，浏览器 paint 跟不上

4. **棋盘背景受影响**：`gridPatternStyle` 依赖 `gridDisplayScale`（由 `viewScale` 派生），同样每次生成新对象 → patch DOM → 背景格子重绘

### 日志验证

```
[worldStyle] new transform object created {prevTx: 'translate3d(176.02px, -113.624px, 0) scale(0.055)', newTx: 'translate3d(174.059px, -113.424px, 0) scale(0.063)'}
[gridPattern] new step {step: '87.3px'}
[worldStyle] new transform object created {prevTx: 'translate3d(174.059px, -113.424px, 0) scale(0.063)', newTx: 'translate3d(172.098px, -113.224px, 0) scale(0.071)'}
[gridPattern] new step {step: '77.46px'}
...
```

每帧都创建新 transform 对象和新 grid step。

## 尝试过的方案（均未彻底解决）

### 方案 1：稳定对象引用

在 `worldStyle` computed 中做值去重，pan/scale 量化到 0.001 精度，相同值返回同一对象引用。

**结果**：无效。因为每次 wheel 事件的 scale 值本身就不同，量化无法消除差异。

### 方案 2：CSS containment + 节点时间累积保持

对 GalaxyCanvas 的 `watch(props.layout?.nodes)` 添加签名比较避免重复触发，并用 `contain: layout style` 隔离重排。

**结果**：部分有效（组件不再重渲染），但 transform 变化仍触发浏览器 paint。

### 方案 3：rAF 节流 worldStyle

用 `requestAnimationFrame` 合并多次 wheel 事件为一次 DOM 更新。

**结果**：反而更差。引入了视觉延迟（用户操作和画面脱节），且 `v-bind()` CSS 的更新时机与 rAF 不同步。

## 最终方案：直接 DOM 操作绕开 Vue 响应式

### 核心思路

将 CSS transform 的更新路径从 Vue 响应式链路中剥离，改为直接操作 DOM 元素。

### 实现代码

`web/src/components/canvas/CanvasRoot.vue`

```typescript
// 持久 rAF 循环，直接读 Pinia raw state 计算 transform，绕开 Vue 响应式
const _worldBgEl = ref<HTMLElement | null>(null)
const _worldSurfaceEl = ref<HTMLElement | null>(null)
let _worldRaf = 0
let _wPrevTx = ''

function _worldTick() {
  _worldRaf = requestAnimationFrame(_worldTick)
  // 读 Pinia $state（raw access，不触发 reactive tracking）
  const raw = canvas.$state
  const tx = `translate3d(${raw.panX}px, ${raw.panY}px, 0) scale(${raw.viewScale})`
  if (tx === _wPrevTx) return  // 值未变，跳过 DOM 写
  _wPrevTx = tx
  // 直接写 DOM style，Vue 完全不参与此路径
  const bg = _worldBgEl.value
  const sf = _worldSurfaceEl.value
  if (bg) bg.style.transform = tx
  if (sf) sf.style.transform = tx
}

onMounted(() => { _worldRaf = requestAnimationFrame(_worldTick) })
onBeforeUnmount(() => cancelAnimationFrame(_worldRaf))

// 初始渲染用 fallback（模板仍需 worldStyle 绑定）
const worldStyle = {
  transformOrigin: 'center center',
  backfaceVisibility: 'hidden' as const,
}
```

模板中对应的两个 div 添加 `ref`：
```html
<div ref="_worldBgEl" class="canvas-root__world canvas-root__world--bg" ...>
<div ref="_worldSurfaceEl" class="canvas-root__world canvas-root__world--surface">
```

### 关键差异

| | 之前 | 之后 |
|---|---|---|
| Pinia 更新触发 | Vue computed → 新对象 → patch → paint | 仅 Pinia $state 数值变化 |
| DOM 写入 | 每次 wheel 事件都写 | rAF 帧对齐，同值跳过 |
| 浏览器 paint | 高频随机触发 | 与 v-sync 同步 |
| Vue 参与度 | 全程 | 完全不参与缩放渲染路径 |

## 涉及的其他修复

### 1. 星系图节点轨道闪烁

`KnowledgeGalaxyCanvas.vue` 的 `watch(props.layout?.nodes)` 在缩放时因布局引用变化而触发，`nodeTimeAcc.clear()` 重置所有节点轨道时间导致弹回。

修复：保留已有节点的时间累积，仅初始化新节点。

### 2. Core 缺少 Graph 路由

`GetKnowledgeGraph` API 返回空节点。

修复：
- 在 `internal/core/cmd/gateway/main.go` 注册 `GET /api/v1/tenants/:t/kbs/:k/documents/graph` 路由
- 修复 `kb_chunks.index_ready` 为 true（`mark_index_ready` 的 `index_version` 匹配 bug）

### 3. Content-hash 去重

Worker 新增 `check_duplicate_by_content_hash` 方法，在 `_ingest` 中根据文本 SHA256 检测重复文档，避免 DLQ 重放产生重复条目。

## 重建命令

```bash
cd /path/to/GrayDraft

# 重建 web 前端
docker compose -f deploy/docker-compose/docker-compose.dev.yml build web
docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d web

# 重建 core (graph 路由)
docker compose -f deploy/docker-compose/docker-compose.dev.yml build core
docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d core

# 重建 worker (去重)
docker compose -f deploy/docker-compose/docker-compose.dev.yml build worker
docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d worker
```

---

## 技术栈与架构

### 整体栈

```
浏览器端 (Vue 3 + TypeScript + Pinia)
    ↕ HTTP / SSE
BFF (Go - graydraftd)
    ↕ HTTP
Core Gateway (Go - Gin)
    ↕ SQL
PostgreSQL (pgvector)
    ↕ NATS JetStream
Worker (Python - asyncpg + httpx)
```

### 星系图数据流

```
┌─────────────────────────────────────────────────────────┐
│ KnowledgeGalaxyCanvas.vue                                │
│   props.layout: GalaxyLayout                             │
│   ↓                                                     │
│   orbit 动画 (rAF loop + orbitParams Map)                │
│   animatedNodes computed → SVG <g> + <circle> + <line>  │
└──────────────┬──────────────────────────────────────────┘
               │ :layout="galaxy.layout.value"
┌──────────────▼──────────────────────────────────────────┐
│ CanvasRoot.vue                                           │
│   galaxy = useKnowledgeGalaxy()                          │
│   世界层 zoom/pan: _worldTick() 直接操作 DOM transform   │
│   棋盘背景: gridPatternStyle computed (CSS --vars)       │
└──────────────┬──────────────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────────────┐
│ useKnowledgeGalaxy.ts                                    │
│   watch(enabled) → store.setActive()                    │
│   galaxy.sync(op) → galaxyGraphStore.sync()             │
└──────────────┬──────────────────────────────────────────┘
               │
┌──────────────▼──────────────────────────────────────────┐
│ galaxyGraph.ts (Pinia Store)                             │
│   layout: computed → applyFullLayout(inputNodes)         │
│   inputNodes: ref<GalaxyInputNode[]>                     │
│   loadFull() / sync(op) → fetchGraphInputs()            │
│       ↓                                                 │
│   getKnowledgeGraph({ limit: 120 })                     │
└──────────────┬──────────────────────────────────────────┘
               │ POST /api/GetKnowledgeGraph
┌──────────────▼──────────────────────────────────────────┐
│ grayraftd BFF (Go)                                       │
│   nexus.go: handleGetKnowledgeGraph()                   │
│     → a.Nexus.GetDocumentGraph(ctx, scope, limit)       │
│       → GET /api/v1/tenants/:t/kbs/:k/documents/graph   │
└──────────────┬──────────────────────────────────────────┘
               │ HTTP
┌──────────────▼──────────────────────────────────────────┐
│ Core Gateway (Go - Gin)                                  │
│   kb_scope.go: DocumentGraph()                          │
│     → docs.ListPanelDocumentGraph(kbType, limit)        │
│       SQL:                                              │
│       SELECT d.id, d.title, d.extensions, sub.embedding │
│       FROM kb_documents d                               │
│       JOIN LATERAL (SELECT embedding FROM kb_chunks     │
│         WHERE index_ready=true AND embedding NOT NULL   │
│         ORDER BY chunk_index LIMIT 1) sub ON true       │
│       WHERE d.kb_type=$1 AND d.index_ready=true         │
│       ORDER BY d.updated_at DESC LIMIT $2               │
└──────────────┬──────────────────────────────────────────┘
               │ SQL
┌──────────────▼──────────────────────────────────────────┐
│ PostgreSQL                                               │
│   kb_documents (id, title, extensions, index_ready, ...)│
│   kb_chunks (doc_id, embedding(pgvector), index_ready)  │
└─────────────────────────────────────────────────────────┘
```

### 关键文件清单

| 文件 | 作用 |
|---|---|
| `web/src/components/canvas/CanvasRoot.vue` | 画布容器：zoom/pan 世界变换、棋盘背景、子组件挂载 |
| `web/src/components/knowledge/KnowledgeGalaxyCanvas.vue` | SVG 星系图：轨道动画、节点/边渲染、hover/active 交互 |
| `web/src/stores/galaxyGraph.ts` | Pinia store：inputNodes → applyFullLayout → layout 计算 |
| `web/src/composables/useKnowledgeGalaxy.ts` | Composable：暴露 loading/error/layout + sync/reload |
| `web/src/utils/knowledgeGalaxy.ts` | 布局引擎：applyFullLayout、applyForceLayout、聚类/轨道环 |
| `web/src/utils/galaxySync.ts` | 同步策略：增量 add/remove/update/metadata 操作 |
| `web/src/services/knowledge.ts` | API：`getKnowledgeGraph()` + SSE `subscribeKnowledgeLibrary()` |
| `internal/graydraft/internal/server/nexus.go` | BFF handler：`handleGetKnowledgeGraph` |
| `internal/graydraft/pkg/nexus/knowledge_graph.go` | Nexus client：`GetDocumentGraph` |
| `internal/core/handler/kb_scope.go` | Core handler：`DocumentGraph()` |
| `internal/core/repository/postgres/document_panel_repo.go` | SQL：`ListPanelDocumentGraph` |

### GalaxyLayout 数据结构

```typescript
interface GalaxyLayout {
  nodes: GalaxyNode[]   // { id, x, y, radius, clusterId, group, title, hasEmbedding, ... }
  edges: GalaxyEdge[]   // { source, target, similarity }
  bounds: { minX, minY, maxX, maxY }
}
```

### Orbit 动画机制

```
KnowledgeGalaxyCanvas.vue

┌─ rAF loop (每帧) ──────────────────────────────────┐
│ for each node in orbitParams:
│   if id is hovered/active → skip (固定在原位)
│   nodeTimeAcc[id] += dt
│ animatedNodes computed:
│   angle = initialAngle + speed * nodeTimeAcc
│   x = cx + orbitRadius * cos(angle)
│   y = cy + orbitRadius * sin(angle)
└────────────────────────────────────────────────────┘

orbitParams: Map<id, { orbitRadius, initialAngle, speed }>
  → 初始化在 watch(props.layout?.nodes) 中
  → 中心节点(clusterId=0): orbitRadius=0, speed=0
  → 外层节点: orbitRadius = 节点到中心距离, speed = 0.15/√(clusterId)
```

### Zoom/Pan 事件流

```
wheel 事件 (CanvasRoot)
  → zoomViewByWheelAt(deltaY, vx, vy, viewW, viewH, step)
    → Pinia canvas.$state.panX/Y, viewScale 更新
    → _schedulePersist() (320ms debounce → SaveWorkspaceSettings)

Pan (pointerdown + pointermove)
  → canvas.panBy(dx, dy)
    → Pinia canvas.$state.panX/Y 更新
    → _schedulePersist()

DOM 更新 (绕过 Vue 响应式)
  → _worldTick() (rAF 循环)
    → 读 canvas.$state (raw, 不触发 tracking)
    → 算 transform 字符串
    → 直接写 bgEl.style.transform / surfaceEl.style.transform
```

### 导入→星系图 完整链路

```
用户输入文本
  → AgentSmartImport (BFF)
    → Nexus Core SmartImport (LLM 分析 + 分段)
    → NATS JetStream (graydraft.ugc.ingest)
    → Worker 消费 (ingest pipeline)
      → quality gate → parse → chunk → embed → index_ready
      → content_hash 去重检查
      → PostgreSQL (kb_documents + kb_chunks)
    → SSE broadcast library-changed (BFF)
      → 前端 SSE 订阅触发 refreshKnowledgeLibrary({ type: 'full' })
        → ListKnowledgeEntries (BFF → Core → DB)
        → GetKnowledgeGraph (BFF → Core → DB)
          → inputNodes 更新
          → applyFullLayout 重算
          → layout 更新
          → KnowledgeGalaxyCanvas 渲染新节点
```
