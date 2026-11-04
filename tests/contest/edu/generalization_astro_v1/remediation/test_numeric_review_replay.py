import unittest
from replay_numeric_review import replay,replay_operation

class ReplayTests(unittest.TestCase):
 def operation(self,**updates):
  return {'rule':'same_unit_scale_v2','left_coefficient':'0.7','left_power10':'9','left_unit':'M⊙','right_coefficient':'7','right_scale':'千万','right_unit':'太阳质量','canonical_unit':'太阳质量','relation':'即','approximate':False,'action':'reject','left_magnitude_rational':'700000000','right_magnitude_rational':'70000000','relative_difference_rational':'9/10',**updates}
 def test_reproduces_mismatch(self):
  result=replay_operation(self.operation());self.assertEqual(result['classification'],'confirmed_arithmetic_mismatch_under_recorded_binding');self.assertEqual(result['relative_difference'],'9/10')
 def test_detects_false_refusal(self):
  row=self.operation(right_scale='亿',right_magnitude_rational='700000000',relative_difference_rational='0')
  self.assertEqual(replay_operation(row)['classification'],'possible_false_refusal')
 def test_detects_approximation_false_refusal(self):
  self.assertEqual(replay_operation(self.operation(approximate=True))['classification'],'possible_false_refusal')
 def test_detects_bad_audit_arithmetic(self):
  self.assertEqual(replay_operation(self.operation(left_magnitude_rational='1'))['classification'],'audit_computation_mismatch')
 def test_old_generic_reason_cannot_be_reconstructed(self):
  self.assertEqual(replay({'numeric_consistency_issues':['explicit_same_unit_scale_mismatch']})['status'],'insufficient_audit_data')
 def test_units_remain_unverified(self):
  self.assertEqual(replay_operation(self.operation(right_unit='秒'))['classification'],'possible_false_refusal')
if __name__=='__main__':unittest.main()
