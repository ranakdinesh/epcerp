import assert from 'node:assert/strict';
import test from 'node:test';
import { randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { createRole } from '../../src/server/auth/administration';
import { pool } from '../../src/server/db';
import { project as fixture } from '../server-inbox/fixtures';
import { handleTeam, ownsTeam } from '../../src/server/projects/team/http';
import { scoped } from '../../src/server/projects/team/validation';
const oracle = (Actor: string, Method: string, Path: string, Body?: unknown) => JSON.parse(execFileSync(process.env.EPCERP_TEAM_ORACLE!, [], { input: JSON.stringify({ Actor, Method, Path, Body }) }).toString());
test('native project team preserves Go scope, eligibility and immutable assignment history', { skip: !process.env.EPCERP_DATABASE_URL }, async t => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /^\/epcerp_auth_qa_team_/);
    await initializeAuth(); const sql = pool(); await sql.query("UPDATE auth.users SET is_active=false WHERE username LIKE 'bound%'"); const password = process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!, token = (await login('superadmin', password)).access_token;
    const actor = (await sql.query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0].id;
    const user = async (codes: string[]) => {
        const role = await createRole(token, { name: 'TeamQA'+randomUUID(), description: 'Synthetic team grants', permission_codes: codes }), id = randomUUID(), username = 'qa'+id.replaceAll('-', '');
        await sql.query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)SELECT $1,$2,$3,password_hash,true,false FROM auth.users WHERE id=$4', [id, username, username+'@fixture.invalid', actor]);
        await sql.query('INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)', [id, role.id]);
        return { id, role: role.id, token: (await login(username, password)).access_token };
    };
    const manager = await user(['projects.team.read','projects.team.write']), other = await user(['projects.team.read','projects.team.write']), specialist = await user(['projects.team.read','projects.dashboard.read','projects.budgets.approve']), portfolio = await user(['projects.team.read','projects.portfolio.read']), writer = await user(['projects.team.read','projects.team.write','projects.portfolio.write']);
    const project = await fixture(manager.id), base = '/'+project+'/responsibilities';
    const body = () => ({ user_id: specialist.id, bundle_id: specialist.role, responsibility: 'Budget reviewer '+randomUUID(), permission_codes: ['projects.team.read','projects.budgets.approve','projects.dashboard.read'], is_active: true, version: 0, reason: 'Synthetic assignment' });
    const raw = (method: string, path: string, value?: unknown, auth = token) => handleTeam(new Request('http://fixture.invalid/api/v1/projects'+path, { method, headers: { Authorization: 'Bearer '+auth, 'Content-Type': 'application/json' }, ...(value === undefined ? {} : { body: JSON.stringify(value) }) }));
    const json = async (method: string, path: string, value?: unknown, status = 200, auth = token) => { const r = await raw(method,path,value,auth), v = await r.json(); assert.equal(r.status,status,JSON.stringify(v)); return v; };
    const compare = async (path: string, auth = token, who = actor) => { const native = await json('GET',path,undefined,200,auth), go = oracle(who,'GET',path); assert.equal(go.status,200,JSON.stringify(go)); assert.deepEqual(native,go.body); return native; };
    const gate = (who: string, code: string, Write = false) => JSON.parse(execFileSync(process.env.EPCERP_TEAM_ORACLE!, [], { input: JSON.stringify({ Actor: who, Operation: 'gate', Project: project, Code: code, Write }) }).toString()).status;
    let p: Awaited<ReturnType<typeof json>>; const b = body();
    try {
        await t.test('anonymous, outsiders and read-only portfolio cannot manage teams', async () => {
            await json('GET',base,undefined,401,''); await json('GET',base,undefined,403,other.token);
            await compare(base,portfolio.token,portfolio.id); await json('GET',base+'/options',undefined,403,portfolio.token); await json('POST',base,b,403,portfolio.token);
            await json('POST',base,b,403,other.token); await json('GET','/'+randomUUID()+'/responsibilities',undefined,404);
        });
        await t.test('options match live Go role directory and scoped permission filtering', async () => {
            const options = await compare(base+'/options',manager.token,manager.id);
            const role = options.bundles.find((r:{id:string})=>r.id===specialist.role); assert.deepEqual(role.permissions,[...b.permission_codes].sort());
            assert.ok(!options.bundles.flatMap((r:{permissions:string[]})=>r.permissions).some((c:string)=>!scoped(c)));
            await sql.query('UPDATE auth.roles SET is_active=false WHERE id=$1',[specialist.role]); const disabled=await compare(base+'/options');assert.ok(!disabled.bundles.some((r:{id:string})=>r.id===specialist.role)); await sql.query('UPDATE auth.roles SET is_active=true WHERE id=$1',[specialist.role]);
        });
        await t.test('native assignment is readable by Go with correlated audit and no project version mutation', async () => {
            const before=(await sql.query('SELECT version FROM projects WHERE id=$1',[project])).rows[0].version;
            p=await json('POST',base,b,201,manager.token);assert.equal(p.version,1);assert.equal(p.user_name,'');assert.equal(p.bundle_name,'');
            await compare(base);const h=await compare(base+'/'+p.id+'/history');assert.equal(h.length,1);assert.equal(h[0].before,null);assert.deepEqual(h[0].after,p);
            assert.equal((await sql.query("SELECT count(*) FROM business_events WHERE id=$1 AND source_module='project_access' AND action='assign' AND entity_version=1",[h[0].id])).rows[0].count,'1');assert.equal((await sql.query('SELECT version FROM projects WHERE id=$1',[project])).rows[0].version,before);
        });
        await t.test('Go-created profiles remain editable natively and both histories match', async () => {
            const draft=body(),go=oracle(manager.id,'POST',base,draft);assert.equal(go.status,201,JSON.stringify(go));
            await compare(base);const edit=await json('PATCH',base+'/'+go.body.id,{...draft,version:1,responsibility:draft.responsibility+' revised'},200,manager.token);assert.equal(edit.version,2);await compare(base+'/'+edit.id+'/history');
            const revised=oracle(manager.id,'PATCH',base+'/'+edit.id,{...draft,version:2,responsibility:draft.responsibility+' Go'});assert.equal(revised.status,200,JSON.stringify(revised));await compare(base);await compare(base+'/'+edit.id+'/history');
        });
        await t.test('scoped team reader works while financial decisions require explicit assignments', async () => {
            await compare(base,specialist.token,specialist.id);await json('GET',base+'/options',undefined,403,specialist.token);
            assert.equal(gate(specialist.id,'projects.budgets.approve',true),200);assert.equal(gate(manager.id,'projects.budgets.approve',true),403);
            const e=await json('PATCH',base+'/'+p.id,{...b,version:1,permission_codes:['projects.team.read']});p=e;await compare(base+'/'+p.id+'/history');assert.equal(gate(specialist.id,'projects.budgets.approve',true),200); // another Go-created profile still grants it
        });
        await t.test('live role, membership and account revocation immediately remove scoped access', async () => {
            await sql.query('DELETE FROM auth.user_roles WHERE user_id=$1 AND role_id=$2',[specialist.id,specialist.role]);await json('GET',base,undefined,403,specialist.token);assert.equal(oracle(specialist.id,'GET',base).status,403);
            await json('POST',base,body(),422);await sql.query('INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)',[specialist.id,specialist.role]);
            await sql.query('UPDATE auth.roles SET is_active=false WHERE id=$1',[specialist.role]);await json('POST',base,body(),422);await json('GET',base,undefined,403,specialist.token);await sql.query('UPDATE auth.roles SET is_active=true WHERE id=$1',[specialist.role]);
            await sql.query('UPDATE auth.users SET is_active=false WHERE id=$1',[specialist.id]);await json('POST',base,body(),422);await json('GET',base,undefined,401,specialist.token);await sql.query('UPDATE auth.users SET is_active=true WHERE id=$1',[specialist.id]);
        });
        await t.test('revoking the last scoped profile immediately removes team visibility', async () => {
            const scopedReader=await user(['projects.team.read']);const draft={...body(),user_id:scopedReader.id,bundle_id:scopedReader.role,permission_codes:['projects.team.read']};
            const record=await json('POST',base,draft,201);await compare(base,scopedReader.token,scopedReader.id);
            await json('PATCH',base+'/'+record.id,{...draft,version:1,is_active:false});await json('GET',base,undefined,403,scopedReader.token);assert.equal(oracle(scopedReader.id,'GET',base).status,403);
        });
        await t.test('ineligible role permissions and globally owning authority cannot be assigned', async () => {
            for(const code of ['projects.team.write','projects.master.write','projects.portfolio.read','projects.manager_assignments.write','projects.purchase_assignments.write','projects.purchase_portfolio.write','projects.processes.bypass','system:admin','admin.users.write']) await json('POST',base,{...body(),permission_codes:[code]},422);
            await json('POST',base,{...body(),permission_codes:['projects.planning.write']},422);
            await json('POST',base,{...body(),bundle_id:manager.role},422);
            for(const responsibility of [' PM ','Project Manager','procurement executive','Purchase Manager'])await json('POST',base,{...body(),responsibility},422);
        });
        await t.test('immutable identities, duplicate active keys and stale versions are enforced', async () => {
            await json('POST',base,b,409);await json('PATCH',base+'/'+p.id,{...b,version:0},409);await json('PATCH',base+'/'+p.id,{...b,version:p.version,user_id:other.id},409);await json('PATCH',base+'/'+p.id,{...b,version:p.version,bundle_id:manager.role},409);
            const go=oracle(actor,'PATCH',base+'/'+p.id,{...b,version:p.version,user_id:other.id});assert.equal(go.status,409);
        });
        await t.test('concurrent edits serialize and immutable snapshots retain original wording', async () => {
            const rs=await Promise.all([raw('PATCH',base+'/'+p.id,{...b,version:p.version,responsibility:b.responsibility+' one'}),raw('PATCH',base+'/'+p.id,{...b,version:p.version,responsibility:b.responsibility+' two'})]);assert.deepEqual(rs.map(r=>r.status).sort(),[200,409]);
            p=await (rs.find(r=>r.ok)!).json();const h=await compare(base+'/'+p.id+'/history');assert.equal(h.at(-1).after.responsibility,b.responsibility);assert.equal(h[0].before.version,p.version-1);
        });
        await t.test('closed or inactive projects forbid active edits but allow revocation despite role loss', async () => {
            await sql.query("UPDATE projects SET lifecycle_status='closed' WHERE id=$1",[project]);await json('POST',base,body(),422);await json('PATCH',base+'/'+p.id,{...b,version:p.version},422);
            await compare(base,manager.token,manager.id);await sql.query('UPDATE auth.roles SET is_active=false WHERE id=$1',[specialist.role]);
            p=await json('PATCH',base+'/'+p.id,{...b,version:p.version,is_active:false},200,manager.token);assert.equal(p.is_active,false);await compare(base+'/'+p.id+'/history');
            await sql.query("UPDATE projects SET lifecycle_status='draft',is_active=false WHERE id=$1",[project]);await json('POST',base,body(),422);await sql.query('UPDATE projects SET is_active=true WHERE id=$1',[project]);await sql.query('UPDATE auth.roles SET is_active=true WHERE id=$1',[specialist.role]);
            await json('POST',base,{...body(),is_active:false},422);
        });
        await t.test('failed event/journal writes roll back profile and history together', async () => {
            const draft=body();await sql.query("CREATE FUNCTION qa_team_fail() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN IF NEW.source_module='project_access' THEN RAISE EXCEPTION 'synthetic journal failure';END IF;RETURN NEW;END$$;CREATE TRIGGER qa_team_fail BEFORE INSERT ON business_events FOR EACH ROW EXECUTE FUNCTION qa_team_fail()");
            try {await json('POST',base,draft,503);}finally{await sql.query('DROP TRIGGER qa_team_fail ON business_events;DROP FUNCTION qa_team_fail()');}
            assert.equal((await sql.query('SELECT count(*)FROM project_responsibility_profiles WHERE responsibility=$1',[draft.responsibility])).rows[0].count,'0');
        });
        await t.test('required types, nulls, unknown fields and bounded input are rejected', async () => {
            for(const key of Object.keys(b)){const copy:Record<string,unknown>={...body()};delete copy[key];await json('POST',base,copy,422);await json('POST',base,{...body(),[key]:null},422);}
            for(const extra of [{version:0.5},{version:2147483648},{is_active:'true'},{permission_codes:['projects.team.read','projects.team.read']},{permission_codes:[]},{reason:'x'.repeat(2001)},{responsibility:'界'.repeat(81)},{surprise:true}])await json('POST',base,{...body(),...extra},422);
            await json('POST',base,{...body(),reason:'x'.repeat(33000)},413);await json('GET',base+'/'+randomUUID()+'/history',undefined,404);
            assert.ok(ownsTeam('/projects/'+project+'/responsibilities'));assert.ok(!ownsTeam('/projects/'+project+'/purchase-workflow'));assert.ok(!ownsTeam('/projects/'+project+'/responsibilities/x/unknown'));
        });
        await t.test('profile bounds reject overfull workspaces and history remains bounded', async () => {
            const target=await fixture(manager.id);await sql.query("INSERT INTO project_responsibility_profiles(id,project_id,user_id,bundle_id,responsibility,responsibility_key,permission_codes,is_active,version)SELECT gen_random_uuid(),$1,$2,$3,'Synthetic bound '||n,'synthetic bound '||n,'[\"projects.team.read\"]',true,1 FROM generate_series(1,1001)n",[target,specialist.id,specialist.role]);
            await json('GET','/'+target+'/responsibilities',undefined,422);assert.equal(oracle(actor,'GET','/'+target+'/responsibilities').status,422);await json('POST','/'+target+'/responsibilities',body(),422);
            await sql.query("INSERT INTO project_responsibility_events(id,profile_id,project_id,action,reason,before_snapshot,after_snapshot,created_by_id)SELECT gen_random_uuid(),$1,$2,'revise','Synthetic bounded history','null',$3,$4 FROM generate_series(1,205)",[p.id,project,JSON.stringify(p),actor]);assert.equal((await compare(base+'/'+p.id+'/history')).length,200);
        });
        await t.test('assignment and role directories preserve their supported bounds', async () => {
            const ids=(await sql.query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)SELECT gen_random_uuid(),'bound'||x||n,'bound'||x||n||'@fixture.invalid',u.password_hash,true,false FROM auth.users u CROSS JOIN generate_series(1,1001)n CROSS JOIN(SELECT replace(gen_random_uuid()::text,'-','')x)s WHERE u.id=$1 RETURNING id",[actor])).rows.map(r=>r.id);
            try{await json('GET',base+'/options',undefined,422);assert.equal(oracle(actor,'GET',base+'/options').status,422);}finally{await sql.query('UPDATE auth.users SET is_active=false WHERE id=ANY($1::uuid[])',[ids]);}
            const roles=(await sql.query("INSERT INTO auth.roles(id,name,description,is_active)SELECT gen_random_uuid(),'BoundRole'||x||n,'Synthetic directory bound',true FROM generate_series(1,1001)n CROSS JOIN(SELECT replace(gen_random_uuid()::text,'-','')x)s RETURNING id")).rows.map(r=>r.id);
            try{await json('GET',base+'/options',undefined,422);assert.equal(oracle(actor,'GET',base+'/options').status,422);}finally{await sql.query('DELETE FROM auth.roles WHERE id=ANY($1::uuid[])',[roles]);}
        });
        await t.test('manager handover immediately changes management scope', async () => {
            await sql.query('UPDATE projects SET project_manager_id=$2 WHERE id=$1',[project,other.id]);await json('GET',base,undefined,403,manager.token);await json('POST',base,body(),403,manager.token);await compare(base,other.token,other.id);await compare(base+'/options',writer.token,writer.id);
        });
    } finally { await sql.end(); }
});
