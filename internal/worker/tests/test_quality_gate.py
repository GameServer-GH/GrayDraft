from worker.quality_gate.gate import check_content


def test_quality_gate_rejects_short():
    r = check_content("hi", min_length=20)
    assert not r.passed
    assert r.reason == "content_too_short"


def test_quality_gate_accepts_normal():
    text = "GrayDraft Worker 负责文档解析、语义分块与质量门禁，确保入库内容可靠。"
    r = check_content(text, min_length=20)
    assert r.passed
