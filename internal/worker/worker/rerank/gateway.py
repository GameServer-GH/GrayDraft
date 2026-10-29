"""Rerank via Model Gateway. The gateway calls a dedicated rerank model."""

from __future__ import annotations

import httpx

from worker.config import Settings
from worker.schemas import Candidate, ScoredCandidate


async def rerank_via_gateway(
    query: str,
    candidates: list[Candidate],
    settings: Settings,
    top_n: int,
) -> list[ScoredCandidate]:
    if not settings.model_gateway_url:
        raise RuntimeError("model_gateway_url not configured")

    url = settings.model_gateway_url.rstrip("/") + "/v1/rerank"
    payload = {
        "query": query,
        "candidates": [{"id": c.id, "text": c.text, "content": c.text} for c in candidates],
        "model": settings.rerank_model or "default",
        "top_n": top_n,
    }
    async with httpx.AsyncClient(timeout=60.0) as client:
        resp = await client.post(url, json=payload)
        resp.raise_for_status()
        data = resp.json()

    results: list[ScoredCandidate] = []
    for item in data.get("results") or []:
        text = item.get("text") or item.get("content") or ""
        results.append(
            ScoredCandidate(
                id=item.get("id", ""),
                score=float(item.get("score", 0)),
                text=text,
            )
        )
    return results
