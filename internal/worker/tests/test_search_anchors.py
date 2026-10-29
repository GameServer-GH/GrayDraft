from worker.enrichment.search_anchors import _heuristic_anchors, _parse_anchor_csv


def test_parse_anchor_csv():
    assert _parse_anchor_csv("巴黎, 法国, 欧洲旅游") == ["巴黎", "法国", "欧洲旅游"]


def test_heuristic_anchors_book_title():
    anchors = _heuristic_anchors("浪漫之都避坑指南", "《法国》巴黎卢浮宫注意事项")
    assert any("法国" in a or "巴黎" in a or "浪漫" in a for a in anchors)
