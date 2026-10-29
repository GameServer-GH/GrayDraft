"""Document parsing routes."""

from fastapi import APIRouter, HTTPException

from api.deps import get_state
from worker.parsers.text_parser import parse_content
from worker.schemas import ChunkRequest, ChunkResponse, ChunkPreview, ParseRequest, ParseResponse
from worker.chunkers.semantic_chunker import chunk_text

router = APIRouter(prefix="/v1/parsing", tags=["parsing"])


@router.post("/parse", response_model=ParseResponse)
async def parse_document(req: ParseRequest) -> ParseResponse:
    text, title = parse_content(req.content, req.mime_type, req.title)
    return ParseResponse(text=text, title=title, char_count=len(text))


@router.post("/chunk", response_model=ChunkResponse)
async def chunk_document(req: ChunkRequest) -> ChunkResponse:
    state = get_state()
    size = req.chunk_size or state.settings.chunk_size
    overlap = req.chunk_overlap or state.settings.chunk_overlap
    parts = chunk_text(req.text, chunk_size=size, chunk_overlap=overlap)
    previews = [ChunkPreview(index=i, content=c, char_count=len(c)) for i, c in enumerate(parts)]
    return ChunkResponse(chunks=previews, total=len(previews))
