#!/usr/bin/env python3
"""Verify the retained Go API can serve schema 101; no live service changes."""
import importlib.util,pathlib,json,subprocess,time,urllib.request,urllib.error,os
os.umask(0o077)
spec=importlib.util.spec_from_file_location('native_prepare','/home/vtplerp/epcerp-native-prepare.py');p=importlib.util.module_from_spec(spec);spec.loader.exec_module(p)
assert os.geteuid()==0
root=pathlib.Path(p.inspect('frontend')['Config']['Labels']['com.docker.compose.project.working_dir']);release=pathlib.Path((root/'native-preflight-path.txt').read_text().strip());verification=json.loads((release/'preflight-verification.json').read_text());old=json.loads((release/'runtime-before.private.json').read_text());qa=json.loads(verification and (release/'preflight-verification.json').read_text())['qa_database'];e=p.environment(old['api']);from urllib.parse import urlsplit,urlunsplit
u=urlsplit(e['EPCERP_DATABASE_URL']);e['EPCERP_DATABASE_URL']=urlunsplit((u.scheme,u.netloc,'/'+qa,u.query,u.fragment));e.update(EPCERP_TSC_CLOUD_APPROVED='false',EPCERP_EMAIL_DELIVERY_ENABLED='false')
for k in list(e):
 if 'API_KEY'in k:e[k]=''
p.write_env(release/'rollback-qa.private.env',e);name=p.PROJECT+'-rollback-preflight'
try:
 p.run(['docker','run','-d','--name',name,'--network',p.PROJECT+'_default','--env-file',str(release/'rollback-qa.private.env'),'-p','127.0.0.1:19692:8001',old['api']['Config']['Image']],stdout=subprocess.DEVNULL)
 for net in ['edge','reading_internal']:p.run(['docker','network','connect',p.PROJECT+'_'+net,name])
 for i in range(60):
  try:
   if urllib.request.urlopen('http://127.0.0.1:19692/api/v1/health/ready',timeout=3).status==200:break
  except (OSError,urllib.error.URLError):time.sleep(1)
 else:raise RuntimeError('Retained API did not become ready on upgraded schema')
 token=p.token(e,qa);checks=[]
 for path in ['/api/v1/auth/me','/api/v1/projects','/api/v1/tender-fms']:
  with urllib.request.urlopen(urllib.request.Request('http://127.0.0.1:19692'+path,headers={'Authorization':'Bearer '+token}),timeout=60)as r:assert r.status==200;checks.append({'path':path,'status':200})
 p.write_json(release/'rollback-preflight-verification.json',{'image':old['api']['Config']['Image'],'schema':101,'startup':True,'checks':checks,'cloud_email_disabled':True,'schema_downgrade_required':False});print('PASS: retained API starts and authenticates against schema 101; rollback requires no database restore or migration reset',flush=True)
finally:subprocess.run(['docker','rm','-f',name],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
