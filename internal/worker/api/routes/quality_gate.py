"""Quality gate routes."""

from fastapi import APIRouter

from api.deps import get_state
from worker.quality_gate.gate import check_content
from worker.schemas import QualityGateRequest, QualityGateResponse

router = APIRouter(prefix="/v1/quality-gate", tags=["quality-gate"])


@router.post("/check", response_model=QualityGateResponse)
async def quality_check(req: QualityGateRequest) -> QualityGateResponse:
    state = get_state()
    result = check_content(
        req.content,
        title=req.title,
        min_length=state.settings.quality_gate_min_length,
        max_length=state.settings.quality_gate_max_length,
    )
    return QualityGateResponse(passed=result.passed, reason=result.reason, score=result.score)
