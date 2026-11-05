#!/usr/bin/env python3
"""Protected development cutover. Never resets/restores the live database."""
import importlib.util,os,pathlib,json,subprocess,fcntl,datetime,hashlib,tarfile,time,urllib.request,urllib.error,copy
os.umask(0o077)
spec=importlib.util.spec_from_file_location('native_prepare','/home/vtplerp/epcerp-native-prepare.py');p=importlib.util.module_from_spec(spec);spec.loader.exec_module(p)
assert os.geteuid()==0
lock=open('/run/lock/epcerp-deployment.lock','w');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
root=pathlib.Path(p.inspect('frontend')['Config']['Labels']['com.docker.compose.project.working_dir']);os.chdir(root);release=pathlib.Path((root/'native-preflight-path.txt').read_text().strip());preflight=json.loads((release/'preflight-verification.json').read_text());rollbackTest=json.loads((release/'rollback-preflight-verification.json').read_text());assert preflight['schema_after']==101 and rollbackTest['schema_downgrade_required']is False
original=json.loads((release/'compose-before.private.json').read_text());candidate=json.loads((release/'compose-candidate.private.json').read_text());snapshots=json.loads((release/'runtime-before.private.json').read_text());assert json.loads((root/'compose.yaml').read_text())==original,'Live configuration changed after rehearsal'
for n,c in snapshots.items():
 current=p.inspect(n);assert current['Image']==c['Image'],'A service image changed after rehearsal'
schemaState=p.query('SELECT version,dirty FROM schema_migrations');assert schemaState in ['98|f','101|f'];schemaBefore=int(schemaState.split('|')[0])
for q in ["SELECT count(*)FROM tender.tsc_jobs WHERE state IN('queued','processing')","SELECT count(*)FROM document_readings WHERE status IN('queued','processing')","SELECT count(*)FROM document_package_receipts WHERE state IN('receiving','verifying','ready')"]:assert p.query(q)=='0','New active work; retry after completion'
backup=root/('pre-native-cutover-'+datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ'));backup.mkdir(mode=0o700)
for name in ['.env','.env.gemini','release.env','compose.yaml','Caddyfile']:
 f=root/name
 if f.exists():(backup/name).write_bytes(f.read_bytes());(backup/name).chmod(0o600)
p.write_json(backup/'runtime-before.private.json',snapshots);p.write_json(backup/'compose-before.private.json',original);p.write_json(backup/'compose-candidate.private.json',candidate)
api=p.environment(snapshots['api']);p.write_env(release/'live-schema.private.env',{'EPCERP_DATABASE_URL':api['EPCERP_DATABASE_URL'],'EPCERP_MIGRATION_ROOT':'/app/migrations'});p.write_env(backup/'storage.private.env',{k:v for k,v in api.items()if k.startswith('VTPL_STORAGE_')})
compose=['docker','compose','--project-directory',str(root)]
if(root/'release.env').exists():compose+=['--env-file',str(root/'release.env')]
compose+=['--env-file',str(root/'.env'),'-p',p.PROJECT,'-f',str(root/'compose.yaml')]
# Check interpolation and all private controls before stopping writers.
resolved=json.loads(p.output(compose[:-2]+['-f',str(release/'compose-candidate.private.json'),'config','--format','json']))
for n in ['frontend','frontend-lan']:
 e=resolved['services'][n]['environment'];assert e['EPCERP_GO_DISABLED']=='true';assert e['EPCERP_API_URL']=='http://127.0.0.1:1/api/v1'
 for k,v in api.items():
  if k.startswith('EPCERP_TSC_')or k in ['EPCERP_DATABASE_URL','VTPL_JWT_SECRET_KEY','EPCERP_IMAGE_ENCRYPTION_KEY','EPCERP_SUPPLIER_DOCUMENT_KEY','EPCERP_DOCUMENT_READING_KEY','VTPL_STORAGE_SECRET_KEY','VTPL_DOCUMENT_ENCRYPTION_KEY']:assert e.get(k)==v,'Private control was not preserved: '+k
writers=['frontend','frontend-lan','api','document-worker','document-package-worker','tsc-worker','tsc-reranker'];stopped=False;schemaApplied=False
sourceVolume=next(m['Name']for m in snapshots['api']['Mounts']if m['Destination']=='/var/lib/epcerp/document-packages');storageVolume=next(m['Name']for m in snapshots['storage']['Mounts']if m['Destination']=='/data')
walk=r"""const fs=require('fs'),crypto=require('crypto'),out={};function walk(dir,rel=''){for(const n of fs.readdirSync(dir).sort()){const f=dir+'/'+n,k=rel+n,s=fs.lstatSync(f);if(s.isDirectory())walk(f,k+'/');else if(s.isFile()){out[k]={size:s.size,sha256:crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex')};}else throw Error('Unsupported storage entry');}}walk('/data');process.stdout.write(JSON.stringify(out));"""
s3=r"""const sdk=require('@aws-sdk/client-s3'),crypto=require('crypto');const client=new sdk.S3Client({endpoint:'http://'+process.env.VTPL_STORAGE_ENDPOINT,region:'us-east-1',forcePathStyle:true,credentials:{accessKeyId:process.env.VTPL_STORAGE_ACCESS_KEY,secretAccessKey:process.env.VTPL_STORAGE_SECRET_KEY}});(async()=>{const out=[];for(const b of(await client.send(new sdk.ListBucketsCommand({}))).Buckets??[]){let t;do{const r=await client.send(new sdk.ListObjectsV2Command({Bucket:b.Name,ContinuationToken:t}));for(const o of r.Contents??[]){const v=await client.send(new sdk.GetObjectCommand({Bucket:b.Name,Key:o.Key})),h=crypto.createHash('sha256');for await(const c of v.Body)h.update(c);out.push({bucket:b.Name,key:o.Key,size:o.Size,etag:o.ETag,sha256:h.digest('hex')});}t=r.IsTruncated?r.NextContinuationToken:undefined;}while(t);}out.sort((a,b)=>(a.bucket+'/'+a.key).localeCompare(b.bucket+'/'+b.key));process.stdout.write(JSON.stringify(out));})().finally(()=>client.destroy()).catch(()=>process.exit(1));"""
def snapshot_volume(volume):return json.loads(p.output(['docker','run','--rm','--network','none','--user','0:0','-v',volume+':/data:ro',p.FRONT,'node','-e',walk]))
def snapshot_s3():return json.loads(p.output(['docker','run','--rm','--network',p.PROJECT+'_default','--env-file',str(backup/'storage.private.env'),p.FRONT,'node','-e',s3]))
def backup_volume(volume,name):
 before=snapshot_volume(volume);p.write_json(backup/(name+'-before.private.json'),before)
 p.run(['docker','run','--rm','--network','none','--user','0:0','-v',volume+':/data:ro','-v',str(backup)+':/backup',p.FRONT,'tar','--numeric-owner','-cf','/backup/'+name+'.tar','-C','/data','.'])
 actual={}
 with tarfile.open(backup/(name+'.tar'),'r')as archive:
  for entry in archive:
   if entry.isfile():
    stream=archive.extractfile(entry);actual[entry.name.removeprefix('./')]={'size':entry.size,'sha256':hashlib.file_digest(stream,'sha256').hexdigest()}
   else:assert entry.isdir(),'Unexpected archive member'
 assert actual==before,'Object archive does not match the original volume';return before
try:
 p.run(['docker','stop',*[p.PROJECT+'-'+n+'-1'for n in writers]]);stopped=True
 before=p.hashes();roles=json.loads(p.query("SELECT coalesce(jsonb_agg(to_jsonb(r)ORDER BY id),'[]')FROM auth.roles r"));p.write_json(backup/'tables-before.private.json',before);p.write_json(backup/'roles-before.private.json',roles)
 with(backup/'database.dump').open('wb')as f:p.run(['docker','exec',p.PROJECT+'-db-1','pg_dump','-U','epcerp','-d','epcerp','-Fc'],stdout=f)
 for args in [['--list'],['--file=/dev/null']]:
  with(backup/'database.dump').open('rb')as f:p.run(['docker','exec','-i',p.PROJECT+'-db-1','pg_restore',*args],stdin=f,stdout=subprocess.DEVNULL)
 p.write_json(backup/'s3-before.private.json',snapshot_s3());p.run(['docker','stop',p.PROJECT+'-storage-1'],stdout=subprocess.DEVNULL)
 try:sourceBefore=backup_volume(sourceVolume,'originals');backup_volume(storageVolume,'storage')
 finally:p.run(['docker','start',p.PROJECT+'-storage-1'],stdout=subprocess.DEVNULL)
 digests={f.name:hashlib.file_digest(f.open('rb'),'sha256').hexdigest()for f in backup.iterdir()if f.suffix in['.dump','.tar']};p.write_json(backup/'archive-sha256.json',digests)
 print('PASS: live writers stopped; database archive fully decoded and both storage archives verified against every original file',flush=True)
 p.run(['docker','run','--rm','--network',p.PROJECT+'_default','--env-file',str(release/'live-schema.private.env'),p.FRONT,'node','/app/tools/migrate.mjs','up']);schemaApplied=True;assert p.query('SELECT version,dirty FROM schema_migrations')=='101|f';assert p.hashes(tables=list(before))==before,'Live schema upgrade changed existing table contents'
 (root/'compose.yaml').write_text(json.dumps(candidate,indent=2)+'\n');(root/'compose.yaml').chmod(0o600)
 p.run(compose+['up','-d','--no-build','--wait','--wait-timeout','240','frontend','frontend-lan','document-processor'])
 after=p.hashes(tables=list(before));changed=[k for k in before if before[k]!=after[k]];assert set(changed)<={'auth.roles'},'Unexpected startup changes: '+str(changed)
 rolesAfter=json.loads(p.query("SELECT coalesce(jsonb_agg(to_jsonb(r)ORDER BY id),'[]')FROM auth.roles r"));assert len(roles)==len(rolesAfter)
 for a,b in zip(roles,rolesAfter):
  for k in a:
   if a[k]!=b[k]:assert a['name']=='superadmin'and k=='edit_version'and b[k]==a[k]+2,'Unexpected role change'
 assert snapshot_volume(sourceVolume)==sourceBefore,'Original files changed';assert snapshot_s3()==json.loads((backup/'s3-before.private.json').read_text()),'Stored objects changed'
 for n in p.LEGACY:
  c=p.inspect(n);assert not c['State']['Running'],'A legacy service is still running'
 # Same HTTPS and LAN reverse-proxy targets, certificates and access policy.
 for args in [['--resolve','erp.vishaltechnopower.in:443:127.0.0.1','https://erp.vishaltechnopower.in/api/health/ready'],['http://192.168.1.50/api/health/ready']]:p.run(['curl','--noproxy','*','--fail','--silent','--show-error','--max-time','30',*args],stdout=subprocess.DEVNULL)
 # Reuse GET-only checks through the LAN frontend; no cloud or mail requests.
 checks=p.http_checks('http://192.168.1.50',resolved['services']['frontend-lan']['environment'],'epcerp')
 assert p.query('SELECT count(*) FROM tender.tsc_model_calls')=='0','Unexpected provider call during cutover'
 for n in ['frontend','frontend-lan']:
  e=p.environment(p.inspect(n));assert e['EPCERP_PLATFORM_OWNER']=='nextjs'and e['EPCERP_GO_DISABLED']=='true'
  for k,v in api.items():
   if k.startswith('EPCERP_TSC_')or k in ['EPCERP_DATABASE_URL','VTPL_JWT_SECRET_KEY','EPCERP_IMAGE_ENCRYPTION_KEY','EPCERP_SUPPLIER_DOCUMENT_KEY','EPCERP_DOCUMENT_READING_KEY','VTPL_STORAGE_SECRET_KEY','VTPL_DOCUMENT_ENCRYPTION_KEY']:assert e.get(k)==v,'Deployed private control differs: '+k
 p.write_json(backup/'verification.json',{'source_application_commit':'72215c9788b40b79167c95bb4349ddff4e9cd939','schema_before_this_attempt':schemaBefore,'initial_schema_before_release':98,'schema_after':101,'database_reset_or_restore':False,'unchanged_existing_tables_after_migration':len(before),'startup_changed_tables':changed,'startup_role_change':'superadmin edit_version +2, one for each native frontend; all other role fields preserved','original_files_preserved':len(sourceBefore),'s3_objects_preserved':len(snapshot_s3()),'provider_calls':0,'mail_calls':0,'https_and_lan_preserved':True,'all_go_services_stopped':True,'private_backups_verified':True,'http_checks':checks,'images':{n:p.inspect(n)['Image']for n in ['frontend','frontend-lan','document-processor']},'verified_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'security_writes':'GET request quotas and document download audit only','preflight':str(release)})
 # Remove retired containers only after all checks; never remove their volumes/images.
 p.run(['docker','rm',*[p.PROJECT+'-'+n+'-1'for n in p.LEGACY]],stdout=subprocess.DEVNULL)
 (root/'native-cutover-backup-path.txt').write_text(str(backup)+'\n');print('PASS: development is on Next.js/Node; legacy containers removed; database, password hashes, encrypted originals, keys, HTTPS and LAN preserved',flush=True)
except BaseException:
 print('Cutover verification failed. Private backup retained at '+str(backup),flush=True)
 if stopped:
  # Keep any additive migrations and new records. No database downgrade or restore.
  (root/'compose.yaml').write_text(json.dumps(original,indent=2)+'\n')
  try:
   p.run(compose+['up','-d','--no-deps','--no-build','--wait','--wait-timeout','240','tsc-reranker','api','document-worker','document-package-worker'])
   p.run(compose+['up','-d','--no-deps','--no-build','--wait','--wait-timeout','240','tsc-worker','frontend','frontend-lan'])
   print('Prior application restored without restoring or resetting the database',flush=True)
  except BaseException:print('Application rollback needs inspection; all backups and volumes retained',flush=True)
 raise
