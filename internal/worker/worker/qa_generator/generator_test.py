"""Tests for QA generation with context anchors."""

from worker.qa_generator.generator import generate_qa_pairs


def test_skips_fluff_chunk():
    chunks = [
        "以上就是本期全部内容，欢迎转载。",
        "《InternLM》推理部署：上下文长度 32768，温度 0.7，批大小按显存调整。",
    ]
    pairs = generate_qa_pairs(
        chunks,
        kb_type="gd_abc",
        max_pairs=3,
        title="《InternLM》InternLM 部署说明",
        extensions={
            "subject": "InternLM",
            "parent_title": "InternLM 部署说明",
            "context_enhancement": "InternLM 推理部署说明",
        },
    )
    assert len(pairs) >= 1
    for p in pairs:
        assert "以上就是" not in p.question
        assert "以上就是" not in (p.entities[0] if p.entities else "")


def test_question_uses_topic_anchor():
    chunk = "《InternLM》推理部署：上下文长度 32768，温度 0.7，批大小按显存调整，视觉任务改用 InternVL。"
    pairs = generate_qa_pairs(
        chunks=[chunk],
        kb_type="gd_test",
        max_pairs=1,
        title="部署说明",
        extensions={
            "subject": "InternLM",
            "context_enhancement": "InternLM 推理部署",
        },
    )
    assert len(pairs) == 1
    assert "InternLM" in pairs[0].question or "部署" in pairs[0].question


def test_no_please_introduce_fluff():
    chunks = ["以上就是全部教程内容的总结。"]
    pairs = generate_qa_pairs(chunks, kb_type="tutorial", max_pairs=2)
    assert pairs == []
