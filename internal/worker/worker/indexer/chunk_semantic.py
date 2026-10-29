"""Per-chunk semantic_tags_4d derivation from document extensions + chunk body."""

from __future__ import annotations

import re
from typing import Any

_HEADING_RE = re.compile(r"(?m)^#{1,3}\s+(.+)$")


def derive_chunk_semantic_tags_4d(
    extensions: dict[str, Any] | None,
    chunk_content: str,
) -> dict[str, Any]:
    ext = extensions or {}
    raw = ext.get("semantic_tags_4d")
    if not isinstance(raw, dict) or not raw:
        return {}
    out: dict[str, Any] = {
        "domain": str(raw.get("domain") or "").strip(),
        "scene": str(raw.get("scene") or "").strip(),
        "style": str(raw.get("style") or "").strip(),
    }
    seen: set[str] = set()
    ents: list[str] = []

    def add(part: str) -> None:
        p = part.strip()
        if not p or p in seen or len(p) < 2:
            return
        seen.add(p)
        ents.append(p)

    for item in raw.get("entities") or []:
        add(str(item))
    for tag in ext.get("segment_tags") or []:
        add(str(tag))
    m = _HEADING_RE.search(chunk_content or "")
    if m:
        add(m.group(1))
    if len(ents) > 8:
        ents = ents[:8]
    if ents:
        out["entities"] = ents
    if not any([out.get("domain"), out.get("scene"), out.get("style"), ents]):
        return {}
    return out


def chunk_metadata_payload(extensions: dict[str, Any] | None) -> dict[str, Any]:
    """Persist per-chunk fields in kb_chunks.metadata (subset of merged extensions)."""
    ext = extensions or {}
    meta: dict[str, Any] = {}
    tags4d = ext.get("semantic_tags_4d")
    if isinstance(tags4d, dict) and tags4d:
        meta["semantic_tags_4d"] = tags4d
    for key in ("semantic_summary", "search_intent"):
        val = ext.get(key)
        if isinstance(val, str) and val.strip():
            meta[key] = val.strip()
    return meta


def extensions_for_chunk(extensions: dict[str, Any] | None, chunk_content: str) -> dict[str, Any]:
    base = dict(extensions or {})
    chunk_4d = derive_chunk_semantic_tags_4d(base, chunk_content)
    if chunk_4d:
        merged = dict(base)
        merged["semantic_tags_4d"] = chunk_4d
        return merged
    return base
