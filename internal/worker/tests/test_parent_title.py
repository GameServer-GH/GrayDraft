from worker.pipeline.parent_title import summarize_parent_title_from_child_titles


def test_summarize_stem_separator() -> None:
    got = summarize_parent_title_from_child_titles(
        ["InternVL · 视觉编码", "InternVL · 文本编码", "InternVL · 对齐训练"]
    )
    assert got == "InternVL"


def test_summarize_common_prefix() -> None:
    got = summarize_parent_title_from_child_titles(
        ["书生多模态接口", "书生多模态部署"]
    )
    assert got == "书生多模态"
