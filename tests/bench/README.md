# pgvector 压测工具 (V1-26/27)

测试 HNSW 不同 (m, ef_construction, ef_search) 参数组合的构建时间、索引大小、查询延迟和召回率。

## 运行

```bash
# 默认 10 万向量 (DIM=1024)
python3 tests/bench/pgvector_bench.py

# 自定义规模
BENCH_N_VECTORS=500000 python3 tests/bench/pgvector_bench.py

# 指定数据库
DATABASE_URL=postgresql://user:pass@host:5432/db python3 tests/bench/pgvector_bench.py
```

## 前置条件

- Postgres 16+ with pgvector extension
- `pip install asyncpg numpy`

## 输出

按 P95 延迟排序的表格，包含 recall@20、构建时间、索引大小。用于选择生产环境的最优参数。

## 典型推荐

| 数据量 | m | ef_construction | ef_search | 说明 |
|--------|---|-----------------|-----------|------|
| < 50 万 | 16 | 200 | 80 | 低延迟优先 |
| 50-100 万 | 24 | 200 | 100 | 平衡 |
| > 100 万 | 32 | 300 | 150-200 | 高召回优先 |
