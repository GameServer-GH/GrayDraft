"""Staged ingest JetStream consumers (P2-1).

accepted → full ingest (bridge; same reliability as graydraft.ugc.ingest)
qa       → post-index QA only (never blocks index_ready)

Other stage subjects remain reserved for future split workers.
"""

from __future__ import annotations

import asyncio
import json
import logging
import uuid
from typing import Any

from nats.aio.client import Client as NATS
from nats.js.api import AckPolicy, ConsumerConfig, DeliverPolicy

from worker.config import Settings
from worker.pipeline.ingest import IngestPipeline
from worker.qa_generator.generator import generate_qa_pairs
from worker.schemas import UGCMessage

logger = logging.getLogger(__name__)

STAGE_ACCEPTED = "accepted"
STAGE_QA = "qa"


class StageMessage:
    """Lightweight parser for Core StageMessage JSON."""

    def __init__(self, raw: dict[str, Any]) -> None:
        self.raw = raw
        self.trace_id = str(raw.get("trace_id") or "")
        self.task_id = str(raw.get("task_id") or "")
        self.stage = str(raw.get("stage") or "")
        self.doc_id = str(raw.get("doc_id") or "") or None
        self.kb_type = str(raw.get("kb_type") or "")
        self.title = str(raw.get("title") or "")
        self.content = str(raw.get("content") or "")
        self.source_url = raw.get("source_url")
        self.mime_type = str(raw.get("mime_type") or "text/plain")
        self.generate_qa = bool(raw.get("generate_qa"))
        self.tags = list(raw.get("tags") or [])
        self.extensions = dict(raw.get("extensions") or {})

    def to_ugc(self, *, force_qa: bool | None = None) -> UGCMessage:
        return UGCMessage(
            trace_id=self.trace_id or None,
            task_id=self.task_id or None,
            doc_id=self.doc_id,
            kb_type=self.kb_type,
            title=self.title,
            content=self.content,
            source_url=self.source_url,
            mime_type=self.mime_type,
            generate_qa=self.generate_qa if force_qa is None else force_qa,
            tags=self.tags,
            extensions=self.extensions,
        )


class StageConsumer:
    def __init__(self, settings: Settings, pipeline: IngestPipeline, repo) -> None:
        self._settings = settings
        self._pipeline = pipeline
        self._repo = repo
        self._nc: NATS | None = None
        self._js = None
        self._tasks: list[asyncio.Task] = []
        self._subs = []

    @property
    def enabled(self) -> bool:
        return bool(self._settings.nats_stage_streams_enabled and self._settings.nats_jetstream_enabled)

    async def start(self, nc: NATS, js) -> None:
        if not self.enabled:
            logger.info("stage consumers disabled")
            return
        self._nc = nc
        self._js = js
        stream = self._settings.nats_js_stage_stream
        for stage, subject, durable, handler in (
            (
                STAGE_ACCEPTED,
                self._settings.nats_stage_accepted,
                self._settings.nats_js_stage_accepted_consumer,
                self._handle_accepted,
            ),
            (
                STAGE_QA,
                self._settings.nats_stage_qa,
                self._settings.nats_js_stage_qa_consumer,
                self._handle_qa,
            ),
        ):
            cfg = ConsumerConfig(
                durable_name=durable,
                ack_policy=AckPolicy.EXPLICIT,
                deliver_policy=DeliverPolicy.ALL,
                max_deliver=self._settings.nats_js_max_deliver,
                ack_wait=self._settings.nats_js_ack_wait_seconds,
                filter_subject=subject,
            )
            try:
                await js.add_consumer(stream, cfg)
            except Exception as e:
                if "consumer name already in use" not in str(e).lower():
                    logger.debug("add_consumer %s: %s", durable, e)
            sub = await js.pull_subscribe(subject, durable=durable, stream=stream)
            self._subs.append(sub)
            self._tasks.append(asyncio.create_task(self._pull_loop(sub, handler, stage)))
            logger.info(
                "stage consumer ready stage=%s stream=%s durable=%s subject=%s",
                stage,
                stream,
                durable,
                subject,
            )

    async def stop(self) -> None:
        for t in self._tasks:
            t.cancel()
        for t in self._tasks:
            try:
                await t
            except asyncio.CancelledError:
                pass
        self._tasks.clear()
        self._subs.clear()

    async def _pull_loop(self, sub, handler, stage: str) -> None:  # noqa: ANN001
        while True:
            try:
                msgs = await sub.fetch(
                    batch=self._settings.nats_js_fetch_batch,
                    timeout=self._settings.nats_js_fetch_timeout_seconds,
                )
            except asyncio.TimeoutError:
                continue
            except Exception as e:
                logger.warning("stage fetch error stage=%s: %s", stage, e)
                await asyncio.sleep(1)
                continue
            for msg in msgs:
                delivered = 1
                if msg.metadata:
                    delivered = msg.metadata.num_delivered
                try:
                    await handler(msg.data)
                    await msg.ack()
                except Exception:
                    logger.exception("stage handler failed stage=%s", stage)
                    if delivered >= self._settings.nats_js_max_deliver:
                        await self._publish_dlq(msg.data, stage=stage, reason="max_deliver_exceeded")
                        await msg.ack()
                    else:
                        await msg.nak(delay=min(30 * delivered, 300))

    async def _handle_accepted(self, data: bytes) -> None:
        payload = json.loads(data.decode("utf-8"))
        # Accept both StageMessage and legacy UGCMessage shapes.
        if "stage" in payload or "parsed_text" in payload:
            stage_msg = StageMessage(payload)
        else:
            ugc = UGCMessage.model_validate(payload)
            stage_msg = StageMessage(
                {
                    "trace_id": ugc.trace_id,
                    "task_id": ugc.task_id,
                    "stage": STAGE_ACCEPTED,
                    "doc_id": ugc.doc_id,
                    "kb_type": ugc.kb_type,
                    "title": ugc.title,
                    "content": ugc.content,
                    "source_url": ugc.source_url,
                    "mime_type": ugc.mime_type,
                    "generate_qa": ugc.generate_qa,
                    "tags": ugc.tags,
                    "extensions": ugc.extensions,
                }
            )

        task_id = stage_msg.task_id
        trace_id = stage_msg.trace_id
        if task_id:
            await self._repo.mark_ugc_task_processing(task_id=uuid.UUID(task_id))
            await self._repo.insert_stage_event(
                task_id=task_id, trace_id=trace_id, stage=STAGE_ACCEPTED, status="started", attempt=1
            )
            await self._repo.set_ugc_task_stage(task_id=task_id, current_stage=STAGE_ACCEPTED)

        # Ready path never waits on QA; defer via qa subject when requested.
        want_qa = stage_msg.generate_qa
        ugc = stage_msg.to_ugc(force_qa=False)
        result = await self._pipeline.ingest_ugc(ugc)

        if task_id:
            await self._repo.update_ugc_task_completed(
                task_id=uuid.UUID(task_id),
                doc_id=uuid.UUID(result.doc_id),
                chunk_count=result.chunk_count,
            )
            await self._repo.insert_stage_event(
                task_id=task_id, trace_id=trace_id, stage="indexed", status="completed", attempt=1
            )
            await self._repo.set_ugc_task_stage(task_id=task_id, current_stage="indexed")

        await self._publish_stage(
            STAGE_QA if want_qa else "indexed",
            {
                **stage_msg.raw,
                "stage": STAGE_QA if want_qa else "indexed",
                "doc_id": result.doc_id,
                "generate_qa": want_qa,
                "index_result": {
                    "doc_id": result.doc_id,
                    "chunk_count": result.chunk_count,
                    "index_ready": result.index_ready,
                },
            },
        )
        logger.info(
            "stage accepted done task_id=%s doc_id=%s chunks=%d qa_queued=%s",
            task_id or "-",
            result.doc_id,
            result.chunk_count,
            want_qa,
        )

    async def _handle_qa(self, data: bytes) -> None:
        payload = json.loads(data.decode("utf-8"))
        stage_msg = StageMessage(payload)
        if not stage_msg.generate_qa:
            return
        doc_id = stage_msg.doc_id
        if not doc_id:
            idx = payload.get("index_result") or {}
            doc_id = idx.get("doc_id")
        if not doc_id or not stage_msg.kb_type:
            logger.warning("qa stage missing doc_id/kb_type task_id=%s", stage_msg.task_id)
            return
        if stage_msg.task_id:
            await self._repo.set_ugc_task_stage(task_id=stage_msg.task_id, current_stage=STAGE_QA)
            await self._repo.insert_stage_event(
                task_id=stage_msg.task_id,
                trace_id=stage_msg.trace_id,
                stage=STAGE_QA,
                status="started",
                attempt=1,
            )

        rows = await self._repo.list_chunk_texts(doc_id=doc_id, kb_type=stage_msg.kb_type, limit=80)
        if not rows:
            logger.info("qa stage skip empty chunks doc_id=%s", doc_id)
            return
        texts = [str(r["content"] or "") for r in rows]
        pairs = await asyncio.to_thread(
            generate_qa_pairs,
            texts,
            kb_type=stage_msg.kb_type,
            max_pairs=self._settings.qa_max_pairs_per_doc,
            title=stage_msg.title,
            extensions=stage_msg.extensions,
        )
        chunk_ids = [uuid.UUID(str(r["id"])) for r in rows]
        await self._repo.insert_qa_pairs(
            doc_id=uuid.UUID(str(doc_id)),
            kb_type=stage_msg.kb_type,
            pairs=[p.model_dump() if hasattr(p, "model_dump") else dict(p) for p in pairs],
            chunk_ids=chunk_ids,
            index_ready=False,
        )
        if stage_msg.task_id:
            await self._repo.insert_stage_event(
                task_id=stage_msg.task_id,
                trace_id=stage_msg.trace_id,
                stage=STAGE_QA,
                status="completed",
                attempt=1,
            )
            await self._repo.set_ugc_task_stage(task_id=stage_msg.task_id, current_stage="qa_done")
        logger.info("stage qa done doc_id=%s pairs=%d", doc_id, len(pairs))

    async def _publish_stage(self, stage: str, payload: dict[str, Any]) -> None:
        if not self._js:
            return
        subject = {
            "indexed": self._settings.nats_stage_indexed,
            STAGE_QA: self._settings.nats_stage_qa,
        }.get(stage)
        if not subject:
            return
        # indexed is observability-only for now; no consumer required.
        if stage == "indexed":
            try:
                await self._js.publish(subject, json.dumps(payload).encode("utf-8"))
            except Exception:
                logger.debug("publish indexed stage failed", exc_info=True)
            return
        if stage == STAGE_QA and payload.get("generate_qa"):
            await self._js.publish(subject, json.dumps(payload).encode("utf-8"))

    async def _publish_dlq(self, data: bytes, *, stage: str, reason: str) -> None:
        if not self._js:
            return
        subject = f"graydraft.ingest.dlq.{stage}"
        try:
            await self._js.publish(subject, data, headers={"X-DLQ-Reason": reason})
        except Exception:
            logger.exception("stage DLQ publish failed stage=%s", stage)
