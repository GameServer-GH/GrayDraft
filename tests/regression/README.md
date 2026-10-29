# Golden Cases Regression Tests (V1-22)

自动化检索回归测试，验证每次变更后召回率、排序、无答案误召回率、
延迟以及候选量统计不退化。

测试数据由 `fixture.json` 定义，`seed.sql` 使用固定 UUID 幂等写入
`regression_fixture` 分区。固定 UUID 只属于本地测试数据，不是生产文档 ID。

## 运行

```bash
# 在已迁移的本地 PostgreSQL 中导入一次（按本机连接参数调整）
psql "$DATABASE_URL" -f tests/regression/seed.sql

# 本地
python3 tests/regression/golden_cases.py

# 指定 Core 地址
CORE_URL=http://localhost:18080 python3 tests/regression/golden_cases.py
```

## 添加新 case

编辑 `fixture.json` 中的 `documents` 与 `cases`，并同步更新幂等
`seed.sql`。质量案例必须填写：
- `query`: 用户问题
- `kb_type`: 知识库分区
- `expected_doc_ids`: 预期出现在 top 10 中的固定 fixture 文档 ID 列表
- `min_recall`: 最低召回率阈值 (0-1)
- `description`: 用例说明

## CI 集成

```bash
# GitHub Actions / Jenkins
python3 tests/regression/golden_cases.py && echo "REG_OK" || echo "REG_FAIL"
```

退出码 0 = 全部通过，1 = 有失败或错误。

输出指标包括 Recall@10、MRR@10、nDCG@10、无答案误召回率、
p50/p95 延迟、waterfall tier 分布，以及 rerank 调用、跳过、失败计数。
缺少 expected IDs 的非无答案案例会直接报错，不能计为通过。
