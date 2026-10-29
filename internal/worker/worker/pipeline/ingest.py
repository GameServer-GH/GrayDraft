"""Document ingest pipeline: gate → parse → chunk → store → index."""

from __future__ import annotations

import asyncio
import hashlib
import logging
import time
import uuid

logger = logging.getLogger(__name__)

try:
    from api.metrics import INGEST_DURATION, INGEST_TOTAL
except ImportError:  # pragma: no cover
    INGEST_DURATION = None
    INGEST_TOTAL = None

from worker.chunkers.semantic_chunker import chunk_text
from worker.config import Settings
from worker.db.repository import DocumentRepository
from worker.indexer.embedder import embed_texts
from worker.indexer.chunk_semantic import chunk_metadata_payload, extensions_for_chunk
from worker.indexer.search_text import build_search_text
from worker.parsers.text_parser import parse_content
from worker.qa_generator.generator import generate_qa_pairs
from worker.quality_gate.gate import check_content
from worker.schemas import IngestRequest, IngestResponse, UGCMessage
from worker.tagger.tagger import extract_tags
from worker.pipeline.contextual_embed import (
    apply_contextual_prefix,
    build_embedding_prefix,
    profile_from_extensions,
)
from worker.pipeline.ingest_enrichment import (
    import_metadata_rich,
    load_enrichment_cache,
    resolve_search_anchors,
    tags_from_extensions,
)
from worker.pipeline.parent_title import summarize_parent_title_from_child_titles
from worker.textutil.emoji import strip_emojis


class IngestPipeline:
    def __init__(self, repo: DocumentRepository, settings: Settings) -> None:
        self._repo = repo
        self._settings = settings

    async def ingest(self, req: IngestRequest) -> IngestResponse:
        start = time.perf_counter()
        try:
            return await self._ingest(req)
        finally:
            if INGEST_DURATION is not None:
                INGEST_DURATION.observe(time.perf_counter() - start)

    async def _ingest(self, req: IngestRequest) -> IngestResponse:
        tags = getattr(req, "tags", None) or []
        gate = check_content(
            req.content,
            title=req.title,
            min_length=self._settings.quality_gate_min_length,
            max_length=self._settings.quality_gate_max_length,
            tags=tags,
            extensions=getattr(req, "extensions", None),
        )
        if not gate.passed:
            raise ValueError(f"quality_gate_rejected: {gate.reason}")
        if gate.needs_llm_cleaning:
            logger.info("smart_import_needs_llm_cleaning", extra={
                "doc_title": req.title,
                "reason": (req.extensions or {}).get("cleaning_reason", "low_density"),
                "warnings": gate.warnings,
            })

        text, title = parse_content(req.content, req.mime_type, req.title)
        text = strip_emojis(text)
        title = strip_emojis(title or req.title)
        title = title or req.title
        merged_ext_early = req.extensions if isinstance(req.extensions, dict) else {}
        hint = str(merged_ext_early.get("import_title_hint") or "").strip()
        if hint:
            title = strip_emojis(hint)

        chunk_strategy = ""
        if isinstance(merged_ext := (req.extensions or {}), dict):
            chunk_strategy = str(merged_ext.get("ingest_chunk_strategy") or "").strip()
        if chunk_strategy == "fact_table" or merged_ext.get("multimodal_consolidated"):
            from worker.chunkers.semantic_chunker import chunk_text_multimodal_consolidated

            chunks = chunk_text_multimodal_consolidated(
                text,
                chunk_size=max(self._settings.chunk_size, 1200),
                chunk_overlap=self._settings.chunk_overlap,
            )
        elif chunk_strategy == "section":
            from worker.chunkers.semantic_chunker import chunk_text_by_sections

            chunks = chunk_text_by_sections(
                text,
                chunk_size=max(self._settings.chunk_size, 1200),
                chunk_overlap=self._settings.chunk_overlap,
            )
        else:
            chunks = chunk_text(
                text,
                chunk_size=self._settings.chunk_size,
                chunk_overlap=self._settings.chunk_overlap,
            )
        if not chunks:
            raise ValueError("no_chunks_produced")

        doc_id = uuid.UUID(req.doc_id) if req.doc_id else uuid.uuid4()
        content_hash = hashlib.sha256(text.encode("utf-8")).hexdigest()

        # Dedup: check if a document with the same content_hash already exists in this workspace
        cached = await load_enrichment_cache(self._repo, kb_type=req.kb_type, content_hash=content_hash)
        if cached:
            # Check if an existing document with the same content_hash also has the same title
            existing = await self._repo.check_duplicate_by_content_hash(
                kb_type=req.kb_type,
                content_hash=content_hash,
                title=title,
            )
            if existing:
                logger.info(
                    "duplicate_content_hash_skipped kb_type=%s title=%s existing_doc_id=%s",
                    req.kb_type, title, existing["id"],
                )
                return IngestResponse(
                    doc_id=str(existing["id"]),
                    kb_type=req.kb_type,
                    chunk_count=existing.get("chunk_count", 0),
                    qa_count=0,
                    tags=[],
                    index_ready=existing.get("index_ready", True),
                )

        merged_ext = dict(req.extensions or {})
        merged_ext = await self._apply_segment_parent_title(req.kb_type, doc_id, title, merged_ext)

        enrich_cache = await load_enrichment_cache(self._repo, kb_type=req.kb_type, content_hash=content_hash)
        rich_meta = import_metadata_rich(merged_ext)

        if rich_meta:
            tags = list(dict.fromkeys((req.tags or []) + tags_from_extensions(merged_ext)))
        else:
            tags = list(dict.fromkeys((req.tags or []) + extract_tags(text)))

        chunk_exts = [extensions_for_chunk(merged_ext, chunk) for chunk in chunks]
        search_texts = [
            build_search_text(
                doc_title=title,
                chunk_content=chunk,
                tags=tags,
                extensions=ext,
            )
            for chunk, ext in zip(chunks, chunk_exts, strict=False)
        ]
        chunk_metadatas = [{"chunk_position": f"seg_{i}", **chunk_metadata_payload(ext)} for i, ext in enumerate(chunk_exts)]

        ctx_profile = profile_from_extensions(merged_ext)
        embed_prefix = build_embedding_prefix(req.kb_type, ctx_profile)
        embed_inputs = [apply_contextual_prefix(t, embed_prefix) for t in search_texts]

        anchors: list[str] = []
        embeddings: list[list[float] | None] = [None] * len(chunks)
        structured_embeddings: list[list[float] | None] = [None] * len(chunks)

        if req.auto_index:
            anchors_task = asyncio.create_task(
                resolve_search_anchors(
                    title,
                    text,
                    self._settings,
                    merged_ext,
                    cached=enrich_cache,
                )
            )
            embed_task = asyncio.create_task(embed_texts(embed_inputs, self._settings))
            anchors, vectors = await asyncio.gather(anchors_task, embed_task)
            embeddings = vectors
            mm = merged_ext.get("multimodal") if isinstance(merged_ext.get("multimodal"), dict) else {}
            structured_text = ""
            if isinstance(mm, dict):
                structured_text = str(mm.get("structured_kv_text") or "").strip()
            if structured_text:
                st_input = apply_contextual_prefix(structured_text, embed_prefix)
                st_vectors = await embed_texts([st_input], self._settings)
                if st_vectors and st_vectors[0]:
                    structured_embeddings = [st_vectors[0]] * len(chunks)
        else:
            anchors = await resolve_search_anchors(
                title,
                text,
                self._settings,
                merged_ext,
                cached=enrich_cache,
            )

        if anchors:
            merged_ext["search_anchors"] = anchors

        await self._repo.upsert_document(
            doc_id=doc_id,
            kb_type=req.kb_type,
            title=title,
            source_url=req.source_url,
            content_hash=content_hash,
            index_ready=False,
            quality_score=gate.score,
            extensions=merged_ext,
        )
        await self._repo.insert_tags(doc_id=doc_id, kb_type=req.kb_type, tags=tags)

        # Start QA generation in background (CPU-bound regex) overlapping with chunk insert I/O.
        qa_task = None
        skip_rule_qa = rich_meta or bool(enrich_cache)
        if req.generate_qa and not skip_rule_qa:
            qa_task = asyncio.create_task(
                asyncio.to_thread(
                    generate_qa_pairs,
                    chunks,
                    kb_type=req.kb_type,
                    max_pairs=self._settings.qa_max_pairs_per_doc,
                    title=title,
                    extensions=merged_ext,
                )
            )

        chunk_ids = await self._repo.insert_chunks(
            doc_id=doc_id,
            kb_type=req.kb_type,
            chunks=chunks,
            embeddings=embeddings,
            structured_embeddings=structured_embeddings,
            index_ready=False,
            quality_score=gate.score,
            doc_title=title,
            tags=tags,
            extensions=merged_ext,
            search_texts=search_texts,
            chunk_metadatas=chunk_metadatas,
        )

        qa_count = 0
        qa_count = 0
        if qa_task:
            # V1-01: QA generation is fire-and-forget; does not block index_ready.
            async def _run_qa():
                try:
                    pairs = await qa_task
                    await self._repo.insert_qa_pairs(
                        doc_id=doc_id,
                        kb_type=req.kb_type,
                        pairs=[p.model_dump() for p in pairs],
                        chunk_ids=chunk_ids,
                        index_ready=False,
                    )
                except Exception:
                    logger.exception("qa generation failed for doc=%s", doc_id)
            asyncio.create_task(_run_qa())

        index_ready = False
        if req.auto_index:
            await self._repo.mark_index_ready(doc_id=doc_id, kb_type=req.kb_type)
            index_ready = True

        if INGEST_TOTAL is not None:
            INGEST_TOTAL.labels(status="ok").inc()
        return IngestResponse(
            doc_id=str(doc_id),
            kb_type=req.kb_type,
            chunk_count=len(chunks),
            qa_count=qa_count,
            tags=tags,
            index_ready=index_ready,
        )

    async def _apply_segment_parent_title(
        self,
        kb_type: str,
        doc_id: uuid.UUID,
        title: str,
        extensions: dict,
    ) -> dict:
        ext = dict(extensions or {})
        if ext.get("segment_index") is None:
            return ext
        old_pt = str(ext.get("parent_title") or "").strip()
        if not old_pt:
            return ext

        siblings = await self._repo.list_segment_group_docs(kb_type=kb_type, parent_title=old_pt)
        titles: list[str] = []
        ids: list[uuid.UUID] = []
        for row in siblings:
            did = row["id"]
            if isinstance(did, str):
                did = uuid.UUID(did)
            ids.append(did)
            t = str(row["title"] or "").strip()
            if did == doc_id:
                t = title.strip()
            if t:
                titles.append(t)
        if doc_id not in ids:
            titles.append(title.strip())
            ids.append(doc_id)

        new_pt = summarize_parent_title_from_child_titles(titles)
        if not new_pt:
            return ext
        ext["parent_title"] = new_pt
        if new_pt != old_pt:
            await self._repo.set_parent_title_on_docs(kb_type=kb_type, doc_ids=ids, parent_title=new_pt)
        return ext

    async def ingest_ugc(self, msg: UGCMessage) -> IngestResponse:
        logger.info(
            "ugc ingest start kb_type=%s task_id=%s trace_id=%s title=%s",
            msg.kb_type,
            msg.task_id or "-",
            msg.trace_id or "-",
            (msg.title or "")[:80],
        )
        req = IngestRequest(
            doc_id=msg.doc_id,
            kb_type=msg.kb_type,
            title=msg.title,
            content=msg.content,
            source_url=msg.source_url,
            mime_type=msg.mime_type,
            generate_qa=msg.generate_qa,
            tags=msg.tags,
            extensions=msg.extensions,
            auto_index=True,
        )
        return await self.ingest(req)
