#!/usr/bin/env python3
"""Contest dual-workspace retest after S0/S1/P2 upgrades.

Usage (stack up on :8787 BFF):
  python3 tests/contest/edu/retest_after_upgrade.py

Reports first-round results only (no query rewrite). Writes JSON under /tmp/graydraft-contest/.
"""

from __future__ import annotations

import json
import os
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path

BFF = os.environ.get("GRAYDRAFT_BFF", "http://localhost:8787")
OUT = Path(os.environ.get("GRAYDRAFT_CONTEST_OUT", "/tmp/graydraft-contest"))
OUT.mkdir(parents=True, exist_ok=True)

CASES = {
    "contest-scp-skills": [
        {"id": "scp-concept-1", "q": "multiomics 整合哪四层组学", "expect": "answer", "keys": ["TCGA", "UniProt", "STRING", "KEGG"]},
        {"id": "scp-name-1", "q": "species 人类 taxonomy ID 是多少", "expect": "answer", "keys": ["9606"]},
        {"id": "scp-name-2", "q": "Sciverse 密钥请求头名是什么", "expect": "answer", "keys": ["SCP-HUB-API-KEY"]},
        {"id": "scp-step-1", "q": "CrystalX 本地上传先调什么，INS 和 HKL 上限分别多少", "expect": "answer", "keys": ["crystalx_prepare_input_upload", "10", "100"]},
        {"id": "scp-param-1", "q": "gsw_example_dynamic_height 的 p_ref 默认值", "expect": "answer", "keys": ["p_ref", "0"]},
        {"id": "scp-use-1", "q": "query_and_filter_tools 必填参数与用途", "expect": "answer", "keys": ["prompt"]},
        {"id": "scp-reject-1", "q": "InternLM2 预训练 token 精确配比是多少", "expect": "reject", "keys": []},
        {"id": "scp-isolation", "q": "InternVL 视觉模型参数是多少", "expect": "reject", "keys": []},
    ],
    "contest-papers": [
        {"id": "paper-concept-1", "q": "COOL RLHF 处理哪两类问题", "expect": "answer", "keys": ["conflict", "reward"]},
        {"id": "paper-name-1", "q": "InternLM2 开源是否包括 1.8B、7B、20B", "expect": "answer", "keys": ["1.8", "7", "20"]},
        {"id": "paper-name-2", "q": "InternVL 视觉规模与 ViT-22B 的关系", "expect": "answer", "keys": ["6", "ViT-22B"]},
        {"id": "paper-step-1", "q": "上下文从 4k 到 32k，Needle 提到 200k 吗", "expect": "answer", "keys": ["4", "32", "200"]},
        {"id": "paper-25-1", "q": "InternVL2.5 基于 2.0，MMMU 分数与提升", "expect": "answer", "keys": ["70.1", "3.7"]},
        {"id": "paper-25-2", "q": "6B 与 600M 相比训练 token 大约是多少分之一", "expect": "answer", "keys": ["10"]},
        {"id": "paper-theme", "q": "三篇论文各自讲什么", "expect": "answer", "keys": ["InternLM", "InternVL"]},
        {"id": "paper-reject", "q": "2027 财年营收是多少", "expect": "reject", "keys": []},
        {"id": "paper-isolation", "q": "CrystalX HKL 上限是多少", "expect": "reject", "keys": []},
    ],
}


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


def ask(key: str, query: str) -> str:
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
    with urllib.request.urlopen(req, timeout=300) as resp:
        for line in resp:
            s = line.decode("utf-8", "replace").strip()
            if not s.startswith("data:"):
                continue
            payload = s[5:].strip()
            if payload in ("", "[DONE]"):
                continue
            try:
                ev = json.loads(payload)
            except json.JSONDecodeError:
                chunks.append(payload)
                continue
            typ = ev.get("type") or ev.get("event") or ""
            msg = ev.get("content") or ev.get("message") or ev.get("delta") or ev.get("text") or ""
            if typ in ("chunk", "answer.delta", "delta") and msg:
                chunks.append(str(msg))
            elif typ in ("done", "run.completed") and msg and not chunks:
                chunks.append(str(msg))
            elif typ == "error":
                err = str(msg or ev)
            elif not typ and msg:
                chunks.append(str(msg))
    text = "".join(chunks).strip()
    if err and not text:
        return f"[error] {err}"
    return text


def grade(case: dict, answer: str) -> str:
    a = answer or ""
    low = a.lower()
    if a.startswith("[error]"):
        return "错误"
    # Denial that quotes the question still contains the expected tokens.
    lead = a[:180]
    if case["expect"] == "answer" and any(
        s in lead for s in ("本工作区资料中没有", "没有找到相关依据", "没有提到", "不足以作答")
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


def main() -> int:
    only = {x for x in os.environ.get("GRAYDRAFT_ONLY", "").split(",") if x}
    results = []
    for ws, cases in CASES.items():
        print("open", ws)
        open_workspace(ws)
        time.sleep(0.5)
        for case in cases:
            if only and case["id"] not in only:
                continue
            print(" ask", case["id"], flush=True)
            try:
                answer = ask(ws, case["q"])
                status = grade(case, answer)
            except Exception as e:
                answer = f"[error] {e}"
                status = "错误"
            row = {
                "workspace": ws,
                "id": case["id"],
                "expect": case["expect"],
                "status": status,
                "answer": answer[:2000],
            }
            results.append(row)
            print("  ->", status, answer[:120].replace("\n", " "))
            time.sleep(0.3)

    name = "qa_retest_upgrade.json"
    if only:
        name = "qa_gate.json"
    path = OUT / name
    path.write_text(json.dumps(results, ensure_ascii=False, indent=2), encoding="utf-8")
    print("wrote", path)

    def summarize(ws: str) -> None:
        rows = [r for r in results if r["workspace"] == ws]
        answerable = [r for r in rows if r["expect"] == "answer"]
        hit = sum(1 for r in answerable if r["status"] == "命中")
        part = sum(1 for r in answerable if r["status"] == "部分")
        err = sum(1 for r in answerable if r["status"] == "错误")
        miss = sum(1 for r in answerable if r["status"] == "未命中")
        print(f"{ws}: hit={hit}/{len(answerable)} part={part} miss={miss} err={err}")

    summarize("contest-scp-skills")
    summarize("contest-papers")
    return 0


if __name__ == "__main__":
    sys.exit(main())
