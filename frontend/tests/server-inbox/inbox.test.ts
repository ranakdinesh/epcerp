import assert from 'node:assert/strict';
import test from 'node:test';
import { randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { createRole } from '../../src/server/auth/administration';
import { pool } from '../../src/server/db';
import { handleInbox, ownsInbox } from '../../src/server/inbox/http';
import { project,fixture } from './fixtures';
const oracle = (value: unknown) => JSON.parse(execFileSync(process.env.EPCERP_INBOX_ORACLE!,[],{input:JSON.stringify(value)}).toString());
test('Inbox preserves Go recipient policy, versions, timestamp precision and immutable history', {skip:!process.env.EPCERP_DATABASE_URL},async t=>{
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname,/^\/epcerp_auth_qa_inbox_/);
    await initializeAuth(); const sql=pool(), password=process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!,token=(await login('superadmin',password)).access_token;
    const actor=(await sql.query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0].id, p=await project(actor);
    const role=await createRole(token,{name:'InboxReader'+randomUUID(),description:'Synthetic Inbox reader',permission_codes:['communication.inbox.read']});
    const other=randomUUID(),username='qa'+other.replaceAll('-','');
    await sql.query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)SELECT $1,$2,$3,password_hash,true,false FROM auth.users WHERE id=$4',[other,username,username+'@fixture.invalid',actor]);await sql.query('INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)',[other,role.id]);
    const reader=(await login(username,password)).access_token;
    const raw=(method:string,path:string,value?:unknown,auth=token)=>handleInbox(new Request('http://fixture.invalid/api/v1/inbox'+path,{method,headers:{authorization:'Bearer '+auth,'content-type':'application/json'},...(value===undefined?{}:{body:JSON.stringify(value)})}));
    const json=async(method:string,path:string,value?:unknown,status=200,auth=token)=>{const r=await raw(method,path,value,auth),v=await r.json();assert.equal(r.status,status,JSON.stringify(v));return v;};
    const ids: Record<string,string>={};
    try {
        for(const kind of ['director','quotation','supplier_review','tender_manager'] as const)ids[kind]=await fixture(kind,actor,actor,p);
        await t.test('anonymous access is denied and personal scope excludes other recipients even for superusers',async()=>{
            await json('GET','',undefined,401,'');await json('GET','/'+ids.director,undefined,404,reader);
            const foreign=await fixture('quotation',other,actor,p);await json('GET','/'+foreign,undefined,404);await json('POST','/'+foreign+'/read',{version:1},404);
            assert.equal((await json('GET','',undefined,200,reader)).items.length,1);
        });
        await t.test('all notice kinds and history serialize identically to Go, retaining microseconds and null UUID fields',async()=>{
            for(const id of Object.values(ids)){
                const native=await json('GET','/'+id),go=oracle({Actor:actor,Method:'GET',Path:'/'+id});assert.equal(go.status,200);assert.deepEqual(native,go.body);assert.equal(native.notice.created_at,'2026-01-01T01:02:03.123456Z');
            }
            const native=await json('GET','?size=50'),go=oracle({Actor:actor,Method:'GET',Path:'?size=50'});assert.deepEqual(native,go.body);
        });
        await t.test('opening a notice does not change its read state or append history',async()=>{
            const before=await json('GET','/'+ids.quotation);const again=await json('GET','/'+ids.quotation);assert.deepEqual(again,before);assert.equal(again.notice.read_at,null);assert.equal(again.notice.version,1);
        });
        await t.test('native read commits one receipt and matching project-scoped journal event, consumable by Go',async()=>{
            const id=ids.quotation,saved=await json('POST','/'+id+'/read',{version:1});assert.equal(saved.notice.version,2);assert.ok(saved.notice.read_at);assert.equal(saved.notice.acknowledged_at,null);assert.equal(saved.history.length,1);assert.equal(saved.history[0].before.version,1);assert.equal(saved.history[0].before.created_at,'2026-01-01T01:02:03.123456Z');
            assert.deepEqual(oracle({Actor:actor,Method:'GET',Path:'/'+id}).body,saved);
            const event=(await sql.query('SELECT * FROM business_events WHERE id=$1',[saved.history[0].id])).rows[0];assert.equal(event.scope_kind,'project');assert.equal(event.scope_id,p);assert.equal(event.correlation_id,saved.notice.source_event_id);
        });
        await t.test('same-version repeated receipts are no-ops; stale receipts fail without duplicate events',async()=>{
            const path='/'+ids.quotation+'/read',saved=await json('POST',path,{version:2});assert.equal(saved.notice.version,2);assert.equal(saved.history.length,1);await json('POST',path,{version:1},409);assert.deepEqual(oracle({Actor:actor,Method:'POST',Path:path,Body:{version:2}}).body,saved);
        });
        await t.test('Go acknowledgement of native receipt history remains readable by Next, without overwriting read time',async()=>{
            const id=ids.quotation,before=await json('GET','/'+id),go=oracle({Actor:actor,Method:'POST',Path:'/'+id+'/acknowledge',Body:{version:2}});assert.equal(go.status,200);const native=await json('GET','/'+id);assert.deepEqual(native,go.body);assert.equal(native.notice.read_at,before.notice.read_at);assert.equal(native.notice.version,3);assert.equal(native.history.length,2);
        });
        await t.test('acknowledging an unread notice sets both timestamps once and publishes platform scope',async()=>{
            const id=ids.supplier_review,saved=await json('POST','/'+id+'/acknowledge',{version:1});assert.equal(saved.notice.version,2);assert.equal(saved.notice.read_at,saved.notice.acknowledged_at);assert.equal(saved.history[0].after.version,2);assert.deepEqual(oracle({Actor:actor,Method:'GET',Path:'/'+id}).body,saved);
            const e=(await sql.query('SELECT scope_kind,scope_id FROM business_events WHERE id=$1',[saved.history[0].id])).rows[0];assert.equal(e.scope_kind,'platform');assert.equal(e.scope_id,null);
        });
        await t.test('concurrent receipts with the same expected version produce one winner and one stale response',async()=>{
            const id=await fixture('quotation',actor,actor,p),rs=await Promise.all([raw('POST','/'+id+'/read',{version:1}),raw('POST','/'+id+'/acknowledge',{version:1})]);assert.deepEqual(rs.map(r=>r.status).sort(),[200,409]);assert.equal((await json('GET','/'+id)).history.length,1);
        });
        await t.test('failed history insertion rolls back the notice and journal update atomically',async()=>{
            const id=await fixture('quotation',actor,actor,p);await sql.query("CREATE FUNCTION qa_inbox_fail() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'synthetic history failure';END$$;CREATE TRIGGER qa_inbox_fail BEFORE INSERT ON communication_notice_events FOR EACH ROW EXECUTE FUNCTION qa_inbox_fail()");
            try{await json('POST','/'+id+'/read',{version:1},503);}finally{await sql.query('DROP TRIGGER qa_inbox_fail ON communication_notice_events;DROP FUNCTION qa_inbox_fail()');}
            const saved=await json('GET','/'+id);assert.equal(saved.notice.version,1);assert.equal(saved.notice.read_at,null);assert.equal(saved.history.length,0);assert.equal((await sql.query("SELECT count(*)FROM business_events WHERE entity_id=$1 AND source_module='communication_inbox'",[id])).rows[0].count,'0');
        });
        await t.test('read-only users cannot record receipts and live revocation immediately denies access',async()=>{
            const out=await json('GET','',undefined,200,reader);assert.equal(out.can_write,false);await json('POST','/'+out.items[0].id+'/read',{version:1},403,reader);
            await sql.query('UPDATE auth.roles SET is_active=false WHERE id=$1',[role.id]);await json('GET','',undefined,403,reader);await sql.query('UPDATE auth.roles SET is_active=true WHERE id=$1',[role.id]);
            await sql.query('UPDATE auth.users SET is_active=false WHERE id=$1',[other]);await json('GET','',undefined,401,reader);await sql.query('UPDATE auth.users SET is_active=true WHERE id=$1',[other]);
        });
        await t.test('obsolete Tender notices retain recipient receipts without altering assignment state',async()=>{
            const id=ids.tender_manager;await sql.query("UPDATE tender.tender_source_actions SET state='cancelled_obsolete',is_active=false WHERE notice_id=$1",[id]);const saved=await json('POST','/'+id+'/acknowledge',{version:1});assert.equal(saved.notice.version,2);assert.equal((await sql.query('SELECT state FROM tender.tender_source_actions WHERE notice_id=$1',[id])).rows[0].state,'cancelled_obsolete');assert.deepEqual(oracle({Actor:actor,Method:'GET',Path:'/'+id}).body,saved);
        });
        await t.test('bounded filters match Go, and malformed commands never change a receipt',async()=>{
            for(const q of ['','?status=unread','?status=read','?status=acknowledged','?page=100000&size=100'])assert.deepEqual(await json('GET',q),oracle({Actor:actor,Method:'GET',Path:q}).body);
            for(const q of ['?size=10','?page=0','?status=other','?size=20&size=50','?unknown=1'])await json('GET',q,undefined,422);
            for(const body of [{version:0},{version:1.5},{version:2147483648},{version:1,extra:true},{}])await json('POST','/'+ids.director+'/read',body,422);
            await json('GET','/'+ids.director+'?unknown=1',undefined,422);assert.equal((await json('GET','/'+ids.director.toUpperCase())).notice.version,1);
            const future=await fixture('quotation',actor,actor,p,'2099-01-01T00:00:00.123456Z');await json('POST','/'+future+'/read',{version:1},422);
            assert.equal(ownsInbox('/inbox'),true);assert.equal(ownsInbox('/inbox/'+ids.director),true);assert.equal(ownsInbox('/inboxes'),false);
        });
    }finally{await sql.end();}
});
