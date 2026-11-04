"""Freeze and run existing suites serially; preserve failures without retries."""
import hashlib,json,subprocess,sys
from datetime import datetime,timezone
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT));import run_evaluation as ev
sys.path.insert(0,str(ROOT.parent));import retest_after_upgrade as api

def main():
 stamp=datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ');out=ROOT/'baselines'/stamp;out.mkdir(parents=True)
 suites=[ROOT/'suite.json',ROOT.parent/'upgrade_regression_v1.json']
 fingerprint=lambda:{str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in suites}
 partitions={}
 for key in ['contest-scp-skills','contest-papers']:
  partitions[key]=api.post_json('/api/OpenWorkspace',{'key':key})['nexusKbType']
 for name in ['workspace-remediation-accepted.json','workspace-remediation-pdf-preserved.json']:
  w=json.loads((ROOT/name).read_text());partitions[w['workspace_key']]=w['nexusKbType']
 def snapshot(label):
  hashes={}
  for key,kb in partitions.items():
   data=ev.database_snapshot(kb)
   if not data['chunks'] or not all(c['ready'] and c['embedding_present'] for c in data['chunks']):raise RuntimeError('Corpus not ready: '+key)
   (out/(key+'-'+label+'.json')).write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n');hashes[key]=ev.corpus_fingerprint(data)
  return {'source':ev.source_fingerprints(),'runtime':ev.runtime(),'suites':fingerprint(),'corpora':hashes}
 before=snapshot('before');manifest={'started_at':stamp,'protocol':'serial, frozen suites, one attempt, no retries, no production changes','before':before,'workspaces':partitions,'runs':[]}
 (out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(out,flush=True)
 commands=[('old',['python3',str(ROOT.parent/'retest_after_upgrade.py')]),('astro',['python3',str(ROOT/'run_evaluation.py'),'--workspace-file','workspace-remediation-accepted.json']),('pdf',['python3',str(ROOT/'remediation/pdf_sentinels.py'),'--workspace-file','workspace-remediation-pdf-preserved.json'])]
 for name,args in commands:
  with (out/(name+'.log')).open('w') as log:r=subprocess.run(args,stdout=log,stderr=subprocess.STDOUT)
  manifest['runs'].append({'suite':name,'exit_code':r.returncode,'log':name+'.log'})
  (out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(name,'exit',r.returncode,flush=True)
 manifest['after']=snapshot('after');manifest['run_valid']=before==manifest['after'];manifest['completed_at']=datetime.now(timezone.utc).isoformat();(out/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print('valid',manifest['run_valid'],flush=True)
if __name__=='__main__':main()
