"""Read-only audit: persisted structured questions and per-step packet hash parity."""
import argparse,base64,collections,hashlib,json,subprocess
from datetime import datetime,timezone
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
def digest(text):return hashlib.sha256(text.encode()).hexdigest()
def query(sql):
 p=subprocess.run(['docker','exec','-i','graydraft-postgres-dev','sh','-c','exec psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At'],input=sql,text=True,capture_output=True,check=True)
 return json.loads(p.stdout)
def main():
 parser=argparse.ArgumentParser();parser.add_argument('baseline');args=parser.parse_args()
 out=ROOT/'baselines'/args.baseline;manifest=json.loads((out/'manifest.json').read_text())
 if not manifest.get('completed_at'):raise ValueError('Complete the baseline before auditing')
 start=datetime.strptime(manifest['started_at'],'%Y%m%dT%H%M%S%fZ').replace(tzinfo=timezone.utc).isoformat()
 end=datetime.fromisoformat(manifest['completed_at']).isoformat()
 partitions=manifest['workspaces']
 for kb in partitions.values():
  if not kb.startswith('gd_') or any(c not in 'gd_0123456789abcdef' for c in kb):raise ValueError('Invalid scope')
 array='ARRAY['+','.join("'"+kb+"'" for kb in partitions.values())+']::text[]'
 scope=f"created_at >= '{start}'::timestamptz AND created_at <= '{end}'::timestamptz AND authorized_kbs && {array} AND idempotency_key NOT LIKE '%:shadow:%'"
 rows=query(f"select coalesce(json_agg(r),'[]') from (select run_id,status,authorized_kbs,encode(prompt,'base64') as input_base64 from agent_runs where {scope} order by created_at) r;")
 events=query(f"select coalesce(json_agg(e),'[]') from (select run_id,seq,type,payload from agent_run_events where run_id in (select run_id from agent_runs where {scope}) and type in ('answer.evidence.prepared','answer.validated','answer.rejected') order by run_id,seq) e;")
 expected=collections.Counter()
 old=json.loads((ROOT.parent/'upgrade_regression_v1.json').read_text())
 for key,cases in old['cases'].items():
  for case in cases:expected[(partitions[key],digest(case['q']))]+=1
 astro=json.loads((ROOT/'suite.json').read_text())
 zh=json.loads((ROOT/'workspace-remediation-accepted.json').read_text())['workspace_key']
 pdf=json.loads((ROOT/'workspace-remediation-pdf-preserved.json').read_text())['workspace_key']
 for c in astro['cases']:
  expected[(partitions[zh],digest(c['question']))]+=1
  if c['id'] in {'gw-time','gw-frequency','gw-masses','gw-significance'}:expected[(partitions[pdf],digest(c['question']))]+=1
 actual=collections.Counter();records=[];prefix='\x1egraydraft-input-v1\n'
 for row in rows:
  text=base64.b64decode(row.pop('input_base64')).decode();structured=text.startswith(prefix)
  data=json.loads(text[len(prefix):]) if structured else {'question':text}
  key=digest(data['question']);kbs=set(row['authorized_kbs']) & set(partitions.values())
  for kb in kbs:actual[(kb,key)]+=1
  records.append({'run_id':row['run_id'],'status':row['status'],'structured_input':structured,'question_sha256':key,'config_fields':sorted(data.get('config',{}))})
 prepared={};pairs=[];refs=0;truncated=0
 for ev in events:
  payload=ev['payload'];message=json.loads(payload['message']);key=(ev['run_id'],payload.get('step_id',0))
  if ev['type']=='answer.evidence.prepared':
   prepared[key]=message['evidence_packet_sha256'];refs+=len((message.get('references') or []));truncated+=sum(bool(x['truncated']) for x in (message.get('references') or []))
  else:
   h=message.get('evidence_packet_sha256');pairs.append({'run_id':key[0],'step':key[1],'gate_event':ev['type'],'matches_prepared':bool(h) and h==prepared.get(key),'packet_sha256':h,'refuse_reason':message.get('refuse_reason','')})
 result={'baseline_valid':manifest['run_valid'],'run_count':len(rows),'expected_requests':sum(expected.values()),'question_multiset_matches_frozen_cases':actual==expected,'all_inputs_structured':bool(records) and all(r['structured_input'] for r in records),'prepared_steps':len(prepared),'gate_steps':len(pairs),'all_gate_hashes_match':bool(pairs) and all(p['matches_prepared'] for p in pairs),'reference_count':refs,'truncated_references':truncated,'runs':records,'gate_pairs':pairs,'limits':'Only matching validation steps are checked. Catalog-only paths and early templates may bypass composition; generation failures may have prepared evidence without a gate. This audits local inputs, not provider-internal processing.'}
 (out/'shared-evidence-audit.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({k:v for k,v in result.items() if k not in ('runs','gate_pairs')},ensure_ascii=False))
 if not(result['baseline_valid'] and result['question_multiset_matches_frozen_cases'] and result['all_inputs_structured'] and result['all_gate_hashes_match']):raise SystemExit(1)
if __name__=='__main__':main()
