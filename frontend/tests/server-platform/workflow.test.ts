import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID, createCipheriv, createHash } from 'node:crypto';
import { pool, transaction } from '../../src/server/db';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { handleOrganisation } from '../../src/server/platform/organisation';
import { handleProfile } from '../../src/server/platform/profile';
import { handleGovernance } from '../../src/server/platform/governance';
import { handleCalendar } from '../../src/server/platform/calendar';
import { inventory } from '../../src/server/platform/inventory';
import { encrypt, decrypt } from '../../src/server/platform/storage';
import { health } from '../../src/server/platform/health';
import { handleLegacy } from '../../src/server/tenders/tsc/legacy';
import { encrypt as seal } from '../../src/server/tenders/tsc/crypto';
import { unzipSync, strFromU8 } from 'fflate';
test('remaining native platform contracts without a Go backend', { skip: !process.env.EPCERP_DATABASE_URL }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /^\/epcerp_auth_qa_retire_/);
    await initializeAuth();
    const sql = pool(), password = process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!, token = (await login('superadmin', password)).access_token, admin = (await sql.query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0].id;
    const call = async (handler: (r: Request) => Promise<Response>, path: string, method = 'GET', value?: unknown, status = 200, access = token) => {
        const response = await handler(new Request('http://fixture.invalid/api/v1' + path, { method, headers: { Authorization: 'Bearer ' + access, 'Content-Type': 'application/json' }, body: value === undefined ? undefined : JSON.stringify(value) }));
        const data = await response.json();
        assert.equal(response.status, status, JSON.stringify(data));
        return data;
    };
    const org = (path: string, method = 'GET', value?: unknown, status = 200, access = token) => call(handleOrganisation, '/auth/organisation/' + path, method, value, status, access);
    const user = randomUUID(), username = 'staff' + user.replaceAll('-', '');
    await sql.query("INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)SELECT $1,$2,$3,password_hash,true,false FROM auth.users WHERE id=$4", [user, username, username + '@fixture.invalid', admin]);
    await sql.query('INSERT INTO auth.user_roles(user_id,role_id)SELECT $1,id FROM auth.roles WHERE name=\'superadmin\'', [user]);
    const staff = (await login(username, password)).access_token;
    try {
        await t.test('health checks the owned clean schema', async () => {
            assert.equal((await health(true)).status, 200);
        });
        await t.test('AES framing reads original Go nonce+cipher+tag and rejects corrupt data', () => {
            const key = createHash('sha256').update(process.env.VTPL_JWT_SECRET_KEY! + ':vtpl_secure_doc_salt').digest(), nonce = Buffer.alloc(12, 7), cipher = createCipheriv('aes-256-gcm', key, nonce), plain = Buffer.from('Old Go public tender Σ'), old = Buffer.concat([nonce, cipher.update(plain), cipher.final(), cipher.getAuthTag()]);
            assert.deepEqual(decrypt(old), plain);
            assert.deepEqual(decrypt(encrypt(plain)), plain);
            old[14] ^= 1;
            assert.throws(() => decrypt(old));
        });
        await t.test('director designation keeps history, identity and stale fences', async () => {
            let v = await call(handleGovernance, '/administration/director');
            assert.equal(v.record, null);
            v = await call(handleGovernance, '/administration/director', 'POST', {
                user_id: admin, version: 0, is_active: true, reason: 'Appoint initial director'
            });
            assert.equal(v.is_effective, true);
            const id = v.record.id;
            await call(handleGovernance, '/administration/director', 'POST', {
                user_id: user, version: 0, is_active: true, reason: 'Reject stale designation'
            }, 409);
            v = await call(handleGovernance, '/administration/director', 'POST', {
                user_id: admin, version: 1, is_active: false, reason: 'Revoke current director'
            });
            assert.equal(v.is_effective, false);
            assert.equal(v.record.id, id);
            assert.equal(v.history.length, 2);
        });
        await t.test('holiday duplicate dates and stale edits roll back audit', async () => {
            const b = {
                name: 'Synthetic closure', holiday_date: '2026-12-24', is_active: true, reason: 'QA calendar update', expected_version: 0
            };
            const v = await call(handleCalendar, '/work-calendar/holidays', 'POST', b);
            await call(handleCalendar, '/work-calendar/holidays', 'POST', b, 409);
            await call(handleCalendar, '/work-calendar/holidays/' + v.id, 'PATCH', { ...b, expected_version: 9 }, 409);
            const list = await call(handleCalendar, '/work-calendar/holidays?year=2026');
            assert.equal(list.length, 1);
            assert.equal(Number((await sql.query('SELECT count(*) FROM work_calendar_changes')).rows[0].count), 1);
        });
        let department: string;
        await t.test('organisation hierarchy blocks cycles and deactivation of an occupied department', async () => {
            department = (await org('departments', 'POST', {
                code: 'QA', name: 'Synthetic department', manager_id: '', is_active: true, version: 0, reason: 'Create QA reporting department'
            })).id;
            const data = {
                first_name: 'QA', last_name: 'Manager', mobile: '+91 9876543210', department_id: department, manager_id: '', is_company_root: true, version: 0, reason: 'Set up company reporting root'
            };
            await org('employees/' + admin, 'PATCH', data);
            await org('employees/' + user, 'PATCH', {
                ...data, first_name: 'QA', last_name: 'Employee', manager_id: admin, is_company_root: false
            });
            await org('employees/' + admin, 'PATCH', {
                ...data, version: 1, manager_id: user, is_company_root: false
            }, 422);
            await org('departments/' + department, 'PATCH', {
                code: 'QA', name: 'Synthetic department', manager_id: '', is_active: false, version: 1, reason: 'Cannot remove occupied department'
            }, 409);
            const team = await org('team');
            assert.ok(team.some((u: {
                id: string;
            }) => u.id === user));
            const work = await org('team/' + user + '/work');
            assert.deepEqual(work, []);
            assert.ok((await transaction(db => inventory(db, admin))).some(w => w.Kind === 'reporting_manager'));
        });
        await t.test('profile contacts use optimistic values and hide personal data from others', async () => {
            const own = await call(handleProfile, '/auth/user_profile/me', 'GET', undefined, 200, staff);
            assert.equal(own.contact.first_name, 'QA');
            await call(handleProfile, '/auth/user_profile/me', 'PATCH', {
                display_name: 'Tender staff', version: own.version, expected_contact: own.contact, contact: { ...own.contact, first_name: 'Updated' }
            }, 200, staff);
            await call(handleProfile, '/auth/user_profile/me', 'PATCH', {
                display_name: 'Stale', version: own.version, expected_contact: own.contact, contact: own.contact
            }, 409, staff);
            const reloaded = await call(handleProfile, '/auth/user_profile/me', 'GET', undefined, 200, staff);
            assert.equal(reloaded.contact.first_name, 'Updated');
            assert.equal(reloaded.version, own.version + 1);
            const updated = await call(handleProfile, '/auth/user_profile/me', 'PATCH', {
                display_name: 'Tender staff', version: reloaded.version, expected_contact: reloaded.contact,
                contact: { ...reloaded.contact, last_name: 'Persisted' }
            }, 200, staff);
            const reopened = await call(handleProfile, '/auth/user_profile/me', 'GET', undefined, 200, staff);
            assert.deepEqual(reopened.contact, updated.contact);
            assert.equal(reopened.contact.last_name, 'Persisted');
            assert.equal(reopened.version, updated.version);
            const publicView = await call(handleProfile, '/auth/user_profile/' + user);
            assert.equal(publicView.display_name, 'Tender staff');
            assert.equal(publicView.contact, undefined);
        });
        await t.test('offboarding requires the actual manager and never resets the account password', async () => {
            const hash = (await sql.query('SELECT password_hash FROM auth.users WHERE id=$1', [user])).rows[0].password_hash;
            let c = await org('offboarding', 'POST', { user_id: user, reason: 'Synthetic departure handover' }, 201);
            await org('offboarding/' + c.id + '/approve', 'POST', { version: c.version, reason: 'Employee cannot approve own case' }, 403, staff);
            c = await org('offboarding/' + c.id + '/approve', 'POST', { version: c.version, reason: 'Actual reporting manager approval' });
            assert.equal(c.state, 'approved');
            c = await org('offboarding/' + c.id + '/complete', 'POST', { version: c.version, reason: 'Complete approved synthetic handover' });
            assert.equal(c.state, 'completed');
            const u = (await sql.query('SELECT password_hash,is_active FROM auth.users WHERE id=$1', [user])).rows[0];
            assert.equal(u.password_hash, hash);
            assert.equal(u.is_active, true);
            const history = await org('offboarding/' + c.id + '/history');
            assert.equal(history.length, 3);
            assert.ok((await org('offboarding')).some((x: {
                id: string;
            }) => x.id === c.id));
        });
        await t.test('historical TSC preserves original response while reviewing and exporting repeatedly', async () => {
            const draft = randomUUID(), job = randomUUID(), input = seal({
                name: 'Legacy tender', questions: [{
                        id: 'q1', section: 'A', question: 'Original question?', printed_number: '1'
                    }], passages: [{
                        id: 's1', filename: 'Tender.pdf', locator: 'Page 1', text: 'Exact retained source evidence.', revision: 1, sha256: '0'.repeat(64)
                    }], warnings: [], coverage: {}
            }), result = seal({ schema: 'v1', answers: [{
                        question_id: 'q1', status: 'answered', answer: 'Original model wording', citations: [{ source_id: 's1', quote: 'Exact retained source evidence.' }], accepted: false, origin: 'model'
                    }], warnings: [] });
            await sql.query('INSERT INTO tender.document_drafts(id,name,owner_id,operation_key)VALUES($1,$2,$3,$4)', [draft, 'Legacy tender', admin, randomUUID()]);
            await sql.query("INSERT INTO tender.tsc_jobs(id,draft_id,draft_version,mode,model,state,input_cipher,input_hash,result_cipher,result_hash,actor_id,operation_key,version)VALUES($1,$2,1,'manual','legacy','ready',$3,$4,$5,$6,$7,$8,1)", [job, draft, input.cipher, input.hash, result.cipher, result.hash, admin, randomUUID()]);
            const request = async (method: string, tail: string[], b?: unknown) => handleLegacy(new Request('http://fixture.invalid', { method, body: b === undefined ? undefined : JSON.stringify(b), headers: { 'Content-Type': 'application/json' } }), draft, job, tail, token);
            const original = await (await request('GET', [])).json();
            assert.equal(original.result.answers[0].answer, 'Original model wording');
            const edited = await request('PATCH', ['answers', 'q1'], {
                expected_version: 1, status: 'answered', answer: 'Manager revised wording', citations: [{ source_id: 's1', quote: 'Exact retained source evidence.' }], accepted: false, reason: 'Reviewed correction'
            });
            assert.equal(edited.status, 200);
            const after = await edited.json();
            assert.equal(after.result.answers[0].origin, 'manual');
            for (let i = 0; i < 2; i++) {
                const doc = await request('GET', ['document']);
                assert.equal(doc.status, 200);
                assert.match(strFromU8(unzipSync(new Uint8Array(await doc.arrayBuffer()))['word/document.xml']), /Manager revised wording/);
            }
            assert.equal(Number((await sql.query('SELECT count(*) FROM tender.tsc_events WHERE job_id=$1', [job])).rows[0].count), 3);
            assert.equal(Number((await sql.query('SELECT count(*) FROM tender.tsc_model_calls')).rows[0].count), 0);
        });
    }
    finally {
        await sql.end();
    }
});
