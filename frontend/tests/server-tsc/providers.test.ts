import assert from 'node:assert/strict';
import test from 'node:test';
import {ProviderSession} from '../../src/server/tenders/tsc/providers';
import {price} from '../../src/server/tenders/tsc/pricing';
import type {Original,Question} from '../../src/server/tenders/tsc/types';
const doc=(id:string):Original=>({id,receipt_id:'receipt',sha256:'a'.repeat(64),path:id+'.pdf',kind:'pdf',byte_size:12,pages:1,claim_id:'claim',data:Buffer.from('%PDF-fixture')});
const question:Question={id:'q1',section:'Facts',printed_number:'1',question:'What is the tender value?',manual_review:false};
const response=()=>Response.json({candidates:[{finishReason:'STOP',content:{parts:[{text:JSON.stringify({answers:[]})}]}}],usageMetadata:{promptTokenCount:10,candidatesTokenCount:2}});

test('bounded Gemini PDF packages use one inline generation request and no temporary file uploads',async()=>{
 const calls:{url:string;init?:RequestInit}[]=[];let sent=0;
 const session=new ProviderSession(async(input,init)=>{calls.push({url:String(input),init});return response()});
 const docs=[doc('one'),doc('two')],reply=await session.propose(price('gemini'),[question],docs,async()=>{sent++});await session.close();
 assert.equal(reply.error,'');assert.equal(sent,1);assert.equal(calls.length,1);assert.match(calls[0].url,/:generateContent$/);
 const parts=JSON.parse(calls[0].init!.body as string).contents[0].parts;
 assert.deepEqual(parts.filter((p:{inlineData?:unknown})=>p.inlineData).map((p:{inlineData:{data:string}})=>Buffer.from(p.inlineData.data,'base64')),docs.map(d=>d.data));
});

test('explicit Gemini overload is retained and never automatically retried',async()=>{
 let calls=0;
 const session=new ProviderSession(async()=>{calls++;return Response.json({error:{code:503,status:'UNAVAILABLE',message:'High demand'}},{status:503})});
 const reply=await session.propose(price('gemini'),[question],[doc('one')],async()=>{});await session.close();
 assert.equal(reply.status,503);assert.equal(reply.error,'provider_unavailable');assert.match(reply.raw,/High demand/);assert.equal(calls,1);
});

test('file-reference uploads and cleanup run with bounded concurrency while preserving document order',async()=>{
 let active=0,peak=0,clean=0,peakClean=0;const docs=Array.from({length:7},(_,i)=>doc('doc'+i));
 const session=new ProviderSession(async(input,init)=>{
  const url=String(input);
  if(init?.method==='DELETE'){clean++;peakClean=Math.max(peakClean,clean);await new Promise(r=>setTimeout(r,5));clean--;return Response.json({deleted:true})}
  if(url.endsWith('/v1/files')){const name=((init?.body as FormData).get('file') as File).name;active++;peak=Math.max(peak,active);await new Promise(r=>setTimeout(r,5));active--;return Response.json({id:'file-'+name.replace('.pdf','')})}
  const body=JSON.parse(init?.body as string);assert.deepEqual(body.input[0].content.slice(1).map((c:{file_id:string})=>c.file_id),docs.map(d=>'file-source-'+d.id));
  return Response.json({status:'completed',output:[{type:'message',content:[{type:'output_text',text:'{"answers":[]}'}]}],usage:{input_tokens:10,output_tokens:2}});
 },undefined,false);
 const reply=await session.propose(price('openai'),[question],docs,async()=>{});await session.close();
 assert.equal(reply.error,'');assert.equal(peak,3);assert.equal(peakClean,3);assert.equal(active,0);assert.equal(clean,0);
});

test('a failed upload waits for other started uploads before cleanup',async()=>{
 const completed:string[]=[],deleted:string[]=[];
 const session=new ProviderSession(async(input,init)=>{
  if(init?.method==='DELETE'){deleted.push(String(input));return Response.json({deleted:true})}
  if(String(input).endsWith('/v1/files')){
   const name=((init?.body as FormData).get('file') as File).name;
   if(name.includes('bad'))return Response.json({error:'bad'},{status:400});
   await new Promise(r=>setTimeout(r,10));completed.push(name);return Response.json({id:'file-'+name.replace('.pdf','')});
  }
  throw Error('generation must not start after upload failure');
 },undefined,false);
 const reply=await session.propose(price('openai'),[question],[doc('one'),doc('bad'),doc('two'),doc('never')],async()=>{throw Error('must not generate')});await session.close();
 assert.equal(reply.error,'document_upload_failed');assert.equal(completed.length,2);assert.equal(deleted.length,2);
});

test('DeepSeek and Groq consume page-numbered local text, retain usage and flag unsupported citations',async()=>{
 for(const provider of ['deepseek','groq'] as const){
  let calls=0,loads=0;const original=doc('one');
  const session=new ProviderSession(async(input,init)=>{
   calls++;assert.match(String(input),provider==='deepseek'?/api.deepseek.com\/chat\/completions/:/api.groq.com\/openai\/v1\/chat\/completions/);
   const body=JSON.parse(init?.body as string),source=JSON.parse(body.messages[1].content);
   assert.equal(source.evidence.pages[0].page,1);assert.equal(source.evidence.pages[0].text,'Tender value INR 100');
   if(provider==='deepseek')assert.equal(body.thinking.type,'disabled');else assert.equal(body.response_format.json_schema.strict,true);
   return Response.json({choices:[{finish_reason:'stop',message:{content:JSON.stringify({answers:[{question_id:'q1',answer:'INR 100',status:'answered',citations:[{document_id:'one',page:1,quote:'Not present in source'}]}]})}}],usage:{prompt_tokens:100,completion_tokens:20,prompt_cache_hit_tokens:10}});
  },undefined,true,async()=>{loads++;return{reader_version:'pdf-text-v1',pages:[{page:1,text:'Tender value INR 100',method:'native'}],warnings:[]}});
  const reply=await session.propose(price(provider),[question],[original],async()=>{});await session.close();
  assert.equal(reply.error,'');assert.equal(calls,1);assert.equal(loads,1);assert.equal(reply.usage?.input_tokens,100);assert.equal(reply.usage?.cached_tokens,10);assert.equal(reply.evidence?.partial,false);
  assert.equal((reply.content as {answers:{status:string}[]}).answers[0].status,'needs_review');
 }
});
