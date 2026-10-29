from worker.indexer.chunk_semantic import (
    chunk_metadata_payload,
    derive_chunk_semantic_tags_4d,
    extensions_for_chunk,
)


def test_derive_chunk_semantic_tags_4d_heading_entity():
    ext = {
        "semantic_tags_4d": {
            "domain": "教程资料",
            "entities": ["模型"],
            "scene": "版本发布",
            "style": "时间表",
        }
    }
    chunk = "## InternVL接口\n\n| 05.22 |"
    out = derive_chunk_semantic_tags_4d(ext, chunk)
    assert "InternVL接口" in (out.get("entities") or [])


def test_chunk_metadata_payload():
    meta = chunk_metadata_payload(
        {
            "semantic_tags_4d": {"domain": "教程资料", "entities": ["A"]},
            "semantic_summary": "片段",
            "search_intent": "意图",
        }
    )
    assert meta["semantic_tags_4d"]["domain"] == "教程资料"
    assert meta["semantic_summary"] == "片段"


def test_extensions_for_chunk_merges():
    ext = {"semantic_tags_4d": {"domain": "教程资料", "entities": []}}
    merged = extensions_for_chunk(ext, "## 段落\n正文")
    assert merged["semantic_tags_4d"]["domain"] == "教程资料"
