"""Build augmented search_text for generic semantic identity injection (GrayDraft 3.5)."""

from __future__ import annotations

from typing import Any


def _as_str_list(value: Any) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        s = value.strip()
        return [s] if s else []
    if isinstance(value, (list, tuple)):
        out: list[str] = []
        for item in value:
            out.extend(_as_str_list(item))
        return out
    s = str(value).strip()
    return [s] if s else []


def collect_anchor_terms(
    *,
    title: str,
    tags: list[str] | None = None,
    extensions: dict[str, Any] | None = None,
) -> list[str]:
    ext = extensions or {}
    seen: set[str] = set()
    ordered: list[str] = []

    def add(part: str) -> None:
        p = part.strip()
        if not p or p in seen:
            return
        seen.add(p)
        ordered.append(p)

    for anchor in _as_str_list(ext.get("search_anchors")):
        add(anchor)
    for alias in _as_str_list(ext.get("aliases")):
        add(alias)
    if title:
        add(title)
    for tag in tags or []:
        add(tag)
    return ordered


def dedupe_terms(parts: list[str]) -> list[str]:
    seen: set[str] = set()
    out: list[str] = []
    for part in parts:
        p = part.strip()
        if not p or p in seen:
            continue
        seen.add(p)
        out.append(p)
    return out


def _format_contextual_head(extensions: dict[str, Any] | None) -> str:
    """Physical 4D prefix for vector/fulltext recall when SQL metadata filter misses."""
    ext = extensions or {}
    raw = ext.get("semantic_tags_4d")
    if not isinstance(raw, dict):
        return ""
    entities = raw.get("entities")
    ent_str = ""
    if isinstance(entities, list):
        ents = [str(e).strip() for e in entities if str(e).strip()]
        if ents:
            ent_str = "、".join(ents[:6])
    scene = str(raw.get("scene") or "").strip()
    if not ent_str and not scene:
        return ""
    parts: list[str] = []
    if ent_str:
        parts.append(f"主体:{ent_str}")
    if scene:
        parts.append(f"场景:{scene}")
    return "[" + " | ".join(parts) + "]"


def _format_tags_4d(extensions: dict[str, Any] | None) -> str:
    ext = extensions or {}
    raw = ext.get("semantic_tags_4d")
    if not isinstance(raw, dict):
        return ""
    parts: list[str] = []
    domain = str(raw.get("domain") or "").strip()
    scene = str(raw.get("scene") or "").strip()
    style = str(raw.get("style") or "").strip()
    if domain:
        parts.append(f"领域:{domain}")
    if scene:
        parts.append(f"场景:{scene}")
    if style:
        parts.append(f"形态:{style}")
    entities = raw.get("entities")
    if isinstance(entities, list):
        ents = [str(e).strip() for e in entities if str(e).strip()]
        if ents:
            parts.append("实体:" + "、".join(ents[:8]))
    return " | ".join(parts)


def _format_path(ext: dict[str, Any], doc_title: str) -> str:
    section = str(ext.get("section_path") or "").strip()
    parent = str(ext.get("parent_title") or "").strip()
    enhancement = str(ext.get("context_enhancement") or "").strip()
    idx = ext.get("segment_index")
    total = ext.get("segment_total")
    parts: list[str] = []
    if section:
        parts.append(section)
    elif parent and parent != doc_title:
        parts.append(parent)
    if doc_title and doc_title not in parts:
        parts.append(doc_title)
    if enhancement and enhancement not in parts:
        parts.append(enhancement)
    if isinstance(idx, int) and isinstance(total, int) and total > 1:
        parts.append(f"第{idx}/{total}段")
    return " / ".join(parts)


def build_search_text(
    *,
    doc_title: str,
    chunk_content: str,
    tags: list[str] | None = None,
    extensions: dict[str, Any] | None = None,
) -> str:
    title = (doc_title or "").strip()
    ext = extensions or {}
    path = _format_path(ext, title)
    anchors = collect_anchor_terms(title=title, tags=tags, extensions=extensions)
    anchor_str = ", ".join(anchors) if anchors else title
    body = (chunk_content or "").strip()
    tags4d = _format_tags_4d(ext)
    seg_summary = str(ext.get("semantic_summary") or "").strip()
    doc_summary = str(ext.get("global_summary") or "").strip()
    summary = seg_summary or doc_summary
    search_intent = str(ext.get("search_intent") or "").strip()
    parts: list[str] = []
    head = _format_contextual_head(ext)
    if head:
        parts.append(head)
    if path:
        parts.append(f"[路径：{path}]")
    parts.append(f"[主题：{title}]")
    if tags4d:
        parts.append(f"[语义标签：{tags4d}]")
    if summary:
        label = "片段摘要" if seg_summary else "摘要"
        parts.append(f"[{label}：{summary}]")
    if search_intent:
        parts.append(f"[检索意图：{search_intent}]")
    parts.append(f"[关联词：{anchor_str}]")
    parts.append(f"[正文：{body}]")
    return " ".join(parts)
