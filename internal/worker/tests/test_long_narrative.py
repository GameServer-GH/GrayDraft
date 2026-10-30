from worker.pipeline.long_narrative import (
    collapse_spam_runs,
    prepare_for_ingest,
    should_auto_split,
    split_long_narrative,
)
from worker.quality_gate.gate import check_content


def test_collapse_spam_runs_allows_gate():
    spammy = "Abstract" + ("†" * 20) + "\n" + ("正文内容足够通过门禁。" * 8)
    cleaned = collapse_spam_runs(spammy)
    assert "†††††††††" not in cleaned
    # Gate collapses runs internally so long-narrative ingest no longer needs manual folding.
    assert check_content(spammy).passed
    assert check_content(cleaned).passed


def test_should_auto_split_long_headed_text():
    body = "# A\n" + ("段落一。" * 200) + "\n## B\n" + ("段落二。" * 200)
    assert should_auto_split(body, max_length=50000, soft_split=1000)


def test_split_long_narrative_shares_doc_group():
    body = "# Intro\n" + ("alpha " * 400) + "\n## Method\n" + ("beta " * 400)
    parts = split_long_narrative(body, title="Paper X", max_part_chars=800)
    assert len(parts) >= 2
    groups = {p.doc_group for p in parts}
    assert len(groups) == 1
    assert all(p.parent_title == "Paper X" for p in parts)
    assert parts[0].segment_index == 1
    assert parts[-1].segment_total == len(parts)


def test_prepare_for_ingest_single_short():
    parts = prepare_for_ingest("短说明足够一次入库。参数 p_ref 默认 0。", title="Tool", max_length=50000)
    assert len(parts) == 1
    assert parts[0].doc_group.startswith("dg_")
