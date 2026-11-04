"""Replay selected accepted answers against verified visible spans only; no LLM calls."""
import argparse,hashlib,json,re,subprocess,tempfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1];REPO=ROOT.parents[3]
def digest(s):return hashlib.sha256(s.encode()).hexdigest()
def load_cases(out, include_m87=False):
 audit=json.loads((out/'claim-reviews.json').read_text());pdf=json.loads((ROOT/'workspace-remediation-pdf-preserved.json').read_text())['workspace_key']
 requested={('contest-scp-skills','scp-name-2'),('contest-scp-skills','scp-hop-2'),(pdf,'gw-significance')}
 if include_m87:
  requested.update((r['workspace'],r['case_id']) for r in audit['records'] if r['case_id']=='m87-mass')
 raw={}
 for suite in ['old','pdf']:
  log=(out/(suite+'.log')).read_text();m=re.search(r'^wrote (.+?\.json)(?: run_valid \w+)?$',log,re.M)
  if not m:raise ValueError('No retained result: '+suite)
  for row in json.loads((REPO / 'tests/contest/edu' / ('runs' if suite=='old' else 'generalization_astro_v1/remediation') / Path(m.group(1)).name).read_text())['results']:raw[(row.get('workspace',pdf),row['id'])]=row
 inputs=[];provenance=[];snapshots={}
 for row in audit['records']:
  key=(row['workspace'],row['case_id'])
  if key not in requested:continue
  reconstructed=key[1]=='m87-mass'
  if reconstructed:
   operations=row['claim_review'].get('numeric_operations',[])
   if row['gate_event']!='answer.rejected' or len(operations)!=1:raise ValueError('Missing M87 rejection audit')
   op=operations[0]
   candidate=f"{op['left_coefficient']}×10^{op['left_power10']} {op['left_unit']}，即{op['right_coefficient']}{op['right_scale']}{op['right_unit']}。"
   question=next(item['question'] for item in json.loads((out/'frozen-suite.json').read_text())['cases'] if item['id']=='m87-mass')
  else:
   if row['gate_event']!='answer.validated':continue
   original=raw[key];candidate=original['answer'];question=original['question']
   if digest(candidate)!=row['claim_review']['candidate_sha256']:raise ValueError('Candidate hash mismatch: '+str(key))
  if key[0] not in snapshots:
   snap=json.loads((out/(key[0]+'-before.json')).read_text());snapshots[key[0]]=({c['id']:c for c in snap['chunks']},{d['id']:d for d in snap['documents']})
  chunks,docs=snapshots[key[0]];hits=[]
  for ref in row['packet_references']:
   chunk=chunks[ref['chunk_id']];text=chunk['content']
   if digest(text)!=ref.get('original_content_sha256'):raise ValueError('Cannot verify original chunk for reconstruction: '+ref['chunk_id'])
   start,end=ref['start_rune'],ref['end_rune']
   if not 0<=start<=end<=len(text):raise ValueError('Invalid visible range')
   visible=text[start:end]+('…' if ref['truncated'] else '')
   if digest(visible)!=ref['content_sha256']:raise ValueError('Visible span hash mismatch: '+ref['chunk_id'])
   hits.append({'id':ref['chunk_id'],'doc_id':chunk['doc_id'],'title':docs[chunk['doc_id']]['title'],'content':visible,'truncated':ref['truncated']})
  case_key=key[0]+'/'+key[1]
  inputs.append({'key':case_key,'question':question,'candidate':candidate,'hits':hits})
  provenance.append({'key':case_key,'candidate_sha256':digest(candidate),'original_packet_sha256':row['evidence_packet_sha256'],'verified_visible_spans':len(hits),'candidate_reconstructed_from_operands':reconstructed})
 if len(inputs)!=len(requested):raise ValueError('Missing accepted retained candidate')
 return inputs,provenance

def review_cases(inputs):
 with tempfile.TemporaryDirectory() as tmp:
  probe=Path(tmp)/'main.go';probe.write_bytes((ROOT/'remediation/claim_role_replay.go.txt').read_bytes())
  process=subprocess.run(['go','run',str(probe)],cwd=REPO,input=json.dumps(inputs,ensure_ascii=False),text=True,capture_output=True,check=True)
 return json.loads(process.stdout)

def main():
 p=argparse.ArgumentParser();p.add_argument('baseline');a=p.parse_args();out=ROOT/'baselines'/a.baseline
 inputs,provenance=load_cases(out)
 result={'implementation_sha256':{f:hashlib.sha256((REPO/f).read_bytes()).hexdigest() for f in ['internal/core/service/agent/claim_review.go','internal/core/service/agent/numeric_consistency.go']},'provenance':provenance,'reviews':review_cases(inputs),'limits':'Uses accepted original candidates and exact visible content only. Flattened packet encoding differs from the original; no LLM calls, no rescore, no rejected candidate recovery.'}
 (out/'visible-role-replay-v2.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({key:[{'kind':row['candidate']['kind'],'label':row['candidate']['label'],'value':row['candidate'].get('value'),'status':row['status']} for row in data.get('role_checks',[])] for key,data in result['reviews'].items()},ensure_ascii=False))
if __name__=='__main__':main()
