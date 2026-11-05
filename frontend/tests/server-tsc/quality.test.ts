import assert from 'node:assert/strict';
import test from 'node:test';
import {ProviderSession} from '../../src/server/tenders/tsc/providers';
import {price,reservePaise} from '../../src/server/tenders/tsc/pricing';
import {sourceIndex,verifyIndexedAnswers} from '../../src/server/tenders/tsc/quality';
import{confirmedRejection}from '../../src/server/tenders/tsc/billing';
import{docx}from '../../src/server/tenders/tsc/export';
import{unzipSync,strFromU8}from'fflate';
import type{Original,Question,Snapshot,Answer}from'../../src/server/tenders/tsc/types';
const original=(id:string):Original=>({id,receipt_id:'r',claim_id:'c',sha256:'a'.repeat(64),path:id+'.pdf',kind:'pdf',byte_size:12,pages:1,data:Buffer.from('%PDF-test')});
const question:Question={id:'payment.q01',section:'payment',printed_number:'1',question:'What are the payment terms?',manual_review:false};
const text=(value:string)=>({reader_version:'pdf-text-v1',pages:[{page:1,text:value,method:'native'}],warnings:[]});
test('quality counts the exact PDF/page index, bounds the input reservation and uniquely repairs working references',async()=>{
 const docs=[original('gcc'),original('payment')],requests:{url:string;body:Record<string,unknown>}[]=[];let fences=0;
 const answer={answers:[{question_id:question.id,status:'answered',answer:'5% per year; 25% total.',citations:[{document_id:'D01',page:1,quote:'25% total retention, released at 5% per year.'}]}]};
 const session=new ProviderSession(async(url,init)=>{const body=JSON.parse(init!.body as string);requests.push({url:String(url),body});return String(url).endsWith(':countTokens')?Response.json({totalTokens:200000}):Response.json({candidates:[{finishReason:'STOP',content:{parts:[{text:JSON.stringify(answer)}]}}],usageMetadata:{promptTokenCount:200000,candidatesTokenCount:1000,thoughtsTokenCount:500}})},undefined,true,async d=>text(d.id==='payment'?'25% total retention, released at 5% per year.':'General conditions.'));
 const p=await session.preparePrice(price('gemini',true),[question],docs,async()=>{fences++});assert.equal(p.max_input_tokens,224097);assert.ok(reservePaise(p)<reservePaise(price('gemini',true)));
 const reply=await session.propose(p,[question],docs,async()=>{fences++});assert.equal(reply.error,'');assert.equal(requests.length,2);assert.equal(fences,3);
 const counted=requests[0].body.generateContentRequest as {contents:unknown};assert.deepEqual(counted.contents,requests[1].body.contents);
 assert.equal((reply.content as typeof answer).answers[0].citations[0].document_id,'payment');assert.equal(answer.answers[0].citations[0].document_id,'D01');assert.match(reply.raw,/D01/);assert.equal(reply.evidence?.supplied_pages,2);await session.close();
});
test('ambiguous quotations and unsupported claims remain marked for review',()=>{
 const index=sourceIndex([{doc:original('first'),text:text('The same common sentence appears in both documents.')},{doc:original('second'),text:text('The same common sentence appears in both documents.')}]);
 const response={answers:[{status:'answered',citations:[{document_id:'D01',page:99,quote:'The same common sentence appears in both documents.'}]},{status:'answered',citations:[]}]};verifyIndexedAnswers(response,index);assert.ok(response.answers.every(a=>a.status==='needs_review'));
});
test('token count failure prevents generation',async()=>{
 let requests=0;const session=new ProviderSession(async()=>{requests++;return Response.json({error:'unavailable'},{status:503})},undefined,true,async()=>text('Tender terms.'));
 await assert.rejects(session.preparePrice(price('gemini',true),[question],[original('doc')],async()=>{}),/token_count_failed/);assert.equal(requests,1);await session.close();
});
test('Word retains all questions with portrait sections, repeating headers, explicit manual fields and compact references',()=>{
 const manual={...question,id:'manual',section:'conclusion',manual_review:true,question:'Should we bid?'};const snapshot={name:'Synthetic tender',format_name:'Standard checklist',questions:[question,manual],sources:[original('source')]}as unknown as Snapshot;
 const answers:Answer[]=[{question_id:question.id,status:'answered',answer:'5% per year; 25% total.',citations:[{document_id:'source',page:1,quote:'Long verbatim quotation retained in the workspace.'}],accepted:false,origin:'gemini'}];
 const files=unzipSync(docx(snapshot,answers,100)),body=strFromU8(files['word/document.xml']);assert.match(body,/Should we bid/);assert.match(body,/Management review required/);assert.match(body,/5% per year; 25% total/);assert.match(body,/Source: source.pdf/);assert.doesNotMatch(body,/Long verbatim quotation|landscape/);assert.equal((body.match(/<w:tblHeader\/>/g)??[]).length,2);assert.ok(files['word/styles.xml']);
});

test('only a structured Google quota refusal resolves a rejected generation hold',()=>{
 const raw=JSON.stringify({error:{code:429,status:'RESOURCE_EXHAUSTED',details:[{'@type':'type.googleapis.com/google.rpc.QuotaFailure',violations:[{quotaMetric:'generativelanguage.googleapis.com/generate_content_free_tier_input_token_count'}]}]}});
 assert.equal(confirmedRejection('gemini',503,JSON.stringify({error:{code:503,status:'UNAVAILABLE'}})),true);assert.equal(confirmedRejection('gemini',504,JSON.stringify({error:{code:504,status:'DEADLINE_EXCEEDED'}})),false);
 assert.equal(confirmedRejection('gemini',429,raw),true);assert.equal(confirmedRejection('gemini',503,raw),false);assert.equal(confirmedRejection('groq',429,raw),false);assert.equal(confirmedRejection('gemini',429,'{}'),false);assert.equal(confirmedRejection('gemini',429,'not JSON'),false);
});

test('tender-specific notices and payment terms remain alongside general evidence',async()=>{
 const {selectQualityEvidence}=await import('../../src/server/tenders/tsc/quality');
 const notice={...original('nit'),path:'viewNitPdf.pdf'},general={...original('gcc'),path:'GCC.pdf'},payment={...original('pay'),path:'PricesAndPaymentClause.pdf'};
 const e=selectQualityEvidence([{doc:notice,text:text('JV bidding is not allowed for this invitation.')},{doc:general,text:text('General conditions permit JV bidding above a threshold.')},{doc:payment,text:text('General items: 70% on supply and 30% on commissioning.')}],[{...question,question:'Are joint ventures permitted?'}],10000);
 assert.ok(e.pages.some(p=>p.document_id==='nit'));assert.ok(e.pages.some(p=>p.document_id==='pay'));assert.ok(e.pages.some(p=>p.document_id==='gcc'));
});
test('cross-checks flag incompatible JV answers and unsupported auction conclusions while preserving reviewed wording',async()=>{
 const {crossCheckAnswers}=await import('../../src/server/tenders/tsc/quality');
 const questions=[{...question,id:'jv1',question:'Is Joint Venture bidding allowed?'},{...question,id:'jv2',question:'Is consortium or joint venture permitted?'},{...question,id:'ra',question:'Reverse auction applicability?'}];
 const base:Answer={question_id:'jv1',answer:'No, not allowed.',status:'answered',citations:[],accepted:false,origin:'gemini'},answers=[base,{...base,question_id:'jv2',answer:'Joint ventures are permitted above the threshold.'},{...base,question_id:'ra',answer:'Not applicable because it is single packet.',citations:[{document_id:'doc',page:1,quote:'Single Packet System'}]}];
 crossCheckAnswers(questions,answers);assert.equal(answers[0].status,'conflicting');assert.equal(answers[1].status,'conflicting');assert.equal(answers[2].status,'needs_review');assert.match(answers[2].answer,/not established/);
 const reviewed={...base,accepted:true,status:'answered' as const,answer:'Reviewed wording'};crossCheckAnswers(questions,[reviewed]);assert.equal(reviewed.answer,'Reviewed wording');assert.equal(reviewed.status,'answered');
});

test('native PDF quality input keeps all PDF bytes and excludes the duplicate full-text index',async()=>{
 const docs=[original('one'),original('two')],bodies:Record<string,unknown>[]=[];
 const s=new ProviderSession(async(url,init)=>{bodies.push(JSON.parse(init!.body as string));return String(url).endsWith(':countTokens')?Response.json({totalTokens:200000}):Response.json({candidates:[{finishReason:'STOP',content:{parts:[{text:'{"answers":[]}'}]}}],usageMetadata:{promptTokenCount:200000,candidatesTokenCount:1}})},undefined,true,async()=>text('LOCAL VERIFICATION TEXT NOT SENT AS DUPLICATE CONTEXT'));
 const p=await s.preparePrice(price('gemini',true),[question],docs,async()=>{},'pdf');await s.propose(p,[question],docs,async()=>{});assert.equal(p.input_mode,'pdf');const request=bodies[1]as {contents:{parts:{inlineData?:{data:string}}[]}[]};assert.deepEqual(request.contents[0].parts.filter(p=>p.inlineData).map(p=>Buffer.from(p.inlineData!.data,'base64')),docs.map(d=>d.data));assert.doesNotMatch(JSON.stringify(bodies[1]),/LOCAL VERIFICATION TEXT/);await s.close();
});
test('credential enhancement is not inferred from a contract price-variation clause',async()=>{
 const {crossCheckAnswers}=await import('../../src/server/tenders/tsc/quality'),q={...question,id:'enhance',question:'Is enhancement of executed work value to the current price level allowed?'};
 const base:Answer={question_id:q.id,answer:'Yes, the Price Variation Clause (PVC) applies.',status:'answered',citations:[{document_id:'gcc',page:9,quote:'Price variation clause (PVC) is applicable.'}],accepted:false,origin:'gemini'};
 const checked=crossCheckAnswers([q],[structuredClone(base)])[0];assert.equal(checked.status,'needs_review');assert.match(checked.answer,/eligibility provisions separately/);
 const supported={...base,citations:[{document_id:'gcc',page:10,quote:'Completed work credentials may be enhanced annually to current price level.'}]};assert.equal(crossCheckAnswers([q],[supported])[0].answer,base.answer);
});
