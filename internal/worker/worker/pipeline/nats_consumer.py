"""NATS JetStream consumer for UGC ingest with ack/nak and DLQ."""

from __future__ import annotations

import asyncio
import json
import logging

from nats.aio.client import Client as NATS
from nats.js.api import AckPolicy, ConsumerConfig, DeliverPolicy

from worker.cache_purge import CachePurger
from worker.config import Settings
from worker.pipeline.ingest import IngestPipeline
from worker.pipeline.jetstream_setup import ensure_streams
from worker.schemas import UGCMessage

logger = logging.getLogger(__name__)


def _nats_header(msg, key: str) -> str | None:  # noqa: ANN001
    headers = getattr(msg, "headers", None)
    if not headers:
        return None
    try:
        val = headers.get(key)
    except Exception:
        return None
    if val is None:
        return None
    if isinstance(val, (list, tuple)):
        return str(val[0]) if val else None
    return str(val)


class UGCConsumer:
    def __init__(self, settings: Settings, pipeline: IngestPipeline, repo) -> None:
        self._settings = settings
        self._pipeline = pipeline
        self._repo = repo
        self._nc: NATS | None = None
        self._js = None
        self._sub = None
        self._pull_task: asyncio.Task | None = None
        self._cache_purger = CachePurger(settings)

    async def start(self) -> None:
        if not self._settings.nats_enabled:
            logger.info("NATS consumer disabled")
            return

        await self._cache_purger.start()
        self._nc = NATS()
        await self._nc.connect(self._settings.nats_url)
        global _nats_client
        _nats_client = self._nc

        if self._settings.nats_jetstream_enabled:
            await self._start_jetstream()
        else:
            await self._nc.subscribe(self._settings.nats_ugc_subject, cb=self._on_core_message)
            logger.info("NATS core subscribed subject=%s", self._settings.nats_ugc_subject)

    async def _start_jetstream(self) -> None:
        self._js = self._nc.jetstream()
        await ensure_streams(self._js, self._settings)

        cfg = ConsumerConfig(
            durable_name=self._settings.nats_js_consumer,
            ack_policy=AckPolicy.EXPLICIT,
            deliver_policy=DeliverPolicy.ALL,
            max_deliver=self._settings.nats_js_max_deliver,
            ack_wait=self._settings.nats_js_ack_wait_seconds,
            filter_subject=self._settings.nats_ugc_subject,
        )
        try:
            await self._js.add_consumer(self._settings.nats_js_stream, cfg)
        except Exception as e:
            if "consumer name already in use" not in str(e).lower():
                logger.debug("add_consumer: %s", e)

        self._sub = await self._js.pull_subscribe(
            self._settings.nats_ugc_subject,
            durable=self._settings.nats_js_consumer,
            stream=self._settings.nats_js_stream,
        )
        self._pull_task = asyncio.create_task(self._pull_loop())
        logger.info(
            "JetStream pull consumer stream=%s durable=%s subject=%s",
            self._settings.nats_js_stream,
            self._settings.nats_js_consumer,
            self._settings.nats_ugc_subject,
        )

    async def _pull_loop(self) -> None:
        assert self._sub is not None
        while True:
            try:
                msgs = await self._sub.fetch(
                    batch=self._settings.nats_js_fetch_batch,
                    timeout=self._settings.nats_js_fetch_timeout_seconds,
                )
            except asyncio.TimeoutError:
                continue
            except Exception as e:
                logger.warning("jetstream fetch error: %s", e)
                await asyncio.sleep(1)
                continue

            for msg in msgs:
                await self._on_jetstream_message(msg)

    async def stop(self) -> None:
        if self._pull_task:
            self._pull_task.cancel()
            try:
                await self._pull_task
            except asyncio.CancelledError:
                pass
            self._pull_task = None
        global _nats_client
        if self._nc:
            await self._nc.drain()
            await self._nc.close()
            self._nc = None
        _nats_client = None
        await self._cache_purger.close()
        self._js = None
        self._sub = None

    async def _on_core_message(self, msg) -> None:  # noqa: ANN001
        await self._process_payload(msg.data, msg.reply, ack=None, nats_msg=msg)

    async def _on_jetstream_message(self, msg) -> None:  # noqa: ANN001
        delivered = 1
        if msg.metadata:
            delivered = msg.metadata.num_delivered
        try:
            await self._process_payload(msg.data, msg.reply, ack=msg, nats_msg=msg)
        except Exception:
            if delivered >= self._settings.nats_js_max_deliver:
                await self._publish_dlq(msg.data, reason="max_deliver_exceeded")
                await msg.ack()
            else:
                delay = min(30 * delivered, 300)
                await msg.nak(delay=delay)

    async def _process_payload(
        self, data: bytes, reply: str | None, ack, *, nats_msg=None
    ) -> None:  # noqa: ANN001
        task_id = None
        trace_id = None
        try:
            payload = json.loads(data.decode("utf-8"))
            ugc = UGCMessage.model_validate(payload)
            trace_id = ugc.trace_id or payload.get("trace_id") or _nats_header(nats_msg, "X-Trace-Id")
            if trace_id and not ugc.trace_id:
                ugc = ugc.model_copy(update={"trace_id": trace_id})
            task_id = ugc.task_id or _nats_header(nats_msg, "X-Task-Id")
            if task_id and not ugc.task_id:
                ugc = ugc.model_copy(update={"task_id": task_id})

            logger.info(
                "UGC message received task_id=%s trace_id=%s kb_type=%s",
                task_id or "-",
                trace_id or "-",
                ugc.kb_type,
            )

            if task_id:
                import uuid as _uuid

                await self._repo.mark_ugc_task_processing(task_id=_uuid.UUID(task_id))
                # V1-09: record stage=ingesting start.
                await self._repo.insert_stage_event(
                    task_id=task_id,
                    trace_id=trace_id,
                    stage="ingesting",
                    status="started",
                    attempt=1,
                )
                await self._repo.set_ugc_task_stage(
                    task_id=task_id,
                    current_stage="ingesting",
                    stage_retry_count=0,
                )

            result = await self._pipeline.ingest_ugc(ugc)
            readiness = "ready" if result.index_ready and result.chunk_count > 0 else "completed_not_indexed"
            queryable = result.index_ready and result.chunk_count > 0
            logger.info(
                (
                    "UGC ingested trace_id=%s task_id=%s doc_id=%s kb_type=%s "
                    "readiness=%s queryable=%s chunks=%d index_ready=%s"
                ),
                trace_id or "-",
                task_id or "-",
                result.doc_id,
                result.kb_type,
                readiness,
                queryable,
                result.chunk_count,
                result.index_ready,
            )

            if task_id:
                import uuid as _uuid

                await self._repo.update_ugc_task_completed(
                    task_id=_uuid.UUID(task_id),
                    doc_id=_uuid.UUID(result.doc_id),
                    chunk_count=result.chunk_count,
                )

            # V1-09: record ingest complete.
            if task_id:
                await self._repo.insert_stage_event(
                    task_id=task_id,
                    trace_id=trace_id,
                    stage="ingested",
                    status="completed",
                    attempt=1,
                )
                await self._repo.set_ugc_task_stage(
                    task_id=task_id,
                    current_stage="ingested",
                )

            await self._cache_purger.purge_kb_type(
                result.kb_type,
                trace_id=trace_id,
                task_id=task_id,
                doc_id=result.doc_id,
                readiness=readiness,
            )

            if ack is not None:
                await ack.ack()
            elif reply:
                await self._nc.publish(reply, json.dumps(result.model_dump()).encode("utf-8"))
        except Exception as e:
            logger.exception(
                "UGC ingest failed task_id=%s trace_id=%s: %s",
                task_id or "-",
                trace_id or "-",
                e,
            )
            if task_id:
                try:
                    import uuid as _uuid

                    await self._repo.update_ugc_task_failed(
                        task_id=_uuid.UUID(task_id),
                        error_message=str(e),
                    )
                    # V1-09: record stage=failed.
                    await self._repo.insert_stage_event(
                        task_id=task_id,
                        trace_id=trace_id,
                        stage="ingesting",
                        status="failed",
                        attempt=1,
                        error_code="worker_error",
                        error_message=str(e),
                    )
                    await self._repo.set_ugc_task_stage(
                        task_id=task_id,
                        current_stage="ingesting",
                    )
                except Exception:
                    logger.exception("failed to update ugc task status")
            if ack is not None:
                raise
            if reply:
                await self._nc.publish(reply, json.dumps({"error": str(e)}).encode("utf-8"))

    async def _publish_dlq(self, data: bytes, reason: str) -> None:
        if not self._js:
            return
        subject = self._settings.nats_js_dlq_subject
        headers = {"X-DLQ-Reason": reason}
        try:
            await self._js.publish(subject, data, headers=headers)
            logger.warning("message moved to DLQ subject=%s reason=%s", subject, reason)
        except Exception:
            logger.exception("failed to publish DLQ")

# V1-16: global NATS client for invalidation event publishing.
_nats_client: "NATS | None" = None
_nats_js = None


def _get_nats_client():
    """Return the global NATS client, if any."""
    return _nats_client


def _get_nats_js():
    """Return the global JetStream context, if any."""
    return _nats_js
