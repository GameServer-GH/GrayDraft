"""Prometheus metrics for Worker (O-12)."""

from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest

INGEST_TOTAL = Counter(
    "graydraft_worker_ingest_total",
    "Document ingest completions",
    ["status"],
)
INGEST_DURATION = Histogram(
    "graydraft_worker_ingest_duration_seconds",
    "Ingest pipeline duration",
    buckets=(0.5, 1, 2, 5, 10, 30, 60, 120),
)


def metrics_response() -> tuple[bytes, str]:
    return generate_latest(), CONTENT_TYPE_LATEST
