"""GrayDraft RAGOps — evaluation & auto-healing service (O-19)."""

from __future__ import annotations

import os
from typing import Any

import httpx
from fastapi import FastAPI, HTTPException

app = FastAPI(title="GrayDraft Ops", version="0.2.0")

CORE_URL = os.getenv("CORE_URL", "http://core:18080").rstrip("/")
ADMIN_USER = os.getenv("ADMIN_USER", "admin")
ADMIN_PASSWORD = os.getenv("ADMIN_PASSWORD", "dev_admin_pass")


async def _admin_token() -> str:
    async with httpx.AsyncClient(timeout=30.0) as client:
        r = await client.post(
            f"{CORE_URL}/api/v1/auth/login",
            json={"username": ADMIN_USER, "password": ADMIN_PASSWORD},
        )
        r.raise_for_status()
        data = r.json()
        token = data.get("token")
        if not token:
            raise HTTPException(status_code=502, detail="core login: no token")
        return token


async def _admin_post(path: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
    token = await _admin_token()
    async with httpx.AsyncClient(timeout=120.0) as client:
        r = await client.post(
            f"{CORE_URL}{path}",
            params=params or {},
            headers={"Authorization": f"Bearer {token}"},
        )
        if r.status_code >= 400:
            raise HTTPException(status_code=r.status_code, detail=r.text)
        return r.json()


async def _admin_get(path: str) -> dict[str, Any]:
    token = await _admin_token()
    async with httpx.AsyncClient(timeout=30.0) as client:
        r = await client.get(
            f"{CORE_URL}{path}",
            headers={"Authorization": f"Bearer {token}"},
        )
        if r.status_code >= 400:
            raise HTTPException(status_code=r.status_code, detail=r.text)
        return r.json()


@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok", "service": "graydraft-ops", "core": CORE_URL}


@app.post("/v1/global-index/refresh")
async def refresh_global_index(concurrent: bool = True) -> dict[str, Any]:
    return await _admin_post("/api/v1/admin/global-index/refresh", {"concurrent": str(concurrent).lower()})


@app.get("/v1/global-index/stats")
async def global_index_stats() -> dict[str, Any]:
    return await _admin_get("/api/v1/admin/global-index/stats")


@app.post("/v1/tier/apply")
async def apply_cold_tier(cold_after_days: int = 90) -> dict[str, Any]:
    return await _admin_post("/api/v1/admin/tier/apply", {"cold_after_days": cold_after_days})


@app.get("/v1/tier/stats")
async def tier_stats() -> dict[str, Any]:
    return await _admin_get("/api/v1/admin/tier/stats")


@app.post("/v1/ragops/scan")
async def ragops_scan() -> dict[str, Any]:
    return await _admin_post("/api/v1/admin/ragops/scan")


@app.get("/v1/ragops/jobs")
async def ragops_jobs(status: str = "", limit: int = 50) -> dict[str, Any]:
    params: dict[str, Any] = {"limit": limit}
    if status:
        params["status"] = status
    token = await _admin_token()
    async with httpx.AsyncClient(timeout=30.0) as client:
        r = await client.get(
            f"{CORE_URL}/api/v1/admin/ragops/jobs",
            params=params,
            headers={"Authorization": f"Bearer {token}"},
        )
        if r.status_code >= 400:
            raise HTTPException(status_code=r.status_code, detail=r.text)
        return r.json()


@app.post("/v1/ragops/jobs/{job_id}/run")
async def ragops_run_job(job_id: str) -> dict[str, Any]:
    token = await _admin_token()
    async with httpx.AsyncClient(timeout=300.0) as client:
        r = await client.post(
            f"{CORE_URL}/api/v1/admin/ragops/jobs/{job_id}/run",
            headers={"Authorization": f"Bearer {token}"},
        )
        if r.status_code >= 400:
            raise HTTPException(status_code=r.status_code, detail=r.text)
        return r.json()


@app.post("/v1/heal/run-all")
async def heal_run_all(limit: int = 20) -> dict[str, Any]:
    """Scan + run pending heal jobs (O-19 闭环入口)."""
    scan = await _admin_post("/api/v1/admin/ragops/scan")
    jobs = await ragops_jobs(status="pending", limit=limit)
    ran: list[dict[str, Any]] = []
    for item in jobs.get("items", [])[:limit]:
        job_id = item.get("id")
        if not job_id:
            continue
        ran.append(await ragops_run_job(job_id))
    return {"scan": scan, "ran_count": len(ran), "results": ran}


@app.get("/v1/cost/summary")
async def cost_summary(tenant_id: str = "", hours: int = 24) -> dict[str, Any]:
    params: dict[str, Any] = {"hours": hours}
    if tenant_id:
        params["tenant_id"] = tenant_id
    token = await _admin_token()
    async with httpx.AsyncClient(timeout=30.0) as client:
        r = await client.get(
            f"{CORE_URL}/api/v1/admin/cost/summary",
            params=params,
            headers={"Authorization": f"Bearer {token}"},
        )
        if r.status_code >= 400:
            raise HTTPException(status_code=r.status_code, detail=r.text)
        return r.json()


@app.get("/v1/agent-evals")
async def agent_evals(tenant_id: str = "", limit: int = 20) -> dict[str, Any]:
    params: dict[str, Any] = {"limit": limit}
    if tenant_id:
        params["tenant_id"] = tenant_id
    token = await _admin_token()
    async with httpx.AsyncClient(timeout=30.0) as client:
        r = await client.get(
            f"{CORE_URL}/api/v1/admin/agent-evals",
            params=params,
            headers={"Authorization": f"Bearer {token}"},
        )
        if r.status_code >= 400:
            raise HTTPException(status_code=r.status_code, detail=r.text)
        return r.json()


@app.post("/v1/agent-evals/run")
async def agent_eval_run(body: dict[str, Any] | None = None) -> dict[str, Any]:
    token = await _admin_token()
    async with httpx.AsyncClient(timeout=300.0) as client:
        r = await client.post(
            f"{CORE_URL}/api/v1/admin/agent-evals/run",
            json=body or {},
            headers={"Authorization": f"Bearer {token}"},
        )
        if r.status_code >= 400:
            raise HTTPException(status_code=r.status_code, detail=r.text)
        return r.json()


@app.get("/v1/agent-evals/quality")
async def agent_eval_quality(tenant_id: str = "", days: int = 7) -> dict[str, Any]:
    params: dict[str, Any] = {"days": days}
    if tenant_id:
        params["tenant_id"] = tenant_id
    token = await _admin_token()
    async with httpx.AsyncClient(timeout=30.0) as client:
        r = await client.get(
            f"{CORE_URL}/api/v1/admin/agent-evals/quality",
            params=params,
            headers={"Authorization": f"Bearer {token}"},
        )
        if r.status_code >= 400:
            raise HTTPException(status_code=r.status_code, detail=r.text)
        return r.json()

