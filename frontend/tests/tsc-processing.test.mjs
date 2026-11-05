import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const load=async path=>{const code=ts.transpileModule(readFileSync(new URL(path,import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;return import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}`)};
const {generationLabel,processingTitle,questionProgress}=await load('../src/components/tenders/tsc-processing-model.ts');
const {processingStatus}=await load('../src/server/tenders/tsc/progress.ts');
const now=Date.parse('2026-11-04T16:11:14Z'),created='2026-11-04T16:08:21Z';
const q={id:'q1',manual_review:false},snapshot={questions:[q,{id:'manual',manual_review:true}],sources:[{id:'pdf'}]},blank={question_id:'q1',answer:'',status:'needs_review',origin:'manual',accepted:false,citations:[]};
test('failed ready drafts are displayed as failed, with elapsed time and unfilled question status',()=>{
 const job={state:'ready',error_code:'provider_unavailable',mode:'gemini',created_at:created};
 const p=processingStatus(job,snapshot,[blank],{state:'failed',created_at:created,updated_at:'2026-11-04T16:11:13Z'},{state:'failed',http_status:503},now);
 assert.equal(p.phase,'failed');assert.equal(p.elapsed_seconds,172);assert.equal(p.returned_answers,0);assert.equal(p.provider_status,503);assert.equal(generationLabel(job),'Generation failed');assert.match(processingTitle(p,'Gemini'),/no answers received/);assert.equal(questionProgress(q,blank,false,true,'Gemini'),'No answer received');
});
test('progress distinguishes preparing, waiting, saving and completed answers from manual review',()=>{
 const job={state:'processing',error_code:'',mode:'gemini',created_at:created};
 for(const[call,phase]of [[{state:'reserved',http_status:null},'preparing_documents'],[{state:'sent',http_status:null},'waiting_for_model'],[{state:'completed',http_status:200},'saving_answers']])assert.equal(processingStatus(job,snapshot,[blank],undefined,call,now).phase,phase);
 assert.equal(questionProgress(q,blank,true,false,'Gemini'),'Waiting for Gemini');
 assert.equal(questionProgress({...q,manual_review:true},blank,true,false,'Gemini'),'Manager judgement required');
 assert.equal(questionProgress(q,{...blank,answer:'INR 100',status:'answered',origin:'gemini'},false,false,'Gemini'),'Answer ready to review');
});
test('a failed request with retained answers is labelled as a partial draft',()=>{
 assert.equal(generationLabel({state:'ready',mode:'gemini',error_code:'provider_unavailable',processing:{phase:'partial'}}),'Partial draft — answers saved');
});
