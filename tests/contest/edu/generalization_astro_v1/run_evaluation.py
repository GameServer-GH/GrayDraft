#!/usr/bin/env python3
"""First-attempt out-of-domain evaluation. Never grades by expected keywords."""
import argparse, hashlib, json, subprocess, sys, time
from datetime import datetime, timezone
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
import retest_after_upgrade as api
ROOT=Path(__file__).resolve().parent
REPO=ROOT.parents[3]

def digest(data):return hashlib.sha256(data).hexdigest()
def source_fingerprints():
 out=api.source_fingerprints()
 for pattern in ['internal/worker/worker/**/*.py','internal/core/service/smartimport/*.go','internal/core/service/multimodal/*.go','internal/core/service/retriever/*.go','internal/core/repository/postgres/*.go','internal/graydraft/internal/server/*.go']:
  for p in REPO.glob(pattern):
   if not p.name.endswith('_test.go'):out[str(p.relative_to(REPO))]=digest(p.read_bytes())
 return out

def runtime():
 names=['graydraft-core-dev','graydraft-gateway-dev','graydraft-worker-dev','graydraft-graydraft-dev']
 p=subprocess.run(['docker','inspect',*names],text=True,capture_output=True,check=True)
 return {x['Name'].lstrip('/'):{'image':x['Image'],'env_sha256':digest(json.dumps(sorted(x['Config'].get('Env',[]))).encode())} for x in json.loads(p.stdout)}

def database_snapshot(kb):
 # Generated workspace partitions only; no user supplied SQL and no database writes.
 if not kb.startswith('gd_') or any(c not in 'gd_0123456789abcdef' for c in kb):raise ValueError('invalid partition')
 sql=f"""select json_build_object('documents',coalesce((select json_agg(json_build_object('id',id,'title',title,'source_url',source_url,'content_hash',content_hash,'version',kb_version,'ready',index_ready,'extensions',extensions) order by id) from kb_documents where kb_type='{kb}'),'[]'::json),'chunks',coalesce((select json_agg(json_build_object('id',id,'doc_id',doc_id,'index',chunk_index,'content',content,'ready',index_ready,'embedding_present',embedding is not null,'embedding_sha',md5(embedding::text),'search_text',search_text) order by id) from kb_chunks where kb_type='{kb}'),'[]'::json));"""
 p=subprocess.run(['docker','exec','-i','graydraft-postgres-dev','sh','-c','exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At'],input=sql,text=True,capture_output=True,check=True)
 return json.loads(p.stdout)

def corpus_fingerprint(snapshot):
 # last_accessed_at and task progress are intentionally absent; content must stay frozen.
 return digest(json.dumps(snapshot,sort_keys=True,ensure_ascii=False).encode())

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--only',nargs='+');parser.add_argument('--workspace-file',default='workspace.json');args=parser.parse_args()
 suite_path=ROOT/'suite.json';suite_bytes=suite_path.read_bytes();suite=json.loads(suite_bytes)
 expected=(ROOT/'suite.sha256').read_text().split()[0]
 if digest(suite_bytes)!=expected:raise ValueError('frozen suite changed')
 selected=[c for c in suite['cases'] if not args.only or c['id'] in args.only]
 if args.only and len(selected)!=len(args.only):raise ValueError('unknown or duplicated case')
 ws=json.loads((ROOT/args.workspace_file).read_text());key=ws['workspace_key'];kb=ws['nexusKbType']
 before=database_snapshot(kb)
 if not before['chunks'] or not all(c['ready'] and c['embedding_present'] for c in before['chunks']):raise ValueError('corpus not ready; do not score empty-workspace refusals')
 stamp=datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ');out=ROOT/'runs'/stamp;out.mkdir(parents=True)
 corpus_files={str(p.relative_to(ROOT)):digest(p.read_bytes()) for p in sorted((ROOT/'corpus_zh_excerpt').glob('*.md'))}
 meta={'started_at':datetime.now(timezone.utc).isoformat(),'suite':suite,'suite_sha256':expected,'workspace':ws,'corpus_condition':'Chinese translated abstracts plus observation excerpts, not unmodified English full papers','corpus_files':corpus_files,'runner_sha256':digest(Path(__file__).read_bytes()),'protocol':'same frozen questions; one client attempt; skip_cache and skip_answer_cache; 5 second pause between cases; no application/config changes','source_before':source_fingerprints(),'runtime_before':runtime(),'corpus_before_sha256':corpus_fingerprint(before)}
 (out/'corpus_before.json').write_text(json.dumps(before,ensure_ascii=False,indent=2)+'\n')
 (out/'manifest.json').write_text(json.dumps(meta,ensure_ascii=False,indent=2)+'\n')
 rows=[]
 for index,c in enumerate(selected):
  if index:time.sleep(5)
  print('ask',index+1,len(selected),c['id'],flush=True)
  answer,diag=api.ask(key,c['question'])
  row={'id':c['id'],'expect':c['expect'],'question':c['question'],'attempt':1,'answer':answer,'diagnostics':diag,'grading':'pending source-based human review'}
  rows.append(row)
  (out/(c['id']+'.json')).write_text(json.dumps(row,ensure_ascii=False,indent=2)+'\n')
  print('done',c['id'],diag['elapsed_ms'],'ms','terminal',diag['terminal_seen'],'error',diag['stream_error'],answer[:90].replace('\n',' '),flush=True)
 after=database_snapshot(kb)
 (out/'corpus_after.json').write_text(json.dumps(after,ensure_ascii=False,indent=2)+'\n')
 meta.update({'completed_at':datetime.now(timezone.utc).isoformat(),'source_after':source_fingerprints(),'runtime_after':runtime(),'corpus_after_sha256':corpus_fingerprint(after)})
 meta['run_valid']=meta['source_before']==meta['source_after'] and meta['runtime_before']==meta['runtime_after'] and meta['corpus_before_sha256']==meta['corpus_after_sha256'] and digest(suite_path.read_bytes())==expected
 (out/'results.json').write_text(json.dumps({'meta':meta,'results':rows},ensure_ascii=False,indent=2)+'\n')
 print('wrote',out/'results.json','run_valid',meta['run_valid'],flush=True)
 if not meta['run_valid']:return 2
 return int(any(r['diagnostics']['stream_error'] or not r['diagnostics']['terminal_seen'] for r in rows))
if __name__=='__main__':raise SystemExit(main())
