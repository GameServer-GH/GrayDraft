"""Unit tests for P2-1 StageMessage / StageConsumer helpers."""

from __future__ import annotations

from worker.pipeline.stage_consumer import STAGE_ACCEPTED, StageMessage


def test_stage_message_to_ugc_force_qa_off():
    raw = {
        "trace_id": "t1",
        "task_id": "11111111-1111-1111-1111-111111111111",
        "stage": STAGE_ACCEPTED,
        "doc_id": "22222222-2222-2222-2222-222222222222",
        "kb_type": "tutorial",
        "title": "hello",
        "content": "world",
        "mime_type": "text/markdown",
        "generate_qa": True,
        "tags": ["a"],
        "extensions": {"k": "v"},
    }
    msg = StageMessage(raw)
    ugc = msg.to_ugc(force_qa=False)
    assert ugc.generate_qa is False
    assert ugc.kb_type == "tutorial"
    assert ugc.title == "hello"
    assert ugc.content == "world"
    assert ugc.tags == ["a"]
    assert ugc.extensions == {"k": "v"}


def test_stage_message_preserves_generate_qa_when_not_forced():
    msg = StageMessage({"generate_qa": True, "kb_type": "paper", "title": "t", "content": "c"})
    assert msg.to_ugc().generate_qa is True
