import assert from 'node:assert/strict';
import test from 'node:test';
import { randomUUID, randomBytes, createCipheriv } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { pool } from '../../src/server/db';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { createRole } from '../../src/server/auth/administration';
import { handleRegister, ownsRegister } from '../../src/server/tenders/register/http';
import { capturePermissions } from '../../src/server/tenders/register/access';
import { digest } from '../../src/server/tenders/tsc/crypto';
import { goJSON } from '../../src/server/tenders/intake/receipts';
const oracle = (value: unknown) => JSON.parse(execFileSync(process.env.EPCERP_TENDER_REGISTER_ORACLE!,[],{input:JSON.stringify(value)}).toString());
test('native register preserves existing Go contracts and transaction boundaries', {skip:!process.env.EPCERP_DATABASE_URL}, async t => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname,/^\/epcerp_auth_qa_register_/);
    await initializeAuth();
    const sql = pool(), token = (await login('superadmin',process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token;
    const actor = (await sql.query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0].id;
    t.beforeEach(async()=>{await sql.query('DELETE FROM auth.login_attempts WHERE identifier_hash=ANY($1::text[])',[['capture','read','assignment'].map(op=>digest(`http:tender:register:${op}:${actor}`))]);});
    const role = await createRole(token,{name:'RegisterQA'+randomUUID(),description:'Synthetic designated bundle',permission_codes:capturePermissions});
    process.env.EPCERP_TENDER_MANAGER_BUNDLE_IDS = role.id;
    const user = async (superuser: boolean) => {
        const id = randomUUID(), username = 'qa'+id.replaceAll('-','');
        await sql.query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)SELECT $1,$2,$3,password_hash,true,$4 FROM auth.users WHERE id=$5',[id,username,username+'@fixture.invalid',superuser,actor]);
        await sql.query('INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)',[id,role.id]);
        return {id,token:(await login(username,process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token};
    };
    const manager = await user(true), second = await user(true), outsider = await user(false);
    const request = (method: string,path: string,value?: unknown,key?: string,auth = token) => handleRegister(new Request('http://fixture.invalid/api/v1/tender-fms/'+path,{method,headers:{authorization:'Bearer '+auth,'content-type':'application/json',...(key?{'idempotency-key':key}:{})},...(value===undefined?{}:{body:JSON.stringify(value)})}));
    const json = async (method: string,path: string,value?: unknown,status = 200,key?: string,auth = token) => {const r = await request(method,path,value,key,auth);const b = await r.json();assert.equal(r.status,status,JSON.stringify(b));return b;};
    const receipt = async (owner = actor, expired = false) => {
        const id = randomUUID(), file = randomUUID(), claim = randomUUID(), member = randomUUID(), data = Buffer.from('%PDF-1.4\nSynthetic tender only\n%%EOF'), nonce = randomBytes(12), cipher = createCipheriv('aes-256-gcm',Buffer.from(process.env.EPCERP_DOCUMENT_READING_KEY!,'hex'),nonce), payload = Buffer.concat([nonce,cipher.update(data),cipher.final(),cipher.getAuthTag()]);
        await sql.query("INSERT INTO document_package_receipts(id,owner_id,state,claim_id,expires_at)VALUES($1,$2,'ready',$3,clock_timestamp()+$4*interval '1 hour')",[id,owner,claim,expired?-1:24]);
        await sql.query("INSERT INTO document_package_files(id,receipt_id,client_file_key,filename,kind,declared_size,declared_sha256,state,object_key)VALUES($1,$2,$3,'Tender.pdf','pdf',$4,$5,'ready',$6)",[file,id,randomUUID(),data.length,digest(data),randomUUID()]);
        await sql.query("INSERT INTO document_package_members(id,receipt_id,file_id,path,normalized_path,filename,kind,byte_size,expanded_bytes,sha256,reader_payload,reader_payload_sha256,validation_claim_id,scanner_revision,pages)VALUES($1,$2,$3,'Tender.pdf','tender.pdf','Tender.pdf','pdf',$4,$4,$5,$6,$7,$8,'synthetic',1)",[member,id,file,data.length,digest(data),payload,digest(payload),claim]);
        return id;
    };
    const input = async () => ({name:'Synthetic tender <&> ₹ '+randomUUID(),receipt_id:await receipt(),expected_receipt_version:1,manager_id:manager.id,external_reference:'QA<&>',source_url:'https://fixture.invalid/tender'});
    let record: Awaited<ReturnType<typeof json>>, captured: Awaited<ReturnType<typeof input>>, captureKey: string;
    try {
        await t.test('authorization precedes malformed bodies and candidate queries disclose only approved managers',async()=>{
            await json('POST','records',{},401,randomUUID(),'');
            await json('GET','assignment-options?page=bad',undefined,401,undefined,'');
            const out = await json('GET','assignment-options');
            assert.ok(out.items.some((c:{id:string})=>c.id===manager.id));assert.ok(!out.items.some((c:{id:string})=>c.id===outsider.id));
            assert.deepEqual(Object.keys(out.items[0]).sort(),['display_name','id']);
            const go = oracle({Operation:'options',Actor:actor});assert.equal(go.error,undefined);assert.deepEqual(out.items.map((c:{id:string})=>c.id),go.items.map((c:{ID:string})=>c.ID));
            await json('GET','assignment-options?page=1&page=2',undefined,422);
        });
        await t.test('empty or revoked designation fails closed even for superusers',async()=>{
            const previous = process.env.EPCERP_TENDER_MANAGER_BUNDLE_IDS;process.env.EPCERP_TENDER_MANAGER_BUNDLE_IDS='';assert.equal((await json('GET','assignment-options')).total,0);process.env.EPCERP_TENDER_MANAGER_BUNDLE_IDS=previous;
            const b = await input();await sql.query('DELETE FROM auth.user_roles WHERE user_id=$1',[manager.id]);await json('POST','records',b,422,randomUUID());await sql.query('INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)',[manager.id,role.id]);
            assert.equal((await sql.query('SELECT state FROM document_package_receipts WHERE id=$1',[b.receipt_id])).rows[0].state,'ready');
        });
        await t.test('capture and concurrent replay commit exactly one record, source, notice and delivery intent',async()=>{
            captured = await input();captureKey=randomUUID();const responses=await Promise.all([request('POST','records',captured,captureKey),request('POST','records',captured,captureKey)]);assert.deepEqual(responses.map(r=>r.status),[202,202]);assert.equal(responses.filter(r=>r.headers.has('idempotency-replayed')).length,1);record=await responses[0].json();assert.equal((await responses[1].json()).id,record.id);
            assert.equal(record.manager_id,manager.id);assert.equal(record.provisioning_state,'pending');assert.equal(record.blocker_code,'project_provider_pending');assert.equal(record.project_id,null);assert.equal(record.estimate_id,null);assert.equal(record.deliveries.length,1);
            assert.equal((await sql.query('SELECT count(*)FROM tender.tender_source_links WHERE tender_id=$1',[record.id])).rows[0].count,'1');
            assert.equal((await sql.query('SELECT count(*)FROM communication_notice_events WHERE notice_id=$1',[record.deliveries[0].notice_id])).rows[0].count,'1');
            const go=oracle({Operation:'read',Actor:actor,Tender:record.id});assert.equal(go.error,undefined);assert.equal(go.record.Name,record.name);assert.equal(go.record.AssignmentVersion,1);
            assert.equal(oracle({Operation:'notice',Action:record.deliveries[0].source_action_id}).notice,record.deliveries[0].notice_id);
            assert.equal((await sql.query('SELECT count(*)FROM communication_notice_events WHERE notice_id=$1',[record.deliveries[0].notice_id])).rows[0].count,'1');
        });
        await t.test('persisted fingerprints match Go and key reuse cannot alter the original input',async()=>{
            const Capture={Name:captured.name,ReceiptID:captured.receipt_id,ManagerID:captured.manager_id,ReceiptVersion:1,ExternalReference:captured.external_reference,SourceURL:captured.source_url};
            assert.equal(oracle({Operation:'fingerprint',Capture}).capture,digest(goJSON(Capture)));
            const saved=(await sql.query('SELECT fingerprint FROM tender.tender_capture_operations WHERE idempotency_key=$1',[captureKey])).rows[0];assert.equal(saved.fingerprint,digest(goJSON(Capture)));
            await json('POST','records',{...captured,name:'Different tender'},409,captureKey);
        });
        await t.test('receipt ownership, draft binding, expiry, version and consumed-state checks roll back all capture writes',async()=>{
            for (const variant of ['foreign','draft','expired','version','consumed']) {
                const b={...await input(),receipt_id:await receipt(variant==='foreign'?outsider.id:actor,variant==='expired')};
                if(variant==='draft'){const d=randomUUID();await sql.query("INSERT INTO tender.document_drafts(id,name,owner_id,operation_key)VALUES($1,'QA bound draft',$2,$3)",[d,actor,randomUUID()]);await sql.query('UPDATE document_package_receipts SET draft_id=$2 WHERE id=$1',[b.receipt_id,d]);}
                if(variant==='version')b.expected_receipt_version=2;
                if(variant==='consumed')b.receipt_id=captured.receipt_id;
                await json('POST','records',b,variant==='foreign'?404:409,randomUUID());assert.equal((await sql.query('SELECT count(*)FROM tender.tender_fms_intakes WHERE title=$1',[b.name])).rows[0].count,'0');
            }
        });
        await t.test('Inbox failure rolls back receipt consumption, record, sources, events and operations',async()=>{
            const b=await input(),key=randomUUID();await sql.query("CREATE FUNCTION public.qa_register_fail_notice() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'synthetic notice failure'; END$$; CREATE TRIGGER qa_register_fail_notice BEFORE INSERT ON communication_notices FOR EACH ROW EXECUTE FUNCTION public.qa_register_fail_notice()");
            try {await json('POST','records',b,503,key);}finally{await sql.query('DROP TRIGGER qa_register_fail_notice ON communication_notices; DROP FUNCTION public.qa_register_fail_notice()');}
            assert.equal((await sql.query('SELECT state FROM document_package_receipts WHERE id=$1',[b.receipt_id])).rows[0].state,'ready');assert.equal((await sql.query('SELECT count(*)FROM tender.tender_capture_operations WHERE idempotency_key=$1',[key])).rows[0].count,'0');assert.equal((await sql.query('SELECT count(*)FROM tender.tender_fms_intakes WHERE title=$1',[b.name])).rows[0].count,'0');
        });
        await t.test('participant visibility grants metadata only and expired or revoked grants deny access',async()=>{
            await json('GET','records/'+record.id,undefined,404,undefined,outsider.token);
            const grant=async(v:number,active:boolean,until:string|null)=>sql.query("INSERT INTO tender.tender_fms_participant_grants(id,intake_id,user_id,version,is_active,valid_until,reason,created_by_id)VALUES($1,$2,$3,$4,$5,$6,'Synthetic visibility grant',$7)",[randomUUID(),record.id,outsider.id,v,active,until,actor]);
            await grant(1,true,null);const visible=await json('GET','records/'+record.id,undefined,200,undefined,outsider.token);assert.equal(visible.can_edit,false);assert.equal(visible.can_assign,false);await json('PATCH','records/'+record.id,{name:record.name,expected_version:1},403,randomUUID(),outsider.token);
            await grant(2,true,'2000-01-01');await json('GET','records/'+record.id,undefined,404,undefined,outsider.token);await grant(3,false,null);await json('GET','records/'+record.id,undefined,404,undefined,outsider.token);
        });
        await t.test('metadata edits distinguish omitted fields from null, preserve content revision and reject stale races',async()=>{
            const key=randomUUID(),b={name:'Renamed synthetic tender',expected_version:1,external_reference:null};const responses=await Promise.all([request('PATCH','records/'+record.id,b,key),request('PATCH','records/'+record.id,b,key)]);assert.deepEqual(responses.map(r=>r.status),[200,200]);record=await responses[0].json();assert.equal(record.business_version,2);assert.equal(record.content_revision,1);assert.equal(record.assignment_version,1);
            const row=(await sql.query('SELECT external_reference,source_url FROM tender.tender_fms_intakes WHERE id=$1',[record.id])).rows[0];assert.equal(row.external_reference,null);assert.equal(row.source_url,captured.source_url);
            const Patch={SetReference:true,SetURL:false,Version:1,Name:b.name,ExternalReference:null,SourceURL:null};assert.equal(oracle({Operation:'fingerprint',Patch}).patch,(await sql.query('SELECT fingerprint FROM tender.tender_capture_operations WHERE idempotency_key=$1',[key])).rows[0].fingerprint);
            await json('PATCH','records/'+record.id,b,409,randomUUID());
        });
        await t.test('manager reassignment cancels obsolete work and mail, preserves immutable history and replays safely',async()=>{
            const prior=record.deliveries[0],key=randomUUID(),b={expected_version:2,expected_assignment_version:1,manager_id:second.id,reason:'Synthetic management reassignment'};
            record=await json('POST','records/'+record.id+'/manager-assignments',b,200,key);assert.equal(record.business_version,3);assert.equal(record.assignment_version,2);assert.equal(record.manager_id,second.id);assert.equal(record.deliveries.length,1);assert.notEqual(record.deliveries[0].notice_id,prior.notice_id);
            assert.equal((await sql.query('SELECT state FROM tender.tender_source_actions WHERE id=$1',[prior.source_action_id])).rows[0].state,'cancelled_obsolete');assert.equal((await sql.query('SELECT state FROM tender.tender_email_jobs WHERE id=$1',[prior.email_job_id])).rows[0].state,'cancelled_obsolete');
            assert.equal((await json('POST','records/'+record.id+'/manager-assignments',b,200,key)).assignment_version,2);
            assert.equal((await sql.query('SELECT count(*)FROM tender.tender_fms_manager_assignments WHERE intake_id=$1',[record.id])).rows[0].count,'2');
            assert.equal(oracle({Operation:'notice',Action:record.deliveries[0].source_action_id}).notice,record.deliveries[0].notice_id);
            const Change={Version:2,AssignmentVersion:1,ManagerID:second.id,Reason:b.reason};assert.equal(oracle({Operation:'fingerprint',Change}).manager,(await sql.query('SELECT fingerprint FROM tender.tender_capture_operations WHERE idempotency_key=$1',[key])).rows[0].fingerprint);
            await json('POST','records/'+record.id+'/manager-assignments',{...b,expected_version:3,expected_assignment_version:2,manager_id:outsider.id},422,randomUUID());
        });
        await t.test('register filters, sorting and scope remain bounded; replay rechecks live account state',async()=>{
            const out=await json('GET','records?search=Renamed%20synthetic&sort=name&direction=asc');assert.ok(out.items.some((r:{id:string})=>r.id===record.id));assert.equal((await json('GET','records?search=Renamed%20synthetic',undefined,200,undefined,outsider.token)).total,0);
            for(const q of ['page_size=10','page=0','direction=bogus','sort=sql','search=x&search=y'])await json('GET','records?'+q,undefined,422);
            await sql.query('UPDATE auth.users SET is_active=false WHERE id=$1',[second.id]);await json('GET','records/'+record.id,undefined,401,undefined,second.token);await sql.query('UPDATE auth.users SET is_active=true WHERE id=$1',[second.id]);
        });
        await t.test('legacy ownership handover preserves history and never changes named managers',async()=>{
            const legacy=randomUUID();await sql.query("INSERT INTO tender.tender_fms_intakes(id,title,legacy_title,owner_id,initial_sha256)VALUES($1,'Legacy synthetic','Legacy synthetic',$2,$3)",[legacy,actor,'b'.repeat(64)]);
            assert.equal((await json('GET',legacy+'/ownership')).owner_id,actor);
            await json('POST',legacy+'/ownership',{owner_id:second.id,version:1,reason:'Synthetic legacy handover'},200);assert.equal((await sql.query('SELECT count(*)FROM tender.tender_fms_owner_changes WHERE intake_id=$1',[legacy])).rows[0].count,'1');
            await json('POST',legacy+'/ownership',{owner_id:manager.id,version:1,reason:'Synthetic stale handover'},422);
            await json('GET',record.id+'/ownership',undefined,404);
            assert.equal((await sql.query('SELECT owner_id FROM tender.tender_fms_intakes WHERE id=$1',[record.id])).rows[0].owner_id,second.id);
        });
        await t.test('route ownership excludes drafts, cloud TSC and retained legacy document reads',()=>{
            for(const p of ['/tender-fms/records','/tender-fms/assignment-options','/tender-fms/'+randomUUID()+'/ownership'])assert.equal(ownsRegister(p),true);
            for(const p of ['/tender-fms/drafts','/tender-fms/receipts','/tender-fms/'+randomUUID()+'/history','/tender-fms/drafts/'+randomUUID()+'/tsc/options'])assert.equal(ownsRegister(p),false);
        });
    } finally {await sql.end();}
});
