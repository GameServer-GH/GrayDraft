"""Offline, exact-rational replay of minimal operation audit records. No network/DB writes."""
import argparse,hashlib,json,re
from fractions import Fraction
from pathlib import Path

SCALES={'千':10**3,'万':10**4,'百万':10**6,'千万':10**7,'亿':10**8,'十亿':10**9}
ALIASES={'M_\\odot':'太阳质量','M⊙':'太阳质量','M☉':'太阳质量','kg':'千克','m':'米','s':'秒'}
def unit(value):
 value=value.strip();return ALIASES.get(value,value)
def replay_operation(row):
 result={'scope':'recorded_same_unit_magnitudes_only','recorded_action':row.get('action'),'binding_verified':False}
 if row.get('rule')!='same_unit_scale_v2':return {**result,'classification':'insufficient_audit_data'}
 try:
  lhs_unit,rhs_unit=unit(row['left_unit']),unit(row['right_unit'])
  if lhs_unit!=rhs_unit:
   return {**result,'classification':'possible_false_refusal' if row.get('action')=='reject' else 'unit_binding_requires_review'}
  if row.get('canonical_unit',lhs_unit)!=lhs_unit:return {**result,'classification':'audit_unit_binding_mismatch'}
  if len(row['left_coefficient'])>64 or len(row['right_coefficient'])>64:raise ValueError('oversized')
  if not all(re.fullmatch(r'[0-9]+(?:\.[0-9]+)?',row[k]) for k in ['left_coefficient','right_coefficient']):raise ValueError('unsupported coefficient')
  exponent=int(row['left_power10'])
  if abs(exponent)>300:raise ValueError('unsupported exponent')
  left=Fraction(row['left_coefficient'])*Fraction(10)**exponent
  right=Fraction(row['right_coefficient'])*SCALES[row['right_scale']]
  largest=max(abs(left),abs(right));relative=abs(left-right)/largest if largest else Fraction(0)
  approx=bool(row.get('approximate')) or row.get('relation')=='相当于'
  contradiction=relative>Fraction(1,1000000)
  expected='review' if approx else ('reject' if contradiction else 'none')
  consistent_record=all(k not in row or Fraction(row[k])==value for k,value in [('left_magnitude_rational',left),('right_magnitude_rational',right),('relative_difference_rational',relative)])
  if Fraction(row.get('relative_tolerance_rational','1/1000000'))!=Fraction(1,1000000):classification='audit_policy_mismatch'
  elif not consistent_record:classification='audit_computation_mismatch'
  elif row.get('action')=='reject' and expected!='reject':classification='possible_false_refusal'
  elif row.get('action')!=expected:classification='audit_policy_mismatch'
  elif approx:classification='approximation_requires_review'
  elif contradiction:classification='confirmed_arithmetic_mismatch_under_recorded_binding'
  else:classification='consistent_magnitudes_under_recorded_binding'
  return {**result,'classification':classification,'expected_action':expected,'left':str(left),'right':str(right),'relative_difference':str(relative),'approximate':approx,'recorded_computation_matches':consistent_record}
 except (KeyError,ValueError,ZeroDivisionError,TypeError,AttributeError):return {**result,'classification':'insufficient_audit_data'}

def replay(document):
 gate=document
 if isinstance(gate.get('payload'),dict):gate=gate['payload']
 if isinstance(gate.get('message'),str):gate=json.loads(gate['message'])
 review=gate.get('claim_review',gate)
 rows=review.get('numeric_operations',[])
 return {'candidate_sha256':review.get('candidate_sha256'),'evidence_packet_sha256':gate.get('evidence_packet_sha256'),'operations':[{'index':i,**replay_operation(row)} for i,row in enumerate(rows)],'coverage':'operation replay only; unit/entity binding and candidate context require review','status':'replayed' if rows else 'insufficient_audit_data'}
def main():
 p=argparse.ArgumentParser();p.add_argument('audit_json',type=Path);p.add_argument('--candidate-file',type=Path);a=p.parse_args()
 result=replay(json.loads(a.audit_json.read_text()))
 if a.candidate_file:
  # Compare bytes to the stored candidate hash without printing the candidate.
  result['candidate_hash_matches']=hashlib.sha256(a.candidate_file.read_bytes()).hexdigest()==result.get('candidate_sha256')
 print(json.dumps(result,ensure_ascii=False,indent=2))
 if result.get('candidate_hash_matches') is False:return 2
 return 0
if __name__=='__main__':raise SystemExit(main())
