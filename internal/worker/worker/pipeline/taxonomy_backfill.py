"""Shadow backfill: align document semantic_tags_4d to workspace blueprint."""

from __future__ import annotations

import json
import logging
from typing import Any

from worker.config import Settings
from worker.db.repository import DocumentRepository
from worker.pipeline.taxonomy_align import realign_semantic_tags_4d

logger = logging.getLogger(__name__)


async def run_shadow_backfill(
    repo: DocumentRepository,
    settings: Settings,
    *,
    kb_type: str,
    taxonomy_version: int,
    limit: int = 80,
) -> dict[str, int]:
    kb_type = (kb_type or "").strip()
    if not kb_type.startswith("gd_"):
        raise ValueError("kb_type must be workspace partition gd_*")

    blueprint = await repo.get_workspace_taxonomy(kb_type)
    if not blueprint or not str(blueprint.get("domain") or "").strip():
        return {"updated": 0, "skipped": 0, "message": "no blueprint"}

    domain = str(blueprint["domain"]).strip()
    docs = await repo.list_docs_for_taxonomy_backfill(
        kb_type=kb_type,
        blueprint_domain=domain,
        blueprint_version=taxonomy_version,
        limit=limit,
    )
    updated = 0
    skipped = 0
    for doc in docs:
        ext = doc.get("extensions") or {}
        if not isinstance(ext, dict):
            ext = {}
        raw_tags = ext.get("semantic_tags_4d")
        if not isinstance(raw_tags, dict):
            raw_tags = {}
        new_tags = await realign_semantic_tags_4d(raw_tags, blueprint, settings)
        if new_tags == raw_tags and str(ext.get("taxonomy_backfill_version") or "") == str(taxonomy_version):
            skipped += 1
            continue
        ext = dict(ext)
        ext["semantic_tags_4d"] = new_tags
        ext["taxonomy_backfill_version"] = taxonomy_version
        ext["taxonomy_shadow_backfill"] = True
        await repo.update_document_extensions(
            doc_id=doc["doc_id"],
            kb_type=kb_type,
            extensions=ext,
        )
        await repo.refresh_chunks_semantic_from_doc(
            doc_id=doc["doc_id"],
            kb_type=kb_type,
            doc_extensions=ext,
        )
        updated += 1
    logger.info(
        "shadow backfill kb_type=%s version=%s updated=%d skipped=%d",
        kb_type,
        taxonomy_version,
        updated,
        skipped,
    )
    return {"updated": updated, "skipped": skipped}
