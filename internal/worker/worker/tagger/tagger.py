"""Lightweight keyword tag extraction."""

from __future__ import annotations

import re
from collections import Counter


_STOP = {
    "的", "了", "是", "在", "和", "与", "或", "及", "等", "这", "那", "有", "为",
    "the", "a", "an", "and", "or", "to", "of", "in", "for", "on", "with",
}


def extract_tags(text: str, *, max_tags: int = 8) -> list[str]:
    tokens = re.findall(r"[\u4e00-\u9fff]{2,}|[a-zA-Z]{3,}", text.lower())
    tokens = [t for t in tokens if t not in _STOP]
    if not tokens:
        return []
    ctr = Counter(tokens)
    return [w for w, _ in ctr.most_common(max_tags)]
