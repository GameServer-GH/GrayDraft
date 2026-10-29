"""QA pair generation with document/segment context anchors (rule-based)."""

from __future__ import annotations

import re
from typing import Any

from worker.schemas import QAPair

# Closing / filler phrases — do not use as QA entities or generate QA from such chunks.
_FLUFF_ENTITY_PHRASES = (
    "以上就是",
    "全文完",
    "欢迎转载",
    "转载请注明出处",
    "敬请期待",
    "评论区留言",
    "下期再见",
    "感谢阅读",
    "全部内容",
    "喜欢的话",
)

_FLUFF_CHUNK_RE = re.compile(
    "|".join(re.escape(p) for p in _FLUFF_ENTITY_PHRASES),
    re.IGNORECASE,
)

_BOOK_TITLE_RE = re.compile(r"[《「]([^》」\n]{2,80})[》」]")


def generate_qa_pairs(
    chunks: list[str],
    *,
    kb_type: str,
    max_pairs: int = 5,
    title: str = "",
    extensions: dict[str, Any] | None = None,
) -> list[QAPair]:
    """Generate QA pairs using chunk text plus document extensions (smart-import anchors)."""
    anchors = _parse_anchors(extensions or {}, title)
    pairs: list[QAPair] = []
    for idx, chunk in enumerate(chunks):
        if len(pairs) >= max_pairs:
            break
        chunk = chunk.strip()
        if len(chunk) < 40:
            continue
        if _is_fluff_chunk(chunk):
            continue

        entities = _extract_entities(chunk, anchors)
        if not entities:
            continue

        entity = entities[0]
        question = _question_for_kb(kb_type, entity, chunk, anchors)
        if _is_low_quality_question(question):
            continue

        answer = _clip_sentence(chunk, max_len=400)
        if not _answer_covers_entities(answer, entities[:3]):
            continue

        pairs.append(
            QAPair(
                question=question,
                answer=answer,
                chunk_index=idx,
                entities=entities[:5],
            )
        )
    return pairs


def _parse_anchors(extensions: dict[str, Any], title: str) -> dict[str, str]:
    """Normalize smart-import / UGC extensions into QA anchor fields."""
    out: dict[str, str] = {
        "subject": _str_field(extensions.get("subject")),
        "parent_title": _str_field(extensions.get("parent_title")),
        "context_enhancement": _str_field(extensions.get("context_enhancement")),
        "semantic_summary": _str_field(extensions.get("semantic_summary")),
        "doc_type": _str_field(extensions.get("doc_type")),
        "doc_title": (title or "").strip(),
    }
    if not out["parent_title"] and out["doc_title"]:
        out["parent_title"] = out["doc_title"]
    if not out["subject"] and out["parent_title"]:
        for sep in ("：", ":"):
            if sep in out["parent_title"]:
                out["subject"] = out["parent_title"].split(sep, 1)[0].strip()
                break
    if not out["context_enhancement"]:
        parts = [p for p in (out["subject"], out["parent_title"]) if p]
        out["context_enhancement"] = " ".join(parts)
    return out


def _str_field(value: Any) -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return value.strip()
    return str(value).strip()


def _is_fluff_chunk(chunk: str) -> bool:
    trimmed = chunk.strip()
    if len(trimmed) < 120 and _FLUFF_CHUNK_RE.search(trimmed):
        return True
    # Very short chunks that are mostly filler
    if len(trimmed) < 80:
        for phrase in _FLUFF_ENTITY_PHRASES:
            if phrase in trimmed:
                return True
    return False


def _is_low_quality_question(question: str) -> bool:
    q = question.strip()
    if len(q) < 6:
        return True
    for phrase in _FLUFF_ENTITY_PHRASES:
        if phrase in q:
            return True
    # e.g. "请介绍 以上就是"
    if q.startswith("请介绍") and len(q) < 20:
        return True
    return False


def _extract_entities(chunk: str, anchors: dict[str, str]) -> list[str]:
    found: list[str] = []

    def add(m: str) -> None:
        m = m.strip()
        if not m or m in found:
            return
        if any(p in m for p in _FLUFF_ENTITY_PHRASES):
            return
        if m in {"根据", "通过", "进行", "实现", "确保", "介绍", "以上"}:
            return
        found.append(m)

    # Anchor entities first (document-level context from smart-import)
    for key in ("subject", "parent_title", "semantic_summary"):
        val = anchors.get(key, "")
        if val and val in chunk:
            add(val)

    for m in _BOOK_TITLE_RE.findall(chunk):
        add(m)

    patterns = [
        r"GrayDraft|GRAYDRAFT|RAG|RRF|pgvector|NATS|index_ready",
        r"[\u4e00-\u9fff]{4,12}",
        r"[A-Z][a-zA-Z0-9_-]{2,}",
    ]
    for pat in patterns:
        for m in re.findall(pat, chunk):
            add(m)

    # If chunk has little explicit entity text, still allow anchor-based QA when chunk is substantive
    if not found and len(chunk) >= 80:
        if anchors.get("subject"):
            add(anchors["subject"])
        elif anchors.get("parent_title"):
            add(anchors["parent_title"][:40])

    return found[:8]


def _question_for_kb(kb_type: str, entity: str, chunk: str, anchors: dict[str, str]) -> str:
    topic = anchors.get("subject") or ""
    doc_title = anchors.get("parent_title") or anchors.get("doc_title") or ""
    ctx = anchors.get("context_enhancement") or ""

    if "如何" in chunk or "怎么" in chunk:
        if topic and topic not in entity:
            return f"{topic}：如何理解{entity}？"
        return f"如何理解{entity}？"

    if "章节" in chunk or "参数" in chunk or "部署" in chunk:
        subject = topic or doc_title or entity
        return f"{subject} 关键参数是什么？"

    if kb_type.startswith("gd_") or _looks_like_guide(chunk, anchors):
        subject = doc_title or ctx or topic or entity
        if "教程" in chunk or "教程" in subject:
            return f"{subject} 有哪些关键教程要点？"
        return f"{subject} 的主要内容是什么？"

    default = f"关于 {entity} 的说明是什么？"
    if topic and topic not in default:
        return f"{topic}：{default}"
    return default


def _looks_like_guide(chunk: str, anchors: dict[str, str]) -> bool:
    if anchors.get("doc_type") == "单篇说明":
        return True
    hints = ("教程", "指南", "参数", "章节", "公式", "接口", "论文")
    return sum(1 for h in hints if h in chunk) >= 2


def _clip_sentence(text: str, max_len: int = 400) -> str:
    text = text.strip()
    if len(text) <= max_len:
        return text
    cut = text[:max_len]
    last = max(cut.rfind("。"), cut.rfind("."), cut.rfind("\n"))
    if last > 80:
        return cut[: last + 1]
    return cut + "…"


def _answer_covers_entities(answer: str, entities: list[str]) -> bool:
    return any(e in answer for e in entities)
