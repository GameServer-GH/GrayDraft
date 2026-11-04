import asyncio,json
from types import SimpleNamespace
from unittest.mock import AsyncMock
import pytest
from worker.pipeline.stage_consumer import StageConsumer

def fixture():
 repo=SimpleNamespace(update_ugc_task_failed=AsyncMock())
 c=StageConsumer(SimpleNamespace(nats_js_max_deliver=5),None,repo)
 c._publish_dlq=AsyncMock()
 msg=SimpleNamespace(data=json.dumps({'task_id':'11111111-1111-1111-1111-111111111111'}).encode(),ack=AsyncMock(),nak=AsyncMock())
 return c,repo,msg

def test_permanent_failure_visible_before_ack():
 async def run():
  c,r,m=fixture();await c._handle_failure(m,'accepted',ValueError('quality_gate_rejected: spam'),1)
  r.update_ugc_task_failed.assert_awaited_once();c._publish_dlq.assert_awaited_once();m.ack.assert_awaited_once();m.nak.assert_not_awaited()
 asyncio.run(run())

def test_temporary_failure_retries_without_terminal_state():
 async def run():
  c,r,m=fixture();await c._handle_failure(m,'accepted',RuntimeError('network'),1)
  r.update_ugc_task_failed.assert_not_awaited();m.ack.assert_not_awaited();m.nak.assert_awaited_once()
 asyncio.run(run())

def test_dlq_failure_never_acknowledges():
 async def run():
  c,r,m=fixture();c._publish_dlq.side_effect=RuntimeError('broker down')
  with pytest.raises(RuntimeError):await c._handle_failure(m,'accepted',RuntimeError('network'),5)
  m.ack.assert_not_awaited()
 asyncio.run(run())

def test_qa_failure_cannot_revoke_document_readiness():
 async def run():
  c,r,m=fixture();await c._handle_failure(m,'qa',RuntimeError('model'),5)
  r.update_ugc_task_failed.assert_not_awaited();m.ack.assert_awaited_once()
 asyncio.run(run())


def test_expired_delivery_does_not_run_pipeline():
 async def run():
  c,r,m=fixture();handler=AsyncMock()
  with pytest.raises(TimeoutError):await c._execute_stage(handler,json.dumps({'deadline_at':'2000-01-01T00:00:00Z'}).encode())
  handler.assert_not_awaited()
 asyncio.run(run())


def test_completed_delivery_is_not_reprocessed_even_after_deadline():
 async def run():
  c,r,m=fixture();r.is_ugc_task_completed=AsyncMock(return_value=True);handler=AsyncMock()
  await c._execute_stage(handler,json.dumps({'task_id':'11111111-1111-1111-1111-111111111111','deadline_at':'2000-01-01T00:00:00Z'}).encode(),stage='accepted')
  handler.assert_not_awaited()
 asyncio.run(run())
