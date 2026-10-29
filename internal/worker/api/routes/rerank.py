"""Rerank routes — API via Gateway or lexical fallback."""

from fastapi import APIRouter

from api.deps import get_state
from worker.rerank.gateway import rerank_via_gateway
from worker.rerank.scorer import rerank as rerank_lexical
from worker.schemas import Candidate, RerankRequest, RerankResponse

router = APIRouter(prefix="/v1/rerank", tags=["rerank"])


async def _rerank(req: RerankRequest, level: str) -> RerankResponse:
    state = get_state()
    top_n = req.top_n or (
        state.settings.rerank_l1_top_n if level == "l1" else state.settings.rerank_l2_top_n
    )

    if state.settings.rerank_use_api and state.settings.model_gateway_url:
        try:
            results = await rerank_via_gateway(req.query, req.candidates, state.settings, top_n)
            model = state.settings.rerank_model or "gateway-api"
            return RerankResponse(results=results, model=model)
        except Exception:
            if not state.settings.rerank_fallback_lexical:
                raise

    results = rerank_lexical(req.query, req.candidates, top_n=top_n, level=level)
    return RerankResponse(results=results, model=f"lexical-{level}")


@router.post("/l1", response_model=RerankResponse)
async def rerank_l1(req: RerankRequest) -> RerankResponse:
    return await _rerank(req, "l1")


@router.post("/l2", response_model=RerankResponse)
async def rerank_l2(req: RerankRequest) -> RerankResponse:
    return await _rerank(req, "l2")


@router.post("/combined", response_model=RerankResponse)
async def rerank_combined(req: RerankRequest) -> RerankResponse:
    """Single-call L1 + L2 cascade: runs L1 with a larger pool then narrows to final top_n.

    Reduces one HTTP round-trip vs. calling /l1 then /l2 sequentially.
    The caller should pass the full candidate pool and set top_n to the final desired count.
    L1 breadth (rerank_l1_top_n) is applied internally before the L2 pass.
    """
    state = get_state()
    l1_top = state.settings.rerank_l1_top_n
    l2_top = req.top_n or state.settings.rerank_l2_top_n

    # L1 pass
    if state.settings.rerank_use_api and state.settings.model_gateway_url:
        try:
            l1_results = await rerank_via_gateway(req.query, req.candidates, state.settings, l1_top)
        except Exception:
            if not state.settings.rerank_fallback_lexical:
                raise
            l1_results = rerank_lexical(req.query, req.candidates, top_n=l1_top, level="l1")
    else:
        l1_results = rerank_lexical(req.query, req.candidates, top_n=l1_top, level="l1")

    # If L1 already small enough, skip L2.
    if len(l1_results) <= l2_top or l2_top >= l1_top:
        end = min(l2_top, len(l1_results)) if l2_top < len(l1_results) else len(l1_results)
        return RerankResponse(results=l1_results[:end], model="combined-l1-only")

    # L2 pass: re-rank the L1 survivors.
    l1_candidates = [Candidate(id=r.id, text=r.text) for r in l1_results]
    if state.settings.rerank_use_api and state.settings.model_gateway_url:
        try:
            l2_results = await rerank_via_gateway(req.query, l1_candidates, state.settings, l2_top)
        except Exception:
            if not state.settings.rerank_fallback_lexical:
                raise
            l2_results = rerank_lexical(req.query, l1_candidates, top_n=l2_top, level="l2")
    else:
        l2_results = rerank_lexical(req.query, l1_candidates, top_n=l2_top, level="l2")

    model = state.settings.rerank_model or "combined"
    return RerankResponse(results=l2_results, model=model)
