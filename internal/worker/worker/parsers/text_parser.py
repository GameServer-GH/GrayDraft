"""Text/HTML/Markdown document parsing."""

from __future__ import annotations

import re
from html import unescape

from bs4 import BeautifulSoup
from markdown import markdown


def parse_content(content: str, mime_type: str = "text/plain", title: str | None = None) -> tuple[str, str | None]:
    """Return normalized plain text and optional title."""
    mime = (mime_type or "text/plain").lower()
    text = content or ""

    if mime in ("text/html", "application/xhtml+xml"):
        text = _html_to_text(text)
    elif mime in ("text/markdown", "text/x-markdown"):
        text = _markdown_to_text(text)
    else:
        text = _normalize_whitespace(text)

    inferred_title = _sanitize_title(title) if title else None
    if not inferred_title:
        inferred_title = _extract_title(text)
    inferred_title = _sanitize_title(inferred_title)

    return text, inferred_title


def _html_to_text(html: str) -> str:
    soup = BeautifulSoup(html, "html.parser")
    for tag in soup(["script", "style", "noscript"]):
        tag.decompose()
    text = soup.get_text(separator="\n")
    return _normalize_whitespace(unescape(text))


def _markdown_to_text(md: str) -> str:
    html = markdown(md)
    return _html_to_text(html)


def _normalize_whitespace(text: str) -> str:
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    text = re.sub(r"[ \t]+", " ", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def _sanitize_title(title: str | None) -> str | None:
    if not title:
        return None
    t = title.strip()
    if not t or t.lower() == "json" or t.startswith("```") or t.startswith("{"):
        return None
    if len(t) < 2:
        return None
    return t[:200]


def _extract_title(text: str) -> str | None:
    for line in text.split("\n"):
        line = line.strip()
        if not line:
            continue
        if line.startswith("# "):
            return line[2:].strip()[:120]
        if line.lower() == "json" or line.startswith("```") or line.startswith("{"):
            continue
        if line.startswith("## 附件"):
            continue
        if len(line) >= 4:
            return line[:120]
    return None
