import pytest

from worker.config import Settings
from worker.pipeline.taxonomy_align import align_to_label, realign_semantic_tags_4d


@pytest.mark.asyncio
async def test_align_to_label_exact():
    settings = Settings()
    got = await align_to_label("参数建议", ["参数建议", "模型使用"], settings)
    assert got == "参数建议"


@pytest.mark.asyncio
async def test_realign_semantic_tags_domain():
    settings = Settings()
    tags = {"domain": "教程资料", "scene": "队伍配置", "entities": ["卡提"]}
    blueprint = {
        "domain": "学科资料",
        "scenes": [{"label": "行程规划"}],
        "styles": [{"label": "时间表"}],
    }
    out = await realign_semantic_tags_4d(tags, blueprint, settings)
    assert out["domain"] == "学科资料"
