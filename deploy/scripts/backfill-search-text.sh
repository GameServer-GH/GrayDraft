#!/usr/bin/env bash
# 回填 kb_chunks.search_text + metadata（与 Worker build_search_text / chunk_semantic 对齐）
set -euo pipefail

KB_TYPE="${1:-}"
ENV="${GRAYDRAFT_ENV:-dev}"
WORKER_CONTAINER="${WORKER_CONTAINER:-graydraft-worker-${ENV}}"

echo "==> 回填 kb_chunks.search_text (${ENV}) kb_type=${KB_TYPE:-all}"

docker exec -e "BACKFILL_KB_TYPE=${KB_TYPE}" "$WORKER_CONTAINER" python3 -c "
import asyncio, json, os, asyncpg

async def main():
    kb = os.environ.get('BACKFILL_KB_TYPE', '').strip()
    user = os.environ.get('DB_USER', 'graydraft_admin')
    password = os.environ.get('DB_PASSWORD', 'dev_password_123')
    host = os.environ.get('DB_HOST', 'postgres')
    name = os.environ.get('DB_NAME', 'graydraft_kb')
    pool = await asyncpg.create_pool(f'postgresql://{user}:{password}@{host}:5432/{name}', min_size=1, max_size=4)
    from worker.indexer.chunk_semantic import chunk_metadata_payload, extensions_for_chunk
    from worker.indexer.search_text import build_search_text
    q = '''SELECT c.id::text, c.content, c.doc_id::text, c.kb_type,
                  COALESCE(d.title,'') AS title, COALESCE(d.extensions,'{}'::jsonb) AS ext,
                  COALESCE(c.metadata,'{}'::jsonb) AS meta
           FROM kb_chunks c JOIN kb_documents d ON d.id = c.doc_id'''
    args = []
    if kb:
        q += ' WHERE c.kb_type = \$1'
        args = [kb]
    async with pool.acquire() as conn:
        rows = await conn.fetch(q, *args)
    updated = 0
    async with pool.acquire() as conn:
        for r in rows:
            ext = dict(r['ext']) if r['ext'] else {}
            meta = dict(r['meta']) if r['meta'] else {}
            merged = {**ext, **meta}
            chunk_ext = extensions_for_chunk(merged, r['content'] or '')
            meta_out = chunk_metadata_payload(chunk_ext)
            st = build_search_text(
                doc_title=r['title'] or '',
                chunk_content=r['content'] or '',
                tags=[],
                extensions=chunk_ext,
            )
            await conn.execute(
                'UPDATE kb_chunks SET search_text=\$1, metadata=\$2::jsonb, updated_at=now() WHERE id=\$3::uuid',
                st,
                json.dumps(meta_out),
                r['id'],
            )
            updated += 1
    await pool.close()
    print('done', updated)

asyncio.run(main())
"

echo "==> 完成（embedding 未重算；需完整重索引请重新 ingest）"

# Purge retrieval and answer caches so updated search_text / metadata take effect.
CORE_HOST="${CORE_HOST:-localhost:8080}"
KB_TYPE_PURGE="${KB_TYPE:-}"
if [[ -n "$KB_TYPE_PURGE" ]] && command -v curl &>/dev/null; then
  echo "==> 清除检索/答案缓存 kb_type=${KB_TYPE_PURGE}"
  curl -sf -X POST "http://${CORE_HOST}/api/v1/kb/${KB_TYPE_PURGE}/workspace-data/purge" \
    -H 'Content-Type: application/json' \
    -d "{\"kb_type\":\"${KB_TYPE_PURGE}\",\"source\":\"backfill-search-text\"}" || true
  echo ""
fi
