import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/inspection-report-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {reportCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('report command preserves exact cleared quantity and sends only declared fields',()=>{
 const q=reportCommand(4,'record',form({status:'issued',ir_number:'IR actual',irn_number:'IRN actual',issue_date:'2026-10-11',issuing_authority:'Actual inspector',cleared_quantity:'1.000001',file_name:'Signed reference',reason:'Actual issuance',uom:'forged',inspection_id:'forged',actor_id:'forged'}));
 assert.deepEqual(q,{version:4,action:'record',reason:'Actual issuance',report:{status:'issued',ir_number:'IR actual',irn_number:'IRN actual',issue_date:'2026-10-11',issuing_authority:'Actual inspector',cleared_quantity:'1.000001',file_name:'Signed reference'}});
});
test('report revision sends only saved version and correction reason',()=>{
 assert.deepEqual(reportCommand(5,'revise',form({reason:'Actual correction',status:'issued',irn_number:'Old IRN',cleared_quantity:'10'})),{version:5,action:'revise',reason:'Actual correction'});
});
test('pending report leaves optional issuance fields empty without inventing identity or files',()=>{
 const q=reportCommand(0,'record',form({status:'pending',cleared_quantity:'0',reason:'Prepare report'}));
 assert.equal(q.report.ir_number,'');assert.equal(q.report.irn_number,'');assert.equal(q.report.issue_date,'');assert.equal(q.report.file_name,'');assert.equal(q.report.cleared_quantity,'0');
});
