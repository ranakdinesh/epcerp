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
import { zipSync,strToU8,unzipSync, strFromU8 } from 'fflate';
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
    await t.test('Word import previews and saves without cloud access, preserving originals and explicit review',async()=>{
      const qs=(await pool().query<Question>('SELECT id,section,question,printed_number,manual_review FROM tender.tsc_format_questions WHERE format_id=$1 ORDER BY position LIMIT 2',[format.id])).rows;
      const xml=`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:tbl><w:tr><w:tc><w:p><w:r><w:t>Question</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Answer</w:t></w:r></w:p></w:tc></w:tr>${[qs[0].question,'A separate condition'].map(q=>`<w:tr><w:tc><w:p><w:r><w:t>${q.replaceAll('&','&amp;')}</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Imported human wording</w:t></w:r></w:p></w:tc></w:tr>`).join('')}</w:tbl></w:body></w:document>`;
      const data=Buffer.from(zipSync({'[Content_Types].xml':strToU8('<Types><Override ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>'),'word/document.xml':strToU8(xml)})),body={expected_version:1,format_id:format.id,format_version:format.version,filename:'Completed TSC.docx',document:data.toString('base64')},before=Number((await pool().query('SELECT count(*) FROM tender.tsc_model_calls')).rows[0].count),approved=process.env.EPCERP_TSC_CLOUD_APPROVED;
      process.env.EPCERP_TSC_CLOUD_APPROVED='false';try{
       const preview=await json('POST',`${other}/tsc/import`,body);assert.equal(preview.rows.length,2);assert.equal(preview.rows[0].question_id,qs[0].id);
       const mapping=[{row_id:1,question_id:qs[0].id},{row_id:1,question_id:qs[1].id}],request={...body,mapping,reviewed:false,skip_unmatched_confirmed:false};await json('POST',`${other}/tsc/import/commit`,request,422);
       const key=randomUUID(),saved=await json('POST',`${other}/tsc/import/commit`,{...request,skip_unmatched_confirmed:true},201,key);assert.equal(saved.snapshot.preparation_method,'upload');assert.equal(saved.processing.phase,'imported');assert.equal(saved.answers.filter((a:Answer)=>a.answer).length,2);assert.ok(saved.answers.filter((a:Answer)=>a.answer).every((a:Answer)=>a.origin==='upload'&&!a.accepted));assert.equal(saved.snapshot.sources.length,0);
       const replay=await json('POST',`${other}/tsc/import/commit`,{...request,skip_unmatched_confirmed:true},201,key);assert.equal(replay.id,saved.id);await json('POST',`${other}/tsc/import/commit`,{...request,skip_unmatched_confirmed:true,reviewed:true},409,key);
       const original=await req('GET',`${other}/tsc/${saved.id}/uploaded-document`);assert.equal(original.status,200);assert.deepEqual(Buffer.from(await original.arrayBuffer()),data);await json('GET',`${draft}/tsc/${saved.id}/uploaded-document`,undefined,404);
       await json('POST',`${other}/tsc/import/commit`,{...request,mapping:[...mapping,{row_id:2,question_id:qs[0].id}],skip_unmatched_confirmed:true},422);
       const reviewed=await json('POST',`${other}/tsc/import/commit`,{...request,skip_unmatched_confirmed:true,reviewed:true},201);assert.ok(reviewed.answers.filter((a:Answer)=>a.answer).every((a:Answer)=>a.accepted));
       const listed=await json('GET',`${other}/tsc`);assert.equal(listed.items.find((j:{id:string})=>j.id===saved.id).preparation_method,'upload');assert.equal(Number((await pool().query('SELECT count(*) FROM tender.tsc_model_calls')).rows[0].count),before);
      }finally{if(approved===undefined)delete process.env.EPCERP_TSC_CLOUD_APPROVED;else process.env.EPCERP_TSC_CLOUD_APPROVED=approved}
    });
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
    const session = () => new ProviderSession(fake, undefined, false);
    await t.test('permission boundaries and manual writing do not require OCR or documents', async () => { await json('GET', `${draft}/tsc/options`, undefined, 401, undefined, ''); const manual = await json('POST', `${other}/tsc`, { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'manual', source_ids: [] }, 201); assert.equal(manual.state, 'ready'); assert.equal(manual.answers.length, manual.snapshot.questions.length); assert.equal(calls, 0); });
    await t.test('same operation replays and conflicting request is rejected', async () => { const key = randomUUID(), body = { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'gemini', source_ids: [source], auto_escalate: false, original_documents_confirmed: true }; job = await json('POST', `${draft}/tsc`, body, 201, key); assert.equal((await json('POST', `${draft}/tsc`, body, 201, key)).id, job.id); await json('POST', `${draft}/tsc`, { ...body, auto_escalate: true }, 409, key); assert.equal(calls, 0); });
    await t.test('verified originals go directly to provider and token cost is saved', async () => { assert.equal(await workOnce(session), true); job = await json('GET', `${draft}/tsc/${job.id}`); assert.equal(job.state, 'ready'); assert.equal(job.error_code, ''); assert.ok(calls > 0); assert.equal(uploads, 1); assert.ok(job.cost.estimated_micro_inr > 0); assert.ok(job.answers.every((a: Answer) => !a.accepted)); assert.ok(job.answers.filter((a: Answer) => a.origin === 'gemini').length > 0); assert.equal((await pool().query('select count(*) from public.document_readings where entity_id=$1', [draft])).rows[0].count, '0'); });
    const getQ = () => job.snapshot.questions.find((q: Question) => !q.manual_review)!, getA = () => job.answers.find((a: Answer) => a.question_id === getQ().id)!;
    let originalID: string;
    await t.test('answers become visible before remote-file cleanup finishes',async()=>{
      const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],original_documents_confirmed:true},201);
      let release!:()=>void,started!:()=>void;
      const gate=new Promise<void>(r=>release=r),cleanup=new Promise<void>(r=>started=r);
      const processing=workOnce(()=>{const session=new ProviderSession(fake,undefined,false),close=session.close.bind(session);session.close=async()=>{started();await gate;await close()};return session});
      try{await cleanup;const visible=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(visible.state,'ready');assert.equal(visible.processing.phase,'complete');assert.ok(visible.processing.returned_answers>0)}finally{release();await processing}
    });
    await t.test('Gemini alternative model uses the existing provider connection and explicit model selection',async()=>{
      const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',stronger_model:true,source_ids:[source],original_documents_confirmed:true},201);
      let requested='',counts=0;const quality:typeof fetch=async(input,init)=>{if(String(input).includes(':countTokens')){counts++;return Response.json({totalTokens:1000})}if(String(input).includes(':generateContent'))requested=String(input);return fake(input,init)};
      process.env.EPCERP_TSC_GEMINI_QUALITY_MODEL='gemini-3.7-flash';
      await workOnce(()=>new ProviderSession(quality,undefined,false,async()=>({reader_version:'pdf-text-v1',pages:[{page:1,text:'Earnest money Rs 10000',method:'native'}],warnings:[]})));delete process.env.EPCERP_TSC_GEMINI_QUALITY_MODEL;const ready=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(ready.error_code,'');assert.match(requested,/gemini-3\.8-flash:generateContent/);assert.equal(ready.snapshot.stronger_model,true);assert.equal(ready.snapshot.quality_model,'gemini-3.8-flash');assert.equal(counts,1);assert.equal(ready.processing.evidence.total_pages,1);const rates=(await pool().query('SELECT pricing FROM tender.tsc_model_calls WHERE job_id=$1',[next.id])).rows;assert.equal(rates[0].pricing.max_input_tokens,5196);
    });
    await t.test('partial quality batches resume only missing questions and retain the original successful batch',async(st)=>{
      const previous=process.env.EPCERP_TSC_GEMINI_QUESTION_BATCH_SIZE;process.env.EPCERP_TSC_GEMINI_QUESTION_BATCH_SIZE='32';st.after(()=>{if(previous===undefined)delete process.env.EPCERP_TSC_GEMINI_QUESTION_BATCH_SIZE;else process.env.EPCERP_TSC_GEMINI_QUESTION_BATCH_SIZE=previous});
      const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',stronger_model:true,source_ids:[source],original_documents_confirmed:true},201),first=next.snapshot.questions.find((q:Question)=>!q.manual_review).id;
      const loader=async()=>({reader_version:'pdf-text-v1',pages:[{page:1,text:'Earnest money Rs 10000',method:'native'}],warnings:[]});
      const partial:typeof fetch=async(input,init)=>{if(String(input).endsWith(':countTokens'))return Response.json({totalTokens:1000});if(String(input).endsWith(':generateContent')){const ids=JSON.parse(init!.body as string).generationConfig.responseJsonSchema.properties.answers.items.properties.question_id.enum;if(!ids.includes(first))return Response.json({error:{code:503,status:'UNAVAILABLE'}},{status:503})}return fake(input,init)};
      await workOnce(()=>new ProviderSession(partial,undefined,true,loader));const stopped=await json('GET',`${draft}/tsc/${next.id}`),retained=stopped.answers.filter((a:Answer)=>a.answer.trim());assert.equal(retained.length,32);assert.equal(stopped.error_code,'provider_unavailable');
      await json('POST',`${draft}/tsc/${next.id}/resume`,{expected_version:stopped.version,original_documents_confirmed:false},422);
      const key=randomUUID(),body={expected_version:stopped.version,original_documents_confirmed:true},queued=await json('POST',`${draft}/tsc/${next.id}/resume`,body,202,key);assert.equal(queued.state,'queued');assert.equal(queued.answers.filter((a:Answer)=>a.answer.trim()).length,32);await json('POST',`${draft}/tsc/${next.id}/resume`,body,202,key);
      let requested:string[]=[];const recovered:typeof fetch=async(input,init)=>{if(String(input).endsWith(':countTokens'))return Response.json({totalTokens:1000});if(String(input).endsWith(':generateContent'))requested=JSON.parse(init!.body as string).generationConfig.responseJsonSchema.properties.answers.items.properties.question_id.enum;return fake(input,init)};
      await workOnce(()=>new ProviderSession(recovered,undefined,true,loader));const ready=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(ready.error_code,'');assert.equal(requested.length,32);assert.ok(retained.every((a:Answer)=>!requested.includes(a.question_id)&&ready.answers.find((b:Answer)=>b.question_id===a.question_id).answer===a.answer));assert.equal(ready.answers.filter((a:Answer)=>a.answer.trim()).length,64);assert.equal(ready.tasks.length,2);assert.equal(ready.cost.requests.length,3);
    });
    await t.test('a structured Google quota refusal retains evidence without retaining a generation allowance hold',async()=>{
      const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],original_documents_confirmed:true},201);
      const rejected:typeof fetch=async(input,init)=>String(input).includes(':generateContent')?Response.json({error:{code:429,status:'RESOURCE_EXHAUSTED',details:[{'@type':'type.googleapis.com/google.rpc.QuotaFailure',violations:[{quotaMetric:'generativelanguage.googleapis.com/generate_content_free_tier_input_token_count'}]}]}},{status:429}):fake(input,init);
      await workOnce(()=>new ProviderSession(rejected,undefined,false));const stopped=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(stopped.error_code,'quota_or_rate_limit');assert.equal(stopped.cost.unknown_requests,0);assert.equal(stopped.cost.unconfirmed_reserved_paise,0);assert.equal(stopped.cost.requests[0].http_status,429);assert.equal(stopped.cost.requests[0].estimated_micro_inr,0);assert.equal(stopped.cost.requests[0].raw_cipher,undefined);
    });
    await t.test('opt-in recovery retries one conclusive rejection and retains both responses',async()=>{
      const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],recover_overload:true,original_documents_confirmed:true},201);
      let rejected=false,attempts=0;
      const recover:typeof fetch=async(input,init)=>{if(String(input).endsWith(':generateContent')){attempts++;if(!rejected){rejected=true;return Response.json({error:{code:503},usageMetadata:{promptTokenCount:0,candidatesTokenCount:0}},{status:503})}}return fake(input,init)};
      await workOnce(()=>new ProviderSession(recover,undefined,false));const ready=await json('GET',`${draft}/tsc/${next.id}`);
      assert.equal(ready.error_code,'');assert.equal(ready.processing.phase,'complete');assert.equal(attempts,2);assert.equal(ready.cost.requests.length,2);
    });
    await t.test('alternative providers persist cited answers, token costs and input coverage',async()=>{
      for(const provider of ['groq','deepseek']){
       const key=provider==='groq'?'EPCERP_TSC_GROQ_API_KEY':'EPCERP_TSC_DEEPSEEK_API_KEY',before=process.env[key];process.env[key]='synthetic-test-key';
       try{
        const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:provider,source_ids:[source],original_documents_confirmed:true},201);let loads=0;
        const fetcher:typeof fetch=async(input,init)=>{const body=JSON.parse(init?.body as string),request=JSON.parse(body.messages[1].content),ids=request.questions.map((q:{id:string})=>q.id);return Response.json({choices:[{finish_reason:'stop',message:{content:JSON.stringify({answers:ids.map((id:string)=>({question_id:id,status:'answered',answer:'Earnest money Rs 10000',citations:[{document_id:source,page:1,quote:'Earnest money Rs 10000'}]}))})}}],usage:{prompt_tokens:1000,completion_tokens:200}})};
        await workOnce(()=>new ProviderSession(fetcher,undefined,true,async()=>{loads++;return{reader_version:'pdf-text-v1',pages:[{page:1,text:'Earnest money Rs 10000',method:'native'}],warnings:[]}}));
        const done=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(done.error_code,'');assert.equal(done.processing.phase,'complete');assert.equal(loads,1);assert.equal(done.processing.evidence.partial,false);assert.ok(done.answers.some((a:Answer)=>a.origin===provider&&a.answer));assert.ok(done.cost.estimated_micro_inr>0);
       }finally{if(before===undefined)delete process.env[key];else process.env[key]=before}
      }
    });
    await t.test('Gemini overload produces an explicit failed generation with no answers and no automatic retry',async()=>{
      const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],original_documents_confirmed:true},201);
      let attempts=0;
      const overloaded:typeof fetch=async(input,init)=>{if(String(input).endsWith(':generateContent')){attempts++;return Response.json({error:{code:503,status:'UNAVAILABLE',message:'High demand'}},{status:503})}return fake(input,init)};
      await workOnce(()=>new ProviderSession(overloaded,undefined,false));
      const stopped=await json('GET',`${draft}/tsc/${next.id}`);
      assert.equal(stopped.processing.phase,'failed');assert.equal(stopped.processing.provider_status,503);assert.equal(stopped.processing.returned_answers,0);assert.equal(stopped.error_code,'provider_unavailable');assert.ok(stopped.answers.every((a:Answer)=>!a.answer));assert.equal(stopped.tasks[0].state,'failed');assert.equal((await req('GET',`${draft}/tsc/${next.id}/document`)).status,409);assert.equal(await workOnce(session),false);assert.equal(attempts,1);
    });
    await t.test('manager edits preserve immutable parsed and exact raw model response', async () => { const q = getQ(), originals = await json('GET', `${draft}/tsc/${job.id}/answers/${q.id}/originals`); assert.equal(originals.length, 1); originalID = originals[0].id; job = await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: job.version, reason: 'Manager corrects the wording', answer: { ...getA(), answer: 'Manager confirmed Rs 12000', origin: 'manual' } }); assert.equal(getA().answer, 'Manager confirmed Rs 12000'); assert.match((await json('GET', `${draft}/tsc/${job.id}/answers/${q.id}/originals`))[0].answer.answer, /Rs 10000/); const raw = await json('GET', `${draft}/tsc/${job.id}/responses/${originalID}`); assert.ok(raw.candidates); await assert.rejects(pool().query('UPDATE tender.tsc_model_responses SET http_status=201 WHERE id=$1', [originalID]), /immutable/i); await json('GET', `${other}/tsc/${job.id}/responses/${originalID}`, undefined, 404); });
    await t.test('OpenAI proposal preserves the working answer and earlier responses', async () => { const q = getQ(); await json('POST', `${draft}/tsc/${job.id}/answers/${q.id}/research`, { expected_version: job.version, provider: 'openai', stronger: false, original_documents_confirmed: true }, 202); assert.equal(await workOnce(session), true); job = await json('GET', `${draft}/tsc/${job.id}`); assert.equal(getA().answer, 'Manager confirmed Rs 12000'); assert.equal((await json('GET', `${draft}/tsc/${job.id}/answers/${q.id}/originals`)).length, 2); const count = calls; assert.equal(await workOnce(session), false); assert.equal(calls, count); });
    await t.test('optimistic editing and reviewed-answer reopen protect manager work', async () => { const q = getQ(); await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: 1, reason: 'Stale edit must fail', answer: getA() }, 409); job = await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: job.version, reason: 'Manager verified original source', answer: { ...getA(), accepted: true } }); await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: job.version, reason: 'Cannot change reviewed wording', answer: { ...getA(), answer: 'changed' } }, 409); job = await json('PATCH', `${draft}/tsc/${job.id}/answers/${q.id}`, { expected_version: job.version, reason: 'Reopen for correction', answer: { ...getA(), accepted: false } }); });
    await t.test('audit history and Word export retain corrected wording', async () => { const events = await json('GET', `${draft}/tsc/${job.id}/history`); assert.ok(events.some((e: {reason:string}) => e.reason === 'Manager corrects the wording')); const response = await req('GET', `${draft}/tsc/${job.id}/document`); assert.equal(response.status, 200); const zip = unzipSync(new Uint8Array(await response.arrayBuffer())); assert.match(strFromU8(zip['word/document.xml']), /Manager confirmed Rs 12000/); });
    await t.test('uncertain paid request stays retained without automatic resend', async () => { const q = getQ(); await json('POST', `${draft}/tsc/${job.id}/answers/${q.id}/research`, { expected_version: job.version, provider: 'gemini', stronger: false, original_documents_confirmed: true }, 202); let attempted = 0; const broken: typeof fetch = async (input, init) => { if (String(input).includes(':generateContent')) {
        attempted++;
        throw Error('connection lost after send');
    } return fake(input, init); }; assert.equal(await workOnce(() => new ProviderSession(broken, undefined, false)), true); const next = await json('GET', `${draft}/tsc/${job.id}`); assert.equal(next.tasks[0].error_code, 'provider_outcome_uncertain');await json('POST',`${draft}/tsc/${job.id}/resume`,{expected_version:next.version,original_documents_confirmed:true},409); assert.ok(next.cost.unknown_requests > 0); assert.equal(await workOnce(session), false); assert.equal(attempted, 1); assert.equal(next.answers.find((a: Answer) => a.question_id === q.id).answer, 'Manager confirmed Rs 12000'); });
    await t.test('cross-tender sources and invalid source references cannot be trusted', async () => { await json('POST', `${other}/tsc`, { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'gemini', source_ids: [source], original_documents_confirmed: true }, 422); const s = options.sources[0], q = getQ(); assert.throws(() => sourceSelection([s.id, s.id], [s], 'gemini')); const [answer] = providerAnswers({ answers: [{ question_id: q.id, answer: 'Unverified', status: 'answered', citations: [{ document_id: randomUUID(), page: 1, quote: 'fake' }] }] }, [q], [s], 'gemini'); assert.equal(answer.status, 'needs_review'); assert.equal(answer.accepted, false); });
    await t.test('allowance failure prevents uploads and does not erase manual answers', async () => { const before = uploads; const next = await json('POST', `${draft}/tsc`, { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'gemini', source_ids: [source], original_documents_confirmed: true }, 201); process.env.EPCERP_TSC_TENDER_LIMIT_PAISE = '100'; assert.equal(await workOnce(session), true); const stopped = await json('GET', `${draft}/tsc/${next.id}`); assert.equal(stopped.error_code, 'tender_allowance_reached'); assert.equal(stopped.answers.length, next.answers.length); assert.equal(uploads, before); delete process.env.EPCERP_TSC_TENDER_LIMIT_PAISE; });
    await t.test('daily allowance prevents additional uploads across the shared account',async()=>{process.env.EPCERP_TSC_DAILY_LIMIT_PAISE='100';const before=uploads;try{const next=await json('POST',`${draft}/tsc`,{expected_version:1,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],original_documents_confirmed:true},201);await workOnce(session);const stopped=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(stopped.error_code,'daily_allowance_reached');assert.equal(uploads,before)}finally{delete process.env.EPCERP_TSC_DAILY_LIMIT_PAISE}});
    await t.test('source changes after a model reply retain evidence without applying stale answers', async () => { const next = await json('POST', `${draft}/tsc`, { expected_version: 1, format_id: format.id, format_version: format.version, mode: 'gemini', source_ids: [source], original_documents_confirmed: true }, 201); const changed: typeof fetch = async (input, init) => { const response = await fake(input, init); if (String(input).includes(':generateContent'))
        await pool().query('update tender.document_drafts set version=version+1 where id=$1', [draft]); return response; }; await workOnce(() => new ProviderSession(changed, undefined, false)); const stopped = await json('GET', `${draft}/tsc/${next.id}`); assert.equal(stopped.error_code, 'permission_or_source_changed'); assert.ok(stopped.answers.every((a: Answer) => a.answer === '')); assert.ok((await json('GET', `${draft}/tsc/${next.id}/answers/${getQ().id}/originals`)).length > 0); assert.equal(stopped.stale, true); });
    await t.test('revoked actor prevents uploads and leaves manual recovery available',async()=>{
       const next=await json('POST',`${draft}/tsc`,{expected_version:2,format_id:format.id,format_version:format.version,mode:'gemini',source_ids:[source],original_documents_confirmed:true},201),before=uploads;
       await pool().query('UPDATE auth.users SET is_active=false WHERE id=$1',[owner]);try{await workOnce(session)}finally{await pool().query('UPDATE auth.users SET is_active=true WHERE id=$1',[owner])}
       const stopped=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(stopped.error_code,'permission_or_source_changed');assert.equal(stopped.answers.length,next.answers.length);assert.equal(uploads,before);
    });
    await t.test('recovery consent never authorizes resending an uncertain request',async()=>{
      const next=await json('POST',`${draft}/tsc`,{expected_version:2,format_id:format.id,format_version:format.version,mode:'gemini',recover_overload:true,source_ids:[source],original_documents_confirmed:true},201);
      let attempts=0;const lost:typeof fetch=async(input,init)=>{if(String(input).includes(':generateContent')){attempts++;throw Error('unknown outcome')}return fake(input,init)};
      await workOnce(()=>new ProviderSession(lost,undefined,false));const stopped=await json('GET',`${draft}/tsc/${next.id}`);assert.equal(stopped.error_code,'provider_outcome_uncertain');assert.equal(attempts,1);assert.equal(await workOnce(session),false);
    });
    await pool().end();
});
