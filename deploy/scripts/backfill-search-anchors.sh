#!/usr/bin/env bash
# 为存量文档 LLM 提取 search_anchors 并刷新 search_text / embedding
set -euo pipefail

KB_TYPE="${1:-}"
if [[ -z "$KB_TYPE" ]]; then
  echo "用法: $0 <kb_type>" >&2
  exit 1
fi

ENV="${GRAYDRAFT_ENV:-dev}"
WORKER_CONTAINER="${WORKER_CONTAINER:-graydraft-worker-${ENV}}"
PG_CONTAINER="${PG_CONTAINER:-graydraft-postgres-${ENV}}"
DB_USER="${POSTGRES_USER:-graydraft_admin}"
DB_NAME="${POSTGRES_DB:-graydraft_kb}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

echo "==> search_anchors LLM 回填 kb_type=${KB_TYPE}"

docker exec -e "BACKFILL_KB_TYPE=${KB_TYPE}" "$WORKER_CONTAINER" python3 -c "
import asyncio, json, os, asyncpg

async def main():
    kb = os.environ['BACKFILL_KB_TYPE']
    user = os.environ.get('DB_USER', 'graydraft_admin')
    password = os.environ.get('DB_PASSWORD', 'dev_password_123')
    host = os.environ.get('DB_HOST', 'postgres')
    name = os.environ.get('DB_NAME', 'graydraft_kb')
    pool = await asyncpg.create_pool(f'postgresql://{user}:{password}@{host}:5432/{name}', min_size=1, max_size=2)
    from worker.config import Settings
    from worker.enrichment.search_anchors import extract_search_anchors
    from worker.indexer.search_text import build_search_text
    from worker.indexer.embedder import embed_texts
    settings = Settings()
    async with pool.acquire() as conn:
        docs = await conn.fetch(
            '''SELECT d.id::text, d.title, COALESCE(d.extensions, '{}'::jsonb) AS ext,
                      (SELECT c.content FROM kb_chunks c WHERE c.doc_id = d.id AND c.kb_type = d.kb_type ORDER BY c.chunk_index LIMIT 1) AS preview
               FROM kb_documents d WHERE d.kb_type = \$1''', kb)
    for doc in docs:
        ext = json.loads(doc['ext']) if isinstance(doc['ext'], str) else dict(doc['ext'])
        preview = (doc['preview'] or '')[:2000]
        anchors = await extract_search_anchors(doc['title'] or '', preview, settings, existing_extensions=ext)
        if anchors:
            ext['search_anchors'] = anchors
        async with pool.acquire() as conn:
            await conn.execute('UPDATE kb_documents SET extensions = \$1::jsonb, updated_at = now() WHERE id = \$2::uuid', json.dumps(ext), doc['id'])
            rows = await conn.fetch('SELECT id::text, content, chunk_index FROM kb_chunks WHERE doc_id = \$1::uuid AND kb_type = \$2', doc['id'], kb)
            tags = await conn.fetch('SELECT tag FROM kb_doc_tags WHERE doc_id = \$1::uuid', doc['id'])
            tag_list = [r['tag'] for r in tags]
            search_texts = [build_search_text(doc_title=doc['title'] or '', chunk_content=r['content'], tags=tag_list, extensions=ext) for r in rows]
            vecs = await embed_texts(search_texts, settings) if settings.model_gateway_url else [None]*len(rows)
            for r, st, vec in zip(rows, search_texts, vecs):
                if vec:
                    literal = '[' + ','.join(str(float(x)) for x in vec) + ']'
                    await conn.execute('UPDATE kb_chunks SET search_text = \$1, embedding = \$2::vector, updated_at = now() WHERE id = \$3::uuid', st, literal, r['id'])
                else:
                    await conn.execute('UPDATE kb_chunks SET search_text = \$1, updated_at = now() WHERE id = \$2::uuid', st, r['id'])
        print('updated', doc['title'])
    await pool.close()

asyncio.run(main())
"

echo "==> 完成"

# Purge retrieval and answer caches so updated anchors / embeddings take effect.
CORE_HOST="${CORE_HOST:-localhost:8080}"
if command -v curl &>/dev/null; then
  echo "==> 清除检索/答案缓存 kb_type=${KB_TYPE}"
  curl -sf -X POST "http://${CORE_HOST}/api/v1/kb/${KB_TYPE}/workspace-data/purge" \
    -H 'Content-Type: application/json' \
    -d "{\"kb_type\":\"${KB_TYPE}\",\"source\":\"backfill-search-anchors\"}" || true
  echo ""
fi
