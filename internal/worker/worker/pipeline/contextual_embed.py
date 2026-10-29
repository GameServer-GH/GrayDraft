"""Workspace background prefix for contextualized embeddings."""

from __future__ import annotations

from typing import Any


def profile_from_extensions(extensions: dict[str, Any] | None) -> dict[str, Any] | None:
    if not isinstance(extensions, dict):
        return None
    raw = extensions.get("workspace_context_profile")
    if not isinstance(raw, dict) or not raw:
        return None
    return raw


def build_embedding_prefix(kb_type: str, profile: dict[str, Any] | None) -> str:
    if not profile:
        return ""
    desc = str(profile.get("description") or "").strip()
    label = desc or str(kb_type or "").strip()
    entities = profile.get("entities") or []
    if not isinstance(entities, list):
        entities = []
    ents = " ".join(str(e).strip() for e in entities if str(e).strip())
    if not label and not ents:
        return ""
    if not ents:
        return f"【{label}】| "
    if not label:
        label = str(kb_type or "").strip()
    return f"【{label}】{ents} | "


def apply_contextual_prefix(text: str, prefix: str) -> str:
    text = (text or "").strip()
    if not prefix:
        return text
    if not text:
        return prefix.strip()
    return prefix + text
