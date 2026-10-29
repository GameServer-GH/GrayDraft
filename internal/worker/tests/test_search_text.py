from worker.indexer.search_text import build_search_text, collect_anchor_terms


def test_build_search_text_includes_anchors():
    text = build_search_text(
        doc_title="书生多模态接口速查",
        chunk_content="# 书生多模态接口速查\n\n## Q",
        tags=["书生文档"],
        extensions={"search_anchors": ["InternVL", "InternVL", "多模态模型"]},
    )
    assert "书生多模态接口速查" in text
    assert "InternVL" in text
    assert "InternVL" in text
    assert "[正文：" in text


def test_build_search_text_includes_4d_and_summary():
    text = build_search_text(
        doc_title="S2模型发布",
        chunk_content="| InternVL | 05.22 |",
        extensions={
            "semantic_tags_4d": {
                "domain": "教程资料",
                "entities": ["InternVL"],
                "scene": "版本发布",
                "style": "时间表",
            },
            "global_summary": "S2赛季模型发布时间汇总",
        },
    )
    assert "领域:教程资料" in text
    assert "InternVL" in text
    assert "S2赛季" in text


def test_build_search_text_prefers_segment_summary_and_intent():
    text = build_search_text(
        doc_title="段标题",
        chunk_content="正文",
        extensions={
            "global_summary": "全文摘要",
            "semantic_summary": "本段模型发布",
            "search_intent": "S2版本发布时间表",
        },
    )
    assert "片段摘要" in text
    assert "本段模型发布" in text
    assert "检索意图" in text
    assert "S2版本发布" in text


def test_build_search_text_contextual_head():
    text = build_search_text(
        doc_title="模型部署",
        chunk_content="InternLM队伍1：InternVL+气动主",
        extensions={
            "semantic_tags_4d": {
                "entities": ["InternLM", "InternVL"],
                "scene": "参数建议",
            }
        },
    )
    assert text.startswith("[主体:InternLM、InternVL | 场景:参数建议]")


def test_collect_anchor_terms_dedupes():
    terms = collect_anchor_terms(
        title="书生多模态",
        tags=["书生多模态"],
        extensions={"search_anchors": ["InternVL", "书生文档"], "aliases": ["InternVL"]},
    )
    assert terms.count("书生多模态") == 1
    assert "InternVL" in terms
    assert "书生文档" in terms
