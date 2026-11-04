"""Raw English PDF retrieval smoke, separate from the Chinese 24-case suite."""
import json,sys,time,hashlib,argparse
from pathlib import Path
from datetime import datetime,timezone
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT.parent))
import retest_after_upgrade as api
sys.path.insert(0,str(ROOT))
import run_evaluation as evidence
parser=argparse.ArgumentParser();parser.add_argument('--workspace-file',default='workspace-remediation-formats.json');args=parser.parse_args()
suite=json.loads((ROOT/'suite.json').read_text());ws=json.loads((ROOT/args.workspace_file).read_text())
source_before=evidence.source_fingerprints();runtime_before=evidence.runtime();corpus_before=evidence.corpus_fingerprint(evidence.database_snapshot(ws['nexusKbType']))
suite_sha=hashlib.sha256((ROOT/'suite.json').read_bytes()).hexdigest()
rows=[]
for case in suite['cases']:
 if case['id'] not in {'gw-time','gw-frequency','gw-masses','gw-significance'}:continue
 answer,diag=api.ask(ws['workspace_key'],case['question']);rows.append({'id':case['id'],'question':case['question'],'answer':answer,'diagnostics':diag});print(case['id'],diag['elapsed_ms'],diag['stream_error'],answer[:100].replace('\n',' '),flush=True);time.sleep(5)
stamp=datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
valid=source_before==evidence.source_fingerprints() and runtime_before==evidence.runtime() and corpus_before==evidence.corpus_fingerprint(evidence.database_snapshot(ws['nexusKbType'])) and suite_sha==hashlib.sha256((ROOT/'suite.json').read_bytes()).hexdigest()
p=ROOT/'remediation'/(stamp+'_pdf_sentinels.json');p.write_text(json.dumps({'condition':'raw original English PDF, deterministic text-layer extraction, no translated corpus in this workspace','workspace':ws,'source_pdf_sha256':hashlib.sha256((ROOT/'sources/1602.03837v1.pdf').read_bytes()).hexdigest(),'source_before':source_before,'runtime_before':runtime_before,'corpus_sha256':corpus_before,'suite_sha256':suite_sha,'run_valid':valid,'results':rows},ensure_ascii=False,indent=2)+'\n');print('wrote',p,'run_valid',valid,flush=True)
