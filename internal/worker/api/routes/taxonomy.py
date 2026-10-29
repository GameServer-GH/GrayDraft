"""Workspace taxonomy maintenance (shadow backfill)."""

from __future__ import annotations

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from api.deps import get_state
from worker.pipeline.taxonomy_backfill import run_shadow_backfill

router = APIRouter(prefix="/v1/taxonomy", tags=["taxonomy"])


class ShadowBackfillRequest(BaseModel):
    kb_type: str = Field(..., description="Workspace partition gd_*")
    taxonomy_version: int = Field(..., ge=1)
    limit: int = Field(80, ge=1, le=200)


@router.post("/shadow-backfill")
async def shadow_backfill(req: ShadowBackfillRequest) -> dict:
    st = get_state()
    if st.repo is None:
        raise HTTPException(503, "repository not ready")
    try:
        resp = await run_shadow_backfill(
            st.repo,
            st.settings,
            kb_type=req.kb_type,
            taxonomy_version=req.taxonomy_version,
            limit=req.limit,
        )
        if st.cache_purger is not None and resp.get("updated", 0) > 0:
            await st.cache_purger.purge_kb_type(
                req.kb_type,
                readiness="taxonomy_shadow_backfill",
            )
        return resp
    except ValueError as e:
        raise HTTPException(400, str(e)) from e
    except Exception as e:
        raise HTTPException(500, str(e)) from e
