#!/usr/bin/env python3
"""Repeated no-cache, no-retry measurements; report all samples, not just winners."""
import argparse
import hashlib
import json
import statistics
import subprocess
import sys
import time
from pathlib import Path

import retest_after_upgrade as retest


def read_sample(path, case_id):
    data = json.loads(Path(path).read_text(encoding="utf-8"))
    if not data["meta"].get("run_valid"):
        raise ValueError(f"invalid or unverifiable run: {path}")
    case = next(c for cases in retest.CASES.values() for c in cases if c["id"] == case_id)
    rows = [r for r in data["results"] if r["id"] == case_id]
    if len(rows) != 1 or rows[0]["question"] != case["q"]:
        raise ValueError(f"case or question mismatch: {path}")
    row = rows[0]
    if len(row.get("attempts", [])) != 1:
        raise ValueError(f"benchmark requires exactly one client attempt: {path}")
    return {"record": str(Path(path).resolve()),
            "sha256": hashlib.sha256(Path(path).read_bytes()).hexdigest(),
            "elapsed_ms": row["diagnostics"].get("elapsed_ms"),
            "first_content_ms": row["diagnostics"].get("first_content_ms"),
            "passed": row["status"] == "命中" and not row["diagnostics"].get("stream_error")
                      and row["diagnostics"].get("terminal_seen") is True,
            "status": row["status"]}


def summarize(samples):
    values = [s["elapsed_ms"] for s in samples if s["elapsed_ms"] is not None]
    return {"samples": len(samples), "passed": sum(s["passed"] for s in samples),
            "failures": sum(not s["passed"] for s in samples),
            "missing_latency": len(samples) - len(values),
            "mean_ms": round(statistics.mean(values)) if values else None,
            "median_ms": round(statistics.median(values)) if values else None,
            "observed_max_ms": max(values) if values else None}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", type=Path, nargs="+", required=True)
    parser.add_argument("--case", default="paper-theme")
    parser.add_argument("--interval-seconds", type=int, choices=range(0, 61), default=0,
                        help="pause between candidate requests; recorded, not applied to historical baseline")
    parser.add_argument("--samples", type=int, choices=range(1, 31), default=5)
    args = parser.parse_args()
    try:
        baseline = [read_sample(path, args.case) for path in args.baseline]
    except (ValueError, KeyError, StopIteration) as exc:
        parser.error(str(exc))
    candidate = []
    runner = Path(retest.__file__).resolve()
    for i in range(args.samples):
        if i and args.interval_seconds:
            time.sleep(args.interval_seconds)
        print(f"candidate {i+1}/{args.samples}", flush=True)
        run = subprocess.run([sys.executable, str(runner), "--only", args.case], text=True, capture_output=True)
        print(run.stdout, end="", flush=True)
        if run.stderr:
            print(run.stderr, file=sys.stderr, end="", flush=True)
        paths = [line[6:] for line in run.stdout.splitlines() if line.startswith("wrote ")]
        if run.returncode not in (0, 1) or len(paths) != 1:
            raise RuntimeError("candidate run invalid; benchmark aborted without dropping samples")
        candidate.append(read_sample(Path(paths[0]), args.case))
    payload = {"meta": {"case": args.case, "protocol": "skip_cache, same question, no client retry",
                        "note": "Sequential small-sample comparison; no population P95 or causal guarantee.",
                        "interval_seconds": args.interval_seconds,
                        "benchmark_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest()},
               "baseline": {"summary": summarize(baseline), "samples": baseline},
               "candidate": {"summary": summarize(candidate), "samples": candidate}}
    retest.write_record(payload, "tail_benchmark")
    print(json.dumps({"baseline": payload["baseline"]["summary"],
                      "candidate": payload["candidate"]["summary"]}, ensure_ascii=False))
    return int(any(not s["passed"] for s in candidate))


if __name__ == "__main__":
    sys.exit(main())
