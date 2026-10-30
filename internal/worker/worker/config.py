"""Configuration loader for Worker service."""

from __future__ import annotations

import os
from functools import lru_cache
from pathlib import Path
from typing import Any

import yaml
from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file_encoding="utf-8", extra="ignore")

    app_name: str = "graydraft-worker"
    app_env: str = "development"
    log_level: str = "debug"
    http_port: int = 18001

    db_host: str = "localhost"
    db_port: int = 5432
    db_user: str = "graydraft_admin"
    db_password: str = "dev_password_123"
    db_name: str = "graydraft_kb"
    db_sslmode: str = "disable"

    redis_host: str = "redis"
    redis_port: int = 6379
    redis_password: str = ""
    redis_db: int = 0
    retrieval_cache_enabled: bool = True
    answer_cache_enabled: bool = True

    nats_url: str = "nats://localhost:24222"
    nats_ugc_subject: str = "graydraft.ugc.ingest"
    cache_invalidation_subject: str = "graydraft.cache.invalidate"
    nats_enabled: bool = True
    nats_jetstream_enabled: bool = True
    nats_js_stream: str = "GRAYDRAFT_UGC"
    nats_js_subjects: list[str] = Field(default_factory=lambda: ["graydraft.ugc.ingest"])
    nats_js_storage: str = "file"
    nats_js_max_age_hours: int = 168
    nats_js_max_bytes: int = 1_073_741_824
    nats_js_replicas: int = 1
    nats_js_dlq_stream: str = "GRAYDRAFT_UGC_DLQ"
    nats_js_dlq_subject: str = "graydraft.ugc.dlq"
    nats_js_dlq_subjects: list[str] = Field(default_factory=lambda: ["graydraft.ugc.dlq"])
    nats_js_consumer: str = "graydraft-worker-ugc"
    nats_js_max_deliver: int = 5
    nats_js_ack_wait_seconds: int = 120
    nats_js_fetch_batch: int = 5
    nats_js_fetch_timeout_seconds: float = 5.0
    nats_stage_streams_enabled: bool = False
    nats_js_stage_stream: str = "GRAYDRAFT_UGC_STAGES"
    nats_stage_accepted: str = "graydraft.ingest.accepted"
    nats_stage_indexed: str = "graydraft.ingest.indexed"
    nats_stage_qa: str = "graydraft.ingest.qa"
    nats_js_stage_accepted_consumer: str = "graydraft-worker-stage-accepted"
    nats_js_stage_qa_consumer: str = "graydraft-worker-stage-qa"
    nats_stage_subjects: list[str] = Field(
        default_factory=lambda: [
            "graydraft.ingest.accepted",
            "graydraft.ingest.parsed",
            "graydraft.ingest.media_extracted",
            "graydraft.ingest.chunked",
            "graydraft.ingest.embedded",
            "graydraft.ingest.indexed",
            "graydraft.ingest.qa",
        ]
    )

    chunk_size: int = 512
    chunk_overlap: int = 64
    embedding_dim: int = 1024
    embedding_model: str = "text-embedding-v4"

    model_gateway_url: str = ""
    vllm_url: str = ""

    rerank_model: str = "qwen3-rerank"
    rerank_use_api: bool = True
    rerank_fallback_lexical: bool = True
    rerank_l1_top_n: int = 15
    rerank_l2_top_n: int = 5
    quality_gate_min_length: int = 20
    quality_gate_max_length: int = 50000

    qa_max_pairs_per_doc: int = 5

    anchor_extraction_enabled: bool = True
    anchor_llm_model: str = "auto"

    @property
    def database_dsn(self) -> str:
        return (
            f"postgresql://{self.db_user}:{self.db_password}"
            f"@{self.db_host}:{self.db_port}/{self.db_name}"
        )

    @classmethod
    def from_yaml(cls, path: str | Path) -> Settings:
        data: dict[str, Any] = {}
        p = Path(path)
        if p.exists():
            with p.open(encoding="utf-8") as f:
                raw = yaml.safe_load(f) or {}
            data = _flatten_yaml(raw)
        base = cls()
        merged = {**base.model_dump(), **{k: v for k, v in data.items() if v is not None}}
        return cls(**merged)


def _flatten_yaml(raw: dict[str, Any]) -> dict[str, Any]:
    """Map nested YAML keys to Settings field names."""
    out: dict[str, Any] = {}
    if app := raw.get("app"):
        out["app_name"] = app.get("name", out.get("app_name"))
        out["app_env"] = app.get("env", out.get("app_env"))
    if http := raw.get("http"):
        out["http_port"] = http.get("port", out.get("http_port"))
    if db := raw.get("database"):
        out["db_host"] = db.get("host", out.get("db_host"))
        out["db_port"] = db.get("port", out.get("db_port"))
        out["db_user"] = db.get("user", out.get("db_user"))
        out["db_password"] = db.get("password", out.get("db_password"))
        out["db_name"] = db.get("name", out.get("db_name"))
        out["db_sslmode"] = db.get("sslmode", out.get("db_sslmode"))
    if redis := raw.get("redis"):
        out["redis_host"] = redis.get("host", out.get("redis_host"))
        out["redis_port"] = redis.get("port", out.get("redis_port"))
        out["redis_password"] = redis.get("password", out.get("redis_password"))
        out["redis_db"] = redis.get("db", out.get("redis_db"))
    if cache := raw.get("cache"):
        out["retrieval_cache_enabled"] = cache.get("enabled", out.get("retrieval_cache_enabled"))
    if ans := raw.get("answer_cache"):
        out["answer_cache_enabled"] = ans.get("enabled", out.get("answer_cache_enabled"))
    if nats := raw.get("nats"):
        out["nats_url"] = nats.get("url", out.get("nats_url"))
        out["nats_ugc_subject"] = nats.get("ugc_subject", out.get("nats_ugc_subject"))
        out["nats_enabled"] = nats.get("enabled", out.get("nats_enabled"))
        if js := nats.get("jetstream"):
            out["nats_jetstream_enabled"] = js.get("enabled", out.get("nats_jetstream_enabled"))
            out["nats_js_stream"] = js.get("stream", out.get("nats_js_stream"))
            out["nats_js_subjects"] = js.get("subjects", out.get("nats_js_subjects"))
            out["nats_js_storage"] = js.get("storage", out.get("nats_js_storage"))
            out["nats_js_max_age_hours"] = js.get("max_age_hours", out.get("nats_js_max_age_hours"))
            out["nats_js_max_bytes"] = js.get("max_bytes", out.get("nats_js_max_bytes"))
            out["nats_js_replicas"] = js.get("replicas", out.get("nats_js_replicas"))
            out["nats_js_dlq_stream"] = js.get("dlq_stream", out.get("nats_js_dlq_stream"))
            out["nats_js_dlq_subject"] = js.get("dlq_subject", out.get("nats_js_dlq_subject"))
            if subs := js.get("dlq_subjects"):
                out["nats_js_dlq_subjects"] = subs
            out["nats_js_consumer"] = js.get("consumer", out.get("nats_js_consumer"))
            out["nats_js_max_deliver"] = js.get("max_deliver", out.get("nats_js_max_deliver"))
            out["nats_js_ack_wait_seconds"] = js.get("ack_wait_seconds", out.get("nats_js_ack_wait_seconds"))
            out["nats_js_fetch_batch"] = js.get("fetch_batch", out.get("nats_js_fetch_batch"))
            out["nats_js_fetch_timeout_seconds"] = js.get(
                "fetch_timeout_seconds", out.get("nats_js_fetch_timeout_seconds")
            )
            out["nats_stage_streams_enabled"] = js.get(
                "stage_streams_enabled", out.get("nats_stage_streams_enabled")
            )
            if stage_stream := js.get("stage_stream"):
                out["nats_js_stage_stream"] = stage_stream
            if stage_subjects := js.get("stage_subjects"):
                out["nats_stage_subjects"] = stage_subjects
            if cons := js.get("stage_accepted_consumer"):
                out["nats_js_stage_accepted_consumer"] = cons
            if cons := js.get("stage_qa_consumer"):
                out["nats_js_stage_qa_consumer"] = cons
        if stages := nats.get("stage_subjects"):
            if isinstance(stages, dict):
                if stages.get("accepted"):
                    out["nats_stage_accepted"] = stages["accepted"]
                if stages.get("indexed"):
                    out["nats_stage_indexed"] = stages["indexed"]
                if stages.get("qa"):
                    out["nats_stage_qa"] = stages["qa"]
    if chunk := raw.get("chunking"):
        out["chunk_size"] = chunk.get("size", out.get("chunk_size"))
        out["chunk_overlap"] = chunk.get("overlap", out.get("chunk_overlap"))
    if emb := raw.get("embedding"):
        out["embedding_dim"] = emb.get("dim", out.get("embedding_dim"))
        out["embedding_model"] = emb.get("model", out.get("embedding_model"))
        out["model_gateway_url"] = emb.get("gateway_url", out.get("model_gateway_url"))
    if rg := raw.get("rerank"):
        out["rerank_model"] = rg.get("model", out.get("rerank_model"))
        out["rerank_use_api"] = rg.get("use_api", out.get("rerank_use_api"))
        out["rerank_fallback_lexical"] = rg.get("fallback_lexical", out.get("rerank_fallback_lexical"))
        out["rerank_l1_top_n"] = rg.get("l1_top_n", out.get("rerank_l1_top_n"))
        out["rerank_l2_top_n"] = rg.get("l2_top_n", out.get("rerank_l2_top_n"))
    if qg := raw.get("quality_gate"):
        out["quality_gate_min_length"] = qg.get("min_length", out.get("quality_gate_min_length"))
        out["quality_gate_max_length"] = qg.get("max_length", out.get("quality_gate_max_length"))
        if url := qg.get("local_model_url"):
            out["vllm_url"] = url
    if qa := raw.get("qa"):
        out["qa_max_pairs_per_doc"] = qa.get("max_pairs", out.get("qa_max_pairs_per_doc"))
    if enr := raw.get("enrichment"):
        out["anchor_extraction_enabled"] = enr.get("anchor_extraction_enabled", out.get("anchor_extraction_enabled"))
        out["anchor_llm_model"] = enr.get("llm_model", out.get("anchor_llm_model"))
    return out


def _overlay_env(s: Settings) -> Settings:
    data = s.model_dump()
    mapping = {
        "HTTP_PORT": "http_port",
        "DB_HOST": "db_host",
        "DB_PORT": "db_port",
        "DB_USER": "db_user",
        "DB_PASSWORD": "db_password",
        "DB_NAME": "db_name",
        "REDIS_HOST": "redis_host",
        "REDIS_PORT": "redis_port",
        "REDIS_PASSWORD": "redis_password",
        "REDIS_DB": "redis_db",
        "CACHE_ENABLED": "retrieval_cache_enabled",
        "ANSWER_CACHE_ENABLED": "answer_cache_enabled",
        "NATS_URL": "nats_url",
        "MODEL_GATEWAY_URL": "model_gateway_url",
        "EMBEDDING_MODEL": "embedding_model",
        "RERANK_MODEL": "rerank_model",
        "RERANK_USE_API": "rerank_use_api",
        "RERANK_FALLBACK_LEXICAL": "rerank_fallback_lexical",
        "VLLM_URL": "vllm_url",
        "NATS_ENABLED": "nats_enabled",
        "NATS_JETSTREAM_ENABLED": "nats_jetstream_enabled",
        "NATS_STREAM": "nats_js_stream",
        "NATS_JS_CONSUMER": "nats_js_consumer",
        "NATS_DLQ_SUBJECT": "nats_js_dlq_subject",
    }
    bool_fields = {
        "nats_enabled",
        "nats_jetstream_enabled",
        "rerank_use_api",
        "rerank_fallback_lexical",
        "retrieval_cache_enabled",
        "answer_cache_enabled",
    }
    int_fields = {
        "http_port",
        "db_port",
        "redis_port",
        "redis_db",
        "chunk_size",
        "chunk_overlap",
        "embedding_dim",
        "nats_js_max_age_hours",
        "nats_js_max_bytes",
        "nats_js_replicas",
        "nats_js_max_deliver",
        "nats_js_ack_wait_seconds",
        "nats_js_fetch_batch",
    }
    float_fields = {"nats_js_fetch_timeout_seconds"}
    for env_key, field in mapping.items():
        if v := os.getenv(env_key):
            if field in bool_fields:
                data[field] = v.lower() in ("1", "true", "yes")
            elif field in int_fields:
                data[field] = int(v)
            elif field in float_fields:
                data[field] = float(v)
            else:
                data[field] = v
    return Settings(**data)


@lru_cache
def get_settings() -> Settings:
    path = os.getenv("CONFIG_PATH", "config/config.dev.yaml")
    cfg = Settings.from_yaml(path)
    return _overlay_env(cfg)
