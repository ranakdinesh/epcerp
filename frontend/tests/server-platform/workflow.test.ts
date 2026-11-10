import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID, createCipheriv, createHash } from 'node:crypto';
import { pool, transaction } from '../../src/server/db';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { handleAuth } from '../../src/server/auth/http';
import { handlePurchasing } from '../../src/server/purchasing/http';
import { handleProjects } from '../../src/server/projects/http';
import { handleIntake } from '../../src/server/tenders/intake/http';
import { initializeProjectPurchase } from '../../src/server/purchasing/project-registration';
import presets from '../../src/server/auth/finance-presets.json';
import { handleOrganisation } from '../../src/server/platform/organisation';
import { handleProfile } from '../../src/server/platform/profile';
import { handleGovernance } from '../../src/server/platform/governance';
import { handleCalendar } from '../../src/server/platform/calendar';
import { inventory } from '../../src/server/platform/inventory';
import { encrypt, decrypt } from '../../src/server/platform/storage';
import { health } from '../../src/server/platform/health';
import { handleTSC } from '../../src/server/tenders/tsc/http';
import { handleLegacy } from '../../src/server/tenders/tsc/legacy';
import { encrypt as seal } from '../../src/server/tenders/tsc/crypto';
import { unzipSync, strFromU8 } from 'fflate';
test('remaining native platform contracts without a Go backend', { skip: !process.env.EPCERP_DATABASE_URL }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /^\/epcerp_auth_qa_retire_/);
    await initializeAuth();
    const sql = pool(), password = process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!, token = (await login('superadmin', password)).access_token, admin = (await sql.query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0].id;
    const call = async (handler: (r: Request) => Promise<Response>, path: string, method = 'GET', value?: unknown, status = 200, access = token) => {
        const response = await handler(new Request('http://fixture.invalid/api/v1' + path, { method, headers: { Authorization: 'Bearer ' + access, 'Content-Type': 'application/json' }, body: value === undefined ? undefined : JSON.stringify(value) }));
        const data = response.status === 204 ? undefined : await response.json();
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
        await t.test('procurement manager grants are seeded on a fresh database and address project notices', async () => {
            const role = presets.find(p => p.name === 'procurement.manager')!;
            const { rows: granted } = await sql.query(`SELECT r.id, p.code FROM auth.roles r
                JOIN auth.role_permissions rp ON rp.role_id=r.id JOIN auth.permissions p ON p.id=rp.permission_id
                WHERE r.name=$1 ORDER BY p.code`, [role.name]);
            assert.deepEqual(granted.map(row => row.code), [...role.permission_codes].sort());
            await sql.query('INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)', [user, granted[0].id]);
        });
        await t.test('tender draft rename and removal retain an audit trail', async () => {
            const path = '/tender-fms/drafts', operation = randomUUID();
            const created = await handleIntake(new Request('http://fixture.invalid/api/v1' + path, {
                method: 'POST', headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json', 'Idempotency-Key': operation },
                body: JSON.stringify({ name: 'Synthetic intake draft' }),
            }));
            assert.equal(created.status, 201);
            const { id } = await created.json();
            const updated = await call(handleIntake, path + '/' + id, 'PATCH', { name: 'Renamed intake draft' });
            assert.equal(updated.name, 'Renamed intake draft');
            await call(handleIntake, path + '/' + id, 'DELETE');
            await call(handleIntake, path + '/' + id, 'GET', undefined, 404);
            const audit = (await sql.query('SELECT action FROM tender.draft_events WHERE draft_id=$1 ORDER BY created_at', [id])).rows;
            assert.deepEqual(audit.slice(-2).map(row => row.action), ['renamed', 'soft_deleted']);
        });
        await t.test('tender costing and award create one project and a reviewable BOQ', async () => {
            const draft = randomUUID(), client = randomUUID(), clientName = 'QA' + client.replaceAll('-', '').slice(0, 12);
            await sql.query(`INSERT INTO clients(id,client_code,legal_name,normalized_legal_name,display_name,parent_organization,client_type,industry,gstin,pan,cin,website,address_line_1,address_line_2,city,state,postal_code,country,legacy_source,legacy_source_id,is_active)
                VALUES($1,$2,$2,$2,$2,'','','','','','','','','','','','','India','','',true)`, [client, clientName]);
            await sql.query('INSERT INTO tender.document_drafts(id,name,owner_id,operation_key) VALUES($1,$2,$3,$4)', [draft, 'Synthetic award tender', admin, randomUUID()]);
            const tender = (tail: string, method = 'GET', value?: unknown, status = 200) => call(handleTSC, `/tender-fms/drafts/${draft}/${tail}`, method, value, status);
            await tender('flow-status');
            await tender('prebid-queries', 'POST', { tender_specification: 'Test specification', query_description: 'Confirm equipment rating' }, 201);
            const item = await tender('bid-items', 'POST', { description: 'Synthetic equipment', quantity: 2, unit: 'NOS' }, 201);
            const quote = await tender('vendor-quotations', 'POST', { bid_item_id: item.id, vendor_name: 'Synthetic supplier', unit_rate: 100 }, 201);
            await tender('vendor-quotations/select-l1', 'POST', { bid_item_id: item.id, quote_id: quote.id });
            const award = { project_title: 'Synthetic award project', contract_reference: 'QA-LOA-1109', client_name: clientName };
            await tender('bid-costing', 'POST', { material_cost: 200 });
            await tender('award-project', 'POST', award, 422);
            await tender('bid-costing', 'POST', { material_cost: 200, status: 'submitted', portal_reference: 'QA-BID' });
            await tender('award-project', 'POST', { ...award, client_name: 'Missing client' }, 422);
            const [first, replay] = await Promise.all([tender('award-project', 'POST', award, 201), tender('award-project', 'POST', award, 201)]);
            assert.equal(first.project_id, replay.project_id);
            assert.equal(first.project_number, replay.project_number);
            const project = (await sql.query('SELECT client_id, project_manager_id, contract_value FROM projects WHERE id=$1', [first.project_id])).rows[0];
            assert.equal(project.client_id, client);
            assert.equal(project.project_manager_id, admin);
            assert.equal(Number(project.contract_value), 237.6);
            const boq = (await sql.query('SELECT b.state,l.quantity,l.item_snapshot FROM purchase_bo_lists b JOIN purchase_bo_lines l ON l.bo_id=b.id WHERE b.project_id=$1', [first.project_id])).rows;
            assert.equal(boq.length, 1);
            assert.equal(boq[0].state, 'draft');
            assert.equal(Number(boq[0].quantity), 2);
            assert.equal(boq[0].item_snapshot.description, 'Synthetic equipment');
            const notices = (await sql.query("SELECT recipient_id FROM communication_notices WHERE project_id=$1 AND notice_kind='procurement_manager'", [first.project_id])).rows;
            assert.deepEqual(notices.map(row => row.recipient_id).sort(), [user, admin].sort());
            const unassigned = randomUUID();
            await transaction(async db => {
                await db.query(`INSERT INTO projects(id,project_number,name,normalized_name,client_id,project_manager_id,created_by_id,updated_by_id,purchase_enabled)
                    VALUES($1,'QA-UNASSIGNED','Synthetic unassigned purchase','synthetic unassigned purchase',$2,$3,$3,$3,true),
                    ($4,'QA-PURCHASE-OFF','Synthetic purchase disabled','synthetic purchase disabled',$2,$3,$3,$3,false)`, [unassigned, client, admin, randomUUID()]);
                await initializeProjectPurchase(db, unassigned, null, admin);
            });
            assert.equal((await sql.query('SELECT purchase_manager_id FROM purchase_fms_owner_history WHERE project_id=$1', [unassigned])).rows[0].purchase_manager_id, null);
            await tender('bid-costing', 'POST', { material_cost: 100 }, 409);
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
        await t.test('user creation supports manual temporary access when email is disabled', async () => {
            const account = { username: 'qa-onboarding', email: 'qa-onboarding@fixture.invalid', first_name: 'QA',
                last_name: 'Onboarding', mobile: '9898980123', role_names: ['procurement.manager'] };
            const temporary = 'Synthetic-Start8!', personal = 'Synthetic-Personal9!';
            assert.equal((await call(handleAuth, '/users')).onboarding_email_enabled, false);
            await call(handleAuth, '/users', 'POST', account, 422);
            await call(handleAuth, '/users', 'POST', { ...account, password: temporary }, 422);
            await call(handleAuth, '/users', 'POST', { ...account, password: temporary, confirmation: personal }, 422);
            assert.equal((await sql.query('SELECT id FROM auth.users WHERE username=$1', [account.username])).rowCount, 0);
            const created = await call(handleAuth, '/users', 'POST', { ...account, password: temporary, confirmation: temporary }, 201);
            assert.equal(created.must_change_password, true);
            assert.deepEqual(created.roles, ['procurement.manager']);
            assert.equal((await sql.query('SELECT id FROM auth.auth_credential_emails WHERE user_id=$1', [created.id])).rowCount, 0);
            const credential = (await sql.query('SELECT action,actor_id FROM auth.auth_credential_changes WHERE user_id=$1', [created.id])).rows;
            assert.deepEqual(credential, [{ action: 'user_created', actor_id: admin }]);
            const first = await login(account.username, temporary);
            assert.equal(first.must_change_password, true);
            await call(handleAuth, '/users', 'GET', undefined, 403, first.access_token);
            await call(handleAuth, '/auth/me/password', 'POST', { current_password: temporary, password: personal }, 204, first.access_token);
            await assert.rejects(login(account.username, temporary));
            const signedIn = await login(account.username, personal);
            assert.equal(signedIn.must_change_password, false);
            const purchase = await call(handlePurchasing, '/purchase-fms', 'GET', undefined, 200, signedIn.access_token);
            assert.equal(purchase.portfolio, true);
            assert.ok(purchase.items.some((project: { project_number: string }) => project.project_number === 'QA-PURCHASE-OFF'));
            const register = await call(handlePurchasing, '/purchase-fms?enabled_only=true', 'GET', undefined, 200, signedIn.access_token);
            const row = register.items.find((project: { project_number: string }) => project.project_number === 'QA-UNASSIGNED');
            assert.equal(row.can_assign, true);
            assert.equal(row.purchase_manager_id, null);
            assert.equal(register.items.some((project: { project_number: string }) => project.project_number === 'QA-PURCHASE-OFF'), false);
            const route = '/purchase-fms/projects/' + row.id;
            const options = await call(handlePurchasing, route + '/assignment-options', 'GET', undefined, 200, signedIn.access_token);
            assert.ok(options.some((option: { id: string }) => option.id === created.id));
            const assignment = { expected_version: row.ownership_version, reason: 'Synthetic procurement register assignment' };
            const delegated = await call(handlePurchasing, route + '/owner', 'PATCH', { ...assignment, purchase_manager_id: user }, 200, signedIn.access_token);
            assert.equal(delegated.purchase_manager_id, user);
            const self = await call(handlePurchasing, route + '/owner', 'PATCH',
                { ...assignment, expected_version: delegated.version, purchase_manager_id: created.id }, 200, signedIn.access_token);
            assert.equal(self.purchase_manager_id, created.id);
            const audit = (await sql.query('SELECT purchase_manager_id,created_by_id FROM purchase_fms_owner_history WHERE project_id=$1 ORDER BY workspace_version', [row.id])).rows;
            assert.deepEqual(audit.slice(-2), [{ purchase_manager_id: user, created_by_id: created.id },
                { purchase_manager_id: created.id, created_by_id: created.id }]);
            const general = await call(handleProjects, '/projects', 'GET', undefined, 200, signedIn.access_token);
            assert.equal(general.total, 0);
        });
        await t.test('configured email onboarding still queues an invitation', async () => {
            const configuration = { EPCERP_EMAIL_DELIVERY_ENABLED: 'true', EPCERP_BREVO_API_KEY: 'synthetic-test-key',
                EPCERP_EMAIL_SENDER_EMAIL: 'sender@fixture.invalid' };
            const previous = Object.fromEntries(Object.keys(configuration).map(name => [name, process.env[name]]));
            Object.assign(process.env, configuration);
            try {
                assert.equal((await call(handleAuth, '/users')).onboarding_email_enabled, true);
                const created = await call(handleAuth, '/users', 'POST', {
                    username: 'qa-email-onboarding', email: 'qa-email-onboarding@fixture.invalid', first_name: 'QA',
                    last_name: 'Email', mobile: '9898980124', role_names: ['procurement.manager']
                }, 201);
                const mail = (await sql.query('SELECT state FROM auth.auth_credential_emails WHERE user_id=$1', [created.id])).rows;
                assert.deepEqual(mail, [{ state: 'pending' }]);
            } finally {
                for (const [name, value] of Object.entries(previous)) {
                    if (value === undefined) delete process.env[name];
                    else process.env[name] = value;
                }
            }
        });
        await t.test('Superadmin sets a confirmed password immediately with an audit entry and no mail', async () => {
            const path = '/users/' + user + '/password';
            const next = 'Synthetic-Reset2!';
            const valid = { password: next, confirmation: next };
            await call(handleAuth, path, 'POST', { reason: 'Former email reset request' }, 422);
            await call(handleAuth, path, 'POST', { password: next }, 422);
            await call(handleAuth, path, 'POST', { ...valid, confirmation: 'Synthetic-Mismatch2!' }, 422);
            await call(handleAuth, path, 'POST', { ...valid, reason: 'Test' }, 422);
            await call(handleAuth, path, 'POST', valid, 403, staff);
            await call(handleAuth, '/auth/me', 'GET', undefined, 200, staff);
            const staleMail = randomUUID();
            await sql.query(`INSERT INTO auth.auth_credential_emails(id,user_id,credential_version,payload,payload_hash,state,expires_at)
                SELECT $1,id,credential_version,$3,'synthetic','sending',now()+interval '1 hour' FROM auth.users WHERE id=$2`,
                [staleMail, user, Buffer.from('synthetic')]);
            for (let attempt = 0; attempt < 5; attempt++) {
                await assert.rejects(login(username, 'Incorrect-Synthetic8!'));
            }
            await assert.rejects(login(username, password));
            await call(handleAuth, path, 'POST', valid, 204);
            await call(handleAuth, '/auth/me', 'GET', undefined, 401, staff);
            await assert.rejects(login(username, password));
            const signedIn = await login(username, next);
            assert.ok(signedIn.access_token);
            assert.equal(signedIn.must_change_password, false);
            const { rows } = await sql.query("SELECT reason FROM auth.auth_credential_changes WHERE user_id=$1 AND action='password_reset'", [user]);
            assert.deepEqual(rows.map(row => row.reason), ['Password set by Superadmin']);
            const mail = await sql.query('SELECT id,state,payload FROM auth.auth_credential_emails WHERE user_id=$1', [user]);
            assert.deepEqual(mail.rows, [{ id: staleMail, state: 'cancelled', payload: null }]);
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
