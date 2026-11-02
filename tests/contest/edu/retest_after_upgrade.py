#!/usr/bin/env python3
"""Contest dual-workspace retest after S0/S1/P2 upgrades.

Usage (stack up on :8787 BFF):
  python3 tests/contest/edu/retest_after_upgrade.py

Runs a frozen regression suite (no query rewrite), with optional bounded retries.
Saves full answers and stream
diagnostics under runs/; /tmp/graydraft-contest/ is only a scratch copy.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

BFF = os.environ.get("GRAYDRAFT_BFF", "http://localhost:8787")
OUT = Path(os.environ.get("GRAYDRAFT_CONTEST_OUT", "/tmp/graydraft-contest"))
OUT.mkdir(parents=True, exist_ok=True)
# Durable copies. Each run gets its own file; nothing here is overwritten.
RUNS = Path(__file__).resolve().parent / "runs"

SUITE_PATH = Path(__file__).with_name("upgrade_regression_v1.json")
SUITE = json.loads(SUITE_PATH.read_text(encoding="utf-8"))
CASES = SUITE["cases"]



def post_json(path: str, body: dict, headers: dict | None = None, timeout: int = 180) -> dict | str:
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(
        BFF + path,
        data=data,
        headers={"Content-Type": "application/json", **(headers or {})},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", "replace")
            ctype = resp.headers.get("Content-Type", "")
            if "json" in ctype:
                return json.loads(raw)
            return raw
    except urllib.error.HTTPError as e:
        body_txt = e.read().decode("utf-8", "replace")
        raise RuntimeError(f"HTTP {e.code} {path}: {body_txt[:500]}") from e


def open_workspace(key: str) -> str:
    post_json("/api/OpenWorkspace", {"key": key})
    return key


def ask(key: str, query: str) -> tuple[str, dict]:
    # BFF maps stream to agent run; collect text chunks from SSE-ish or JSON stream.
    data = json.dumps(
        {
            "query": query,
            "chat_mode": "hybrid",
            "top_k": 5,
            "skip_cache": True,
            "skip_answer_cache": True,
        }
    ).encode("utf-8")
    req = urllib.request.Request(
        BFF + "/api/knowledge/chat/stream",
        data=data,
        headers={
            "Content-Type": "application/json",
            "Accept": "text/event-stream",
            "X-Graydraft-Workspace-Key": key,
        },
        method="POST",
    )
    chunks: list[str] = []
    err = ""
    events: list[dict] = []
    started = time.monotonic()
    first_content_ms = None
    terminal_seen = False
    try:
        with urllib.request.urlopen(req, timeout=300) as resp:
            for line in resp:
                s = line.decode("utf-8", "replace").strip()
                if not s.startswith("data:"):
                    continue
                payload = s[5:].strip()
                if payload == "[DONE]":
                    terminal_seen = True
                    continue
                if not payload:
                    continue
                try:
                    ev = json.loads(payload)
                except json.JSONDecodeError:
                    chunks.append(payload)
                    continue
                typ = ev.get("type") or ev.get("event") or ""
                if typ in ("done", "run.completed"):
                    terminal_seen = True
                msg = ev.get("content") or ev.get("message") or ev.get("delta") or ev.get("text") or ""
                if typ and typ not in ("chunk", "answer.delta", "delta", "llm.reasoning.delta"):
                    events.append(ev)
                if typ in ("chunk", "answer.delta", "delta") and msg:
                    if first_content_ms is None:
                        first_content_ms = round((time.monotonic() - started) * 1000)
                    chunks.append(str(msg))
                elif typ in ("done", "run.completed") and msg and not chunks:
                    chunks.append(str(msg))
                elif typ in ("error", "run.failed", "run.expired", "run.cancelled"):
                    err = str(msg or ev)
                elif not typ and msg:
                    chunks.append(str(msg))
    except Exception as exc:
        err = str(exc)
    if not terminal_seen and not err:
        err = "stream ended without completion event"
    text = "".join(chunks).strip()
    if err and not text:
        text = f"[error] {err}"
    return text, {
        "elapsed_ms": round((time.monotonic() - started) * 1000),
        "first_content_ms": first_content_ms,
        "stream_error": err or None,
        "terminal_seen": terminal_seen,
        "events": events,
    }


def grade(case: dict, answer: str) -> str:
    a = answer or ""
    low = a.lower().replace("*", "")
    if a.startswith("[error]"):
        return "错误"
    # Denial that quotes the question still contains the expected tokens.
    lead = low[:180]
    if case["expect"] == "answer" and any(
        s in lead for s in ("本工作区资料中没有", "本工作区资料没有", "没有找到相关依据", "没有提到", "资料没有", "不足以作答")
    ):
        return "未命中"
    if case["expect"] == "reject":
        refuse_signals = ["没有", "未找到", "未写明", "无法回答", "资料中没有", "本工作区"]
        if any(s in a for s in refuse_signals) or "补充" in a:
            if any(k.lower() in low for k in ("100 mib", "100mi", "vit-22b", "6b")) and case["id"].endswith("isolation"):
                return "错误"  # leaked
            return "正确拒答" if any(s in a for s in ("没有", "未找到", "未写明", "无法回答", "资料中没有")) else "澄清拒答"
        return "错误"
    keys = case.get("keys") or []
    hits = sum(1 for k in keys if k.lower() in low)
    if keys and hits == len(keys):
        return "命中"
    if keys and hits > 0:
        return "部分"
    if len(a) > 40 and "补充" not in a and "没有" not in a:
        return "部分"
    if "补充" in a or "请" in a[:20]:
        return "未命中"
    return "未命中"


def load_suite(path: Path) -> dict:
    suite = json.loads(path.read_text(encoding="utf-8"))
    if suite.get("grading_version") != "keyword_v1":
        raise ValueError("unsupported grading_version")
    if not suite.get("suite_id") or not isinstance(suite.get("cases"), dict) or not suite["cases"]:
        raise ValueError("suite_id and cases are required")
    ids = set()
    for cases in suite["cases"].values():
        for case in cases:
            if case["id"] in ids or case["expect"] not in ("answer", "reject") or not case["q"].strip():
                raise ValueError("duplicate ID or invalid case")
            ids.add(case["id"])
    if not ids:
        raise ValueError("suite contains no cases")
    return suite


def assessment(case: dict, answer: str, diagnostics: dict) -> str:
    if (diagnostics.get("stream_error") or diagnostics.get("exception")
            or diagnostics.get("terminal_seen") is False or not answer.strip()):
        return "错误"
    return grade(case, answer)


def run_attempt(ws: str, case: dict, number: int) -> dict:
    started = time.monotonic()
    try:
        answer, diagnostics = ask(ws, case["q"])
    except Exception as exc:
        answer = f"[error] {exc}"
        diagnostics = {"elapsed_ms": round((time.monotonic() - started) * 1000), "exception": str(exc)}
    return {"number": number, "question": case["q"], "answer": answer,
            "diagnostics": diagnostics, "status": assessment(case, answer, diagnostics)}


def run_case(ws: str, case: dict, max_retries: int) -> dict:
    attempts = []
    for i in range(max_retries + 1):
        attempt = run_attempt(ws, case, i + 1)
        attempts.append(attempt)
        if attempt["status"] in ("命中", "正确拒答"):
            break
    final = attempts[-1]
    return {"workspace": ws, "id": case["id"], "expect": case["expect"],
            "question": case["q"], "status": final["status"],
            "first_status": attempts[0]["status"], "answer": final["answer"],
            "diagnostics": final["diagnostics"], "attempts": attempts}


def container_images() -> dict[str, str]:
    # Local observations only. A remote BFF is not identified by this Docker host.
    if BFF not in ("http://localhost:8787", "http://127.0.0.1:8787"):
        return {}
    names = ["graydraft-core-dev", "graydraft-gateway-dev", "graydraft-graydraft-dev"]
    try:
        values = subprocess.check_output(
            ["docker", "inspect", "--format", "{{.Image}}", *names],
            text=True, stderr=subprocess.DEVNULL, timeout=5,
        ).splitlines()
        return dict(zip(names, values)) if len(values) == len(names) else {}
    except (OSError, subprocess.SubprocessError):
        return {}


def write_record(payload: dict, label: str) -> Path:
    RUNS.mkdir(parents=True, exist_ok=True)
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
    path = RUNS / f"{stamp}_{label}.json"
    text = json.dumps(payload, ensure_ascii=False, indent=2)
    # Exclusive creation makes even an accidental filename collision visible.
    with path.open("x", encoding="utf-8") as stream:
        stream.write(text)
    (OUT / path.name).write_text(text, encoding="utf-8")
    print("wrote", path)
    return path


def summarize(results: list[dict]) -> None:
    for ws in sorted({r["workspace"] for r in results}):
        rows = [r for r in results if r["workspace"] == ws]
        answerable = [r for r in rows if r["expect"] == "answer"]
        rejects = [r for r in rows if r["expect"] == "reject"]
        first = sum(r.get("first_status", r["status"]) == "命中" for r in answerable)
        final = sum(r["status"] == "命中" for r in answerable)
        refused = sum(r["status"] == "正确拒答" for r in rejects)
        print(f"{ws}: first={first}/{len(answerable)} final={final}/{len(answerable)} reject={refused}/{len(rejects)}")


def replay(path: Path, suite: dict) -> int:
    original = json.loads(path.read_text(encoding="utf-8"))
    source_valid = original.get("meta", {}).get("run_valid")
    frozen = original.get("meta", {}).get("suite_snapshot")
    if frozen is None:
        expected = original.get("meta", {}).get("cases_sha256")
        actual = hashlib.sha256(json.dumps(suite["cases"], ensure_ascii=False, sort_keys=True).encode()).hexdigest()
        if expected != actual:
            raise ValueError("record has no suite snapshot and does not match the selected suite")
        frozen = suite
    if frozen.get("grading_version") != "keyword_v1":
        raise ValueError("unsupported replay grading version")
    index = {(ws, c["id"]): c for ws, cases in frozen["cases"].items() for c in cases}
    results = []
    for row in original["results"]:
        case = index[(row["workspace"], row["id"])]
        if row.get("question") != case["q"]:
            raise ValueError("record question missing or different from frozen suite")
        rescored = dict(row)
        rescored["previous_status"] = row["status"]
        rescored["status"] = assessment(case, row["answer"], row.get("diagnostics", {}))
        if row.get("attempts"):
            rescored["attempts"] = []
            for attempt in row["attempts"]:
                if attempt.get("question") != case["q"]:
                    raise ValueError("retry changed the question")
                new_attempt = dict(attempt)
                new_attempt["status"] = assessment(case, attempt["answer"], attempt.get("diagnostics", {}))
                rescored["attempts"].append(new_attempt)
            rescored["first_status"] = rescored["attempts"][0]["status"]
        results.append(rescored)
    changed = sum(r["status"] != r["previous_status"] for r in results)
    write_record({"meta": {"replay_of": str(path.resolve()),
                          "original_sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                          "source_run_valid": source_valid,
                          "grading_version": "keyword_v1", "suite_snapshot": frozen,
                          "scorer_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest()},
                  "changed_scores": changed, "results": results}, "replay")
    summarize(results)
    print("changed_scores", changed)
    if source_valid is False:
        return 2
    return int(any(r["status"] not in ("命中", "正确拒答") for r in results))


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--suite", type=Path, default=SUITE_PATH)
    parser.add_argument("--only", default=os.environ.get("GRAYDRAFT_ONLY", ""))
    parser.add_argument("--max-retries", type=int, choices=range(3), default=0,
                        help="0..2 client retries; question and all attempts are preserved")
    parser.add_argument("--replay", type=Path, help="rescore saved answers offline; no API calls")
    args = parser.parse_args(argv)
    try:
        suite = load_suite(args.suite)
        if args.replay:
            return replay(args.replay, suite)
    except (OSError, ValueError, KeyError) as exc:
        parser.error(str(exc))
    cases_by_ws = suite["cases"]
    only = {x for x in args.only.split(",") if x}
    known = {c["id"] for cases in cases_by_ws.values() for c in cases}
    if only - known:
        parser.error("unknown case IDs: " + ", ".join(sorted(only - known)))
    started_at = datetime.now(timezone.utc)
    sources_at_start = source_fingerprints()
    suite_at_start = hashlib.sha256(args.suite.read_bytes()).hexdigest()
    images_at_start = container_images()
    results = []
    for ws, cases in cases_by_ws.items():
        selected = [case for case in cases if not only or case["id"] in only]
        if not selected:
            continue
        print("open", ws, flush=True)
        try:
            open_workspace(ws)
        except Exception as exc:
            for case in selected:
                results.append({"workspace": ws, "id": case["id"], "expect": case["expect"],
                                "question": case["q"], "status": "错误", "first_status": "错误",
                                "answer": "", "diagnostics": {"exception": str(exc), "stage": "open_workspace"}})
            continue
        for case in selected:
            print(" ask", case["id"], flush=True)
            row = run_case(ws, case, args.max_retries)
            results.append(row)
            print("  ->", row["status"], row["answer"][:120].replace("\n", " "), flush=True)
    source_changed = sources_at_start != source_fingerprints()
    suite_changed = suite_at_start != hashlib.sha256(args.suite.read_bytes()).hexdigest()
    images_changed = images_at_start != container_images()
    try:
        sha = subprocess.check_output(["git", "rev-parse", "--short", "HEAD"], text=True).strip()
        dirty = bool(subprocess.check_output(["git", "status", "--porcelain"], text=True).strip())
    except (OSError, subprocess.SubprocessError):
        sha, dirty = "", None
    valid = not (source_changed or suite_changed or images_changed)
    write_record({"meta": {
        "started_at": started_at.isoformat(), "completed_at": datetime.now(timezone.utc).isoformat(),
        "git": sha, "git_dirty": dirty, "bff": BFF, "only": sorted(only),
        "protocol": f"skip_cache, no query rewrite, max_client_retries={args.max_retries}",
        "suite": suite["suite_id"], "suite_snapshot": suite,
        "grading_version": suite["grading_version"],
        "suite_sha256": suite_at_start,
        "cases_sha256": hashlib.sha256(json.dumps(cases_by_ws, ensure_ascii=False, sort_keys=True).encode()).hexdigest(),
        "source_sha256": sources_at_start, "source_changed_during_run": source_changed,
        "suite_changed_during_run": suite_changed,
        "local_container_images": images_at_start, "local_images_changed_during_run": images_changed,
        "run_valid": valid,
    }, "results": results}, "gate" if only else "full")
    summarize(results)
    if not valid:
        print("INVALID RUN: source, suite or local runtime images changed", file=sys.stderr)
        return 2
    return int(any(r["status"] not in ("命中", "正确拒答") for r in results))


def source_fingerprints() -> dict[str, str]:
    """Fingerprint the affected pipeline, including uncommitted production files."""
    root = Path(__file__).resolve().parents[3]
    patterns = ["internal/core/service/agent/*.go", "internal/core/service/llmclient/*.go",
                "internal/gateway/service/llm/*.go", "internal/gateway/service/provider/*.go",
                "internal/gateway/handler/chat.go", "internal/graydraft/pkg/nexus/chat_stream.go",
                "internal/core/repository/postgres/group_span.go", "internal/core/repository/postgres/chunk_repo.go",
                "internal/core/repository/nats/agent_queue.go",
                "internal/core/config/config.go", "internal/core/config/config.dev.yaml",
                "internal/core/config/config.prod.yaml", "internal/core/cmd/gateway/main.go",
                "tests/contest/edu/retest_after_upgrade.py"]
    paths = sorted({p for pattern in patterns for p in root.glob(pattern) if not p.name.endswith("_test.go")})
    return {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in paths}


if __name__ == "__main__":
    sys.exit(main())
