import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import test from "node:test";
import { initializeAuth } from "../../src/server/auth/bootstrap";
import { login } from "../../src/server/auth/sessions";
import { createRole, updateRole } from "../../src/server/auth/administration";
import { hashPassword } from "../../src/server/auth/security";
import { pool } from "../../src/server/db";
import { handleMaster } from "../../src/server/master/http";
test("supplier profiles and lifecycle preserve existing SQL, audit and Finance boundaries", { skip: !process.env.EPCERP_DATABASE_URL }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /\/epcerp_(auth|master)_qa_/u, "Refusing to mutate an application database");
    try {
        await initializeAuth();
        const token = (await login("superadmin", process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token, prefix = 'VQ' + randomUUID().replaceAll('-', '').slice(0, 10).toUpperCase();
        async function raw(method: string, path: string, body?: string, access = token) { return handleMaster(new Request('http://localhost/api/v1' + path, { method, headers: { Authorization: `Bearer ${access}` }, body })); }
        async function call(method: string, path: string, body?: unknown, status = 200, access = token) { const r = await raw(method, path, body === undefined ? undefined : JSON.stringify(body), access), value = await r.json(); assert.equal(r.status, status, JSON.stringify(value)); return value; }
        const credentials: Record<string, {
            token: string;
            role: Awaited<ReturnType<typeof createRole>>;
            id: string;
        }> = {};
        for (const [kind, permission] of [['reader', 'settings.suppliers.read'], ['finance', 'accounts.vendor_verification.read']] as const) {
            const role = await createRole(token, { name: prefix + kind, description: 'Synthetic supplier ' + kind, permission_codes: [permission] }), id = randomUUID(), name = prefix + kind;
            await pool().query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,true,false)', [id, name, name + '@fixture.invalid', await hashPassword('Vendor-Reader1!')]);
            await pool().query('INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)', [id, role.id]);
            credentials[kind] = { id, role, token: (await login(name, 'Vendor-Reader1!')).access_token };
        }
        const input = (code = prefix) => ({ code: code.toLowerCase(), name: '  ' + prefix + '  Supplier  ', data: { contact_person_name: 'QA contact', email: 'vendor@fixture.invalid', phone: '+91 9000000000', qualification: 'unverified', payment_terms: { advance_pct: 33.33, delivery_pct: 66.67, credit_days: 30 }, indent_terms: 'Synthetic terms only', delivery_terms: 'Synthetic delivery basis' }, reason: ' QA created ' });
        let supplier: Awaited<ReturnType<typeof call>>;
        const evidenceTables = ['supplier_documents', 'supplier_document_changes', 'supplier_verifications', 'supplier_verification_history', 'supplier_document_approval_tasks', 'supplier_document_approval_events'];
        const evidenceBefore: Record<string, string> = {};
        for (const table of evidenceTables) {
            const exists = (await pool().query('SELECT to_regclass($1)::text AS name', ['public.' + table])).rows[0].name;
            if (exists)
                evidenceBefore[table] = (await pool().query(`SELECT md5(coalesce(string_agg(to_jsonb(v)::text,'|' ORDER BY to_jsonb(v)::text),'')) AS hash FROM public.${table} v`)).rows[0].hash;
        }
        await t.test('supplier readers and Finance-only readers can list; neither gains write or import authority', async () => {
            await call('GET', '/suppliers', undefined, 401, '');
            for (const c of Object.values(credentials)) {
                await call('GET', '/suppliers', undefined, 200, c.token);
                await call('POST', '/suppliers', input(), 403, c.token);
                await call('POST', '/suppliers/legacy-import', {}, 403, c.token);
            }
        });
        await t.test('create normalizes code and outer whitespace while preserving fields, actors and response shape', async () => {
            supplier = await call('POST', '/suppliers', input(), 201);
            assert.equal(supplier.code, prefix);
            assert.equal(supplier.name, prefix + '  Supplier');
            assert.equal(supplier.is_active, true);
            assert.equal(supplier.is_imported, false);
            assert.equal(supplier.version, 1);
            assert.ok(supplier.created_by_id);
            assert.equal(supplier.updated_by_id, supplier.created_by_id);
            assert.equal(supplier.data.payment_terms.advance_pct, 33.33);
            assert.deepEqual(Object.keys(supplier).sort(), ['id', 'code', 'name', 'data', 'is_active', 'version', 'created_at', 'updated_at', 'created_by_id', 'updated_by_id', 'is_imported'].sort());
            const history = (await pool().query('SELECT * FROM supplier_changes WHERE supplier_id=$1', [supplier.id])).rows;
            assert.equal(history.length, 1);
            assert.equal(history[0].action, 'created');
            assert.equal(history[0].reason, ' QA created ');
            assert.equal(history[0].snapshot.id, supplier.id);
            assert.equal((await pool().query("SELECT count(*) FROM business_events WHERE source_module='suppliers' AND entity_id=$1 AND action='created'", [supplier.id])).rows[0].count, '1');
            await call('POST', '/suppliers', input(), 409);
        });
        await t.test('profile updates retain code, ID, creation metadata and omitted status', async () => {
            const original = supplier;
            supplier = await call('PATCH', '/suppliers/' + supplier.id, { name: prefix + ' Edited', data: { ...supplier.data, notes: 'QA profile correction' }, version: supplier.version, reason: 'QA updated' });
            assert.equal(supplier.id, original.id);
            assert.equal(supplier.code, original.code);
            assert.equal(supplier.created_at, original.created_at);
            assert.equal(supplier.created_by_id, original.created_by_id);
            assert.equal(supplier.version, 2);
            assert.equal(supplier.is_active, true);
            await call('PATCH', '/suppliers/' + supplier.id, { name: prefix + ' Forged code', code: 'OTHER', data: supplier.data, version: supplier.version, reason: 'QA forged' }, 422);
            await call('PATCH', '/suppliers/' + supplier.id, { name: supplier.name, data: supplier.data, version: 1, reason: 'QA stale' }, 409);
        });
        await t.test('activation, suspension and reactivation keep qualification separate from document verification', async () => {
            supplier = await call('PATCH', '/suppliers/' + supplier.id, { name: supplier.name, data: supplier.data, version: supplier.version, is_active: false, reason: 'QA deactivate' });
            assert.equal(supplier.is_active, false);
            supplier = await call('PATCH', '/suppliers/' + supplier.id, { name: supplier.name, data: supplier.data, version: supplier.version, is_active: null, reason: 'QA omitted lifecycle' });
            assert.equal(supplier.is_active, false);
            supplier = await call('PATCH', '/suppliers/' + supplier.id, { name: supplier.name, data: { ...supplier.data, qualification: 'approved', notes: 'Reviewed qualification only', onboarding_verified: true }, version: supplier.version, is_active: true, reason: 'QA reactivate' });
            assert.equal(supplier.is_active, true);
            assert.equal(supplier.data.qualification, 'approved');
            assert.equal((await pool().query('SELECT count(*) FROM supplier_verifications WHERE supplier_id=$1', [supplier.id])).rows[0].count, '0');
        });
        await t.test('concurrent edits with one version issue only one correction and reject the stale caller', async () => {
            const version = supplier.version;
            const responses = await Promise.all(['A', 'B'].map(label => raw('PATCH', '/suppliers/' + supplier.id, JSON.stringify({ name: prefix + ' concurrent ' + label, data: supplier.data, version, reason: 'QA concurrent ' + label }))));
            assert.deepEqual(responses.map(r => r.status).sort(), [200, 409]);
            supplier = await responses.find(r => r.status === 200)!.json();
            assert.equal(supplier.version, version + 1);
            assert.equal((await pool().query('SELECT count(*) FROM supplier_changes WHERE supplier_id=$1 AND version=$2', [supplier.id, supplier.version])).rows[0].count, '1');
        });
        await t.test('profile and history changes roll back together when audit or journal writes fail', async () => {
            await pool().query("ALTER TABLE supplier_changes ADD CONSTRAINT epcerp_qa_supplier_change_failure CHECK(reason <> 'Reject vendor QA') NOT VALID");
            try {
                await call('PATCH', '/suppliers/' + supplier.id, { name: 'Must not persist', data: supplier.data, version: supplier.version, reason: 'Reject vendor QA' }, 503);
                assert.equal((await pool().query('SELECT version,name FROM suppliers WHERE id=$1', [supplier.id])).rows[0].name, supplier.name);
                await call('POST', '/suppliers', { ...input(prefix + 'X'), reason: 'Reject vendor QA' }, 503);
                assert.equal((await pool().query('SELECT count(*) FROM suppliers WHERE code=$1', [prefix + 'X'])).rows[0].count, '0');
            }
            finally {
                await pool().query('ALTER TABLE supplier_changes DROP CONSTRAINT epcerp_qa_supplier_change_failure');
            }
            await pool().query("ALTER TABLE business_events ADD CONSTRAINT epcerp_qa_supplier_event_failure CHECK(source_module <> 'suppliers') NOT VALID");
            try {
                await call('POST', '/suppliers', input(prefix + 'Y'), 503);
                assert.equal((await pool().query('SELECT count(*) FROM suppliers WHERE code=$1', [prefix + 'Y'])).rows[0].count, '0');
            }
            finally {
                await pool().query('ALTER TABLE business_events DROP CONSTRAINT epcerp_qa_supplier_event_failure');
            }
            await assert.rejects(pool().query("UPDATE supplier_changes SET reason='forged' WHERE supplier_id=$1", [supplier.id]), /immutable|cannot|append.only/i);
        });
        await t.test('legacy source records remain idempotent and their provenance survives native edits', async () => {
            const source = { id: randomUUID(), version: 17, created_at: '2020-01-01T05:30:00.123456+05:30', updated_at: '2020-02-01T00:00:00.654321Z' };
            const payload = { source, code: prefix + 'I', name: prefix + ' Legacy', data: { contact: 'Legacy contact', qualification: 'unverified', payment_terms: { advance_pct: 100, credit_days: 30 } }, is_active: false };
            let imported = await call('POST', '/suppliers/legacy-import', payload);
            assert.notEqual(imported.id, source.id);
            assert.equal(imported.is_imported, true);
            assert.equal(imported.is_active, false);
            assert.equal(imported.source_id, undefined);
            assert.equal((await call('POST', '/suppliers/legacy-import', payload)).id, imported.id);
            await call('POST', '/suppliers/legacy-import', { ...payload, data: { ...payload.data, contact: 'Conflicting source' } }, 409);
            const provenance = (await pool().query("SELECT source_id,source_version,to_char(source_created_at AT TIME ZONE 'UTC','YYYY-MM-DD\"T\"HH24:MI:SS.US\"Z\"') AS created,to_char(source_updated_at AT TIME ZONE 'UTC','YYYY-MM-DD\"T\"HH24:MI:SS.US\"Z\"') AS updated FROM suppliers WHERE id=$1", [imported.id])).rows[0];
            assert.equal(provenance.source_id, source.id);
            assert.equal(provenance.source_version, 17);
            assert.equal(provenance.created, '2020-01-01T00:00:00.123456Z');
            imported = await call('PATCH', '/suppliers/' + imported.id, { name: imported.name + ' edited', data: imported.data, version: imported.version, is_active: true, reason: 'QA native legacy edit' });
            assert.equal(imported.is_imported, true);
            const kept = (await pool().query('SELECT source_id,source_version FROM suppliers WHERE id=$1', [imported.id])).rows[0];
            assert.equal(kept.source_id, source.id);
            assert.equal(kept.source_version, 17);
            assert.equal((await pool().query('SELECT count(*) FROM supplier_changes WHERE supplier_id=$1', [imported.id])).rows[0].count, '1');
            assert.equal((await pool().query("SELECT count(*) FROM business_events WHERE source_module='suppliers' AND entity_id=$1 AND action='legacy_imported'", [imported.id])).rows[0].count, '1');
        });
        await t.test('unsafe fields, precise percentage violations and streamed oversize bodies cannot persist', async () => {
            await call('POST', '/suppliers', { ...input(prefix + 'Z'), data: { qualification: 'approved' } }, 422);
            await call('POST', '/suppliers', { ...input(prefix + 'Z'), data: { finance_verified: true } }, 422);
            await call('POST', '/suppliers', { ...input(prefix + 'Z'), data: { item_ids: [randomUUID()] } }, 422);
            const body = `{"code":"${prefix}Z","name":"QA","data":{"payment_terms":{"advance_pct":50,"delivery_pct":50.01000000000000000001}},"reason":"QA"}`;
            assert.equal((await raw('POST', '/suppliers', body)).status, 422);
            assert.equal((await raw('PATCH', '/suppliers/' + supplier.id, `{"name":"QA","data":{},"version":1.0,"reason":"QA"}`)).status, 422);
            await call('PATCH', '/suppliers/' + supplier.id.toUpperCase(), { name: supplier.name, data: supplier.data, version: supplier.version, reason: 'QA ID casing' }, 404);
            const stream = new ReadableStream({ start(c) { c.enqueue(new TextEncoder().encode('x'.repeat(128 * 1024 + 1))); c.close(); } });
            assert.equal((await handleMaster(new Request('http://localhost/api/v1/suppliers', { method: 'POST', body: stream, duplex: 'half' } as RequestInit))).status, 413);
        });
        await t.test('live role revocation applies to supplier and alternative Finance read access', async () => {
            for (const c of Object.values(credentials)) {
                await updateRole(token, c.role.id, { permission_codes: [], expected_version: c.role.edit_version });
                await call('GET', '/suppliers', undefined, 403, c.token);
            }
            await pool().query('UPDATE auth.users SET is_active=false WHERE id=$1', [credentials.finance.id]);
            await call('GET', '/suppliers', undefined, 401, credentials.finance.token);
        });
        for (const [table, hash] of Object.entries(evidenceBefore))
            assert.equal((await pool().query(`SELECT md5(coalesce(string_agg(to_jsonb(v)::text,'|' ORDER BY to_jsonb(v)::text),'')) AS hash FROM public.${table} v`)).rows[0].hash, hash, 'Profile writes touched document/Finance evidence: ' + table);
        assert.equal((await pool().query('SELECT version FROM schema_migrations')).rows[0].version, '98');
    }
    finally {
        await pool().end();
    }
});
