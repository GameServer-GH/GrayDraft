"""API request/response schemas."""

from __future__ import annotations

from typing import Any, Optional

from pydantic import BaseModel, Field


class HealthResponse(BaseModel):
    status: str
    service: str
    database: str = "unknown"
    nats: str = "unknown"


class ParseRequest(BaseModel):
    content: str
    mime_type: str = "text/plain"
    title: Optional[str] = None


class ParseResponse(BaseModel):
    text: str
    title: Optional[str] = None
    char_count: int


class ChunkPreview(BaseModel):
    index: int
    content: str
    char_count: int


class ChunkRequest(BaseModel):
    text: str
    chunk_size: Optional[int] = None
    chunk_overlap: Optional[int] = None


class ChunkResponse(BaseModel):
    chunks: list[ChunkPreview]
    total: int


class Candidate(BaseModel):
    id: str
    text: str


class RerankRequest(BaseModel):
    query: str
    candidates: list[Candidate]
    top_n: Optional[int] = None


class ScoredCandidate(BaseModel):
    id: str
    score: float
    text: Optional[str] = None


class RerankResponse(BaseModel):
    results: list[ScoredCandidate]
    model: str


class QualityGateRequest(BaseModel):
    content: str
    title: Optional[str] = None


class QualityGateResponse(BaseModel):
    passed: bool
    reason: Optional[str] = None
    score: float


class QAGenerateRequest(BaseModel):
    doc_id: Optional[str] = None
    kb_type: str
    chunks: list[str]
    max_pairs: Optional[int] = None


class QAPair(BaseModel):
    question: str
    answer: str
    chunk_index: int
    entities: list[str] = Field(default_factory=list)


class QAGenerateResponse(BaseModel):
    pairs: list[QAPair]
    total: int


class IngestRequest(BaseModel):
    kb_type: str
    title: str
    content: str
    source_url: Optional[str] = None
    mime_type: str = "text/plain"
    doc_id: Optional[str] = None
    generate_qa: bool = True
    auto_index: bool = True
    tags: list[str] = Field(default_factory=list)
    extensions: dict[str, Any] = Field(default_factory=dict)


class IngestResponse(BaseModel):
    doc_id: str
    kb_type: str
    chunk_count: int
    qa_count: int
    tags: list[str]
    index_ready: bool
    task_id: Optional[str] = None


class UGCMessage(BaseModel):
    trace_id: Optional[str] = None
    task_id: Optional[str] = None
    doc_id: Optional[str] = None
    kb_type: str
    title: str
    content: str
    source_url: Optional[str] = None
    mime_type: str = "text/plain"
    generate_qa: bool = True
    tags: list[str] = Field(default_factory=list)
    extensions: dict[str, Any] = Field(default_factory=dict)
