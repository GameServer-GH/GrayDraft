"""UGC quality gate — domain-agnostic content validation.

Entity recognition is delegated to the smart-import unified LLM pipeline.
This gate performs only domain-neutral checks:
- Basic length / structure sanity
- Spam / repetition / URL dominance detection
- Information density assessment
- LLM cleaning trigger pass-through
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field

_SPAM_PATTERNS = [
    re.compile(r"(.)\1{8,}"),
    re.compile(r"加微信|免费领取|点击链接", re.I),
]
_SENSITIVE: set[str] = set()  # 生产环境从配置/DB 加载


@dataclass
class GateResult:
    passed: bool
    reason: str | None
    score: float
    warnings: list[str] = field(default_factory=list)
    is_smart_import: bool = False
    needs_llm_cleaning: bool = False


def check_content(
    content: str,
    *,
    title: str | None = None,
    min_length: int = 20,
    max_length: int = 50000,
    tags: list[str] | None = None,
    extensions: dict | None = None,
) -> GateResult:
    """Run domain-agnostic quality checks on content before ingest.

    All domain-specific entity recognition is performed upstream by the
    smart-import unified LLM; this gate only validates structural quality.
    """
    # Collapse long identical-char runs before spam checks (arXiv HTML / OCR noise).
    # Ingest also normalizes via long_narrative; this keeps direct API checks aligned.
    from worker.pipeline.long_narrative import collapse_spam_runs

    text = collapse_spam_runs((content or "").strip())
    combined = f"{title or ''}\n{text}".strip()

    tags_list = tags or []
    is_smart_import = "smart-import" in tags_list

    needs_llm_cleaning = False
    if isinstance(extensions, dict):
        needs_llm_cleaning = bool(extensions.get("needs_llm_cleaning"))

    # ── Length checks ──────────────────────────────────────────────
    if len(text) < min_length:
        if is_smart_import and len(text) >= 40 and _count_entities(extensions) >= 1:
            return GateResult(
                True, "short_content_exempted",
                0.45, warnings=["content_shorter_than_minimum"],
                is_smart_import=True, needs_llm_cleaning=needs_llm_cleaning,
            )
        return GateResult(False, "content_too_short", 0.0)

    if len(text) > max_length:
        return GateResult(False, "content_too_long", 0.0)

    # ── Spam / sensitive ───────────────────────────────────────────
    for pat in _SPAM_PATTERNS:
        if pat.search(combined):
            return GateResult(False, "spam_pattern_detected", 0.1)

    for word in _SENSITIVE:
        if word in combined:
            return GateResult(False, "sensitive_content", 0.0)

    # ── Structural garbage ─────────────────────────────────────────
    if _repeated_char_ratio(text) > 0.40:
        return GateResult(False, "excessive_repetition", 0.05)

    # ── Smart-import domain-neutral checks ─────────────────────────
    warnings: list[str] = []
    entity_count = 0
    if is_smart_import:
        entity_count = _count_entities(extensions)

        if not re.search(r"[\u4e00-\u9fff]", text):
            return GateResult(
                False, "no_chinese_characters",
                0.0, is_smart_import=True, needs_llm_cleaning=needs_llm_cleaning,
            )

        url_ratio = len(re.findall(r"https?://[^\s]+", text)) / max(len(text), 1)
        if url_ratio > 0.3:
            return GateResult(
                False, "url_dominated_content",
                0.0, is_smart_import=True, needs_llm_cleaning=needs_llm_cleaning,
            )

    # ── Information density ────────────────────────────────────────
    unique_chars = len(set(text))
    score = min(1.0, 0.3 + unique_chars / max(len(text), 1) * 2)
    if score < 0.35:
        if is_smart_import and entity_count >= 2:
            warnings.append("low_information_density_with_entities")
            score = 0.35
        else:
            return GateResult(False, "low_information_density", score)

    return GateResult(
        True, None, score,
        warnings=warnings,
        is_smart_import=is_smart_import,
        needs_llm_cleaning=needs_llm_cleaning,
    )


# ── Entity extraction (delegated to upstream LLM) ──────────────────


def _count_entities(extensions: dict | None) -> int:
    """Count entities pre-extracted by smart-import unified LLM."""
    if not isinstance(extensions, dict):
        return 0
    unified = extensions.get("import_llm_unified")
    if not isinstance(unified, dict):
        return 0
    entities = unified.get("entities")
    if not isinstance(entities, list):
        return 0
    return sum(1 for e in entities if isinstance(e, str) and e.strip())


# ── Character-level heuristics ─────────────────────────────────────


def _repeated_char_ratio(text: str) -> float:
    """Ratio of characters that appear in runs of >= 4 identical chars."""
    if len(text) < 4:
        return 0.0
    total = len(text)
    repeated = 0
    i = 0
    while i < total:
        j = i
        while j < total and text[j] == text[i]:
            j += 1
        run_len = j - i
        if run_len >= 4:
            repeated += run_len
        i = j
    return repeated / total
