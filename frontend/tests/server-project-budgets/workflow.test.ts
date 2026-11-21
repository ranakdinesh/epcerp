import test from 'node:test';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { pool, transaction } from '../../src/server/db';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { createRole } from '../../src/server/auth/administration';
import { project as fixture } from '../server-inbox/fixtures';
import { handleBudget, ownsBudget } from '../../src/server/projects/budgets/http';
import * as budgets from '../../src/server/projects/budgets/service';
import { effectiveApproved } from '../../src/server/projects/budgets/approved';
import { fingerprint } from '../../src/server/projects/budgets/finance';
import { handleMaster } from '../../src/server/master/http';
import { definition } from '../server-master/catalogue-fixtures';
import * as bo from '../../src/server/purchasing/bo/service';
import { save as assign } from '../../src/server/projects/team/service';
const oracle = (Actor: string, Method: string, Path: string, Body?: unknown, Operation?: string) => JSON.parse(execFileSync(process.env.EPCERP_PURCHASING_ORACLE!, [], { input: JSON.stringify({
        Actor, Method, Path, Body, Operation
    }) }).toString());
test('native project budgets preserve Go evidence, item pricing, authority and financial transactions', { skip: !process.env.EPCERP_DATABASE_URL }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /^\/epcerp_auth_qa_purchase_/);
    await initializeAuth();
    const sql = pool(), password = process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!, token = (await login('superadmin', password)).access_token, admin = (await sql.query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0].id;
    const makeUser = async (codes: string[]) => {
        const role = await createRole(token, { name: 'BudgetQA' + randomUUID(), description: 'Synthetic budget permissions', permission_codes: codes }), id = randomUUID(), username = 'qa' + id.replaceAll('-', '');
        await sql.query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)SELECT $1,$2,$3,password_hash,true,false FROM auth.users WHERE id=$4', [id, username, username + '@fixture.invalid', admin]);
        await sql.query('INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)', [id, role.id]);
        return { id, role: role.id, token: (await login(username, password)).access_token };
    };
    const manager = await makeUser(['projects.budgets.read', 'projects.budgets.write', 'projects.budgets.approve']), reviewer = await makeUser(['projects.budgets.read', 'projects.budgets.approve']), outsider = await makeUser(['projects.budgets.read', 'projects.budgets.write']), p = await fixture(manager.id), path = '/projects/' + p + '/budget';
    await sql.query("UPDATE projects SET contract_value=1000000,currency='INR',purchase_enabled=true WHERE id=$1", [p]);
    const call = async (raw: unknown, who = token, expected = 200) => {
        const response = await handleBudget(new Request('http://fixture.invalid/api/v1' + path, { method: 'POST', headers: { Authorization: 'Bearer ' + who, 'Content-Type': 'application/json' }, body: typeof raw === 'string' ? raw : JSON.stringify(raw) }));
        const value = await response.json();
        assert.equal(response.status, expected, JSON.stringify(value));
        return value;
    };
    const compare = async () => {
        const native = await budgets.workspace(token, p), go = oracle(admin, 'GET', path);
        assert.equal(go.status, 200, JSON.stringify(go));
        assert.deepEqual(native, go.body);
        return native;
    };
    let current: Awaited<ReturnType<typeof budgets.workspace>>;
    try {
        await t.test('empty workspace, capabilities and live owner directory match Go', async () => {
            assert.equal(ownsBudget(path), true);
            assert.equal(ownsBudget(path + '/other'), false);
            current = await compare();
            assert.equal(current.record, null);
            const r = await handleBudget(new Request('http://fixture.invalid/api/v1' + path));
            assert.equal(r.status, 401);
            await call({
                action: 'price_items', version: 0, reason: 'Denied', prices: []
            }, outsider.token, 403);
        });
        const head = randomUUID(), draft = {
            total: '1000', currency: 'INR', owner_id: manager.id, notes: 'Budget <evidence> Σ', heads: [{
                    id: head, parent_id: null, name: 'Purchase', kind: 'purchase', amount: '1000', owner_id: manager.id, is_active: true
                }], allocations: []
        };
        await t.test('native save, Go edits, immutable audit and exact reports interoperate', async () => {
            await call({
                action: 'save', version: 0, reason: 'Synthetic budget', data: draft
            }, manager.token);
            current = await compare();
            assert.equal(current.record!.version, 1);
            const go = oracle(manager.id, 'POST', path, {
                action: 'save', version: 1, reason: 'Go saved edit', data: { ...draft, notes: 'Original Go edit' }
            });
            assert.equal(go.status, 200, JSON.stringify(go));
            current = await compare();
            assert.equal(current.record!.version, 2);
            await call({
                action: 'save', version: 1, reason: 'Stale', data: draft
            }, token, 409);
            await call({
                action: 'save', version: 2, reason: 'Deletion forbidden', data: { ...draft, heads: [] }
            }, token, 422);
        });
        await t.test('hierarchy, over-budget values and cross-project owners are rejected atomically', async () => {
            const baseline = await compare();
            for (const data of [{ ...draft, total: '1000000.01' }, { ...draft, owner_id: outsider.id }, { ...draft, heads: [{ ...draft.heads[0], parent_id: head }] }, { ...draft, heads: [{ ...draft.heads[0], amount: '1001' }] }])
                await call({
                    action: 'save', version: 2, reason: 'Invalid data', data
                }, token, 422);
            assert.deepEqual(await compare(), baseline);
        });
        await t.test('approval requires explicit financial scope even for a project manager', async () => {
            await call({ action: 'approve', version: 2, reason: 'Manager cannot implicitly approve' }, manager.token, 403);
            await assign(token, p, undefined, {
                user_id: reviewer.id, bundle_id: reviewer.role, responsibility: 'Budget approval', permission_codes: ['projects.budgets.read', 'projects.budgets.approve'], version: 0, is_active: true, reason: 'Synthetic financial scope'
            });
            await call({
                action: 'approve', version: 2, reason: 'Saved data only', data: null
            }, reviewer.token, 422);
        });
        await t.test('missing Director rolls back decision, notice, budget and event together', async () => {
            await sql.query('UPDATE organisation_director_designation SET is_active=false,version=version+1 WHERE slot=true');
            const before = await compare(), count = (await sql.query('SELECT count(*) FROM finance_budget_decisions WHERE project_id=$1', [p])).rows[0].count;
            await call({ action: 'approve', version: 2, reason: 'Director required' }, reviewer.token, 409);
            assert.equal((await sql.query('SELECT count(*) FROM finance_budget_decisions WHERE project_id=$1', [p])).rows[0].count, count);
            assert.deepEqual(await compare(), before);
            await sql.query('INSERT INTO organisation_director_designation(id,user_id,version)VALUES($1,$2,1)ON CONFLICT(slot)DO UPDATE SET user_id=EXCLUDED.user_id,is_active=true,version=organisation_director_designation.version+1', [randomUUID(), admin]);
        });
        await t.test('approval fingerprint matches original Go and produces one Director notice', async () => {
            await call({ action: 'approve', version: 2, reason: 'Synthetic approval' }, reviewer.token);
            current = await compare();
            const b = current.record!;
            assert.equal(b.state, 'approved');
            const expected = oracle(admin, 'GET', path, b.approved!.data, 'budget_fingerprint').fingerprint;
            assert.equal(fingerprint(b.approved!.data), expected);
            const decision = (await sql.query('SELECT source_fingerprint,source_version,source_revision FROM finance_budget_decisions WHERE id=$1', [b.approved!.decision_id])).rows[0];
            assert.equal(decision.source_fingerprint, expected);
            assert.equal(decision.source_version, 2);
            assert.equal((await sql.query('SELECT count(*) FROM communication_notices WHERE decision_id=$1', [b.approved!.decision_id])).rows[0].count, '1');
            await call({
                action: 'save', version: 3, reason: 'Must revise', data: draft
            }, token, 422);
        });
        await t.test('revision preserves effective approved baseline until replacement approval', async () => {
            const baseline = await transaction(db => effectiveApproved(db, p));
            await call({ action: 'revise', version: 3, reason: 'Synthetic revision' }, manager.token);
            current = await compare();
            assert.equal(current.record!.revision, 2);
            assert.deepEqual(await transaction(db => effectiveApproved(db, p)), baseline);
        });
        const master = async (path: string, body: unknown) => {
            const r = await handleMaster(new Request('http://fixture.invalid/api/v1' + path, { method: 'POST', headers: { Authorization: 'Bearer ' + token }, body: JSON.stringify(body) }));
            const v = await r.json();
            assert.ok(r.ok, JSON.stringify(v));
            return v;
        };
        let boID: string, lineID: string;
        await t.test('per-item rates retain complete BO evidence and exact rounded totals', async () => {
            const code = 'BQ' + randomUUID().replaceAll('-', '').slice(0, 9).toUpperCase(), unit = await master('/categories/units', { code, name: code, reason: 'Synthetic' }), category = await master('/categories/categories', {
                code: code + 'C', name: code, unit_id: unit.id, reason: 'Synthetic'
            }), sub = await master('/categories/subcategories', {
                code: code + 'T', name: code, category_id: category.id, reason: 'Synthetic'
            }), template = await master('/configuration-templates/' + sub.id + '/publish', { definition: { ...definition(), subcategory_code: sub.code, template_name: code }, expected_revision: 0, reason: 'Synthetic' }), make = await master('/manufacturers', {
                code, name: code, data: { eligible: [category.code] }, reason: 'Synthetic'
            }), item = await master('/catalogue-items', {
                configuration_id: template.id, manufacturer_id: make.id, specifications: { SIZE: 'S1' }, model: code
            });
            boID = randomUUID();
            lineID = randomUUID();
            await bo.save(token, p, boID, {
                version: 0, name: 'Synthetic budget BO', reason: 'Synthetic', lines: [{
                        id: lineID, item_id: item.id, quantity: '2.505', required_date: '2026-12-31', reference: '', notes: ''
                    }]
            });
            current = await compare();
            await call({
                action: 'price_items', version: 4, reason: 'Synthetic unit budget', prices: [{
                        bo_id: boID, line_id: lineID, bo_version: 1, quantity: '2.505', unit_price: '12.34', changed: true
                    }]
            });
            current = await compare();
            assert.equal(current.record!.data.allocations[0].amount, '30.91');
            assert.equal(current.record!.data.allocations[0].source!.bo_version, 1);
            assert.equal(current.report!.purchase_total, '30.91');
            await call({
                action: 'price_items', version: 5, reason: 'Stale source', prices: [{
                        bo_id: boID, line_id: lineID, bo_version: 2, quantity: '2.505', unit_price: '12.34', changed: true
                    }]
            }, token, 409);
            await call({
                action: 'price_items', version: 5, reason: 'Missing source', prices: []
            }, token, 422);
        });
        await t.test('unchanged and cleared rates retain audit identities and deactivate instead of deleting', async () => {
            const allocation = current.record!.data.allocations[0];
            await call({
                action: 'price_items', version: 5, reason: 'Keep prior rate', prices: [{
                        bo_id: boID, line_id: lineID, bo_version: 1, quantity: '2.505', unit_price: '999.99', changed: false
                    }]
            });
            current = await compare();
            assert.equal(current.record!.data.allocations[0].unit_price, '12.34');
            await call({
                action: 'price_items', version: 6, reason: 'Clear rate', prices: [{
                        bo_id: boID, line_id: lineID, bo_version: 1, quantity: '2.505', unit_price: null, changed: true
                    }]
            });
            current = await compare();
            assert.equal(current.record!.data.allocations[0].is_active, false);
            assert.equal(current.record!.data.allocations[0].id, allocation.id);
            assert.equal(current.record!.data.allocations[0].created_at, allocation.created_at);
        });
        await t.test('revoked approval role and held projects cannot write', async () => {
            await sql.query('UPDATE auth.roles SET is_active=false WHERE id=$1', [reviewer.role]);
            await call({ action: 'approve', version: 7, reason: 'Revoked' }, reviewer.token, 403);
            await sql.query('UPDATE auth.roles SET is_active=true WHERE id=$1', [reviewer.role]);
            await sql.query("UPDATE projects SET lifecycle_status='on_hold' WHERE id=$1", [p]);
            await call({
                action: 'price_items', version: 7, reason: 'Held', prices: []
            }, token, 422);
            await compare();
            await sql.query("UPDATE projects SET lifecycle_status='draft' WHERE id=$1", [p]);
        });
        await t.test('replacement approval hashes historical item evidence and publishes revision two', async () => {
            await call({ action: 'approve', version: 7, reason: 'Synthetic replacement approval' }, reviewer.token);
            current = await compare();
            const b = current.record!, expected = oracle(admin, 'GET', path, b.approved!.data, 'budget_fingerprint').fingerprint;
            assert.equal(fingerprint(b.approved!.data), expected);
            assert.equal((await sql.query('SELECT source_fingerprint FROM finance_budget_decisions WHERE id=$1', [b.approved!.decision_id])).rows[0].source_fingerprint, expected);
            const baseline = await transaction(db => effectiveApproved(db, p));
            assert.equal(baseline!.revision, 2);
            assert.deepEqual(baseline!.allocations, []);
        });
        await t.test('HTTP numeric decimal lexemes preserve the maximum exact budget', async () => {
            const target = await fixture(manager.id), head = randomUUID();
            await sql.query("UPDATE projects SET contract_value=99999999999999.99,currency='INR' WHERE id=$1", [target]);
            const raw = '{"action":"save","version":0,"reason":"Exact HTTP decimal","data":{"total":99999999999999.99,"currency":"INR","owner_id":"' + manager.id + '","heads":[{"id":"' + head + '","parent_id":null,"name":"Exact purchase","kind":"purchase","amount":99999999999999.99,"owner_id":"' + manager.id + '","is_active":true}],"allocations":[]}}';
            const r = await handleBudget(new Request('http://fixture.invalid/api/v1/projects/' + target + '/budget', { method: 'POST', headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' }, body: raw })), value = await r.json();
            assert.equal(r.status, 200, JSON.stringify(value));
            assert.equal(value.record.data.total, '99999999999999.99');
            assert.deepEqual(value, oracle(admin, 'GET', '/projects/' + target + '/budget').body);
        });
    }
    finally {
        await sql.end();
    }
});
