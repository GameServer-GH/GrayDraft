"""Best-effort Redis cache purge after Worker-side document mutation."""

from __future__ import annotations

import logging

from worker.config import Settings

logger = logging.getLogger(__name__)

RETRIEVAL_PREFIX = "knex:sr:"
RETRIEVAL_INDEX_PREFIX = "knex:sr:idx:"
ANSWER_PREFIX = "knex:ans:"
ANSWER_KB_INDEX_PREFIX = "knex:ans:idxkb:"


class CachePurger:
    def __init__(self, settings: Settings) -> None:
        self._settings = settings
        self._client = None
        self._available = False

    async def start(self) -> None:
        try:
            from redis.asyncio import Redis
        except Exception as e:
            logger.info("Redis purge disabled: redis package unavailable: %s", e)
            return
        try:
            client = Redis(
                host=self._settings.redis_host,
                port=self._settings.redis_port,
                password=self._settings.redis_password or None,
                db=self._settings.redis_db,
                decode_responses=True,
            )
            await client.ping()
            self._client = client
            self._available = True
        except Exception as e:
            logger.warning("Redis purge disabled: %s", e)

    async def close(self) -> None:
        if self._client is not None:
            await self._client.aclose()
            self._client = None
        self._available = False

    async def purge_kb_type(
        self,
        kb_type: str,
        *,
        trace_id: str | None = None,
        task_id: str | None = None,
        doc_id: str | None = None,
        readiness: str | None = None,
        entities: list[str] | None = None,
        title_terms: list[str] | None = None,
        taxonomy_ids: list[str] | None = None,
    ) -> None:
        kb_type = (kb_type or "").strip()
        if not kb_type or not self._available or self._client is None:
            return
        retrieval_removed = 0
        answer_removed = 0
        if self._settings.retrieval_cache_enabled:
            retrieval_removed = await self._purge_retrieval(kb_type)
        if self._settings.answer_cache_enabled:
            answer_removed = await self._purge_answer(kb_type)

        # V1-16: publish fine-grained invalidation event for targeted eviction.
        if entities or title_terms or taxonomy_ids:
            await self._publish_invalidation(
                kb_type=kb_type,
                trace_id=trace_id,
                task_id=task_id,
                doc_id=doc_id,
                readiness=readiness,
                entities=entities,
                title_terms=title_terms,
                taxonomy_ids=taxonomy_ids,
            )

        logger.info(
            (
                "worker cache purged trace_id=%s task_id=%s doc_id=%s kb_type=%s "
                "readiness=%s retrieval_removed=%d answer_removed=%d entities=%d title_terms=%d"
            ),
            trace_id or "-",
            task_id or "-",
            doc_id or "-",
            kb_type,
            readiness or "-",
            retrieval_removed,
            answer_removed,
            len(entities or []),
            len(title_terms or []),
        )

    async def _publish_invalidation(
        self,
        *,
        kb_type: str,
        trace_id: str | None,
        task_id: str | None,
        doc_id: str | None,
        readiness: str | None,
        entities: list[str] | None,
        title_terms: list[str] | None,
        taxonomy_ids: list[str] | None,
    ) -> None:
        """Publish fine-grained cache invalidation event via NATS."""
        import json

        try:
            from worker.pipeline.nats_consumer import _get_nats_client
        except ImportError:
            return

        nc = _get_nats_client()
        if nc is None:
            return

        subject = self._settings.cache_invalidation_subject or "graydraft.cache.invalidate"
        payload = {
            "trace_id": trace_id,
            "task_id": task_id,
            "doc_id": doc_id,
            "kb_type": kb_type,
            "readiness": readiness,
            "entities": entities or [],
            "title_terms": title_terms or [],
            "taxonomy_ids": taxonomy_ids or [],
            "purge_all": not (entities or title_terms or taxonomy_ids),
        }
        try:
            await nc.publish(subject, json.dumps(payload).encode("utf-8"))
        except Exception:
            logger.debug("failed to publish invalidation event to NATS", exc_info=True)

    async def _purge_retrieval(self, kb_type: str) -> int:
        assert self._client is not None
        removed = 0
        cursor = 0
        pattern = f"{RETRIEVAL_INDEX_PREFIX}{kb_type}:*"
        while True:
            cursor, keys = await self._client.scan(cursor=cursor, match=pattern, count=64)
            for idx_key in keys:
                members = await self._client.lrange(idx_key, 0, -1)
                del_keys = [f"{RETRIEVAL_PREFIX}{m}" for m in members]
                del_keys.append(idx_key)
                removed += int(await self._client.delete(*del_keys))
            if cursor == 0:
                return removed

    async def _purge_answer(self, kb_type: str) -> int:
        assert self._client is not None
        idx = f"{ANSWER_KB_INDEX_PREFIX}{kb_type}"
        members = await self._client.smembers(idx)
        del_keys = [f"{ANSWER_PREFIX}{m}" for m in members]
        del_keys.append(idx)
        return int(await self._client.delete(*del_keys))
