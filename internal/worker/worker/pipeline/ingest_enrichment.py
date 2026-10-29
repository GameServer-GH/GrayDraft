"""Import enrichment helpers: 4D tags, content-hash cache, parallel I/O."""

from __future__ import annotations

from typing import Any

from worker.config import Settings
from worker.db.repository import DocumentRepository
from worker.enrichment.search_anchors import extract_search_anchors


def tags_from_extensions(extensions: dict[str, Any] | None) -> list[str]:
    ext = extensions or {}
    out: list[str] = []
    seen: set[str] = set()

    def add(s: str) -> None:
        s = s.strip()
        if not s or s in seen:
            return
        seen.add(s)
        out.append(s)

    raw = ext.get("semantic_tags_4d")
    if isinstance(raw, dict):
        for key in ("domain", "scene", "style"):
            add(str(raw.get(key) or ""))
        for item in raw.get("entities") or []:
            add(str(item))
    add(str(ext.get("parent_title") or ""))
    add(str(ext.get("subject") or ""))
    return out[:12]


def mapping_delta_present(extensions: dict[str, Any] | None) -> bool:
    """Core Smart Import writes mapping_delta; taxonomy evolve runs there (Loop A)."""
    ext = extensions or {}
    raw = ext.get("mapping_delta")
    if not isinstance(raw, dict):
        return False
    phrases = raw.get("key_phrases")
    return isinstance(phrases, list) and len(phrases) > 0


def import_metadata_rich(extensions: dict[str, Any] | None) -> bool:
    ext = extensions or {}
    if ext.get("import_enrichment_complete"):
        return True
    raw = ext.get("semantic_tags_4d")
    if isinstance(raw, dict) and (raw.get("domain") or raw.get("entities")):
        return bool(str(ext.get("global_summary") or "").strip())
    return False


async def load_enrichment_cache(
    repo: DocumentRepository,
    *,
    kb_type: str,
    content_hash: str,
) -> dict[str, Any] | None:
    return await repo.get_enrichment_cache_by_hash(kb_type=kb_type, content_hash=content_hash)


async def resolve_search_anchors(
    title: str,
    text: str,
    settings: Settings,
    extensions: dict[str, Any],
    *,
    cached: dict[str, Any] | None,
) -> list[str]:
    if cached and cached.get("search_anchors"):
        raw = cached["search_anchors"]
        if isinstance(raw, list):
            return [str(x).strip() for x in raw if str(x).strip()]
    return await extract_search_anchors(title, text, settings, existing_extensions=extensions)
