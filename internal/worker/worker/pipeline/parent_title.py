"""Derive parent_title from segment document titles (mirrors core smartimport/parent_title.go)."""

from __future__ import annotations

import re

_SEGMENT_SUFFIX = re.compile(r"[（(]\s*\d+\s*[/／]\s*\d+\s*[）)]\s*$")
_TRAILING_NOISE = re.compile(r"[\s·\-—|/：:]+$")
_STEM_SEPARATORS = (" · ", "：", ":", " - ", " — ", "｜", "|", " / ")


def summarize_parent_title_from_child_titles(titles: list[str]) -> str:
    cleaned: list[str] = []
    seen: set[str] = set()
    for raw in titles:
        t = _normalize_child_doc_title(raw)
        if not t or t in seen:
            continue
        seen.add(t)
        cleaned.append(t)
    if not cleaned:
        return ""
    if len(cleaned) == 1:
        return cleaned[0]

    stem = _common_title_stem(cleaned)
    if stem:
        return stem
    prefix = _longest_common_prefix(cleaned)
    if len(prefix) >= 2:
        return _trim_parent_title_suffix(prefix)
    shortest = _shortest_prefix_of_all(cleaned)
    if shortest:
        return shortest
    return cleaned[0]


def _normalize_child_doc_title(title: str) -> str:
    title = (title or "").strip()
    if not title:
        return ""
    title = _SEGMENT_SUFFIX.sub("", title).strip()
    return title


def _common_title_stem(titles: list[str]) -> str:
    stems = [_title_stem(t) for t in titles]
    first = stems[0]
    if not first:
        return ""
    if all(s == first for s in stems[1:]):
        return first
    return ""


def _title_stem(title: str) -> str:
    title = _normalize_child_doc_title(title)
    if not title:
        return ""
    best = title
    for sep in _STEM_SEPARATORS:
        if sep in title:
            part = title.split(sep, 1)[0].strip()
            if part and (best == title or len(part) < len(best)):
                best = part
    return _trim_parent_title_suffix(best)


def _longest_common_prefix(titles: list[str]) -> str:
    if not titles:
        return ""
    prefix = titles[0]
    for t in titles[1:]:
        while t and not t.startswith(prefix):
            if not prefix:
                return ""
            prefix = prefix[:-1]
    return _trim_parent_title_suffix(prefix)


def _shortest_prefix_of_all(titles: list[str]) -> str:
    shortest = min(titles, key=len)
    for t in titles:
        if t == shortest or t.startswith(shortest):
            continue
        return ""
    return _trim_parent_title_suffix(shortest)


def _trim_parent_title_suffix(s: str) -> str:
    return _TRAILING_NOISE.sub("", (s or "").strip()).strip()
