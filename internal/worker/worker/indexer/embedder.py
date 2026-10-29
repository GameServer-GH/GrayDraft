"""Embedding providers — gateway HTTP or deterministic dev fallback."""

from __future__ import annotations

import hashlib
import math

import httpx
import numpy as np

from worker.config import Settings
from worker.indexer.embed_cache import get_embed_cache

# Module-level shared HTTP client with connection pooling for reuse across calls.
_http_client: httpx.AsyncClient | None = None


def _get_client() -> httpx.AsyncClient:
    global _http_client
    if _http_client is None or _http_client.is_closed:
        _http_client = httpx.AsyncClient(
            timeout=30.0,
            limits=httpx.Limits(max_keepalive_connections=20, max_connections=50),
        )
    return _http_client


async def embed_texts(texts: list[str], settings: Settings) -> list[list[float]]:
    if not texts:
        return []

    model = settings.embedding_model or "default"
    dim = settings.embedding_dim or 1024

    # V1-05: check process-local cache first
    cache = get_embed_cache()
    cached: dict[int, list[float]] = {}
    uncached: list[tuple[int, str]] = []
    for i, t in enumerate(texts):
        vec = cache.get(t, model, dim)
        if vec is not None:
            cached[i] = vec
        else:
            uncached.append((i, t))

    if not uncached:
        return [cached[i] for i in range(len(texts))]

    uncached_texts = [t for _, t in uncached]
    if settings.model_gateway_url:
        try:
            vecs = await _embed_via_gateway(uncached_texts, settings)
            for (idx, t), vec in zip(uncached, vecs):
                cache.set(t, model, dim, vec)
                cached[idx] = vec
            return [cached[i] for i in range(len(texts))]
        except Exception:
            pass

    for idx, t in uncached:
        vec = _pseudo_embedding(t, dim)
        cached[idx] = vec
    return [cached[i] for i in range(len(texts))]


async def _embed_via_gateway(texts: list[str], settings: Settings) -> list[list[float]]:
    url = settings.model_gateway_url.rstrip("/") + "/v1/embeddings"
    client = _get_client()
    model = settings.embedding_model or "default"
    resp = await client.post(url, json={"texts": texts, "model": model})
    resp.raise_for_status()
    data = resp.json()
    vectors = data.get("vectors") or []
    return [[float(x) for x in v] for v in vectors]


def _pseudo_embedding(text: str, dim: int) -> list[float]:
    """Deterministic dev embedding for vector index smoke tests."""
    seed = hashlib.sha256(text.encode("utf-8")).digest()
    rng = np.random.default_rng(int.from_bytes(seed[:8], "big"))
    vec = rng.standard_normal(dim).astype(np.float32)
    norm = np.linalg.norm(vec)
    if norm > 0:
        vec = vec / norm
    return vec.tolist()
