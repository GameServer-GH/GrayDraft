#!/usr/bin/env bash
# 对指定 kb_type 按 search_text 重新计算 embedding（3.5 向量层对齐）
set -euo pipefail

KB_TYPE="${1:-}"
if [[ -z "$KB_TYPE" ]]; then
  echo "用法: $0 <kb_type>" >&2
  exit 1
fi

ENV="${GRAYDRAFT_ENV:-dev}"
WORKER_CONTAINER="${WORKER_CONTAINER:-graydraft-worker-${ENV}}"

echo "==> reembed kb_type=${KB_TYPE} via ${WORKER_CONTAINER}"

docker exec -e "REEMBED_KB_TYPE=${KB_TYPE}" "$WORKER_CONTAINER" python3 -c "
import asyncio, os, asyncpg

async def main():
    kb = os.environ['REEMBED_KB_TYPE']
    user = os.environ.get('DB_USER', 'graydraft_admin')
    password = os.environ.get('DB_PASSWORD', 'dev_password_123')
    host = os.environ.get('DB_HOST', 'postgres')
    name = os.environ.get('DB_NAME', 'graydraft_kb')
    dsn = f'postgresql://{user}:{password}@{host}:5432/{name}'
    pool = await asyncpg.create_pool(dsn, min_size=1, max_size=2)
    from worker.config import Settings
    from worker.indexer.embedder import embed_texts
    settings = Settings()
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            '''SELECT c.id::text, COALESCE(NULLIF(c.search_text, ''), c.content) AS st
               FROM kb_chunks c WHERE c.kb_type = \$1 AND c.index_ready = true''',
            kb,
        )
    if not rows:
        print('no chunks')
        await pool.close()
        return
    texts = [r['st'] for r in rows]
    ids = [r['id'] for r in rows]
    vecs = await embed_texts(texts, settings)
    async with pool.acquire() as conn:
        for cid, vec in zip(ids, vecs):
            if not vec:
                continue
            literal = '[' + ','.join(str(float(x)) for x in vec) + ']'
            await conn.execute(
                'UPDATE kb_chunks SET embedding = \$1::vector, updated_at = now() WHERE id = \$2::uuid',
                literal, cid,
            )
    await pool.close()
    print(f'updated {len(ids)} chunks')

asyncio.run(main())
"

echo "==> reembed 完成"
