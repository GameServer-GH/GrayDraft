import pytest

from worker.cache_purge import CachePurger
from worker.config import Settings


class FakeRedis:
    def __init__(self) -> None:
        self.store = {
            "knex:sr:idx:kb1:v1": ["search-key-1", "search-key-2"],
            "knex:sr:search-key-1": "a",
            "knex:sr:search-key-2": "b",
            "knex:ans:idxkb:kb1": {"answer-key-1"},
            "knex:ans:answer-key-1": "c",
        }

    async def scan(self, cursor=0, match=None, count=64):  # noqa: ANN001, ARG002
        keys = [k for k in self.store if match is None or k.startswith(match.rstrip("*"))]
        return 0, keys

    async def lrange(self, key, start, end):  # noqa: ANN001, ARG002
        return list(self.store.get(key, []))

    async def smembers(self, key):  # noqa: ANN001
        return set(self.store.get(key, set()))

    async def delete(self, *keys):  # noqa: ANN001
        removed = 0
        for key in keys:
            if key in self.store:
                removed += 1
                del self.store[key]
        return removed


@pytest.mark.asyncio
async def test_cache_purger_removes_retrieval_and_answer_keys():
    purger = CachePurger(Settings())
    fake = FakeRedis()
    purger._client = fake
    purger._available = True

    await purger.purge_kb_type("kb1")

    assert "knex:sr:idx:kb1:v1" not in fake.store
    assert "knex:sr:search-key-1" not in fake.store
    assert "knex:sr:search-key-2" not in fake.store
    assert "knex:ans:idxkb:kb1" not in fake.store
    assert "knex:ans:answer-key-1" not in fake.store
