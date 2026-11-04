import unittest

from offline_repair import complete_bound, constrain_count, constrain_keep, controlled_cases, select
from replay_visible_role_cases import digest


class VisibleSelectionTest(unittest.TestCase):
    def setUp(self):
        self.case = next(case for case in controlled_cases() if case['key'] == 'truncated-statistic')

    def test_selection_reconstructs_visible_source(self):
        quote = {'id': '1', 'start': '1', 'end': '1'}
        answer, citations = select(self.case, {'decision': 'insufficient', 'quotes': [quote]})
        visible = self.case['hits'][0]['content']
        self.assertEqual(answer, '当前可见证据不足以确认完整答案。\n' + visible)
        self.assertEqual(citations[0]['sha256'], digest(visible))

    def test_rejects_hidden_and_unknown_ranges(self):
        for quote in [{'id': 1, 'start': 1, 'end': 2}, {'id': 2, 'start': 1, 'end': 1}]:
            with self.subTest(quote=quote), self.assertRaises(ValueError):
                select(self.case, {'decision': 'repair', 'quotes': [quote]})

    def test_comparison_extension_skips_only_isolated_uncertainty(self):
        lines = ['significance greater', 'þ0.03', 'than 5.1σ.']
        self.assertEqual(complete_bound(lines, 1, 2), 3)
        self.assertEqual(complete_bound(lines[:2], 1, 2), 2)
        self.assertEqual(complete_bound(['significance greater', 'Unrelated event', 'than 5.1σ.'], 1, 1), 1)

    def test_keep_cannot_bypass_statistic_ambiguity(self):
        proposal = {'decision': 'keep'}
        review = {'role_checks': [{'status': 'statistic_role_binding_ambiguous'}]}
        self.assertEqual(constrain_keep(proposal, review)['decision'], 'repair')
        self.assertEqual(constrain_keep(proposal, {})['decision'], 'keep')

    def test_available_total_cannot_prove_example_total(self):
        case = {'question': '展示了几个工具的调用方法？',
                'hits': [{'id': 'tools', 'content': '服务器提供 5 个工具。'}]}
        proposal = {'decision': 'repair', 'quotes': [{'id': 1, 'start': 1, 'end': 1}]}
        answer, citations = select(case, proposal)
        decision, _ = constrain_count(case, proposal, answer, citations)
        self.assertEqual(decision['decision'], 'insufficient')
        case['hits'][0]['content'] = '本节共 3 个调用示例。'
        answer, citations = select(case, proposal)
        decision, _ = constrain_count(case, proposal, answer, citations)
        self.assertEqual(decision['decision'], 'repair')


if __name__ == '__main__':
    unittest.main()
