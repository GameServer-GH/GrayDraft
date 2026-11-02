import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import retest_after_upgrade as retest


class Response:
    def __init__(self, events):
        self.lines = [f"data: {json.dumps(e)}\n".encode() for e in events]

    def __enter__(self):
        return iter(self.lines)

    def __exit__(self, *args):
        return False


class RetestRecordTests(unittest.TestCase):
    def test_stream_error_is_preserved_with_partial_answer(self):
        events = [
            {"type": "answer.delta", "message": "包含答案关键词的半成品"},
            {"type": "error", "message": "deadline exceeded"},
        ]
        with patch.object(retest.urllib.request, "urlopen", return_value=Response(events)):
            answer, diagnostics = retest.ask("test-workspace", "question")
        self.assertEqual(answer, "包含答案关键词的半成品")
        self.assertEqual(diagnostics["stream_error"], "deadline exceeded")
        self.assertIsNotNone(diagnostics["first_content_ms"])
        self.assertEqual(diagnostics["events"], [events[1]])

    def test_refusal_containing_expected_names_is_not_a_hit(self):
        case = {"expect": "answer", "keys": ["FlashAttention", "BF16"]}
        self.assertEqual(retest.grade(case, "资料没有 FlashAttention 和 BF16 的依据。"), "未命中")

    def test_eof_with_answer_but_without_terminal_event_is_an_error(self):
        with patch.object(retest.urllib.request, "urlopen", return_value=Response([
            {"type": "answer.delta", "message": "看似完整的答案。"},
        ])):
            answer, diagnostics = retest.ask("test-workspace", "question")
        self.assertEqual(answer, "看似完整的答案。")
        self.assertFalse(diagnostics["terminal_seen"])
        self.assertEqual(diagnostics["stream_error"], "stream ended without completion event")

    def test_code_fingerprints_include_uncommitted_overview_source(self):
        fingerprints = retest.source_fingerprints()
        self.assertIn("internal/core/service/agent/answer_overview.go", fingerprints)
        self.assertTrue(all(len(v) == 64 for v in fingerprints.values()))

    def test_retry_keeps_question_and_first_attempt(self):
        case = {"id": "fact", "expect": "answer", "q": "原问题", "keys": ["9606"]}
        diag = {"terminal_seen": True, "stream_error": None}
        with patch.object(retest, "ask", side_effect=[("没有找到相关依据", diag), ("9606", diag)]) as ask:
            row = retest.run_case("workspace", case, 2)
        self.assertEqual(row["first_status"], "未命中")
        self.assertEqual(row["status"], "命中")
        self.assertEqual(len(row["attempts"]), 2)
        self.assertEqual(ask.call_args_list[0], ask.call_args_list[1])

    def test_partial_answer_keywords_do_not_override_stream_failure(self):
        case = {"id": "fact", "expect": "answer", "q": "原问题", "keys": ["9606"]}
        self.assertEqual(retest.assessment(case, "9606", {"stream_error": "length"}), "错误")
        self.assertEqual(retest.assessment(case, "9606", {"terminal_seen": False}), "错误")

    def test_retry_never_exceeds_two_retries(self):
        case = {"id": "fact", "expect": "answer", "q": "原问题", "keys": ["9606"]}
        with patch.object(retest, "ask", return_value=("没有找到相关依据", {"terminal_seen": True})) as ask:
            row = retest.run_case("workspace", case, 2)
        self.assertEqual(ask.call_count, 3)
        self.assertEqual(len(row["attempts"]), 3)

    def test_unknown_case_is_rejected_before_network(self):
        with patch.object(retest, "open_workspace") as open_workspace:
            with self.assertRaises(SystemExit) as raised:
                retest.main(["--only", "nonexistent-case"])
        self.assertEqual(raised.exception.code, 2)
        open_workspace.assert_not_called()

    def test_offline_replay_does_not_call_api_or_overwrite_source(self):
        suite = {"suite_id": "test_v1", "grading_version": "keyword_v1", "cases": {
            "workspace": [{"id": "fact", "q": "原问题", "expect": "answer", "keys": ["9606"]}],
        }}
        record = {"meta": {"suite_snapshot": suite}, "results": [{
            "workspace": "workspace", "id": "fact", "question": "原问题", "expect": "answer",
            "answer": "9606", "status": "未命中", "diagnostics": {"terminal_seen": True},
        }]}
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "original.json"
            source.write_text(json.dumps(record))
            before = source.read_bytes()
            with patch.object(retest, "ask") as ask, patch.object(retest, "write_record") as write:
                code = retest.replay(source, suite)
            self.assertEqual(code, 0)
            self.assertEqual(write.call_args.args[0]["changed_scores"], 1)
            self.assertEqual(write.call_args.args[0]["results"][0]["status"], "命中")
            ask.assert_not_called()
            self.assertEqual(source.read_bytes(), before)

    def test_changed_source_invalidates_run_even_when_answer_hits(self):
        with patch.object(retest, "source_fingerprints", side_effect=[{"a": "old"}, {"a": "new"}]), \
                patch.object(retest, "container_images", return_value={}), \
                patch.object(retest, "open_workspace"), \
                patch.object(retest, "ask", return_value=("9606", {"terminal_seen": True})), \
                patch.object(retest, "write_record") as write:
            code = retest.main(["--only", "scp-name-1"])
        self.assertEqual(code, 2)
        self.assertFalse(write.call_args.args[0]["meta"]["run_valid"])
        self.assertTrue(write.call_args.args[0]["meta"]["source_changed_during_run"])


if __name__ == "__main__":
    unittest.main()
