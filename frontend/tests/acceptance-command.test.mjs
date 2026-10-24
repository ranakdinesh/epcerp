import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/acceptance-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {acceptanceCommand,indiaDate}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('vendor response carries declared response fields and saved version without forged PO or actor',()=>{
 const q=acceptanceCommand(2,form({status:'accepted_with_exceptions',accepted_date:'2026-10-11',signatory_name:'Actual vendor',document_reference:'Email reference',remarks:'Actual exception',reason:'Correct response',supplier_id:'forged',order_id:'forged',total:'0',actor_id:'forged',approved:'yes'}));
 assert.deepEqual(q,{version:2,action:'record',reason:'Correct response',response:{status:'accepted_with_exceptions',accepted_date:'2026-10-11',signatory_name:'Actual vendor',document_reference:'Email reference',remarks:'Actual exception'}});
});
test('pending clears old acknowledged date and leaves optional reference/signatory empty',()=>{
 const q=acceptanceCommand(0,form({status:'pending',accepted_date:'2026-10-11',reason:'Awaiting response'}));
 assert.equal(q.response.accepted_date,'');assert.equal(q.response.signatory_name,'');assert.equal(q.response.document_reference,'');
});
test('date picker uses the India calendar across UTC midnight',()=>{
 assert.equal(indiaDate(new Date('2026-10-10T20:00:00Z')),'2026-10-11');
 assert.equal(indiaDate(new Date('2026-10-10T18:29:00Z')),'2026-10-10');
});
