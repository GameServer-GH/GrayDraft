"""Lightweight chat completion via Model Gateway (stream aggregated)."""

from __future__ import annotations

import json
from typing import Any

import httpx

from worker.config import Settings


async def chat_complete(settings: Settings, *, system: str, user: str, max_tokens: int = 256) -> str:
    if not settings.model_gateway_url:
        raise RuntimeError("model_gateway_url not configured")
    url = settings.model_gateway_url.rstrip("/") + "/v1/chat/completions"
    model = settings.anchor_llm_model or "auto"
    payload: dict[str, Any] = {
        "model": model,
        "stream": True,
        "max_tokens": max_tokens,
        "temperature": 0.2,
        "messages": [
            {"role": "system", "content": system},
            {"role": "user", "content": user},
        ],
    }
    parts: list[str] = []
    async with httpx.AsyncClient(timeout=60.0) as client:
        async with client.stream("POST", url, json=payload) as resp:
            resp.raise_for_status()
            async for line in resp.aiter_lines():
                if not line.startswith("data:"):
                    continue
                raw = line[5:].strip()
                if not raw or raw == "[DONE]":
                    continue
                try:
                    data = json.loads(raw)
                except json.JSONDecodeError:
                    continue
                for choice in data.get("choices") or []:
                    delta = choice.get("delta") or {}
                    content = delta.get("content")
                    if content:
                        parts.append(content)
    return "".join(parts).strip()
