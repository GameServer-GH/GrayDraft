"""PostgreSQL persistence for documents, chunks, QA, and tags."""

from __future__ import annotations

import asyncio
import json
import uuid
from typing import Any

import asyncpg

from worker.config import Settings


def _section_label(meta: dict[str, Any], doc_title: str) -> str:
    return str(meta.get("section_path") or meta.get("parent_title") or doc_title or "").strip()


def _parent_chunk_ids(ids: list[uuid.UUID], sections: list[str]) -> list[str | None]:
    parents: list[str | None] = [None] * len(ids)
    for i, section in enumerate(sections):
        if not section:
            continue
        prefix = section.rsplit(" / ", 1)[0] if " / " in section else ""
        for j in range(i - 1, -1, -1):
            earlier = sections[j]
            if not earlier or earlier == section:
                continue
            if (prefix and earlier == prefix) or section.startswith(earlier + " / "):
                parents[i] = str(ids[j])
                break
    return parents


def _entity_edge(key: str) -> tuple[str, str]:
    k = key.strip()
    if k.startswith("--") or (k.startswith("-") and len(k) > 1):
        return "parameter", "参数属于"
    if k.endswith("()") or ("." in k and " " not in k):
        return "api", "定义于"
    return "concept", "定义于"


class DocumentRepository:
    def __init__(self, pool: asyncpg.Pool) -> None:
        self._pool = pool

    @classmethod
    async def create(cls, settings: Settings) -> DocumentRepository:
        pool = await asyncpg.create_pool(
            dsn=settings.database_dsn,
            min_size=1,
            max_size=10,
            command_timeout=60,
        )
        return cls(pool)

    async def close(self) -> None:
        await self._pool.close()

    async def ping(self) -> bool:
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.fetchval("SELECT 1")
        return True

    # --- V1-09: stage event logging ---

    async def insert_stage_event(
        self,
        *,
        task_id: str,
        trace_id: str | None = None,
        stage: str,
        status: str,
        attempt: int = 1,
        index_version: int | None = None,
        chunker_version: str | None = None,
        error_code: str | None = None,
        error_message: str | None = None,
    ) -> None:
        async with self._pool.acquire() as conn:
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            task_id_uuid = uuid.UUID(task_id)
            await conn.execute(
                """
                INSERT INTO ugc_ingest_stage_events
                    (task_id, trace_id, stage, status, attempt,
                     index_version, chunker_version, error_code, error_message)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
                """,
                task_id_uuid, trace_id, stage, status, attempt,
                index_version, chunker_version, error_code, error_message,
            )

    async def set_ugc_task_stage(
        self,
        *,
        task_id: str,
        current_stage: str,
        stage_retry_count: int = 0,
    ) -> None:
        task_id_uuid = uuid.UUID(task_id)
        async with self._pool.acquire() as conn:
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                """
                UPDATE ugc_ingest_tasks
                SET current_stage = $2,
                    stage_retry_count = $3,
                    updated_at = now()
                WHERE id = $1
                """,
                task_id_uuid, current_stage, stage_retry_count,
            )

    async def upsert_document(
        self,
        *,
        doc_id: uuid.UUID,
        kb_type: str,
        title: str,
        source_url: str | None,
        content_hash: str,
        index_ready: bool = False,
        quality_score: float = 0.5,
        extensions: dict[str, Any] | None = None,
    ) -> None:
        ext = json.dumps(extensions or {})
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                """
                INSERT INTO kb_documents (id, kb_type, title, source_url, content_hash, index_ready, quality_score, extensions)
                VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
                ON CONFLICT (id) DO UPDATE SET
                    title = EXCLUDED.title,
                    source_url = EXCLUDED.source_url,
                    content_hash = EXCLUDED.content_hash,
                    index_ready = EXCLUDED.index_ready,
                    quality_score = EXCLUDED.quality_score,
                    extensions = EXCLUDED.extensions,
                    kb_version = kb_documents.kb_version + CASE
                        WHEN kb_documents.content_hash IS DISTINCT FROM EXCLUDED.content_hash THEN 1
                        ELSE 0
                    END,
                    updated_at = now()
                """,
                doc_id,
                kb_type,
                title,
                source_url,
                content_hash,
                index_ready,
                quality_score,
                ext,
            )

    async def insert_chunks(
        self,
        *,
        doc_id: uuid.UUID,
        kb_type: str,
        chunks: list[str],
        embeddings: list[list[float] | None],
        structured_embeddings: list[list[float] | None] | None = None,
        index_ready: bool = False,
        quality_score: float = 0.5,
        doc_title: str = "",
        tags: list[str] | None = None,
        extensions: dict[str, Any] | None = None,
        search_texts: list[str] | None = None,
        chunk_metadatas: list[dict[str, Any]] | None = None,
    ) -> list[uuid.UUID]:
        from worker.indexer.search_text import build_search_text

        if structured_embeddings is None:
            structured_embeddings = [None] * len(chunks)
        ids: list[uuid.UUID] = [uuid.uuid4() for _ in chunks]
        edge_rows: list[tuple] = []
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                "DELETE FROM kb_chunks WHERE doc_id = $1 AND kb_type = $2",
                doc_id,
                kb_type,
            )

            rows_both: list[tuple] = []
            rows_primary: list[tuple] = []
            rows_none: list[tuple] = []
            prepared: list[dict[str, Any]] = []
            sections: list[str] = []
            for i in range(len(ids)):
                meta: dict[str, Any] = {}
                if chunk_metadatas and i < len(chunk_metadatas) and isinstance(chunk_metadatas[i], dict):
                    meta = dict(chunk_metadatas[i])
                section = _section_label(meta, doc_title)
                prepared.append(meta)
                sections.append(section)
            parents = _parent_chunk_ids(ids, sections)

            for i, (text, emb) in enumerate(zip(chunks, embeddings, strict=False)):
                st_emb = structured_embeddings[i] if i < len(structured_embeddings) else None
                cid = ids[i]
                if search_texts and i < len(search_texts):
                    search_text = search_texts[i]
                else:
                    chunk_ext = extensions
                    if chunk_metadatas and i < len(chunk_metadatas):
                        chunk_ext = {**(extensions or {}), **chunk_metadatas[i]}
                    search_text = build_search_text(
                        doc_title=doc_title,
                        chunk_content=text,
                        tags=tags,
                        extensions=chunk_ext,
                    )
                meta = prepared[i] if i < len(prepared) else {}
                if i > 0:
                    meta["prev_chunk_id"] = str(ids[i - 1])
                if i + 1 < len(ids):
                    meta["next_chunk_id"] = str(ids[i + 1])
                if i < len(parents) and parents[i]:
                    meta["parent_chunk_id"] = parents[i]
                section = sections[i] if i < len(sections) else ""
                if section:
                    meta["section_path"] = section
                    edge_rows.append((kb_type, "section", section, "定义于", str(cid)))
                kind = str(meta.get("doc_type") or meta.get("media_type") or "").strip().lower()
                if kind in {"figure", "image", "chart"}:
                    edge_rows.append((kb_type, "figure", section or doc_title or "figure", "图示", str(cid)))
                tags4d = meta.get("semantic_tags_4d")
                if isinstance(tags4d, dict):
                    for ent in tags4d.get("entities") or []:
                        key = str(ent).strip()
                        if key:
                            src_kind, rel = _entity_edge(key)
                            edge_rows.append((kb_type, src_kind, key, rel, str(cid)))
                meta_json = json.dumps(meta if meta else {})
                emb_literal = _vector_literal(emb) if emb else None
                st_literal = _vector_literal(st_emb) if st_emb else None

                if emb_literal and st_literal:
                    rows_both.append((str(cid), str(doc_id), kb_type, i, text, search_text,
                                      emb_literal, st_literal, index_ready, quality_score, meta_json))
                elif emb_literal:
                    rows_primary.append((str(cid), str(doc_id), kb_type, i, text, search_text,
                                         emb_literal, index_ready, quality_score, meta_json))
                else:
                    rows_none.append((str(cid), str(doc_id), kb_type, i, text, search_text,
                                      index_ready, quality_score, meta_json))

            if rows_both:
                await conn.executemany(
                    """
                    INSERT INTO kb_chunks
                        (id, doc_id, kb_type, chunk_index, content, search_text, embedding, embedding_structured, index_ready, quality_score, metadata)
                    VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::vector, $8::vector, $9, $10, $11::jsonb)
                    """,
                    rows_both,
                )
            if rows_primary:
                await conn.executemany(
                    """
                    INSERT INTO kb_chunks
                        (id, doc_id, kb_type, chunk_index, content, search_text, embedding, index_ready, quality_score, metadata)
                    VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::vector, $8, $9, $10::jsonb)
                    """,
                    rows_primary,
                )
            if rows_none:
                await conn.executemany(
                    """
                    INSERT INTO kb_chunks
                        (id, doc_id, kb_type, chunk_index, content, search_text, index_ready, quality_score, metadata)
                    VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9::jsonb)
                    """,
                    rows_none,
                )
            if isinstance(extensions, dict):
                for raw_citation in extensions.get("citations") or []:
                    citation = str(raw_citation).strip()
                    try:
                        uuid.UUID(citation)
                    except ValueError:
                        continue
                    edge_rows.append((kb_type, "note", doc_title or "note", "笔记沉淀自", citation))
            if edge_rows:
                await conn.execute("DELETE FROM edu_edges WHERE dst_chunk_id = ANY($1::uuid[])", ids)
                await conn.executemany(
                    """
                    INSERT INTO edu_edges (kb_type, src_kind, src_key, rel, dst_chunk_id)
                    VALUES ($1, $2, $3, $4, $5::uuid)
                    """,
                    edge_rows,
                )
        return ids

    async def list_chunk_texts(
        self,
        *,
        doc_id: str | uuid.UUID,
        kb_type: str,
        limit: int = 80,
    ) -> list[dict[str, Any]]:
        lim = max(1, min(int(limit or 80), 200))
        async with self._pool.acquire() as conn:
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            rows = await conn.fetch(
                """
                SELECT id, content
                FROM kb_chunks
                WHERE doc_id = $1::uuid AND kb_type = $2
                ORDER BY chunk_index ASC
                LIMIT $3
                """,
                str(doc_id),
                kb_type,
                lim,
            )
        return [{"id": r["id"], "content": r["content"] or ""} for r in rows]

    async def insert_qa_pairs(
        self,
        *,
        doc_id: uuid.UUID,
        kb_type: str,
        pairs: list[dict[str, Any]],
        chunk_ids: list[uuid.UUID],
        index_ready: bool,
    ) -> int:
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                "DELETE FROM kb_qa_pairs WHERE doc_id = $1 AND kb_type = $2",
                doc_id,
                kb_type,
            )
            count = 0
            for p in pairs:
                chunk_id = chunk_ids[p["chunk_index"]] if p["chunk_index"] < len(chunk_ids) else None
                await conn.execute(
                    """
                    INSERT INTO kb_qa_pairs (doc_id, kb_type, chunk_id, question, answer, index_ready, quality_score)
                    VALUES ($1::uuid, $2, $3::uuid, $4, $5, $6, $7)
                    """,
                    str(doc_id),
                    kb_type,
                    str(chunk_id) if chunk_id else None,
                    p["question"],
                    p["answer"],
                    index_ready,
                    p.get("quality_score", 0.6),
                )
                count += 1
        return count

    async def insert_tags(self, *, doc_id: uuid.UUID, kb_type: str, tags: list[str]) -> None:
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute("DELETE FROM kb_doc_tags WHERE doc_id = $1", doc_id)
            for tag in tags:
                await conn.execute(
                    "INSERT INTO kb_doc_tags (doc_id, kb_type, tag) VALUES ($1::uuid, $2, $3) ON CONFLICT DO NOTHING",
                    str(doc_id),
                    kb_type,
                    tag[:64],
                )

    async def mark_index_ready(self, *, doc_id: uuid.UUID, kb_type: str) -> None:
        """V1-12: atomic version switch — new chunks go ready, old chunks go not-ready."""
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            async with conn.transaction():
                await conn.execute("SET LOCAL statement_timeout = '30s'")
                await conn.execute("SET LOCAL lock_timeout = '5s'")
                # Increment index_version for this doc.
                await conn.execute(
                    "UPDATE kb_documents SET index_version = index_version + 1, updated_at = now() WHERE id = $1::uuid",
                    str(doc_id),
                )
                new_version_row = await conn.fetchrow(
                    "SELECT index_version FROM kb_documents WHERE id = $1::uuid",
                    str(doc_id),
                )
                new_version = new_version_row["index_version"] if new_version_row else 1

                # Mark all chunks for this doc as ready.
                await conn.execute(
                    "UPDATE kb_chunks SET index_ready = true, updated_at = now() "
                    "WHERE doc_id = $1::uuid AND kb_type = $2",
                    str(doc_id), kb_type,
                )

                # QA pairs follow suit.
                await conn.execute(
                    "UPDATE kb_qa_pairs SET index_ready = true "
                    "WHERE doc_id = $1::uuid AND kb_type = $2 AND index_version = $3",
                    str(doc_id), kb_type, new_version,
                )
                await conn.execute(
                    "UPDATE kb_qa_pairs SET index_ready = false "
                    "WHERE doc_id = $1::uuid AND kb_type = $2 AND index_version < $3",
                    str(doc_id), kb_type, new_version,
                )

                # Mark doc as ready.
                await conn.execute(
                    "UPDATE kb_documents SET index_ready = true, updated_at = now() WHERE id = $1::uuid",
                    str(doc_id),
                )

                # Bump catalog version for cache invalidation.
                await conn.execute(
                    """
                    INSERT INTO kb_catalog_version (kb_type, version, latest_doc_updated_at, updated_at)
                    VALUES ($1, 1, now(), now())
                    ON CONFLICT (kb_type) DO UPDATE SET
                        version = kb_catalog_version.version + 1,
                        latest_doc_updated_at = now(),
                        updated_at = now()
                    """,
                    kb_type,
                )

        # Defer materialized view refresh to avoid blocking ingest pipeline.
        # V1-13: bulk mode — during batch import, skip frequent global refresh.
        asyncio.create_task(self._refresh_global_index_async())


    _bulk_mode: bool = False
    _last_global_refresh: float = 0.0

    async def _refresh_global_index_async(self) -> None:
        """V1-13: Debounced global index refresh with bulk-mode awareness."""
        import time as _time
        now = _time.monotonic()
        if DocumentRepository._bulk_mode:
            return
        # debounce: at most once per 5 minutes
        if now - DocumentRepository._last_global_refresh < 300:
            return
        DocumentRepository._last_global_refresh = now
        try:
            async with self._pool.acquire() as conn:
                # V1-28: protect writes from long-running transactions
                await conn.execute("SET LOCAL statement_timeout = '30s'")
                await conn.execute("SET LOCAL lock_timeout = '5s'")
                await conn.execute("REFRESH MATERIALIZED VIEW kb_global_index")
        except Exception:
            pass  # Best-effort; periodic refresh will retry.


    async def update_ugc_task_completed(
        self,
        *,
        task_id: uuid.UUID,
        doc_id: uuid.UUID,
        chunk_count: int,
    ) -> None:
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                """
                UPDATE ugc_ingest_tasks
                SET status = 'completed', doc_id = $2::uuid, chunk_count = $3, updated_at = now()
                WHERE id = $1::uuid
                """,
                str(task_id),
                str(doc_id),
                chunk_count,
            )

    async def is_ugc_task_completed(self, task_id: str) -> bool:
        async with self._pool.acquire() as conn:
            return bool(await conn.fetchval("SELECT status = 'completed' FROM ugc_ingest_tasks WHERE id=$1::uuid", uuid.UUID(task_id)))

    async def update_ugc_task_failed(self, *, task_id: uuid.UUID, error_message: str) -> None:
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                """
                UPDATE ugc_ingest_tasks
                SET status = 'failed', error_message = $2, updated_at = now()
                WHERE id = $1::uuid AND status <> 'completed'
                """,
                str(task_id),
                error_message[:2000],
            )

    async def mark_ugc_task_processing(self, *, task_id: uuid.UUID) -> None:
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                """
                UPDATE ugc_ingest_tasks
                SET status = 'processing', current_stage = 'accepted', updated_at = now()
                WHERE id = $1::uuid AND status <> 'completed'
                """,
                str(task_id),
            )

    async def set_task_stage(self, *, task_id: str | uuid.UUID, stage: str) -> None:
        """Record pipeline stage on the UGC task (P2-1 observability on single-subject path)."""
        tid = str(task_id or "").strip()
        st = (stage or "").strip()
        if not tid or not st:
            return
        async with self._pool.acquire() as conn:
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            if st == "indexed":
                await conn.execute(
                    """
                    UPDATE ugc_ingest_tasks
                    SET current_stage = $2, ready_at = COALESCE(ready_at, now()), updated_at = now()
                    WHERE id = $1::uuid
                    """,
                    tid,
                    st,
                )
            else:
                await conn.execute(
                    """
                    UPDATE ugc_ingest_tasks
                    SET current_stage = $2, updated_at = now()
                    WHERE id = $1::uuid
                    """,
                    tid,
                    st,
                )

    async def get_enrichment_cache_by_hash(
        self,
        *,
        kb_type: str,
        content_hash: str,
    ) -> dict[str, Any] | None:
        h = (content_hash or "").strip()
        if not h:
            return None
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            row = await conn.fetchrow(
                """
                SELECT extensions
                FROM kb_documents
                WHERE kb_type = $1 AND content_hash = $2
                ORDER BY updated_at DESC
                LIMIT 1
                """,
                kb_type,
                h,
            )
        if not row:
            return None
        ext_raw = row["extensions"]
        if isinstance(ext_raw, dict):
            ext = ext_raw
        elif isinstance(ext_raw, str) and ext_raw.strip():
            ext = json.loads(ext_raw)
        else:
            ext = {}
        anchors = ext.get("search_anchors")
        if not anchors and not ext.get("semantic_tags_4d"):
            return None
        return {
            "search_anchors": anchors,
            "semantic_tags_4d": ext.get("semantic_tags_4d"),
            "global_summary": ext.get("global_summary"),
        }

    async def list_segment_group_docs(
        self,
        *,
        kb_type: str,
        parent_title: str = "",
        doc_group: str = "",
    ) -> list[dict[str, Any]]:
        pt = (parent_title or "").strip()
        dg = (doc_group or "").strip()
        if not pt and not dg:
            return []
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            if dg:
                rows = await conn.fetch(
                    """
                    SELECT id, title, extensions
                    FROM kb_documents
                    WHERE kb_type = $1
                      AND trim(extensions->>'doc_group') = $2
                    """,
                    kb_type,
                    dg,
                )
            else:
                rows = await conn.fetch(
                    """
                    SELECT id, title, extensions
                    FROM kb_documents
                    WHERE kb_type = $1
                      AND trim(extensions->>'parent_title') = $2
                      AND (extensions ? 'segment_index' OR extensions ? 'doc_group')
                    """,
                    kb_type,
                    pt,
                )
        out: list[dict[str, Any]] = []
        for r in rows:
            ext_raw = r["extensions"]
            if isinstance(ext_raw, dict):
                ext = ext_raw
            elif isinstance(ext_raw, str) and ext_raw.strip():
                ext = json.loads(ext_raw)
            else:
                ext = {}
            out.append({"id": r["id"], "title": r["title"] or "", "extensions": ext})
        return out


    async def check_duplicate_by_content_hash(
        self,
        *,
        kb_type: str,
        content_hash: str,
        title: str,
    ) -> dict[str, Any] | None:
        """Check if a document with the same content_hash and similar title exists in the workspace."""
        h = (content_hash or "").strip()
        if not h:
            return None
        async with self._pool.acquire() as conn:
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '20s'")
            row = await conn.fetchrow(
                """
                SELECT id, title, status, chunk_count, index_ready
                FROM kb_documents
                WHERE kb_type = $1 AND content_hash = $2 AND status = 'active'
                LIMIT 1
                """,
                kb_type,
                h,
            )
        if not row:
            return None
        return {
            "id": str(row["id"]),
            "title": row["title"],
            "chunk_count": int(row["chunk_count"] or 0),
            "index_ready": bool(row["index_ready"]),
        }

    async def set_parent_title_on_docs(
        self,
        *,
        kb_type: str,
        doc_ids: list[uuid.UUID],
        parent_title: str,
    ) -> None:
        pt = (parent_title or "").strip()
        if not pt or not doc_ids:
            return
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                """
                UPDATE kb_documents
                SET extensions = jsonb_set(
                        COALESCE(extensions, '{}'::jsonb),
                        '{parent_title}',
                        to_jsonb($2::text),
                        true
                    ),
                    updated_at = now()
                WHERE id = ANY($1::uuid[]) AND kb_type = $3
                """,
                doc_ids,
                pt,
                kb_type,
            )

    async def get_workspace_taxonomy(self, kb_type: str) -> dict[str, Any] | None:
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            row = await conn.fetchrow(
                """
                SELECT domain, version, scenes, styles, stats
                FROM workspace_4d_taxonomy
                WHERE kb_type = $1
                """,
                kb_type,
            )
        if not row:
            return None
        stats_raw = row["stats"]
        if isinstance(stats_raw, str) and stats_raw.strip():
            stats = json.loads(stats_raw)
        elif isinstance(stats_raw, dict):
            stats = stats_raw
        else:
            stats = {}
        scenes = row["scenes"]
        styles = row["styles"]
        if isinstance(scenes, str):
            scenes = json.loads(scenes)
        if isinstance(styles, str):
            styles = json.loads(styles)
        return {
            "domain": row["domain"] or "",
            "version": int(row["version"] or 0),
            "scenes": scenes if isinstance(scenes, list) else [],
            "styles": styles if isinstance(styles, list) else [],
            "stats": stats,
        }

    async def list_docs_for_taxonomy_backfill(
        self,
        *,
        kb_type: str,
        blueprint_domain: str,
        blueprint_version: int,
        limit: int = 80,
    ) -> list[dict[str, Any]]:
        ver_s = str(blueprint_version)
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            rows = await conn.fetch(
                """
                SELECT id::text AS doc_id, extensions
                FROM kb_documents
                WHERE kb_type = $1 AND status = 'active'
                  AND (
                    NOT (extensions ? 'semantic_tags_4d')
                    OR COALESCE(extensions->'semantic_tags_4d'->>'domain', '') IS DISTINCT FROM $2
                    OR COALESCE(extensions->>'taxonomy_backfill_version', '') IS DISTINCT FROM $3
                  )
                ORDER BY updated_at DESC
                LIMIT $4
                """,
                kb_type,
                blueprint_domain,
                ver_s,
                limit,
            )
        out: list[dict[str, Any]] = []
        for r in rows:
            ext_raw = r["extensions"]
            if isinstance(ext_raw, dict):
                ext = ext_raw
            elif isinstance(ext_raw, str) and ext_raw.strip():
                ext = json.loads(ext_raw)
            else:
                ext = {}
            out.append({"doc_id": r["doc_id"], "extensions": ext})
        return out

    async def update_document_extensions(
        self,
        *,
        doc_id: str,
        kb_type: str,
        extensions: dict[str, Any],
    ) -> None:
        ext = json.dumps(extensions or {})
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                """
                UPDATE kb_documents
                SET extensions = $1::jsonb, updated_at = now()
                WHERE id = $2::uuid AND kb_type = $3
                """,
                ext,
                doc_id,
                kb_type,
            )

    async def refresh_chunks_semantic_from_doc(
        self,
        *,
        doc_id: str,
        kb_type: str,
        doc_extensions: dict[str, Any],
    ) -> None:
        from worker.indexer.chunk_semantic import chunk_metadata_payload, extensions_for_chunk
        from worker.indexer.search_text import build_search_text

        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            rows = await conn.fetch(
                """
                SELECT id::text, content, COALESCE(metadata, '{}'::jsonb) AS metadata
                FROM kb_chunks
                WHERE doc_id = $1::uuid AND kb_type = $2
                ORDER BY chunk_index
                """,
                doc_id,
                kb_type,
            )
            title = await conn.fetchval(
                "SELECT COALESCE(title, '') FROM kb_documents WHERE id = $1::uuid AND kb_type = $2",
                doc_id,
                kb_type,
            )
        title = title or ""
        for r in rows:
            content = r["content"] or ""
            merged_ext = extensions_for_chunk(doc_extensions, content)
            meta_raw = r["metadata"]
            if isinstance(meta_raw, str) and meta_raw.strip():
                meta = json.loads(meta_raw)
            elif isinstance(meta_raw, dict):
                meta = meta_raw
            else:
                meta = {}
            meta.update(chunk_metadata_payload(merged_ext))
            search_text = build_search_text(
                doc_title=title,
                chunk_content=content,
                tags=None,
                extensions=merged_ext,
            )
            async with self._pool.acquire() as conn:
                # V1-28: protect writes from long-running transactions
                await conn.execute("SET LOCAL statement_timeout = '30s'")
                await conn.execute("SET LOCAL lock_timeout = '5s'")
                await conn.execute(
                    """
                    UPDATE kb_chunks
                    SET metadata = $1::jsonb, search_text = $2, updated_at = now()
                    WHERE id = $3::uuid AND kb_type = $4
                    """,
                    json.dumps(meta),
                    search_text,
                    r["id"],
                    kb_type,
                )

    async def mark_index_not_ready(self, *, doc_id: uuid.UUID, kb_type: str) -> None:
        async with self._pool.acquire() as conn:
            # V1-28: protect writes from long-running transactions
            await conn.execute("SET LOCAL statement_timeout = '30s'")
            await conn.execute("SET LOCAL lock_timeout = '5s'")
            await conn.execute(
                "UPDATE kb_documents SET index_ready = false WHERE id = $1::uuid",
                str(doc_id),
            )
            await conn.execute(
                "UPDATE kb_chunks SET index_ready = false WHERE doc_id = $1::uuid AND kb_type = $2",
                str(doc_id),
                kb_type,
            )


def _vector_literal(values: list[float]) -> str:
    parts = ",".join(str(float(v)) for v in values)
    return f"[{parts}]"
