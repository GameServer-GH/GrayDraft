#!/usr/bin/env python3
"""pgvector HNSW parameter benchmark (V1-26/27).

Tests different (m, ef_construction, ef_search) combinations
against a local Postgres + pgvector instance.
"""
from __future__ import annotations

import asyncio
import os
import random
import time
from dataclasses import dataclass

import asyncpg
import numpy as np

DB_URL = os.environ.get("DATABASE_URL", "postgresql://postgres:postgres@localhost:25432/graydraft")
TABLE_NAME = "bench_vectors"
DIM = 1024
SEED = 42

# Test configs
HNSW_CONFIGS = [
    # (m, ef_construction, label)
    (16, 100, "baseline"),
    (16, 200, "default"),
    (24, 200, "m24"),
    (32, 200, "m32"),
    (16, 300, "efc300"),
    (32, 300, "m32_efc300"),
]

EF_SEARCH_VALUES = [40, 80, 100, 150, 200]

QUERY_COUNT = 100
TOP_K = 20


@dataclass
class BenchResult:
    label: str
    m: int
    ef_construction: int
    ef_search: int
    build_seconds: float
    index_size_mb: float
    query_p50_ms: float
    query_p95_ms: float
    query_p99_ms: float
    recall_at_20: float  # fraction of brute-force top-20 found


async def create_table(conn: asyncpg.Connection):
    await conn.execute(f"""
        DROP TABLE IF EXISTS {TABLE_NAME};
        CREATE TABLE {TABLE_NAME} (
            id SERIAL PRIMARY KEY,
            embedding vector({DIM})
        );
    """)


async def insert_vectors(conn: asyncpg.Connection, n: int):
    rng = np.random.default_rng(SEED)
    batch_size = 500
    for start in range(0, n, batch_size):
        end = min(start + batch_size, n)
        rows = []
        for _ in range(start, end):
            vec = rng.standard_normal(DIM).astype(np.float32)
            vec = vec / np.linalg.norm(vec)
            rows.append((f"[{','.join(f'{v:.6f}' for v in vec)}]",))
        # asyncpg COPY
        await conn.copy_records_to_table(
            TABLE_NAME, columns=["embedding"], records=rows,
        )


async def build_index(conn: asyncpg.Connection, m: int, ef_construction: int) -> float:
    t0 = time.monotonic()
    await conn.execute(f"SET maintenance_work_mem = '512MB'")
    await conn.execute(f"""
        CREATE INDEX IF NOT EXISTS idx_{TABLE_NAME}_hnsw
        ON {TABLE_NAME} USING hnsw (embedding vector_cosine_ops)
        WITH (m = {m}, ef_construction = {ef_construction});
    """)
    return time.monotonic() - t0


async def drop_index(conn: asyncpg.Connection):
    await conn.execute(f"DROP INDEX IF EXISTS idx_{TABLE_NAME}_hnsw")


async def get_index_size(conn: asyncpg.Connection) -> float:
    row = await conn.fetchrow(f"""
        SELECT pg_total_relation_size('idx_{TABLE_NAME}_hnsw') / 1048576.0 AS size_mb
    """)
    return float(row["size_mb"]) if row else 0.0


async def brute_force_topk(conn: asyncpg.Connection, queries: list[list[float]], k: int) -> list[set[str]]:
    results = []
    for q in queries:
        q_str = f"[{','.join(f'{x:.6f}' for x in q)}]"
        rows = await conn.fetch(f"""
            SELECT id FROM {TABLE_NAME}
            ORDER BY embedding <=> '{q_str}'::vector
            LIMIT {k}
        """)
        results.append({str(r["id"]) for r in rows})
    return results


async def hnsw_topk(conn: asyncpg.Connection, queries: list[list[float]], k: int, ef_search: int) -> tuple[list[set[str]], list[float]]:
    await conn.execute(f"SET LOCAL hnsw.ef_search = {ef_search}")
    results = []
    latencies = []
    for q in queries:
        q_str = f"[{','.join(f'{x:.6f}' for x in q)}]"
        t0 = time.monotonic()
        rows = await conn.fetch(f"""
            SELECT id FROM {TABLE_NAME}
            ORDER BY embedding <=> '{q_str}'::vector
            LIMIT {k}
        """)
        lat = (time.monotonic() - t0) * 1000
        latencies.append(lat)
        results.append({str(r["id"]) for r in rows})
    return results, latencies


def compute_recall(pred: list[set[str]], truth: list[set[str]]) -> float:
    scores = []
    for p, t in zip(pred, truth):
        if not t:
            scores.append(1.0)
        else:
            scores.append(len(p & t) / len(t))
    return sum(scores) / len(scores)


def percentile(values: list[float], p: float) -> float:
    if not values:
        return 0.0
    sorted_vals = sorted(values)
    idx = int(p / 100.0 * (len(values) - 1))
    return sorted_vals[idx]


async def run_bench(conn: asyncpg.Connection, n_vectors: int) -> list[BenchResult]:
    print(f"Generating {n_vectors} vectors (dim={DIM})...")
    await create_table(conn)
    await insert_vectors(conn, n_vectors)

    rng = np.random.default_rng(SEED + 1)
    queries = []
    for _ in range(QUERY_COUNT):
        q = rng.standard_normal(DIM).astype(np.float32)
        queries.append((q / np.linalg.norm(q)).tolist())

    print("Computing brute-force ground truth...")
    truth = await brute_force_topk(conn, queries, TOP_K)

    results: list[BenchResult] = []
    for m, efc, label in HNSW_CONFIGS:
        await drop_index(conn)
        build_time = await build_index(conn, m, efc)
        index_size = await get_index_size(conn)
        print(f"  [{label}] m={m} efc={efc}  build={build_time:.1f}s  size={index_size:.1f}MB")

        for ef in EF_SEARCH_VALUES:
            pred, lats = await hnsw_topk(conn, queries, TOP_K, ef)
            rec = compute_recall(pred, truth)
            r = BenchResult(
                label=label, m=m, ef_construction=efc, ef_search=ef,
                build_seconds=build_time, index_size_mb=index_size,
                query_p50_ms=percentile(lats, 50),
                query_p95_ms=percentile(lats, 95),
                query_p99_ms=percentile(lats, 99),
                recall_at_20=rec,
            )
            results.append(r)
            print(f"    ef_search={ef:3d}  p50={r.query_p50_ms:.1f}ms  p95={r.query_p95_ms:.1f}ms  recall@20={r.recall_at_20:.3f}")

    return results


async def main():
    n_vectors = int(os.environ.get("BENCH_N_VECTORS", "100000"))
    conn = await asyncpg.connect(DB_URL)
    try:
        results = await run_bench(conn, n_vectors)
        print("\n--- Summary (sorted by p95) ---")
        results.sort(key=lambda r: r.query_p95_ms)
        print(f"{'label':<16} {'m':>3} {'efc':>4} {'efs':>4} {'p50_ms':>8} {'p95_ms':>8} {'p99_ms':>8} {'rec@20':>7} {'build_s':>8} {'size_mb':>8}")
        for r in results:
            print(f"{r.label:<16} {r.m:>3} {r.ef_construction:>4} {r.ef_search:>4} "
                  f"{r.query_p50_ms:>8.1f} {r.query_p95_ms:>8.1f} {r.query_p99_ms:>8.1f} "
                  f"{r.recall_at_20:>7.3f} {r.build_seconds:>8.1f} {r.index_size_mb:>8.1f}")

    finally:
        await conn.close()


if __name__ == "__main__":
    asyncio.run(main())
