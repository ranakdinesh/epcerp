import assert from 'node:assert/strict';
import test from 'node:test';
import { randomUUID, randomBytes, createCipheriv } from 'node:crypto';
import { initializeAuth } from '../../src/server/auth/bootstrap';
import { login } from '../../src/server/auth/sessions';
import { createRole } from '../../src/server/auth/administration';
import { hashPassword } from '../../src/server/auth/security';
import { pool } from '../../src/server/db';
import { handleTSC } from '../../src/server/tenders/tsc/http';
import { workOnce } from '../../src/server/tenders/tsc/worker';
import { ProviderSession, parseUsage } from '../../src/server/tenders/tsc/providers';
import { estimateMicroINR, price } from '../../src/server/tenders/tsc/pricing';
import { digest, decrypt, encrypt } from '../../src/server/tenders/tsc/crypto';
import { providerAnswers, sourceSelection } from '../../src/server/tenders/tsc/validation';
import type {Answer,Question} from '../../src/server/tenders/tsc/types';
import type {create} from '../../src/server/tenders/tsc/service';
import { unzipSync, strFromU8 } from 'fflate';
test('usage accounts for caching, thoughts, long contexts, FX and configured free tier', () => {
    const u = { input_tokens: 1000, output_tokens: 200, cached_tokens: 400, cache_write_tokens: 0, thought_tokens: 100 };
    assert.equal(estimateMicroINR(price('gemini'), u), 57950);
    assert.equal(parseUsage('gemini', { usageMetadata: { promptTokenCount: 1000, candidatesTokenCount: 200, thoughtsTokenCount: 100 } })?.thought_tokens, 100);
    assert.equal(estimateMicroINR(price('openai'), { ...u, thought_tokens: 0 }), 15580);
    assert.equal(estimateMicroINR(price('openai'), { ...u, input_tokens: 300000, thought_tokens: 0 }), 5707410);
    assert.equal(parseUsage('openai', { usage: { input_tokens: -1, output_tokens: 10 } }), null);
    assert.throws(() => estimateMicroINR(price('gemini'), { ...u, cached_tokens: 1001 }));
    const paid = price('gemini');
    assert.equal(estimateMicroINR({ ...paid, tier: 'free' }, u), 0);
    assert.equal(price('gemini', true, new Date('2027-01-01')).input_usd, 1.5);
    const e = encrypt('unaltered raw response');
    assert.equal(decrypt(e.cipher, e.hash), 'unaltered raw response');
    assert.throws(() => decrypt(e.cipher, '0'.repeat(64)));
});
test('native TSC retains original responses through edits and refinement, with tender isolation', { skip: !process.env.EPCERP_DATABASE_URL }, async (t) => {
    assert.match(new URL(process.env.EPCERP_DATABASE_URL!).pathname, /^\/epcerp_auth_qa_tsc_/);
    await initializeAuth();
    const token = (await login('superadmin', process.env.VTPL_BOOTSTRAP_SUPERADMIN_PASSWORD!)).access_token;
    const owner = (await pool().query("select id from auth.users where username='superadmin'")).rows[0].id, draft = randomUUID(), other = randomUUID(), receipt = randomUUID(), file = randomUUID(), source = randomUUID(), claim = randomUUID();
    for (const id of [draft, other])
        await pool().query('insert into tender.document_drafts(id,name,owner_id,operation_key)values($1,$2,$3,$4)', [id, 'Synthetic public tender ' + id, owner, randomUUID()]);
    const pdf = Buffer.from('%PDF-1.4\nSynthetic public tender: Earnest money Rs 10000.\n%%EOF'), nonce = randomBytes(12), cipher = createCipheriv('aes-256-gcm', Buffer.from(process.env.EPCERP_DOCUMENT_READING_KEY!, 'hex'), nonce), sealed = Buffer.concat([nonce, cipher.update(pdf), cipher.final(), cipher.getAuthTag()]);
    await pool().query("insert into public.document_package_receipts(id,owner_id,draft_id,state,claim_id)values($1,$2,$3,'ready',$4)", [receipt, owner, draft, claim]);
    await pool().query("insert into public.document_package_files(id,receipt_id,client_file_key,filename,kind,declared_size,declared_sha256)values($1,$2,$3,'Tender.pdf','pdf',$4,$5)", [file, receipt, randomUUID(), pdf.length, digest(pdf)]);
    await pool().query("insert into public.document_package_members(id,receipt_id,file_id,path,normalized_path,filename,kind,byte_size,expanded_bytes,sha256,reader_payload,reader_payload_sha256,validation_claim_id,scanner_revision,pages)values($1,$2,$3,'Tender.pdf','tender.pdf','Tender.pdf','pdf',$4,$4,$5,$6,$7,$8,'synthetic-verified',1)", [source, receipt, file, pdf.length, digest(pdf), sealed, digest(sealed), claim]);
    async function req(method: string, path: string, body?: unknown, key: string = randomUUID(), access = token) { return handleTSC(new Request(`http://localhost/api/v1/tender-fms/drafts/${path}`, { method, headers: { Authorization: `Bearer ${access}`, 'Content-Type': 'application/json', 'Idempotency-Key': key }, body: body === undefined ? undefined : JSON.stringify(body) })); }
    async function json(method: string, path: string, body?: unknown, status = 200, key?: string, access?: string) { const r = await req(method, path, body, key, access), v = await r.json(); assert.equal(r.status, status, JSON.stringify(v)); return v; }
    const options = await json('GET', `${draft}/tsc/options`);
    assert.equal(options.engine, 'nextjs');
    assert.equal(options.sources[0].id, source);
    assert.ok(options.formats.length);
    const format = options.formats.find((f: {
        question_count: number;
    }) => f.question_count > 1);
    await t.test('original-document permission alone cannot authorize paid TSC work', async()=>{
      const name='tscqa'+randomUUID().replaceAll('-',''), role=await createRole(token,{name,description:'Synthetic partial TSC writer',permission_codes:['tenders.register.view','tenders.register.read','tenders.register.write','tenders.processing.read','tenders.processing.write','tenders.tsc.read','tenders.tsc.originals.write']}),user=randomUUID(),owned=randomUUID();
      await pool().query('INSERT INTO auth.users(id,username,email,password_hash,is_active,is_superuser)VALUES($1,$2,$3,$4,true,false)',[user,name,name+'@fixture.invalid',await hashPassword('TSC-Partial1!')]);await pool().query('INSERT INTO auth.user_roles(user_id,role_id)VALUES($1,$2)',[user,role.id]);await pool().query('INSERT INTO tender.document_drafts(id,name,owner_id,operation_key)VALUES($1,$2,$3,$4)',[owned,'Partial writer tender',user,randomUUID()]);
      const access=(await login(name,'TSC-Partial1!')).access_token;await json('POST',`${owned}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],original_documents_confirmed:true},403,undefined,access);await json('GET',`${draft}/tsc/options`,undefined,404,undefined,access);
    });
    let job: Awaited<ReturnType<typeof create>>, calls = 0, uploads = 0;
    const fake: typeof fetch = async (input, init) => {
        const url = String(input);
        if (init?.method === 'DELETE')
            return Response.json({ deleted: true });
        if (url.includes('/upload/v1beta/files')) {
            if ((init?.headers as Record<string, string>)['X-Goog-Upload-Command'] === 'start') {
                uploads++;
                return new Response('', { headers: { 'x-goog-upload-url': 'https://generativelanguage.googleapis.com/upload/v1beta/files?upload_id=test' } });
            }
            return Response.json({ file: { name: 'files/synthetic', uri: 'https://generativelanguage.googleapis.com/v1beta/files/synthetic', state: 'ACTIVE' } });
        }
        if (url.endsWith('/v1/files')) {
            uploads++;
            assert.equal((init?.body as FormData).get('purpose'), 'user_data');
            assert.equal((init?.body as FormData).get('expires_after[seconds]'), '86400');
            return Response.json({ id: 'file-synthetic' });
        }
        calls++;
        const payload = JSON.parse(init?.body as string), schema = url.includes('google') ? payload.generationConfig.responseJsonSchema : payload.text.format.schema, ids = schema.properties.answers.items.properties.question_id.enum, answers = ids.map((id: string) => ({ question_id: id, answer: `Model wording ${calls}: Rs 10000`, status: 'answered', citations: [{ document_id: source, page: 1, quote: 'Earnest money Rs 10000' }] }));
        if (url.includes('google'))
            return Response.json({ candidates: [{ finishReason: 'STOP', content: { parts: [{ text: JSON.stringify({ answers }) }] } }], usageMetadata: { promptTokenCount: 1000, candidatesTokenCount: 200, thoughtsTokenCount: 0 } }, { headers: { 'x-request-id': 'fixture' } });
        assert.equal(payload.store, false);
        assert.equal(payload.input[0].content[1].file_id, 'file-synthetic');
        return Response.json({ status: 'completed', output: [{ type: 'message', content: [{ type: 'output_text', text: JSON.stringify({ answers }) }] }], usage: { input_tokens: 1000, output_tokens: 200 } });
    };
    const session = () => new ProviderSession(fake);
    await t.test('permission boundaries and manual writing do not require OCR or documents', async () => { await json('GET', `${draft}/tsc/options`, undefined, 401, undefined, ''); const manual = await json('POST', `${other}/tsc`, { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'manual', source_ids: [] }, 201); assert.equal(manual.state, 'ready'); assert.equal(manual.answers.length, manual.snapshot.questions.length); assert.equal(calls, 0); });
    await t.test('same operation replays and conflicting request is rejected', async () => { const key = randomUUID(), body = { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'gemini', source_ids: [source], auto_escalate: false, original_documents_confirmed: true }; job = await json('POST', `${draft}/tsc`, body, 201, key); assert.equal((await json('POST', `${draft}/tsc`, body, 201, key)).id, job.id); await json('POST', `${draft}/tsc`, { ...body, auto_escalate: true }, 409, key); assert.equal(calls, 0); });
    await t.test('verified originals go directly to provider and token cost is saved', async () => { assert.equal(await workOnce(session), true); job = await json('GET', `${draft}/tsc/${job.id}`); assert.equal(job.state, 'ready'); assert.equal(job.error_code, ''); assert.ok(calls > 0); assert.equal(uploads, 1); assert.ok(job.cost.estimated_micro_inr > 0); assert.ok(job.answers.every((a: Answer) => !a.accepted)); assert.ok(job.answers.filter((a: Answer) => a.origin === 'gemini').length > 0); assert.equal((await pool().query('select count(*) from public.document_readings where entity_id=$1', [draft])).rows[0].count, '0'); });
    const getQ = () => job.snapshot.questions.find((q: Question) => !q.manual_review)!, getA = () => job.answers.find((a: Answer) => a.question_id === getQ().id)!;
    let originalID: string;
    await t.test('manager edits preserve immutable parsed and exact raw model response', async () => { const q = getQ(), originals = await json('GET', `${draft}/tsc/${job.id}/answers/${q.id}/originals`); assert.equal(originals.length, 1); originalID = originals[0].id; job = await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: job.version, reason: 'Manager corrects the wording', answer: { ...getA(), answer: 'Manager confirmed Rs 12000', origin: 'manual' } }); assert.equal(getA().answer, 'Manager confirmed Rs 12000'); assert.match((await json('GET', `${draft}/tsc/${job.id}/answers/${q.id}/originals`))[0].answer.answer, /Rs 10000/); const raw = await json('GET', `${draft}/tsc/${job.id}/responses/${originalID}`); assert.ok(raw.candidates); await assert.rejects(pool().query('UPDATE tender.tsc_model_responses SET http_status=201 WHERE id=$1', [originalID]), /immutable/i); await json('GET', `${other}/tsc/${job.id}/responses/${originalID}`, undefined, 404); });
    await t.test('OpenAI proposal preserves the working answer and earlier responses', async () => { const q = getQ(); await json('POST', `${draft}/tsc/${job.id}/answers/${q.id}/research`, { expected_version: job.version, provider: 'openai', stronger: false, original_documents_confirmed: true }, 202); assert.equal(await workOnce(session), true); job = await json('GET', `${draft}/tsc/${job.id}`); assert.equal(getA().answer, 'Manager confirmed Rs 12000'); assert.equal((await json('GET', `${draft}/tsc/${job.id}/answers/${q.id}/originals`)).length, 2); const count = calls; assert.equal(await workOnce(session), false); assert.equal(calls, count); });
    await t.test('optimistic editing and reviewed-answer reopen protect manager work', async () => { const q = getQ(); await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: 1, reason: 'Stale edit must fail', answer: getA() }, 409); job = await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: job.version, reason: 'Manager verified original source', answer: { ...getA(), accepted: true } }); await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: job.version, reason: 'Cannot change reviewed wording', answer: { ...getA(), answer: 'changed' } }, 409); job = await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: job.version, reason: 'Reopen for correction', answer: { ...getA(), accepted: false } }); });
    await t.test('audit history and Word export retain corrected wording', async () => { const events = await json('GET', `${draft}/tsc/${job.id}/history`); assert.ok(events.some((e: {reason:string}) => e.reason === 'Manager corrects the wording')); const response = await req('GET', `${draft}/tsc/${job.id}/document`); assert.equal(response.status, 200); const zip = unzipSync(new Uint8Array(await response.arrayBuffer())); assert.match(strFromU8(zip['word/document.xml']), /Manager confirmed Rs 12000/); });
    await t.test('uncertain paid request stays retained without automatic resend', async () => { const q = getQ(); await json('POST', `${draft}/tsc/${job.id}/answers/${q.id}/research`, { expected_version: job.version, provider: 'gemini', stronger: false, original_documents_confirmed: true }, 202); let attempted = 0; const broken: typeof fetch = async (input, init) => { if (String(input).includes(':generateContent')) {
        attempted++;
        throw Error('connection lost after send');
    } return fake(input, init); }; assert.equal(await workOnce(() => new ProviderSession(broken)), true); const next = await json('GET', `${draft}/tsc/${job.id}`); assert.equal(next.tasks[0].error_code, 'provider_outcome_uncertain'); assert.ok(next.cost.unknown_requests > 0); assert.equal(await workOnce(session), false); assert.equal(attempted, 1); assert.equal(next.answers.find((a: Answer) => a.question_id === q.id).answer, 'Manager confirmed Rs 12000'); });
    await t.test('cross-tender sources and invalid source references cannot be trusted', async () => { await json('POST', `${other}/tsc`, { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'gemini', source_ids: [source], original_documents_confirmed: true }, 422); const s = options.sources[0], q = getQ(); assert.throws(() => sourceSelection([s.id, s.id], [s], 'gemini')); const [answer] = providerAnswers({ answers: [{ question_id: q.id, answer: 'Unverified', status: 'answered', citations: [{ document_id: randomUUID(), page: 1, quote: 'fake' }] }] }, [q], [s], 'gemini'); assert.equal(answer.status, 'needs_review'); assert.equal(answer.accepted, false); });
    await t.test('allowance failure prevents uploads and does not erase manual answers', async () => { process.env.EPCERP_TSC_TENDER_LIMIT_PAISE = '100'; const before = uploads; const next = await json('POST', `${draft}/tsc`, { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'gemini', source_ids: [source], original_documents_confirmed: true }, 201); assert.equal(await workOnce(session), true); const stopped = await json('GET', `${draft}/tsc/${next.id}`); assert.equal(stopped.error_code, 'source_or_budget_changed'); assert.equal(stopped.answers.length, next.answers.length); assert.equal(uploads, before); delete process.env.EPCERP_TSC_TENDER_LIMIT_PAISE; });
    await t.test('daily allowance prevents additional uploads across the shared account',async()=>{process.env.EPCERP_TSC_DAILY_LIMIT_PAISE='100';const before=uploads;try{const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],original_documents_confirmed:true},201);await workOnce(session);const stopped=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(stopped.error_code,'daily_allowance_reached');assert.equal(uploads,before)}finally{delete process.env.EPCERP_TSC_DAILY_LIMIT_PAISE}});
    await t.test('source changes after a model reply retain evidence without applying stale answers', async () => { const next = await json('POST', `${draft}/tsc`, { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'gemini', source_ids: [source], original_documents_confirmed: true }, 201); const changed: typeof fetch = async (input, init) => { const response = await fake(input, init); if (String(input).includes(':generateContent'))
        await pool().query('update tender.document_drafts set version=version+1 where id=$1', [draft]); return response; }; await workOnce(() => new ProviderSession(changed)); const stopped = await json('GET', `${draft}/tsc/${next.id}`); assert.equal(stopped.error_code, 'permission_or_source_changed'); assert.ok(stopped.answers.every((a: Answer) => a.answer === '')); assert.ok((await json('GET', `${draft}/tsc/${next.id}/answers/${getQ().id}/originals`)).length > 0); assert.equal(stopped.stale, true); });
    await t.test('revoked actor prevents uploads and leaves manual recovery available',async()=>{
       const next=await json('POST',`${draft}/tsc`,{expected_version:2,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],original_documents_confirmed:true},201),before=uploads;
       await pool().query('UPDATE auth.users SET is_active=false WHERE id=$1',[owner]);try{await workOnce(session)}finally{await pool().query('UPDATE auth.users SET is_active=true WHERE id=$1',[owner])}
       const stopped=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(stopped.error_code,'permission_or_source_changed');assert.equal(stopped.answers.length,next.answers.length);assert.equal(uploads,before);
    });
    await pool().end();
});
