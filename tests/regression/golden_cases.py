#!/usr/bin/env python3
"""Golden cases regression test for GrayDraft retrieval (V1-22).

Runs a set of predefined query/kb_type pairs against the retrieval API
and verifies recall@K, MRR, and candidate stats.
"""
from __future__ import annotations

import json
import math
import os
import statistics
import sys
import time
import urllib.request
from dataclasses import dataclass
from typing import Any

CORE_URL = os.environ.get("CORE_URL", "http://localhost:18080")
RETRIEVAL_PATH = "/api/v1/retrieval/search"
DIAGNOSE_PATH = "/api/v1/retrieval/diagnose"


@dataclass
class GoldenCase:
    query: str
    kb_type: str
    expected_doc_ids: list[str]  # doc IDs that should appear in top 10
    min_recall: float = 0.5     # minimum recall@10 fraction of expected docs found
    description: str = ""
    hard_negative_doc_ids: list[str] | None = None
    scenario: str = ""
    no_answer: bool = False


@dataclass
class CaseResult:
    case: GoldenCase
    passed: bool
    recall: float
    mrr: float
    ndcg: float
    false_positive: bool
    total: int
    strategy: str
    candidate_stats: dict | None
    latency_ms: float
    error: str = ""


def load_cases() -> list[GoldenCase]:
    fixture_path = os.environ.get(
        "GOLDEN_FIXTURE",
        os.path.join(os.path.dirname(__file__), "fixture.json"),
    )
    with open(fixture_path, encoding="utf-8") as fh:
        fixture = json.load(fh)
    kb_type = fixture["kb_type"]
    cases = []
    for row in fixture["cases"]:
        no_answer = row.get("scenario") == "无答案"
        cases.append(GoldenCase(
            query=row["query"],
            kb_type=row.get("kb_type", kb_type),
            expected_doc_ids=row.get("expected_doc_ids", []),
            min_recall=float(row.get("min_recall", 0.5)),
            description=row.get("id", ""),
            hard_negative_doc_ids=row.get("hard_negative_doc_ids"),
            scenario=row.get("scenario", ""),
            no_answer=no_answer,
        ))
    return cases


GOLDEN_CASES = load_cases()


def run_retrieval(query: str, kb_type: str, top_k: int = 10) -> dict[str, Any]:
    return post_json(RETRIEVAL_PATH, {"query": query, "kb_type": kb_type, "top_k": top_k})


def run_diagnose(query: str, kb_type: str, top_k: int = 10) -> dict[str, Any]:
    return post_json(DIAGNOSE_PATH, {"query": query, "kb_type": kb_type, "top_k": top_k})


def post_json(path: str, payload: dict[str, Any]) -> dict[str, Any]:
    request = urllib.request.Request(
        f"{CORE_URL}{path}",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        return json.load(response)


def compute_recall(results: list[dict], expected_ids: list[str]) -> float:
    if not expected_ids:
        raise ValueError("case has no expected_doc_ids; refusing to score recall")
    found = {c.get("doc_id", "") for c in results}
    hits = sum(1 for eid in expected_ids if eid in found)
    return hits / len(expected_ids)


def compute_mrr(results: list[dict], expected_ids: list[str]) -> float:
    if not expected_ids:
        raise ValueError("case has no expected_doc_ids; refusing to score MRR")
    for rank, c in enumerate(results, start=1):
        if c.get("doc_id", "") in expected_ids:
            return 1.0 / rank
    return 0.0


def compute_ndcg(results: list[dict], expected_ids: list[str], k: int = 10) -> float:
    if not expected_ids:
        raise ValueError("case has no expected_doc_ids; refusing to score nDCG")
    relevant = set(expected_ids)
    dcg = sum(
        1.0 / math.log2(rank + 1)
        for rank, chunk in enumerate(results[:k], start=1)
        if chunk.get("doc_id") in relevant
    )
    ideal_hits = min(len(relevant), k)
    idcg = sum(1.0 / math.log2(rank + 1) for rank in range(1, ideal_hits + 1))
    return dcg / idcg if idcg else 0.0


def run_case(case: GoldenCase) -> CaseResult:
    start = time.monotonic()
    try:
        data = run_retrieval(case.query, case.kb_type)
        lat = (time.monotonic() - start) * 1000
        chunks = data.get("chunks", [])
        total = data.get("total", 0)
        strategy = data.get("strategy", "")
        stats = data.get("candidate_stats")
        if case.no_answer:
            false_positive = len(chunks) > 0
            return CaseResult(
                case=case, passed=not false_positive, recall=0, mrr=0, ndcg=0,
                false_positive=false_positive, total=total, strategy=strategy,
                candidate_stats=stats, latency_ms=lat,
            )
        if not case.expected_doc_ids:
            raise ValueError("missing expected_doc_ids (use an explicit skip outside quality scoring)")
        recall = compute_recall(chunks, case.expected_doc_ids)
        mrr = compute_mrr(chunks, case.expected_doc_ids)
        ndcg = compute_ndcg(chunks, case.expected_doc_ids)
        passed = recall >= case.min_recall
        return CaseResult(
            case=case, passed=passed, recall=recall, mrr=mrr, ndcg=ndcg,
            false_positive=False,
            total=total, strategy=strategy, candidate_stats=stats,
            latency_ms=lat,
        )
    except Exception as exc:
        lat = (time.monotonic() - start) * 1000
        return CaseResult(case=case, passed=False, recall=0, mrr=0, ndcg=0,
                          false_positive=False,
                          total=0, strategy="", candidate_stats=None,
                          latency_ms=lat, error=str(exc))


def main() -> int:
    print(f"GrayDraft Golden Cases Regression (V1-22)")
    print(f"Core: {CORE_URL}")
    print(f"Cases: {len(GOLDEN_CASES)}")
    print("-" * 60)

    results: list[CaseResult] = []
    for case in GOLDEN_CASES:
        r = run_case(case)
        results.append(r)
        status = "✅ PASS" if r.passed else "❌ FAIL"
        if r.error:
            status = f"⚠️  ERROR: {r.error}"
        print(f"[{status}] {case.description or case.query[:60]}")
        print(f"  recall={r.recall:.2f}  mrr={r.mrr:.2f}  ndcg={r.ndcg:.2f}  total={r.total}  "
              f"latency={r.latency_ms:.0f}ms  strategy={r.strategy}")
        if r.candidate_stats:
            print(f"  candidates: {json.dumps(r.candidate_stats)}")

    passed = sum(1 for r in results if r.passed)
    failed = sum(1 for r in results if not r.passed and not r.error)
    errors = sum(1 for r in results if r.error)

    print("-" * 60)
    print(f"Total: {len(results)} | Passed: {passed} | Failed: {failed} | Errors: {errors}")

    if errors > 0:
        print("\n⚠️  Some cases errored — check Core connectivity.")
    if failed > 0:
        print("\n❌ Some golden cases failed recall thresholds.")
        return 1
    if errors == 0 and failed == 0:
        print("\n✅ All golden cases passed.")

    # Also validate candidate stats are present (V1-21 check)
    stats_present = sum(1 for r in results if r.candidate_stats is not None)
    print(f"Candidate stats present: {stats_present}/{len(results)}")
    if stats_present < len(results):
        print("⚠️  Some queries missing candidate_stats — check V1-21 deployment.")

    latencies = sorted(r.latency_ms for r in results if not r.error)
    quality = [r for r in results if not r.error and not r.case.no_answer]
    no_answer = [r for r in results if not r.error and r.case.no_answer]
    if latencies:
        def percentile(values: list[float], p: float) -> float:
            index = min(len(values) - 1, math.ceil(len(values) * p) - 1)
            return values[index]
        print(f"Latency: p50={statistics.median(latencies):.0f}ms p95={percentile(latencies, 0.95):.0f}ms")
    if quality:
        print(f"Quality: Recall@10={statistics.mean(r.recall for r in quality):.3f} "
              f"MRR@10={statistics.mean(r.mrr for r in quality):.3f} "
              f"nDCG@10={statistics.mean(r.ndcg for r in quality):.3f}")
    if no_answer:
        rate = sum(r.false_positive for r in no_answer) / len(no_answer)
        print(f"No-answer false-positive rate: {rate:.3f}")
    tiers: dict[str, int] = {}
    rerank = {"calls": 0, "skipped": 0, "failed": 0}
    for r in results:
        suffix = "_wf_soft" if "_wf_soft" in r.strategy else ("strict" if "sem4d" in r.strategy else "none")
        tiers[suffix] = tiers.get(suffix, 0) + 1
        stats = r.candidate_stats or {}
        rerank["calls"] += int(stats.get("rerank_calls", 0))
        rerank["skipped"] += int(stats.get("rerank_skipped", 0))
        rerank["failed"] += int(stats.get("rerank_failed", 0))
    print(f"Waterfall tiers: {json.dumps(tiers, ensure_ascii=False, sort_keys=True)}")
    print(f"Rerank: {json.dumps(rerank, sort_keys=True)}")

    return 0 if failed == 0 and errors == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
