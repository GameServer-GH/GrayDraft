"""Embedding micro-batch buffer (V1-02 / V1-03).

Accumulates chunks and flushes them as a single batch request to the Gateway
when max_items, max_tokens, or max_wait is reached.
"""

from __future__ import annotations

import asyncio
import hashlib
import logging
import time
from dataclasses import dataclass
from typing import Callable

logger = logging.getLogger(__name__)


@dataclass
class BatcherConfig:
    max_items: int = 64
    max_wait_ms: int = 100
    max_tokens: int = 6000
    flush_at_token_ratio: float = 0.8


class EmbedBatcher:
    """Non-blocking micro-batch buffer for embedding calls."""

    def __init__(
        self,
        embed_fn: Callable,
        cfg: BatcherConfig | None = None,
    ):
        self._embed_fn = embed_fn
        self._cfg = cfg or BatcherConfig()
        self._buffer: list[tuple[str, "asyncio.Future[tuple[list[float], str]]"]] = []
        self._token_count = 0
        self._last_flush = time.monotonic()
        self._lock = asyncio.Lock()
        self._flush_task: asyncio.Task | None = None

    async def embed_one(self, text: str, model: str = "default") -> list[float]:
        """Enqueue a single text and return its embedding.

        The caller gets a future that resolves when the batch is flushed.
        """
        # approximate token count: ~2 chars per token for CJK
        est_tokens = max(1, len(text) // 2)
        cache_key = _embed_cache_key(text, model)

        # TODO: per-process embedding cache lookup (V1-05)
        # For now, always enqueue.

        loop = asyncio.get_running_loop()
        fut: asyncio.Future = loop.create_future()
        async with self._lock:
            self._buffer.append((text, fut, cache_key))
            self._token_count += est_tokens
            should_flush = (
                len(self._buffer) >= self._cfg.max_items
                or (
                    self._cfg.max_tokens > 0
                    and self._token_count >= int(self._cfg.max_tokens * self._cfg.flush_at_token_ratio)
                )
            )
        if should_flush:
            await self._do_flush()
        else:
            self._ensure_scheduled_flush()
        return await fut

    async def _do_flush(self):
        async with self._lock:
            if not self._buffer:
                return
            batch = list(self._buffer)
            self._buffer.clear()
            self._token_count = 0
            self._last_flush = time.monotonic()

        texts = [t[0] for t in batch]
        futures = [t[1] for t in batch]
        try:
            vectors = await self._embed_fn(texts)
            for i, vec in enumerate(vectors):
                if not futures[i].done():
                    futures[i].set_result(vec)
        except Exception as exc:
            # per-item failure: set all futures to the same error
            for f in futures:
                if not f.done():
                    f.set_exception(exc)

    def _ensure_scheduled_flush(self):
        if self._flush_task is None or self._flush_task.done():
            self._flush_task = asyncio.create_task(self._delayed_flush())

    async def _delayed_flush(self):
        await asyncio.sleep(self._cfg.max_wait_ms / 1000.0)
        async with self._lock:
            if not self._buffer:
                return
            elapsed = time.monotonic() - self._last_flush
            if elapsed >= self._cfg.max_wait_ms / 1000.0 and self._buffer:
                pass  # will flush below
            else:
                return
        await self._do_flush()

    async def flush_all(self):
        """Force flush remaining items."""
        await self._do_flush()


def _embed_cache_key(text: str, model: str) -> str:
    h = hashlib.sha256((text + "|" + model).encode()).hexdigest()[:32]
    return h
