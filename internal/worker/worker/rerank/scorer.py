"""Lexical reranker — fast L1, refined L2 (no GPU required in dev)."""

from __future__ import annotations

import math
import re
from collections import Counter

from worker.schemas import Candidate, ScoredCandidate


def rerank(
    query: str,
    candidates: list[Candidate],
    *,
    top_n: int,
    level: str = "l1",
) -> list[ScoredCandidate]:
    if not candidates:
        return []

    scored: list[ScoredCandidate] = []
    for c in candidates:
        score = _score_pair(query, c.text, level=level)
        scored.append(ScoredCandidate(id=c.id, score=score, text=c.text))

    scored.sort(key=lambda x: x.score, reverse=True)
    return scored[:top_n]


def _score_pair(query: str, text: str, *, level: str) -> float:
    q = query.strip().lower()
    t = text.lower()
    if q and q in t:
        phrase_bonus = 0.85
    else:
        phrase_bonus = 0.0

    q_tokens = _tokenize(query)
    t_tokens = _tokenize(text)
    if not q_tokens or not t_tokens:
        return phrase_bonus

    q_ctr = Counter(q_tokens)
    t_ctr = Counter(t_tokens)
    overlap = sum(min(q_ctr[tok], t_ctr[tok]) for tok in q_ctr)
    precision = overlap / max(len(t_tokens), 1)
    recall = overlap / max(len(q_tokens), 1)
    f1 = 2 * precision * recall / max(precision + recall, 1e-9)

    bigram_bonus = 0.0
    if level == "l2":
        q_bi = set(_bigrams(q_tokens))
        t_bi = set(_bigrams(t_tokens))
        if q_bi:
            bigram_bonus = len(q_bi & t_bi) / len(q_bi) * 0.2

    length_penalty = 1.0 / (1.0 + math.log1p(max(len(text) - 500, 0) / 500))

    title_bonus = 0.0
    if "主题:" in text or "主题：" in text:
        head = text.split("|", 1)[0].lower()
        for tok in q_tokens:
            if tok in head:
                title_bonus = max(title_bonus, 0.35)
                break
    if "关联词:" in text or "关联词：" in text:
        parts = text.split("|")
        anchor_part = parts[1].lower() if len(parts) > 1 else ""
        for tok in q_tokens:
            if tok in anchor_part:
                title_bonus = max(title_bonus, 0.4)
                break

    return min(1.0, phrase_bonus + f1 * 0.75 + bigram_bonus + precision * 0.1 + title_bonus) * length_penalty


def _tokenize(text: str) -> list[str]:
    text = text.lower()
    # 中英文混合：按字母数字块与 CJK 字符切分
    parts = re.findall(r"[\w]+|[\u4e00-\u9fff]", text)
    return [p for p in parts if len(p) > 1 or ("\u4e00" <= p <= "\u9fff")]


def _bigrams(tokens: list[str]) -> list[str]:
    return [f"{tokens[i]}_{tokens[i+1]}" for i in range(len(tokens) - 1)]
