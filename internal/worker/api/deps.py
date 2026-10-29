"""FastAPI shared dependencies."""

from __future__ import annotations

from dataclasses import dataclass

from worker.config import Settings
from worker.cache_purge import CachePurger
from worker.db.repository import DocumentRepository
from worker.pipeline.ingest import IngestPipeline
from worker.pipeline.nats_consumer import UGCConsumer


@dataclass
class AppState:
    settings: Settings
    repo: DocumentRepository
    pipeline: IngestPipeline
    cache_purger: CachePurger | None = None
    nats: UGCConsumer | None = None


_state: AppState | None = None


def set_state(state: AppState) -> None:
    global _state
    _state = state


def get_state() -> AppState:
    if _state is None:
        raise RuntimeError("application not initialized")
    return _state
