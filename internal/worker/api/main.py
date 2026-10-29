"""GrayDraft AI Worker — FastAPI entry."""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI, Response

from api.deps import AppState, set_state
from api.metrics import metrics_response
from api.routes import ingest, parsing, qa, quality_gate, rerank, taxonomy
from worker.cache_purge import CachePurger
from worker.config import get_settings
from worker.db.repository import DocumentRepository
from worker.pipeline.ingest import IngestPipeline
from worker.pipeline.nats_consumer import UGCConsumer
from worker.schemas import HealthResponse

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    settings = get_settings()
    repo = await DocumentRepository.create(settings)
    pipeline = IngestPipeline(repo, settings)
    cache_purger = CachePurger(settings)
    nats: UGCConsumer | None = None

    db_status = "ok"
    try:
        await repo.ping()
    except Exception as e:
        db_status = f"unavailable: {e}"
        logger.warning("database ping failed: %s", e)

    await cache_purger.start()
    if settings.nats_enabled:
        nats = UGCConsumer(settings, pipeline, repo)
        try:
            await nats.start()
        except Exception as e:
            logger.warning("NATS consumer failed to start: %s", e)
            nats = None

    set_state(AppState(settings=settings, repo=repo, pipeline=pipeline, cache_purger=cache_purger, nats=nats))
    app.state.db_status = db_status
    if nats and settings.nats_jetstream_enabled:
        app.state.nats_status = "jetstream"
    elif nats:
        app.state.nats_status = "core"
    else:
        app.state.nats_status = "disabled"

    yield

    if nats:
        await nats.stop()
    await cache_purger.close()
    await repo.close()


app = FastAPI(title="GrayDraft Worker", version="0.1.0", lifespan=lifespan)

app.include_router(parsing.router)
app.include_router(rerank.router)
app.include_router(qa.router)
app.include_router(quality_gate.router)
app.include_router(ingest.router)
app.include_router(taxonomy.router)


@app.get("/metrics")
async def metrics() -> Response:
    body, ctype = metrics_response()
    return Response(content=body, media_type=ctype)


@app.get("/health", response_model=HealthResponse)
async def health() -> HealthResponse:
    db = getattr(app.state, "db_status", "unknown")
    nats = getattr(app.state, "nats_status", "unknown")
    status = "ok" if db == "ok" else "degraded"
    return HealthResponse(status=status, service="graydraft-worker", database=db, nats=nats)
