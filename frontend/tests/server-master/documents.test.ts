import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { execFileSync } from "node:child_process";
import test from "node:test";
import sharp from "sharp";
import { initializeAuth } from "../../src/server/auth/bootstrap";
import { login } from "../../src/server/auth/sessions";
import { createRole, updateRole } from "../../src/server/auth/administration";
import { hashPassword } from "../../src/server/auth/security";
import { pool } from "../../src/server/db";
import { handleMaster, ownsMasterPath } from "../../src/server/master/http";
import { seal, open, inspect, getObject } from "../../src/server/master/documents/files";
const pdf = Buffer.from('%PDF-1.4\nSynthetic fixture only\n%%EOF');
function oracle(operation: string, content: Buffer, hash?: string) { return JSON.parse(execFileSync(process.env.EPCERP_DOCUMENT_COMPATIBILITY_BINARY!, [], { input: JSON.stringify({ Operation: operation, Content: content.toString('base64'), Hash: hash }) }).toString()); }
test('document encryption and supported media remain compatible with Go', { skip: !process.env.EPCERP_DOCUMENT_COMPATIBILITY_BINARY }, async (t) => {
    await t.test('Go ciphertext opens in Node and Node ciphertext opens in Go', () => { const old = oracle('seal', pdf); assert.deepEqual(open(Buffer.from(old.content, 'base64'), old.hash), pdf); const next = seal(pdf); assert.equal(oracle('open', next.encrypted, next.sha256).content, pdf.toString('base64')); const tampered = Buffer.from(next.encrypted); tampered[13] ^= 1; assert.throws(() => open(tampered, next.sha256)); assert.throws(() => open(next.encrypted, '0'.repeat(64))); assert.throws(() => open(Buffer.alloc(10), next.sha256)); });
    await t.test('real PNG, GIF, JPEG and PDF detection matches the Go inspector', async () => { const image = sharp({ create: { width: 2, height: 2, channels: 3, background: '#8090a0' } }); for (const data of [pdf, await image.clone().png().toBuffer(), await image.clone().jpeg().toBuffer(), await image.clone().gif().toBuffer()])
        assert.equal(await inspect(data), oracle('inspect', data).kind); });
    await t.test('disguised, truncated, oversized and unsupported content is rejected', async () => { for (const data of [Buffer.from('<svg/>'), Buffer.from('<html/>'), Buffer.from('%PDF-1.4 no eof'), Buffer.from('GIF89a'), Buffer.alloc(15 * 1024 * 1024 + 1), Buffer.concat([pdf, Buffer.alloc(2048)])]) {
        assert.ok(oracle('inspect', data).error);
        await assert.rejects(inspect(data));
    } const webp = await sharp({ create: { width: 1, height: 1, channels: 3, background: '#fff' } }).webp().toBuffer(); await assert.rejects(inspect(webp)); });
});
test('native supplier document transactions, approval hooks, storage and authorization', { skip: !process.env.EPCERP_DATABASE_URL || !process.env.VTPL_STORAGE_ENDPOINT }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /\/epcerp_auth_qa_/);
    assert.match(process.env.VTPL_STORAGE_BUCKET!, /^epcerp-documents-qa-/);
    try {
        await initializeAuth();
        const admin = (await login('superadmin', process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token, prefix = 'DQ' + randomUUID().replaceAll('-', '').slice(0, 10).toUpperCase();
        async function raw(method: string, path: string, body?: BodyInit, token = admin, headers: Record<string, string> = {}) { return handleMaster(new Request('http://localhost/api/v1' + path, { method, body, headers: { Authorization: `Bearer ${token}`, ...headers } })); }
        async function json(method: string, path: string, body?: unknown, status = 200, token = admin) { const r = await raw(method, path, body === undefined ? undefined : JSON.stringify(body), token), v = await r.json(); assert.equal(r.status, status, JSON.stringify(v)); return v; }
        const supplier = await json('POST', '/suppliers', { code: prefix, name: 'Document QA', data: {}, reason: 'Synthetic test supplier' }, 201), base = `/suppliers/${supplier.id}/documents`;
        async function upload(kind = 'pan', token = admin, status = 201, replaces = '', data = pdf, filename = 'QA document.pdf') { const form = new FormData(); form.set('doc_type', kind); form.set('file', new Blob([new Uint8Array(data)], { type: 'text/html' }), filename); if (replaces)
            form.set('replaces_document_id', replaces); const r = await raw('POST', base, form, token), value = await r.json(); assert.equal(r.status, status, JSON.stringify(value)); return value; }
        const users: Record<string, {
            id: string;
            token: string;
            role: Awaited<ReturnType<typeof createRole>>;
        }> = {};
        for (const [kind, permissions] of [['reader', ['settings.supplier_documents.read']], ['uploader', ['settings.supplier_documents.read', 'settings.supplier_documents.write']], ['finance', ['accounts.vendor_verification.view', 'accounts.vendor_verification.read', 'accounts.vendor_verification.write', 'accounts.vendor_verification.auto_approve']]] as const) {
            const role = await createRole(admin, { name: prefix + kind, description: 'Synthetic document QA', permission_codes: [...permissions] }), id = randomUUID(), name = prefix + kind;
            await pool().query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser) VALUES($1,$2,$3,$4,true,false)', [id, name, name + '@fixture.invalid', await hashPassword('Document-QA1!')]);
            await pool().query('INSERT INTO auth.user_roles(user_id,role_id) VALUES($1,$2)', [id, role.id]);
            users[kind] = { id, role, token: (await login(name, 'Document-QA1!')).access_token };
        }
        let first: Awaited<ReturnType<typeof upload>>;
        await t.test('native ownership excludes Finance and reconciliation routes', () => { assert.ok(ownsMasterPath(base)); for (const p of [base + '/reconcile-approvals', `/suppliers/${supplier.id}/verification`, `/suppliers/${supplier.id}/review-assignment`])
            assert.equal(ownsMasterPath(p), false); });
        await t.test('authorization precedes multipart buffering and uses live roles', async () => { assert.equal((await raw('HEAD', base, undefined, users.reader.token)).status, 403); assert.equal((await raw('HEAD', base, undefined, users.uploader.token)).status, 204); await upload('pan', users.reader.token, 403); await json('GET', base, undefined, 401, ''); await json('GET', base, undefined, 200, users.finance.token); });
        await t.test('encrypted object upload publishes metadata, history and automatic approval atomically', async () => { first = await upload(); assert.equal(first.content_type, 'application/pdf'); assert.equal(first.version, 1); assert.equal(first.is_encrypted, true); for (const key of ['encrypted_content', 'object_bucket', 'object_key', 'source_id'])
            assert.equal(first[key], undefined); const row = (await pool().query('SELECT * FROM supplier_documents WHERE id=$1', [first.id])).rows[0]; assert.equal(row.encrypted_content, null); const encrypted = await getObject(row.object_bucket, row.object_key); assert.notDeepEqual(encrypted, pdf); assert.deepEqual(open(encrypted, row.sha256), pdf); assert.equal(oracle('open', encrypted, row.sha256).content, pdf.toString('base64')); const list = await json('GET', base); assert.equal(list[0].approval_state, 'accept'); assert.equal((await pool().query('SELECT count(*) FROM supplier_document_changes WHERE document_id=$1', [first.id])).rows[0].count, '1'); assert.equal((await pool().query("SELECT count(*) FROM business_events WHERE entity_id=$1 AND source_module IN ('supplier_documents','supplier_document_approval')", [first.id])).rows[0].count, '2'); });
        await t.test('preview and download preserve bytes and authenticated security headers', async () => { for (const action of ['preview', 'download']) {
            const r = await raw('GET', base + '/' + first.id + '/' + action);
            assert.equal(r.status, 200);
            assert.deepEqual(Buffer.from(await r.arrayBuffer()), pdf);
            assert.equal(r.headers.get('cache-control'), 'private, no-store');
            assert.equal(r.headers.get('x-content-type-options'), 'nosniff');
            if (action === 'preview') {
                assert.equal(r.headers.get('content-type'), 'application/pdf');
                assert.match(r.headers.get('content-security-policy')!, /sandbox/);
            }
            else
                assert.equal(r.headers.get('x-encrypted-storage'), 'true');
        } await json('GET', `/suppliers/${randomUUID()}/documents/${first.id}/download`, undefined, 404); });
        await t.test('pending upload creates a Finance task; replacement retires tasks and retains old evidence', async () => { await pool().query('INSERT INTO supplier_review_assignments(id,supplier_id,user_id,version,is_active,created_by_id,updated_by_id) VALUES($1,$2,$3,1,true,$3,$3)', [randomUUID(), supplier.id, users.finance.id]); const pending = await upload('gst', users.uploader.token); let tasks = (await pool().query('SELECT * FROM supplier_document_tasks WHERE document_id=$1', [pending.id])).rows; assert.equal(tasks.length, 1); assert.equal(tasks[0].user_id, users.finance.id); await pool().query("INSERT INTO supplier_document_tasks(id,document_id,supplier_id,user_id,kind,comment,created_by_id,updated_by_id) VALUES($1,$2,$3,$4,'correction','Synthetic rejection',$4,$4)", [randomUUID(), pending.id, supplier.id, users.uploader.id]); const next = await upload('gst', users.uploader.token, 201, pending.id); assert.notEqual(next.id, pending.id); tasks = (await pool().query('SELECT * FROM supplier_document_tasks WHERE document_id=$1 ORDER BY kind', [pending.id])).rows; assert.equal(tasks[0].outcome, 'resubmitted'); assert.equal(tasks[1].outcome, 'replaced'); assert.equal((await raw('GET', base + '/' + pending.id + '/download', undefined, users.reader.token)).status, 404); assert.equal((await raw('GET', base + '/' + pending.id + '/download', undefined, users.uploader.token)).status, 200); assert.equal((await raw('GET', base + '/' + pending.id + '/download', undefined, users.finance.token)).status, 200); await upload('gst', admin, 409, pending.id); });
        await t.test('archive uses optimistic versions and concurrent replacements leave one active singleton', async () => { const [a, b] = await Promise.all([upload('pan'), upload('pan')]); assert.notEqual(a.id, b.id); const active = (await json('GET', base)).filter((d: {
            doc_type: string;
            is_active: boolean;
        }) => d.doc_type === 'pan' && d.is_active); assert.equal(active.length, 1); await json('POST', base + '/' + active[0].id + '/archive', { version: 99, reason: 'QA stale' }, 409); const archived = await json('POST', base + '/' + active[0].id + '/archive', { version: active[0].version, reason: 'QA archive' }); assert.equal(archived.is_active, false); assert.equal(archived.version, 2); await json('POST', base + '/' + active[0].id + '/archive', { version: 2, reason: 'QA repeated' }, 409); });
        await t.test('unknown, duplicate, corrupt and oversized uploads cannot create metadata', async () => { await upload('other', admin, 422, '', Buffer.from('<script/>')); const form = new FormData(); form.append('doc_type', 'other'); form.append('doc_type', 'pan'); assert.equal((await raw('POST', base, form)).status, 422); await upload('other', admin, 413, '', Buffer.alloc(15 * 1024 * 1024 + 1)); });
        await t.test('approval or journal failures roll back document and replacement changes together', async () => { const before = await json('GET', base); await pool().query("ALTER TABLE supplier_document_approval_changes ADD CONSTRAINT epcerp_qa_document_failure CHECK(action <> 'uploaded') NOT VALID"); try {
            await upload('gst', admin, 503);
            assert.deepEqual(await json('GET', base), before);
        }
        finally {
            await pool().query('ALTER TABLE supplier_document_approval_changes DROP CONSTRAINT epcerp_qa_document_failure');
        } });
        await t.test('legacy import stays idempotent, private and compatible with Go inline encryption', async () => { const source = { id: randomUUID(), created_at: '2020-01-01T00:00:00.123456Z', updated_at: '2020-02-01T00:00:00.654321Z' }, body = { source, doc_type: 'other', title: 'Synthetic legacy', original_filename: 'legacy.pdf', content_type: 'application/pdf', content: pdf.toString('base64'), is_active: true }, saved = await json('POST', base + '/legacy-import', body); assert.equal((await json('POST', base + '/legacy-import', body)).id, saved.id); await json('POST', base + '/legacy-import', { ...body, title: 'Conflicting' }, 409); assert.equal((await pool().query('SELECT count(*) FROM supplier_document_approvals WHERE document_id=$1', [saved.id])).rows[0].count, '0'); const old = oracle('seal', pdf), id = randomUUID(); await pool().query("INSERT INTO supplier_documents(id,supplier_id,doc_type,title,original_filename,content_type,byte_size,encrypted_content,sha256,is_active,created_by_id,updated_by_id) VALUES($1,$2,'other','Go inline fixture','go.pdf','application/pdf',$3,$4,$5,true,$6,$6)", [id, supplier.id, pdf.length, Buffer.from(old.content, 'base64'), old.hash, users.uploader.id]); const r = await raw('GET', base + '/' + id + '/download'); assert.equal(r.status, 200); assert.deepEqual(Buffer.from(await r.arrayBuffer()), pdf); await pool().query("UPDATE supplier_documents SET sha256=repeat('0',64) WHERE id=$1", [id]); try { await json('GET', base + '/' + id + '/download', undefined, 409); } finally { await pool().query('UPDATE supplier_documents SET sha256=$2 WHERE id=$1', [id,old.hash]); } });
        await t.test('revoked uploader and deactivated reviewer cannot create new tasks', async () => { await pool().query('UPDATE auth.users SET is_active=false WHERE id=$1', [users.finance.id]); await upload('other', users.uploader.token, 422); await updateRole(admin, users.uploader.role.id, { permission_codes: [], expected_version: users.uploader.role.edit_version }); await upload('other', users.uploader.token, 403); });
        assert.equal((await pool().query('SELECT version,dirty FROM schema_migrations')).rows[0].version, '98');
    }
    finally {
        await pool().end();
    }
});
