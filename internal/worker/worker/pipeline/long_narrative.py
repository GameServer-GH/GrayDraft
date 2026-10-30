"""Domain-agnostic long-narrative normalize + auto-split for ingest.

Used for papers, long manuals, recipes, drug inserts — any material that
trips length / spam-run gates before chunking. Splits by structure, not by
vertical taxonomy.
"""

from __future__ import annotations

import hashlib
import re
from dataclasses import dataclass

_SPAM_RUN = re.compile(r"(.)\1{8,}", re.DOTALL)
_HEADING = re.compile(r"(?m)^(#{1,3}\s+\S.*)$")
_BLANK_SPLIT = re.compile(r"\n{2,}")

# Soft threshold: above this, prefer section splits even under max_length.
DEFAULT_SOFT_SPLIT_CHARS = 8000
DEFAULT_MAX_PART_CHARS = 12000


@dataclass(frozen=True)
class NarrativePart:
    title: str
    content: str
    segment_index: int
    segment_total: int
    doc_group: str
    parent_title: str


def collapse_spam_runs(text: str, *, keep: int = 3) -> str:
    """Fold runs of 9+ identical characters down to `keep` (fixes arXiv HTML spam)."""
    if not text:
        return text
    keep = max(1, keep)

    def _repl(m: re.Match[str]) -> str:
        return m.group(1) * keep

    return _SPAM_RUN.sub(_repl, text)


def make_doc_group(parent_title: str, content: str) -> str:
    seed = (parent_title or "").strip() or (content or "")[:240]
    digest = hashlib.sha256(seed.encode("utf-8")).hexdigest()[:16]
    return f"dg_{digest}"


def should_auto_split(
    text: str,
    *,
    max_length: int = 50000,
    soft_split: int = DEFAULT_SOFT_SPLIT_CHARS,
    already_segmented: bool = False,
) -> bool:
    if already_segmented:
        return False
    n = len(text or "")
    if n > max_length:
        return True
    if n >= soft_split and (_HEADING.search(text or "") or text.count("\n\n") >= 4):
        return True
    return False


def split_long_narrative(
    text: str,
    *,
    title: str,
    max_part_chars: int = DEFAULT_MAX_PART_CHARS,
    parent_title: str | None = None,
    doc_group: str | None = None,
) -> list[NarrativePart]:
    """Split long text into gate-friendly parts with shared doc_group."""
    body = (text or "").strip()
    if not body:
        return []
    parent = (parent_title or title or "untitled").strip() or "untitled"
    group = (doc_group or "").strip() or make_doc_group(parent, body)

    sections = _split_by_headings(body)
    if len(sections) <= 1:
        sections = _split_by_paragraphs(body, max_part_chars)

    packed = _pack_sections(sections, max_part_chars)
    if not packed:
        packed = [body]
    total = len(packed)
    out: list[NarrativePart] = []
    for i, part in enumerate(packed, start=1):
        part_title = _part_title(title, parent, i, total, part)
        out.append(
            NarrativePart(
                title=part_title,
                content=part.strip(),
                segment_index=i,
                segment_total=total,
                doc_group=group,
                parent_title=parent,
            )
        )
    return out


def prepare_for_ingest(
    content: str,
    *,
    title: str,
    max_length: int = 50000,
    extensions: dict | None = None,
) -> list[NarrativePart]:
    """Normalize spam runs, then optionally auto-split into doc_group segments."""
    ext = extensions if isinstance(extensions, dict) else {}
    already = ext.get("segment_index") is not None or bool(ext.get("doc_group"))
    text = collapse_spam_runs(content or "")
    parent = str(ext.get("parent_title") or title or "").strip() or (title or "untitled")
    existing_group = str(ext.get("doc_group") or "").strip() or None

    if not should_auto_split(
        text,
        max_length=max_length,
        already_segmented=already,
    ):
        group = existing_group or make_doc_group(parent, text)
        return [
            NarrativePart(
                title=title or parent,
                content=text,
                segment_index=int(ext["segment_index"]) if ext.get("segment_index") is not None else 1,
                segment_total=int(ext.get("segment_total") or 1),
                doc_group=group,
                parent_title=parent,
            )
        ]

    return split_long_narrative(
        text,
        title=title or parent,
        max_part_chars=min(DEFAULT_MAX_PART_CHARS, max(2000, max_length // 2)),
        parent_title=parent,
        doc_group=existing_group,
    )


def _split_by_headings(text: str) -> list[str]:
    matches = list(_HEADING.finditer(text))
    if not matches:
        return [text]
    parts: list[str] = []
    if matches[0].start() > 0:
        preamble = text[: matches[0].start()].strip()
        if preamble:
            parts.append(preamble)
    for i, m in enumerate(matches):
        end = matches[i + 1].start() if i + 1 < len(matches) else len(text)
        block = text[m.start() : end].strip()
        if block:
            parts.append(block)
    return parts or [text]


def _split_by_paragraphs(text: str, max_part_chars: int) -> list[str]:
    paras = [p.strip() for p in _BLANK_SPLIT.split(text) if p.strip()]
    if not paras:
        return [text]
    return _pack_sections(paras, max_part_chars)


def _pack_sections(sections: list[str], max_part_chars: int) -> list[str]:
    packed: list[str] = []
    buf = ""
    for sec in sections:
        if not sec:
            continue
        if len(sec) > max_part_chars:
            if buf:
                packed.append(buf)
                buf = ""
            packed.extend(_hard_slice(sec, max_part_chars))
            continue
        candidate = f"{buf}\n\n{sec}".strip() if buf else sec
        if len(candidate) <= max_part_chars:
            buf = candidate
        else:
            if buf:
                packed.append(buf)
            buf = sec
    if buf:
        packed.append(buf)
    return packed


def _hard_slice(text: str, max_part_chars: int) -> list[str]:
    out: list[str] = []
    i = 0
    n = len(text)
    while i < n:
        out.append(text[i : i + max_part_chars])
        i += max_part_chars
    return out


def _part_title(title: str, parent: str, index: int, total: int, part: str) -> str:
    base = (title or parent or "untitled").strip()
    if total <= 1:
        return base
    heading = _HEADING.match(part or "")
    if heading:
        h = heading.group(1).lstrip("#").strip()
        if h:
            return f"{base} · {h}"[:200]
    return f"{base} ({index}/{total})"
