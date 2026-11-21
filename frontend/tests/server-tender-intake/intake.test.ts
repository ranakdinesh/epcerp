import assert from 'node:assert/strict';
import test from 'node:test';
import { randomUUID, randomBytes, createCipheriv } from 'node:crypto';
import { mkdtemp, chmod, rm, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { createRole } from '../../src/server/auth/administration';
import { hashPassword } from '../../src/server/auth/security';
import { pool } from '../../src/server/db';
import { handleIntake, ownsIntake } from '../../src/server/tenders/intake/http';
import { goJSON } from '../../src/server/tenders/intake/receipts';
import { writeObject, readObject } from '../../src/server/tenders/intake/objects';
import { digest } from '../../src/server/tenders/tsc/crypto';
import * as tsc from '../../src/server/tenders/tsc/service';
import { publishOnce } from '../../src/server/tenders/intake/publisher';
import { word } from '../../src/server/tenders/formats/document';
import type { Format } from '../../src/server/tenders/formats/service';
import { unzipSync, strFromU8 } from 'fflate';
const oracle = (value: unknown) => JSON.parse(execFileSync(process.env.EPCERP_TENDER_INTAKE_ORACLE!, [], { input: JSON.stringify(value), maxBuffer: 12 * 1048576 }).toString());
test('Go and Next share encrypted upload framing and operation fingerprints', { skip: !process.env.EPCERP_TENDER_INTAKE_ORACLE }, async () => { const root = await mkdtemp(join(tmpdir(), 'intake-objects-')); await chmod(root, 0o700); const previous = process.env.EPCERP_DOCUMENT_PACKAGE_ROOT; process.env.EPCERP_DOCUMENT_PACKAGE_ROOT = root; try {
    const data = Buffer.alloc((2 << 20) + 17, 97), Object = randomUUID(), key = process.env.EPCERP_DOCUMENT_READING_KEY!, hash = digest(data);
    await writeObject(Object, new Request('http://localhost', { method: 'PUT', body: data }), data.length, hash);
    const opened = oracle({ Operation: 'read', Root: root, Key: key, Object, Size: data.length, Hash: hash });
    assert.equal(opened.error, undefined);
    assert.deepEqual(Buffer.from(opened.data, 'base64'), data);
    const second = randomUUID();
    assert.equal(oracle({ Operation: 'write', Root: root, Key: key, Object: second, Data: data.toString('base64'), Size: data.length, Hash: hash }).error, undefined);
    assert.deepEqual(await readObject(second, data.length, hash), data);
    const path = join(root, second + '.blob'), bytes = await readFile(path);
    bytes[100] ^= 1;
    await writeFile(path, bytes);
    await assert.rejects(readObject(second, data.length, hash));
    await assert.rejects(writeObject(randomUUID(), new Request('http://localhost', { method: 'PUT', body: 'wrong' }), 5, '0'.repeat(64)));
    await assert.rejects(readObject('../escape', 1, hash));
    const Files = [{ ClientKey: randomUUID(), Filename: 'Tender<&>–₹.pdf', Kind: 'pdf', SHA256: hash, Size: 4 }];
    assert.equal(oracle({ Operation: 'fingerprint', Files }).hash, digest(goJSON(Files)));
}
finally {
    if (previous == null)
        delete process.env.EPCERP_DOCUMENT_PACKAGE_ROOT;
    else
        process.env.EPCERP_DOCUMENT_PACKAGE_ROOT = previous;
    await rm(root, { recursive: true, force: true });
} });
test('native tender drafts, receipts and formats preserve ownership, versions and histories', { skip: !process.env.EPCERP_DATABASE_URL }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /^\/epcerp_auth_qa_intake_/);
    await initializeAuth();
    const token = (await login('superadmin', process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token, actor = (await pool().query("SELECT id FROM auth.users WHERE username='superadmin'")).rows[0].id;
    const hashes = ['draft:create', 'draft:read', 'receipt:create', 'receipt:read', 'receipt:upload', 'receipt:seal', 'format:write', 'format:read', 'format:export'].map(v => digest('http:tender:' + v + ':' + actor));
    await pool().query('DELETE FROM auth.login_attempts WHERE identifier_hash=ANY($1::text[])', [hashes]);
    const root = await mkdtemp(join(tmpdir(), 'intake-http-'));
    await chmod(root, 0o700);
    process.env.EPCERP_DOCUMENT_PACKAGE_ROOT = root;
    async function raw(method: string, path: string, value?: unknown, key = randomUUID() as string, access = token) { return handleIntake(new Request('http://localhost/api/v1' + path, { method, headers: { Authorization: `Bearer ${access}`, 'Content-Type': 'application/json', 'Idempotency-Key': key }, body: value === undefined ? undefined : JSON.stringify(value) })); }
    async function json(method: string, path: string, value?: unknown, status = 200, key?: string, access?: string) { const r = await raw(method, path, value, key, access), v = r.status === 204 ? undefined : await r.json(); assert.equal(r.status, status, JSON.stringify(v)); return v; }
    const base = '/tender-fms', name = 'Intake QA ' + randomUUID();
    let draft: Awaited<ReturnType<typeof json>>, receipt: Awaited<ReturnType<typeof json>>, format: Format;
    const data = Buffer.from('%PDF-1.4\nSynthetic public tender\n%%EOF'), declaration = { client_file_key: randomUUID(), filename: 'Tender<&>.pdf', kind: 'pdf', size_bytes: data.length, sha256: digest(data) };
    try {
        await t.test('anonymous and read-only writers are rejected before consuming payloads', async () => { await json('POST', base + '/drafts', { name }, 401, undefined, ''); const role = await createRole(token, { name: 'IntakeReader' + randomUUID(), description: 'Synthetic read-only role', permission_codes: ['tenders.register.view', 'tenders.register.read', 'tenders.processing.read', 'settings.tsc_formats.read'] }), reader = randomUUID(), username = 'ir' + randomUUID().replaceAll('-', ''); await pool().query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,$4,true,false)', [reader, username, username + '@fixture.invalid', await hashPassword('Intake-Reader1!')]); await pool().query('INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)', [reader, role.id]); const access = (await login(username, 'Intake-Reader1!')).access_token; await json('POST', base + '/drafts', { name }, 403, undefined, access); await json('POST', base + '/receipts', { files: [declaration] }, 403, undefined, access); await json('GET', base + '/drafts', undefined, 200, undefined, access); await json('GET', '/settings/tsc-formats', undefined, 200, undefined, access); });
        await t.test('draft creation replays without duplicate events and rejects key conflicts', async () => { const key = randomUUID(); draft = await json('POST', base + '/drafts', { name }, 201, key); assert.equal((await json('POST', base + '/drafts', { name }, 201, key)).id, draft.id); await json('POST', base + '/drafts', { name: name + ' changed' }, 409, key); assert.equal((await pool().query("SELECT count(*)FROM tender.draft_events WHERE draft_id=$1 AND action='created'", [draft.id])).rows[0].count, '1'); const listed = await json('GET', base + '/drafts?search=' + encodeURIComponent(name) + '&sort=name&direction=asc'); assert.equal(listed.items[0].id, draft.id); await json('GET', base + '/drafts?page=-1', undefined, 422); });
        await t.test('even a superadmin cannot read or attach another owner’s intake',async()=>{const other=randomUUID(),foreign=randomUUID(),foreignReceipt=randomUUID(),username='foreign'+other.replaceAll('-','');await pool().query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)SELECT $1,$2,$3,password_hash,true,true FROM auth.users WHERE id=$4',[other,username,username+'@fixture.invalid',actor]);await pool().query('INSERT INTO tender.document_drafts(id,name,owner_id,operation_key)VALUES($1,$2,$3,$4)',[foreign,'Foreign synthetic draft',other,randomUUID()]);await pool().query('INSERT INTO document_package_receipts(id,owner_id)VALUES($1,$2)',[foreignReceipt,other]);await json('GET',`${base}/drafts/${foreign}`,undefined,404);await json('GET',`${base}/receipts/${foreignReceipt}`,undefined,404);assert.equal((await raw('POST',`${base}/drafts/${draft.id}/batches`,{receipt_id:foreignReceipt})).status,404);});
        await t.test('receipt declarations preserve Go idempotency and prohibit unsafe sources', async () => { const key = randomUUID(); receipt = await json('POST', base + '/receipts', { files: [declaration] }, 201, key); assert.equal((await json('POST', base + '/receipts', { files: [declaration] }, 201, key)).id, receipt.id); await json('POST', base + '/receipts', { files: [{ ...declaration, filename: 'changed.pdf' }] }, 409, key); await json('POST', base + '/receipts', { files: [{ ...declaration, filename: '../bad.pdf' }] }, 422); const fp = (await pool().query('SELECT fingerprint FROM document_package_operations WHERE receipt_id=$1', [receipt.id])).rows[0].fingerprint; assert.equal(fp, oracle({ Operation: 'fingerprint', Files: [{ ClientKey: declaration.client_file_key, Filename: declaration.filename, Kind: 'pdf', SHA256: declaration.sha256, Size: data.length }] }).hash); });
        await t.test('batch retention is idempotent and changes only its draft version once', async () => { for (let i = 0; i < 2; i++)
            assert.equal((await raw('POST', `${base}/drafts/${draft.id}/batches`, { receipt_id: receipt.id })).status, 204); const saved = await json('GET', `${base}/drafts/${draft.id}`); assert.equal(saved.version, 2); assert.equal(saved.batches[0].id, receipt.id); const row = (await pool().query('SELECT expires_at FROM document_package_receipts WHERE id=$1', [receipt.id])).rows[0]; assert.equal(new Date(row.expires_at).getUTCFullYear(), 9999); });
        await t.test('raw upload is authenticated, encrypted and replayable, while versions fence other writers', async () => { const f = receipt.files[0], key = randomUUID(); const request = (version: number, k = key, bytes = data) => new Request(`http://localhost/api/v1${base}/receipts/${receipt.id}/files/${f.file_id}`, { method: 'PUT', headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/octet-stream', 'X-Upload-Version': String(version), 'Idempotency-Key': k }, body: bytes }); assert.equal((await handleIntake(new Request(request(1).url, { method: 'HEAD', headers: request(1).headers }))).status, 204); let response = await handleIntake(request(1)); assert.equal(response.status, 200, await response.clone().text()); receipt = await response.json(); assert.equal(receipt.files[0].state, 'received'); response = await handleIntake(request(1)); assert.equal(response.headers.get('idempotency-replayed'), 'true'); assert.equal((await handleIntake(request(1, randomUUID()))).status, 409); const original = await raw('GET', `${base}/receipts/${receipt.id}/files/${f.file_id}`); assert.equal(original.status, 200); assert.deepEqual(Buffer.from(await original.arrayBuffer()), data); const row = (await pool().query('SELECT object_key FROM document_package_files WHERE id=$1', [f.file_id])).rows[0]; assert.equal(oracle({ Operation: 'read', Root: root, Key: process.env.EPCERP_DOCUMENT_READING_KEY, Object: row.object_key, Size: data.length, Hash: declaration.sha256 }).error, undefined); });
        await t.test('verification remains queued, replayable and unavailable as TSC evidence until verified', async () => { const key = randomUUID(), body = { expected_version: receipt.version }; const queued = await json('POST', `${base}/receipts/${receipt.id}/seal`, body, 202, key); assert.equal(queued.state, 'verifying'); assert.equal((await json('POST', `${base}/receipts/${receipt.id}/seal`, body, 202, key)).state, 'verifying'); const saved = await json('GET', `${base}/drafts/${draft.id}`); assert.equal(saved.batches[0].sources.length, 0); assert.equal((await pool().query('SELECT count(*)FROM document_package_members WHERE receipt_id=$1', [receipt.id])).rows[0].count, '0'); });
        await t.test('verified draft publication is atomic, scoped and never schedules local reading', async () => { assert.equal(await publishOnce(), false); const claim = randomUUID(), nonce = randomBytes(12), cipher = createCipheriv('aes-256-gcm', Buffer.from(process.env.EPCERP_DOCUMENT_READING_KEY!, 'hex'), nonce), payload = Buffer.concat([nonce, cipher.update(data), cipher.final(), cipher.getAuthTag()]); await pool().query("UPDATE document_package_receipts SET state='ready',claim_id=$2,version=version+1 WHERE id=$1", [receipt.id, claim]); await pool().query("UPDATE document_package_files SET state='ready',version=version+1 WHERE receipt_id=$1", [receipt.id]); await pool().query("INSERT INTO document_package_members(id,receipt_id,file_id,path,normalized_path,filename,kind,byte_size,expanded_bytes,sha256,reader_payload,reader_payload_sha256,validation_claim_id,scanner_revision,pages)VALUES($1,$2,$3,$4,$5,$4,'pdf',$6,$6,$7,$8,$9,$10,'synthetic-verified',1)", [randomUUID(), receipt.id, receipt.files[0].file_id, declaration.filename, declaration.filename.toLowerCase(), data.length, declaration.sha256, payload, digest(payload), claim]); assert.equal(await publishOnce(), true); assert.equal(await publishOnce(), false); const saved = await json('GET', `${base}/drafts/${draft.id}`); assert.equal(saved.batches[0].state, 'consumed'); assert.equal(saved.version, 2); assert.equal((await pool().query('SELECT count(*)FROM document_readings WHERE entity_id=$1', [draft.id])).rows[0].count, '0'); assert.equal((await pool().query("SELECT count(*)FROM tender.draft_events WHERE draft_id=$1 AND action='source_documents_ready'", [draft.id])).rows[0].count, '1'); });
        await t.test('verified documents download independently; removal is authorized, versioned and preserves saved TSC evidence', async () => {
            let saved = await json('GET', `${base}/drafts/${draft.id}`);
            assert.equal(saved.batches[0].documents.length, 1);
            const document = saved.batches[0].documents[0], path = `${base}/drafts/${draft.id}/documents/${document.id}`;
            const download = await raw('GET', path + '/original');
            assert.equal(download.status, 200);
            assert.deepEqual(Buffer.from(await download.arrayBuffer()), data);
            const foreign = (await pool().query("SELECT id FROM tender.document_drafts WHERE name='Foreign synthetic draft' LIMIT 1")).rows[0].id;
            await json('DELETE', `${base}/drafts/${foreign}/documents/${document.id}`, {expected_version:1}, 404);
            const reader = (await pool().query("SELECT username FROM auth.users WHERE username LIKE 'ir%' ORDER BY created_at DESC LIMIT 1")).rows[0];
            const readerToken = (await login(reader.username, 'Intake-Reader1!')).access_token;
            await json('DELETE', path, {expected_version:saved.version}, 403, undefined, readerToken);
            await json('DELETE', path, {expected_version:saved.version+1}, 409);
            const options = await tsc.options(token, draft.id), format = options.formats[0];
            assert.equal(options.sources.length, 1);
            const job = await tsc.create(token, draft.id, randomUUID(), {expected_version:saved.version,format_id:format.id,format_version:format.version,mode:'manual',source_ids:[document.id]});
            const task = randomUUID();
            await pool().query("INSERT INTO tender.tsc_workspace_tasks(id,job_id,draft_id,actor_id,operation_key,request_hash,provider)VALUES($1,$2,$3,$4,$5,$6,'gemini')",[task,job.id,draft.id,actor,randomUUID(),'a'.repeat(64)]);
            await json('DELETE', path, {expected_version:saved.version}, 409);
            await pool().query("UPDATE tender.tsc_workspace_tasks SET state='failed' WHERE id=$1",[task]);
            await json('DELETE', path, {expected_version:saved.version}, 204);
            await json('DELETE', path, {expected_version:saved.version}, 204);
            saved = await json('GET', `${base}/drafts/${draft.id}`);
            assert.equal(saved.version, 3);
            assert.equal(saved.batches[0].documents.length, 0);
            assert.equal((await tsc.options(token,draft.id)).sources.length, 0);
            assert.equal((await raw('GET',path+'/original')).status, 410);
            const historical = await tsc.get(token,draft.id,job.id);
            assert.equal(historical.snapshot.sources[0].id, document.id);
            assert.equal(historical.stale, true);
            assert.equal((await pool().query('SELECT count(*) FROM tender.draft_document_removals WHERE draft_id=$1',[draft.id])).rows[0].count, '1');
            assert.equal((await pool().query('SELECT count(*) FROM tender.tsc_model_calls WHERE draft_id=$1',[draft.id])).rows[0].count, '0');
        });
        await t.test('formats save atomically and preserve extraction identities only for unchanged legacy wording', async () => { const source = await json('GET', '/settings/tsc-formats/00000000-0000-4000-8000-000000000097'), first = source.questions[0]; format = await json('POST', '/settings/tsc-formats', { name: 'QA format ' + randomUUID(), description: 'Native format fixture', purpose: 'Tender', is_active: true, expected_version: 0, questions: [first, { id: 'new.question', section: 'Manual', printed_number: '2', question: 'Should management approve this bid?', manual_review: true }] }, 201); assert.equal(format.questions[0].id, first.id); assert.match(format.questions[1].id, /^[a-f0-9-]{36}$/); const changed = await json('PATCH', '/settings/tsc-formats/' + format.id, { ...{ name: format.name, description: format.description, purpose: format.purpose, is_active: true, expected_version: 1 }, questions: [{ ...first, question: 'What is the revised tender name?' }, format.questions[1]] }); assert.notEqual(changed.questions[0].id, first.id); assert.equal(changed.questions[0].row, 0); assert.equal(changed.version, 2); await json('PATCH', '/settings/tsc-formats/' + format.id, { name: format.name, is_active: true, expected_version: 1, questions: format.questions }, 409); format = changed; assert.equal((await pool().query('SELECT count(*)FROM tender.tsc_format_events WHERE format_id=$1', [format.id])).rows[0].count, '2'); assert.equal((await pool().query("SELECT count(*)FROM business_events WHERE entity_id=$1 AND action='tsc_format_saved'", [format.id])).rows[0].count, '2'); });
        await t.test('duplicate questions roll back format changes and Word export uses the frozen order', async () => { await json('PATCH', '/settings/tsc-formats/' + format.id, { name: format.name, is_active: true, expected_version: format.version, questions: [format.questions[0], { ...format.questions[1], question: format.questions[0].question + '!' }] }, 422); const latest = await json('GET', '/settings/tsc-formats/' + format.id); assert.equal(latest.version, 2); const doc = word(latest), zip = unzipSync(doc), xml = strFromU8(zip['word/document.xml']); assert.match(xml, /What is the revised tender name/); assert.ok(xml.indexOf('What is the revised tender name') < xml.indexOf('Should management approve')); assert.doesNotMatch(xml, /&lt;script/); const response = await raw('GET', '/settings/tsc-formats/' + format.id + '/template.docx'); assert.equal(response.status, 200); assert.match(response.headers.get('content-type')!, /wordprocessingml/); });
        await t.test('retired local OCR retry fails explicitly and native ownership excludes TSC routes', async () => { await json('POST', `${base}/drafts/${draft.id}/documents/${randomUUID()}/retry`, { expected_version: 1 }, 409); assert.equal(ownsIntake(`${base}/drafts/${draft.id}/tsc/options`), false); assert.equal(ownsIntake('/settings/tsc-formats'), true); assert.equal(ownsIntake(base + '/records'), false); });
    }
    finally {
        await rm(root, { recursive: true, force: true });
        await pool().end();
    }
});
