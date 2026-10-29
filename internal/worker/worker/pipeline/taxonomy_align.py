"""Align free-text scene/style labels to workspace blueprint via embedding similarity."""

from __future__ import annotations

import math
from typing import Any

from worker.config import Settings
from worker.indexer.embedder import embed_texts


def _cosine(a: list[float], b: list[float]) -> float:
    if not a or len(a) != len(b):
        return 0.0
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(x * x for x in b))
    if na <= 0 or nb <= 0:
        return 0.0
    return dot / (na * nb)


def _label_list(defs: list[dict[str, Any]]) -> list[str]:
    out: list[str] = []
    for d in defs or []:
        if not isinstance(d, dict):
            continue
        label = str(d.get("label") or "").strip()
        if label:
            out.append(label)
    return out


async def align_to_label(
    raw: str,
    labels: list[str],
    settings: Settings,
    *,
    min_sim: float = 0.85,
) -> str:
    raw = (raw or "").strip()
    if not raw or not labels:
        return raw
    if raw in labels:
        return raw
    for lb in labels:
        if lb and (lb in raw or raw in lb):
            return lb
    try:
        vecs = await embed_texts([raw] + labels, settings)
        if len(vecs) != len(labels) + 1:
            return raw
        qv = vecs[0]
        best = raw
        best_sim = min_sim
        for i, lb in enumerate(labels):
            sim = _cosine(qv, vecs[i + 1])
            if sim >= best_sim:
                best_sim = sim
                best = lb
        return best
    except Exception:
        return raw


async def realign_semantic_tags_4d(
    tags: dict[str, Any],
    blueprint: dict[str, Any],
    settings: Settings,
) -> dict[str, Any]:
    if not isinstance(tags, dict):
        tags = {}
    out = dict(tags)
    domain = str(blueprint.get("domain") or "").strip()
    if domain:
        out["domain"] = domain
    scene_labels = _label_list(blueprint.get("scenes") or [])
    style_labels = _label_list(blueprint.get("styles") or [])
    if str(out.get("scene") or "").strip():
        out["scene"] = await align_to_label(str(out["scene"]), scene_labels, settings)
    if str(out.get("style") or "").strip():
        out["style"] = await align_to_label(str(out["style"]), style_labels, settings)
    return out
