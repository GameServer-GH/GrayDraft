"""JetStream stream bootstrap (idempotent)."""

from __future__ import annotations

import logging

from nats.js.api import RetentionPolicy, StorageType, StreamConfig

from worker.config import Settings

logger = logging.getLogger(__name__)


def _stream_exists_ok(exc: Exception) -> bool:
    msg = str(exc).lower()
    return "stream name already in use" in msg or "already in use with" in msg


async def _add_or_update_stream(js, desired: StreamConfig) -> None:
    try:
        info = await js.stream_info(desired.name)
    except Exception:
        await js.add_stream(desired)
        return

    current = info.config
    current.subjects = desired.subjects
    current.storage = desired.storage
    current.retention = desired.retention
    current.max_age = desired.max_age
    current.max_bytes = desired.max_bytes
    current.num_replicas = desired.num_replicas
    await js.update_stream(current)


async def ensure_streams(js, settings: Settings) -> None:
    if not settings.nats_jetstream_enabled:
        return

    storage = StorageType.FILE
    if settings.nats_js_storage == "memory":
        storage = StorageType.MEMORY

    subjects = settings.nats_js_subjects or [settings.nats_ugc_subject]
    max_age = settings.nats_js_max_age_hours * 3600

    main = StreamConfig(
        name=settings.nats_js_stream,
        subjects=subjects,
        retention=RetentionPolicy.LIMITS,
        storage=storage,
        max_age=max_age,
        max_bytes=settings.nats_js_max_bytes,
        num_replicas=settings.nats_js_replicas,
    )
    try:
        await _add_or_update_stream(js, main)
        logger.info("jetstream stream ready name=%s", settings.nats_js_stream)
    except Exception as e:
        if not _stream_exists_ok(e):
            raise

    if not settings.nats_js_dlq_stream:
        return

    dlq_subjects = settings.nats_js_dlq_subjects or [settings.nats_js_dlq_subject]
    dlq = StreamConfig(
        name=settings.nats_js_dlq_stream,
        subjects=dlq_subjects,
        retention=RetentionPolicy.LIMITS,
        storage=storage,
        max_age=max_age,
        max_bytes=max(settings.nats_js_max_bytes // 4, 1),
        num_replicas=settings.nats_js_replicas,
    )
    try:
        await _add_or_update_stream(js, dlq)
        logger.info("jetstream dlq stream ready name=%s", settings.nats_js_dlq_stream)
    except Exception as e:
        if not _stream_exists_ok(e):
            raise
