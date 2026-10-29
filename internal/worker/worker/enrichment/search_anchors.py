"""LLM-based search anchor extraction (generic cross-industry semantic enrichment)."""

from __future__ import annotations

import logging
import re
from typing import Any

from worker.config import Settings
from worker.enrichment.llm_chat import chat_complete
from worker.indexer.search_text import dedupe_terms
from worker.textutil.emoji import strip_emojis

logger = logging.getLogger(__name__)

_ANCHOR_SYSTEM = (
    "你是教育资料知识库助手，熟悉教程、论文、API 文档、参数说明和课件。"
    "请根据用户给出的文章标题与开头内容，列出 5-8 个用户可能用来搜索该主题的核心关键词、"
    "同义词、简称或上位词。只输出逗号分隔的关键词，不要解释、不要编号，不要使用 Emoji 或绘文字。"
)

_BOOK_RE = re.compile(r"[《「]([^》」\n]{2,40})[》」]")
_CJK_WORD_RE = re.compile(r"[\u4e00-\u9fff]{2,8}")


async def extract_search_anchors(
    title: str,
    content: str,
    settings: Settings,
    *,
    existing_extensions: dict[str, Any] | None = None,
) -> list[str]:
    """Return search_anchors; merges smart-import anchors and LLM extraction."""
    ext = existing_extensions or {}
    merged: list[str] = []

    def add(parts: list[str]) -> None:
        merged.extend(parts)

    add(_anchors_from_smart_import(ext, title))
    preview = _preview_text(content, 500)
    llm_anchors: list[str] = []
    if settings.anchor_extraction_enabled and settings.model_gateway_url:
        try:
            user = f"标题：{title.strip() or '（无）'}\n\n正文开头：\n{preview}"
            raw = await chat_complete(settings, system=_ANCHOR_SYSTEM, user=user, max_tokens=120)
            llm_anchors = _parse_anchor_csv(raw)
        except Exception as exc:
            logger.warning("search_anchors llm failed: %s", exc)
    if not llm_anchors:
        llm_anchors = _heuristic_anchors(title, preview)
    add(llm_anchors)
    cleaned = [strip_emojis(x) for x in merged]
    return dedupe_terms([x for x in cleaned if x])[:8]


def _anchors_from_smart_import(ext: dict[str, Any], title: str) -> list[str]:
    out: list[str] = []
    for key in ("aliases", "search_anchors"):
        v = ext.get(key)
        if isinstance(v, list):
            out.extend(str(x).strip() for x in v if str(x).strip())
        elif isinstance(v, str) and v.strip():
            out.extend(_parse_anchor_csv(v))
    if title.strip():
        out.append(title.strip())
    return out


def _parse_anchor_csv(raw: str) -> list[str]:
    raw = (raw or "").strip()
    if not raw:
        return []
    parts = re.split(r"[,，、;；\n]+", raw)
    return [p.strip() for p in parts if len(p.strip()) >= 2]


def _heuristic_anchors(title: str, preview: str) -> list[str]:
    text = f"{title}\n{preview}"
    out: list[str] = []
    if title.strip():
        out.append(title.strip())
    for m in _BOOK_RE.finditer(text):
        out.append(m.group(1).strip())
    for w in _CJK_WORD_RE.findall(text):
        if len(w) >= 2:
            out.append(w)
    return dedupe_terms(out)[:8]


def _preview_text(content: str, limit: int) -> str:
    text = (content or "").strip()
    if len(text) <= limit:
        return text
    return text[:limit]
