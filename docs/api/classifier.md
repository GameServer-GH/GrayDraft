# 查询分类器

轻量级规则分类器（<5ms），在检索前自动调整全文/向量权重与全局索引策略。

## 意图类型

| intent | 说明 | 默认权重倾向 |
|--------|------|-------------|
| `proper_noun` | 实体、型号、代码（如 GrayDraft、v3.0） | 全文 ↑ |
| `fact` | 是什么、多少、何时 | 略偏全文 |
| `concept` | 如何、为什么、原理、机制 | 向量 ↑ |
| `hybrid` | 跨领域、对比、综合 | 均衡 + 全局索引 |

## 路由行为

- **hybrid** 且未指定 `kb_type` → 走 `kb_global_index` 物化视图
- **hybrid** 且指定 `kb_type` → 分区内混合检索 + **global_boost**（跨区补充召回）
- 用户显式传 `fulltext_weight` / `vector_weight` 时覆盖分类器权重
- `skip_classifier: true` 跳过分类，使用配置默认权重

## `POST /api/v1/classifier/preview`

调试接口，仅返回分类与路由计划，不执行检索。

```bash
curl -X POST http://localhost:18080/api/v1/classifier/preview \
  -H 'Content-Type: application/json' \
  -d '{"query":"GrayDraft index_ready 是什么"}'
```

## 检索响应新增字段

- `query_intent`
- `fulltext_weight` / `vector_weight`（实际使用的融合权重）
- `strategy` 示例：`hybrid_rrf+concept+rerank_l1_l2`
