import unittest

import benchmark_tail


class TailBenchmarkTests(unittest.TestCase):
    def test_failures_remain_in_summary(self):
        summary = benchmark_tail.summarize([
            {"elapsed_ms": 1000, "passed": True},
            {"elapsed_ms": 35000, "passed": False},
        ])
        self.assertEqual(summary["samples"], 2)
        self.assertEqual(summary["failures"], 1)
        self.assertEqual(summary["mean_ms"], 18000)
        self.assertEqual(summary["observed_max_ms"], 35000)
        self.assertNotIn("p95_ms", summary)
