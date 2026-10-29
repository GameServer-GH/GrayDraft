"""Semantic-ish chunking by size with paragraph-aware boundaries."""

from __future__ import annotations

import re


def chunk_text(
    text: str,
    *,
    chunk_size: int = 512,
    chunk_overlap: int = 64,
) -> list[str]:
    text = (text or "").strip()
    if not text:
        return []
    if len(text) <= chunk_size:
        return [text]

    paragraphs = [p.strip() for p in re.split(r"\n\s*\n", text) if p.strip()]
    if not paragraphs:
        paragraphs = [text]

    chunks: list[str] = []
    buf: list[str] = []
    buf_len = 0

    def flush() -> None:
        nonlocal buf, buf_len
        if not buf:
            return
        joined = "\n\n".join(buf).strip()
        if joined:
            chunks.append(joined)
        buf = []
        buf_len = 0

    for para in paragraphs:
        if len(para) > chunk_size:
            flush()
            chunks.extend(_split_long(para, chunk_size, chunk_overlap))
            continue

        extra = len(para) + (2 if buf else 0)
        if buf_len + extra > chunk_size:
            flush()
        buf.append(para)
        buf_len += extra

    flush()

    if chunk_overlap > 0 and len(chunks) > 1:
        chunks = _apply_overlap(chunks, chunk_overlap)

    return chunks


def _split_long(text: str, size: int, overlap: int) -> list[str]:
    out: list[str] = []
    start = 0
    while start < len(text):
        end = min(start + size, len(text))
        piece = text[start:end].strip()
        if piece:
            out.append(piece)
        if end >= len(text):
            break
        start = max(end - overlap, start + 1)
    return out


def chunk_text_multimodal_consolidated(
    text: str,
    *,
    chunk_size: int = 1200,
    chunk_overlap: int = 64,
) -> list[str]:
    """4.0+: isolate the fact summary table as the primary retrieval chunk; drop per-image detail noise."""
    text = (text or "").strip()
    if not text:
        return []

    # Drop legacy per-image detail sections if still present in stored markdown.
    text = re.sub(r"(?ms)^##\s*分图识别详情\s*\n.*", "", text).strip()

    doc_title = ""
    m_title = re.search(r"(?m)^#\s+(.+)$", text)
    if m_title:
        doc_title = m_title.group(1).strip()

    m_table = re.search(r"(?ms)^##\s*要点汇总\s*\n+(.*?)(?=^##\s|\Z)", text)
    chunks: list[str] = []
    if m_table:
        table_body = m_table.group(1).strip()
        if table_body:
            head = f"# {doc_title}\n\n" if doc_title else ""
            chunks.append(f"{head}{table_body}".strip())
    if chunks:
        return chunks

    return chunk_text_by_sections(text, chunk_size=chunk_size, chunk_overlap=chunk_overlap)


def chunk_text_by_sections(
    text: str,
    *,
    chunk_size: int = 1200,
    chunk_overlap: int = 64,
) -> list[str]:
    """Chunk by markdown headings; keeps tables and spawn guides intact when possible."""
    text = (text or "").strip()
    if not text:
        return []
    parts = re.split(r"(?m)(?=^#{1,3}\s)", text)
    parts = [p.strip() for p in parts if p.strip()]
    if len(parts) <= 1:
        return chunk_text(text, chunk_size=chunk_size, chunk_overlap=chunk_overlap)
    out: list[str] = []
    for part in parts:
        if len(part) <= chunk_size:
            out.append(part)
        else:
            out.extend(chunk_text(part, chunk_size=chunk_size, chunk_overlap=chunk_overlap))
    return out or [text]


def _apply_overlap(chunks: list[str], overlap: int) -> list[str]:
    merged: list[str] = []
    prev_tail = ""
    for i, ch in enumerate(chunks):
        if i > 0 and prev_tail:
            merged.append((prev_tail + "\n" + ch).strip())
        else:
            merged.append(ch)
        prev_tail = ch[-overlap:] if len(ch) > overlap else ch
    return merged
