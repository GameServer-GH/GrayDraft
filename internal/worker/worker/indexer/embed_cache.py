"""Process-local embedding cache (V1-05).

Avoids redundant Gateway calls for identical chunk texts across import tasks.
Keyed by SHA256(text + model + dimension).
"""

from __future__ import annotations

import hashlib
import time
from threading import Lock

class EmbedCache:
    """Thread-safe LRU-ish embedding cache with TTL."""

    def __init__(self, max_size: int = 5000, ttl_seconds: float = 600.0):
        self._max_size = max_size
        self._ttl = ttl_seconds
        self._entries: dict[str, tuple[list[float], float]] = {}  # key -> (vec, expiry)
        self._lock = Lock()

    def get(self, text: str, model: str, dim: int) -> list[float] | None:
        key = _cache_key(text, model, dim)
        with self._lock:
            entry = self._entries.get(key)
            if entry is None:
                return None
            vec, expiry = entry
            if time.monotonic() > expiry:
                del self._entries[key]
                return None
            return vec

    def set(self, text: str, model: str, dim: int, vec: list[float]) -> None:
        key = _cache_key(text, model, dim)
        with self._lock:
            if len(self._entries) >= self._max_size:
                # evict 20% oldest entries
                sorted_entries = sorted(self._entries.items(), key=lambda kv: kv[1][1])
                evict_count = max(1, self._max_size // 5)
                for k, _ in sorted_entries[:evict_count]:
                    del self._entries[k]
            self._entries[key] = (vec, time.monotonic() + self._ttl)

    def stats(self) -> dict:
        with self._lock:
            return {"size": len(self._entries), "max": self._max_size}


def _cache_key(text: str, model: str, dim: int) -> str:
    raw = f"{text}|{model}|{dim}"
    return hashlib.sha256(raw.encode()).hexdigest()[:32]


# Singleton for worker process.
_global_cache: EmbedCache | None = None


def get_embed_cache(max_size: int = 5000, ttl_seconds: float = 600.0) -> EmbedCache:
    global _global_cache
    if _global_cache is None:
        _global_cache = EmbedCache(max_size=max_size, ttl_seconds=ttl_seconds)
    return _global_cache
