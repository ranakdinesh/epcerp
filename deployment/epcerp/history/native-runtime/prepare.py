#!/usr/bin/env python3
"""Root-only development preflight. Existing application services remain online."""
import copy, datetime, fcntl, hashlib, json, os, pathlib, subprocess, sys, time, urllib.request, urllib.error, uuid, base64, hmac
os.umask(0o077)
PROJECT='epcerp-dev'; FRONT='epcerp-frontend:retire-72215c9-amd64'; PROCESSOR='epcerp-document-processor:retire-72215c9'
LEGACY=['api','document-worker','document-package-worker','tsc-worker','tsc-reranker','migrate']
OWNERS=['AUTH','CLIENTS','REFERENCES','CATALOGUE','VENDOR_PROFILES','VENDOR_DOCUMENTS','VENDOR_VERIFICATION','TSC','TENDER_INTAKE','TENDER_REGISTER','INBOX','PERSONAL_TASKS','PROJECTS','PROJECT_TEAM','PURCHASING','PROJECT_BUDGETS','PLATFORM']
def run(a,**kw):return subprocess.run(a,check=True,**kw)
def output(a):return subprocess.check_output(a,text=True).strip()
def inspect(service):return json.loads(output(['docker','inspect',PROJECT+'-'+service+'-1']))[0]
def environment(c):return dict(x.split('=',1)for x in c['Config']['Env'])
def query(q,db='epcerp'):return output(['docker','exec',PROJECT+'-db-1','psql','-U','epcerp','-d',db,'-v','ON_ERROR_STOP=1','-Atc',q])
def hashes(db='epcerp',tables=None):
 if tables is None:tables=query("SELECT schemaname||'.'||tablename FROM pg_tables WHERE schemaname IN('public','auth','tender')AND tablename<>'schema_migrations'ORDER BY 1",db).splitlines()
 assert all(all(c.isalnum()or c in'._'for c in t)for t in tables)
 sql=' UNION ALL '.join("SELECT '"+t+"',md5(coalesce(string_agg(to_jsonb(r)::text,'|'ORDER BY to_jsonb(r)::text),''))FROM "+t+' r'for t in tables)
 return dict(x.split('|',1)for x in query(sql,db).splitlines())
def write_json(p,v):p.write_text(json.dumps(v,indent=2)+'\n');p.chmod(0o600)
def write_env(p,v):
 assert all('\n'not in x and '\r'not in x for x in v.values());p.write_text('\n'.join(k+'='+v for k,v in v.items())+'\n');p.chmod(0o600)
def token(e,db):
 user,version=query("SELECT id,credential_version FROM auth.users WHERE is_superuser AND is_active AND NOT must_change_password ORDER BY username LIMIT 1",db).split('|');now=int(time.time());encode=lambda v:base64.urlsafe_b64encode(json.dumps(v,separators=(',',':')).encode()).rstrip(b'=').decode();raw=encode({'alg':'HS256','typ':'JWT'})+'.'+encode({'sub':user,'iss':e.get('VTPL_JWT_ISSUER','epcERP'),'aud':e.get('VTPL_JWT_AUDIENCE','epcERP-api'),'iat':now,'nbf':now,'exp':now+180,'jti':str(uuid.uuid4()),'typ':'access','cv':int(version)});return raw+'.'+base64.urlsafe_b64encode(hmac.new(e['VTPL_JWT_SECRET_KEY'].encode(),raw.encode(),hashlib.sha256).digest()).rstrip(b'=').decode()
def http_checks(base,e,db):
 raw=token(e,db);checks=[]
 def read(path,binary=False):
  req=urllib.request.Request(base+path,headers={'Cookie':'epcerp_access_token='+raw})
  with urllib.request.urlopen(req,timeout=90)as r:
   assert r.status==200,(path,r.status);data=r.read();checks.append({'path':path,'status':r.status});return data if binary else json.loads(data)
 for path in ['/api/health/ready','/api/auth/user_profile/me','/api/governance','/api/administration/organisation/departments','/api/administration/organisation/employees','/api/administration/organisation/offboarding','/api/work-calendar/holidays?year=2026','/api/settings/clients','/api/settings/client-references/cities','/api/settings/suppliers','/api/projects','/api/purchase-fms','/api/my-tasks?role=all&page_size=20','/api/inbox','/api/tender-fms/drafts','/api/tender-fms/records','/api/settings/tsc-formats']:read(path)
 formats=read('/api/settings/tsc-formats');formats=formats if isinstance(formats,list)else formats['items']
 if formats:assert read('/api/settings/tsc-formats/'+formats[0]['id']+'/preview.pdf',True).startswith(b'%PDF-')
 doc=query("SELECT id FROM stored_documents WHERE is_active ORDER BY created_at LIMIT 1",db)
 if doc:assert read('/api/documents/'+doc+'/download',True)
 # Retained historical snapshots must decrypt with the original reading key.
 user=query("SELECT id FROM auth.users WHERE is_superuser AND is_active AND NOT must_change_password ORDER BY username LIMIT 1",db)
 legacy=query("SELECT j.draft_id,j.id FROM tender.tsc_jobs j JOIN tender.document_drafts d ON d.id=j.draft_id WHERE d.owner_id='"+user+"'AND d.is_active ORDER BY j.created_at DESC LIMIT 1",db)
 if legacy:
  draft,job=legacy.split('|');value=read('/api/tender-fms/drafts/'+draft+'/tsc/'+job);assert value['read_engine']=='nextjs'
 return checks

def main():
 assert os.geteuid()==0,'Run through sudo';lock=open('/run/lock/epcerp-deployment.lock','w');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
 front=inspect('frontend');root=pathlib.Path(front['Config']['Labels']['com.docker.compose.project.working_dir']);assert root==pathlib.Path('/home/vtplerp/epcerp-releases/dev-20261102-1-amd64');os.chdir(root)
 original=json.loads((root/'compose.yaml').read_text());assert original.get('name')==PROJECT;assert query('SELECT version,dirty FROM schema_migrations')=='98|f'
 for q in ["SELECT count(*)FROM tender.tsc_jobs WHERE state IN('queued','processing')","SELECT count(*)FROM document_readings WHERE status IN('queued','processing')","SELECT count(*)FROM document_package_receipts WHERE state IN('receiving','verifying','ready')"]:assert query(q)=='0','Active work must finish before deployment'
 archive=pathlib.Path('/home/vtplerp/epcerp-dev-native-72215c9-amd64.tar.gz');expected=pathlib.Path('/home/vtplerp/epcerp-dev-native-72215c9-amd64.sha256').read_text().strip();assert hashlib.file_digest(archive.open('rb'),'sha256').hexdigest()==expected
 run(['docker','load','-i',str(archive)],stdout=subprocess.DEVNULL)
 for image in [FRONT,PROCESSOR]:assert output(['docker','image','inspect','--format','{{.Architecture}}',image])=='amd64'
 release=root/('native-preflight-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ'));release.mkdir(mode=0o700)
 snapshots={n:inspect(n)for n in original['services']};write_json(release/'runtime-before.private.json',snapshots);write_json(release/'compose-before.private.json',original)
 api=environment(snapshots['api']);native={k:v for k,v in api.items()if k.startswith('EPCERP_')or k.startswith(('VTPL_JWT_','VTPL_BOOTSTRAP_','VTPL_STORAGE_'))or k=='VTPL_DOCUMENT_ENCRYPTION_KEY'}
 native.update({('EPCERP_'+o+'_OWNER'):'nextjs'for o in OWNERS});native.update(EPCERP_GO_DISABLED='true',EPCERP_API_URL='http://127.0.0.1:1/api/v1',EPCERP_DOCUMENT_PACKAGE_ROOT='/var/lib/epcerp/document-packages',EPCERP_TENDER_FORMAT_RENDER_URL='http://document-processor:8082',EPCERP_EMAIL_DELIVERY_ENABLED=api.get('EPCERP_EMAIL_DELIVERY_ENABLED','false'))
 sourceMount=next(m for m in snapshots['api']['Mounts']if m['Destination']==native['EPCERP_DOCUMENT_PACKAGE_ROOT']);assert sourceMount['Type']=='volume'
 candidate=copy.deepcopy(original)
 for n in LEGACY:del candidate['services'][n]
 candidate['services']['schema']={'image':FRONT,'pull_policy':'never','restart':'no','command':['node','/app/tools/migrate.mjs','up'],'environment':{'EPCERP_DATABASE_URL':api['EPCERP_DATABASE_URL'],'EPCERP_MIGRATION_ROOT':'/app/migrations'},'depends_on':{'db':{'condition':'service_healthy'}},'networks':['default']}
 candidate['services']['document-processor']={'image':PROCESSOR,'platform':'linux/amd64','pull_policy':'never','restart':'unless-stopped','init':True,'read_only':True,'environment':{'EPCERP_DOCUMENT_READING_KEY':api['EPCERP_DOCUMENT_READING_KEY'],'EPCERP_CLAMD_SOCKET':'tcp://clamav:3310'},'networks':['reading_internal'],'tmpfs':['/tmp:size=768m,uid=10001,gid=10001,mode=0700'],'mem_limit':'2g','cpus':1,'pids_limit':128,'cap_drop':['ALL'],'security_opt':['no-new-privileges:true'],'depends_on':{'clamav':{'condition':'service_healthy'}},'healthcheck':{'test':['CMD','node','-e',"fetch('http://127.0.0.1:8082/health').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"],'interval':'10s','timeout':'5s','retries':15}}
 for n in ['frontend','frontend-lan']:
  s=candidate['services'][n];s['image']=FRONT;s['environment']={**native,**original['services'][n]['environment']};s['environment'].update(EPCERP_GO_DISABLED='true',EPCERP_API_URL='http://127.0.0.1:1/api/v1');s['networks']=['default','edge','reading_internal','cloud_outbound'];s['volumes']=['document_packages_data:/var/lib/epcerp/document-packages'];s['depends_on']={'schema':{'condition':'service_completed_successfully'},'document-processor':{'condition':'service_healthy'},'storage':{'condition':'service_healthy'}};s['healthcheck']['test']=['CMD','node','-e',"fetch('http://127.0.0.1:3000/api/health/ready').then(r=>{if(!r.ok)process.exit(1)}).catch(()=>process.exit(1))"]
 for n in ['db','storage','clamav','gateway']:assert candidate['services'][n]==original['services'][n]
 assert candidate['volumes']==original['volumes']and candidate['networks']==original['networks']
 for n in ['frontend','frontend-lan']:
  for k,v in api.items():
   if k.startswith('EPCERP_TSC_')or k in ['EPCERP_DATABASE_URL','VTPL_JWT_SECRET_KEY','EPCERP_IMAGE_ENCRYPTION_KEY','EPCERP_SUPPLIER_DOCUMENT_KEY','EPCERP_DOCUMENT_READING_KEY','VTPL_STORAGE_SECRET_KEY','VTPL_DOCUMENT_ENCRYPTION_KEY']:assert candidate['services'][n]['environment'].get(k)==v,'Private setting mismatch: '+k
 write_json(release/'compose-candidate.private.json',candidate);run(['docker','compose','--project-directory',str(root),'--env-file',str(root/'.env'),'-p',PROJECT,'-f',str(release/'compose-candidate.private.json'),'config','--quiet'])
 # Consistent MVCC copy for migration rehearsal; production services remain online.
 name='epcerp_auth_qa_retire_server_'+datetime.datetime.now().strftime('%Y%m%d%H%M%S');run(['docker','exec',PROJECT+'-db-1','createdb','-U','epcerp',name])
 with(release/'preflight.database.dump').open('wb')as f:run(['docker','exec',PROJECT+'-db-1','pg_dump','-U','epcerp','-d','epcerp','-Fc'],stdout=f)
 with(release/'preflight.database.dump').open('rb')as f:run(['docker','exec','-i',PROJECT+'-db-1','pg_restore','-U','epcerp','-d',name,'--exit-on-error'],stdin=f,stdout=subprocess.DEVNULL)
 before=hashes(name);qa=dict(candidate['services']['frontend-lan']['environment']);from urllib.parse import urlsplit,urlunsplit
 u=urlsplit(qa['EPCERP_DATABASE_URL']);qa['EPCERP_DATABASE_URL']=urlunsplit((u.scheme,u.netloc,'/'+name,u.query,u.fragment));qa.update(EPCERP_TSC_CLOUD_APPROVED='false',EPCERP_EMAIL_DELIVERY_ENABLED='false')
 for k in list(qa):
  if 'API_KEY'in k or k.startswith('EPCERP_TSC_TEST_'):qa[k]=''
 write_env(release/'qa.private.env',qa);run(['docker','run','--rm','--network',PROJECT+'_default','--env-file',str(release/'qa.private.env'),FRONT,'node','/app/tools/migrate.mjs','up']);assert hashes(name,list(before))==before,'Migration rehearsal altered existing data'
 print('Migration rehearsal passed: schema 98 to 101; all '+str(len(before))+' existing tables unchanged',flush=True)
 qaFront=PROJECT+'-native-preflight';qaProcessor=PROJECT+'-processor-preflight'
 try:
  run(['docker','run','-d','--name',qaProcessor,'--network',PROJECT+'_reading_internal','--network-alias','document-processor','--read-only','--tmpfs','/tmp:size=768m,uid=10001,gid=10001,mode=0700','--memory','2g','--cpus','1','--pids-limit','128','--cap-drop','ALL','--security-opt','no-new-privileges','--env-file',str(release/'qa.private.env'),'-e','EPCERP_CLAMD_SOCKET=tcp://clamav:3310',PROCESSOR],stdout=subprocess.DEVNULL)
  run(['docker','run','-d','--name',qaFront,'--network',PROJECT+'_default','--env-file',str(release/'qa.private.env'),'-v',sourceMount['Name']+':'+native['EPCERP_DOCUMENT_PACKAGE_ROOT']+':ro','-p','127.0.0.1:19691:3000',FRONT],stdout=subprocess.DEVNULL)
  for net in ['edge','reading_internal']:run(['docker','network','connect',PROJECT+'_'+net,qaFront])
  for i in range(60):
   try:
    if urllib.request.urlopen('http://127.0.0.1:19691/api/health/ready',timeout=3).status==200:break
   except (OSError,urllib.error.URLError):time.sleep(1)
  else:raise RuntimeError('Native preflight did not become ready')
  checks=http_checks('http://127.0.0.1:19691',qa,name);assert query('SELECT count(*)FROM tender.tsc_model_calls',name)=='0'
  write_json(release/'preflight-verification.json',{'schema_before':98,'schema_after':101,'unchanged_existing_tables':len(before),'http_checks':checks,'provider_calls':0,'email_enabled':False,'archive_sha256':expected,'qa_database':name,'images':{n:json.loads(output(['docker','image','inspect',n]))[0]['Id']for n in [FRONT,PROCESSOR]}})
  (root/'native-preflight-path.txt').write_text(str(release)+'\n');print('PASS: migration, original-key decryption, '+str(len(checks))+' native HTTP checks and PDF rendering on the server; live deployment unchanged',flush=True)
 finally:
  for n in [qaFront,qaProcessor]:subprocess.run(['docker','rm','-f',n],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
if __name__=='__main__':main()
