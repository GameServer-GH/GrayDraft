"""Read-only collection of minimal claim audits and independent numeric replays."""
import argparse,collections,hashlib,json,subprocess,uuid
from pathlib import Path
from audit_shared_evidence import query
from replay_numeric_review import replay
ROOT=Path(__file__).resolve().parents[1]
def digest(text):return hashlib.sha256(text.encode()).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('baseline');a=p.parse_args();out=ROOT/'baselines'/a.baseline
 subprocess.run(['python3',str(ROOT/'remediation/audit_shared_evidence.py'),a.baseline],check=True,capture_output=True,text=True)
 audit=json.loads((out/'shared-evidence-audit.json').read_text());manifest=json.loads((out/'manifest.json').read_text())
 ids=','.join("'"+str(uuid.UUID(r['run_id']))+"'" for r in audit['runs'])
 if not ids:raise ValueError('No agent runs to audit')
 data=query(f"select json_build_object('runs',(select json_agg(t) from (select run_id,authorized_kbs from agent_runs where run_id in ({ids})) t),'events',(select json_agg(t) from (select run_id,seq,type,payload from agent_run_events where run_id in ({ids}) and type in ('answer.evidence.prepared','answer.validated','answer.rejected') order by run_id,seq) t));")
 key_by_kb={v:k for k,v in manifest['workspaces'].items()};cases={}
 old_path=out/'frozen-upgrade_regression_v1.json';astro_path=out/'frozen-suite.json'
 old=json.loads((old_path if old_path.exists() else ROOT.parent/'upgrade_regression_v1.json').read_text())
 for key,rows in old['cases'].items():
  for c in rows:cases[(key,digest(c['q']))]=c['id']
 zh=json.loads((ROOT/'workspace-remediation-accepted.json').read_text())['workspace_key'];pdf=json.loads((ROOT/'workspace-remediation-pdf-preserved.json').read_text())['workspace_key']
 for c in json.loads((astro_path if astro_path.exists() else ROOT/'suite.json').read_text())['cases']:
  cases[(zh,digest(c['question']))]=c['id']
  if c['id'] in {'gw-time','gw-frequency','gw-masses','gw-significance'}:cases[(pdf,digest(c['question']))]=c['id']
 runinfo={}
 for r in data['runs']:
  record=next(x for x in audit['runs'] if x['run_id']==r['run_id']);keys=[key_by_kb[k] for k in r['authorized_kbs'] if k in key_by_kb]
  key=keys[0];runinfo[r['run_id']]={'workspace':key,'case_id':cases.get((key,record['question_sha256']),'unknown'),'run_status':record['status']}
 packets={};records=[];dispositions=collections.Counter();roles=collections.Counter();replays=collections.Counter();bad_refs=[];missing_trace=[];gated=set()
 for ev in data['events']:
  payload=ev['payload'];value=json.loads(payload['message']);key=(ev['run_id'],payload.get('step_id',0))
  if ev['type']=='answer.evidence.prepared':packets[key]=value;continue
  gated.add(key);review=value.get('claim_review')
  if not review:missing_trace.append({'run_id':key[0],'step':key[1]});continue
  dispositions[review['disposition']]+=1
  allowed={(r['chunk_id'],r['content_sha256'],r['truncated']) for r in (packets[key].get('references') or [])}
  for row in review.get('role_checks',[]):
   roles[(row['candidate']['kind'],row['status'])]+=1
   for ref in row.get('evidence',[]):
    if (ref.get('chunk_id'),ref.get('packet_content_sha256'),ref.get('packet_reference_truncated',False)) not in allowed:bad_refs.append({'run_id':key[0],'step':key[1],'reference':ref})
  replayed=replay(value)
  for op in replayed['operations']:replays[op['classification']]+=1
  records.append({'run_id':key[0],'step':key[1],**runinfo[key[0]],'gate_event':ev['type'],'refuse_reason':value.get('refuse_reason',''),'evidence_packet_sha256':value.get('evidence_packet_sha256'),'packet_references':packets[key].get('references') or [],'claim_review':review,'independent_numeric_replay':replayed})
 result={'baseline_valid':manifest['run_valid'],'run_count':len(audit['runs']),'gate_steps':len(records),'all_role_references_in_visible_packet':not bad_refs,'missing_claim_traces':missing_trace,'invalid_role_references':bad_refs,'dispositions':dict(dispositions),'role_status_counts':[{'kind':k,'status':s,'count':n} for (k,s),n in sorted(roles.items())],'numeric_replay_counts':dict(replays),'prepared_without_gate':[{'run_id':k[0],'step':k[1],**runinfo[k[0]]} for k in packets if k not in gated],'records':records,'limits':'Semantic roles are review observations only, not verified entailment. Numeric replay verifies recorded magnitudes, not entity/unit/context binding. Full rejected candidates are not retained.'}
 (out/'claim-reviews.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({k:v for k,v in result.items() if k not in ('records','invalid_role_references')},ensure_ascii=False))
 if bad_refs or missing_trace:raise SystemExit(1)
if __name__=='__main__':main()
