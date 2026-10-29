"""Document ingest & UGC pipeline routes."""

import logging

from fastapi import APIRouter, HTTPException, Request

from api.deps import get_state
from worker.schemas import IngestRequest, IngestResponse, UGCMessage

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/v1/ingest", tags=["ingest"])


@router.post("", response_model=IngestResponse)
async def ingest_document(req: IngestRequest) -> IngestResponse:
    state = get_state()
    try:
        return await state.pipeline.ingest(req)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e)) from e


@router.post("/ugc", response_model=IngestResponse)
async def ingest_ugc(req: UGCMessage, request: Request) -> IngestResponse:
    """Simulates /v1/ugc/notify callback (sync path)."""
    if not req.trace_id:
        req = req.model_copy(
            update={
                "trace_id": request.headers.get("X-Trace-Id")
                or request.headers.get("X-Request-ID")
            }
        )
    if not req.task_id:
        tid = request.headers.get("X-Task-Id")
        if tid:
            req = req.model_copy(update={"task_id": tid})
    logger.info(
        "sync ugc ingest task_id=%s trace_id=%s kb_type=%s",
        req.task_id or "-",
        req.trace_id or "-",
        req.kb_type,
    )
    state = get_state()
    try:
        return await state.pipeline.ingest_ugc(req)
    except ValueError as e:
        raise HTTPException(status_code=400, detail=str(e)) from e
