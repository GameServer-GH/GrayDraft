from worker.rerank.scorer import rerank
from worker.schemas import Candidate


def test_rerank_orders_by_relevance():
    candidates = [
        Candidate(id="1", text="无关内容关于天气和降雨"),
        Candidate(id="2", text="混合检索根据查询类型动态调整向量与全文权重"),
    ]
    results = rerank("混合检索", candidates, top_n=2, level="l1")
    assert results[0].id == "2"
    assert results[0].score > results[1].score
