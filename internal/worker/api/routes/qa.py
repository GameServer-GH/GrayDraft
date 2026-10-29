"""QA generation routes."""

from fastapi import APIRouter

from api.deps import get_state
from worker.qa_generator.generator import generate_qa_pairs
from worker.schemas import QAGenerateRequest, QAGenerateResponse

router = APIRouter(prefix="/v1/qa", tags=["qa"])


@router.post("/generate", response_model=QAGenerateResponse)
async def generate(req: QAGenerateRequest) -> QAGenerateResponse:
    state = get_state()
    max_pairs = req.max_pairs or state.settings.qa_max_pairs_per_doc
    pairs = generate_qa_pairs(req.chunks, kb_type=req.kb_type, max_pairs=max_pairs)
    return QAGenerateResponse(pairs=pairs, total=len(pairs))
