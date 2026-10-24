import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/dispatch-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {dispatchCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('dispatch retains all Python logistics fields and excludes caller source/actors/quantity',()=>{
 const fields={status:'dispatched',dispatch_date:'2026-10-11',transporter_name:'Actual carrier',lr_number:'LR-1',lr_date:'2026-10-11',vehicle_number:'Vehicle reference',eway_bill_number:'Eway reference',eway_bill_valid_until:'2027-01-01',invoice_number:'Invoice reference',invoice_date:'2026-10-11',driver_contact:'Contact reference',file_name:'Logistics reference'};
 assert.deepEqual(dispatchCommand(1,'record',form({...fields,reason:'Actual dispatch',clearance_id:'forged',actor_id:'forged',cleared_quantity:'999'})),{version:1,action:'record',reason:'Actual dispatch',dispatch:fields});
});
test('pending dispatch invents no logistics or optional document evidence',()=>{
 const q=dispatchCommand(0,'record',form({reason:'Prepare',status:'pending'}));
 assert.equal(q.dispatch.status,'pending');for(const[k,v]of Object.entries(q.dispatch)){if(k!=='status')assert.equal(v,'')}
 assert.equal(Object.keys(q.dispatch).length,12);
});
test('dispatch revision sends only current version and reason',()=>{
 assert.deepEqual(dispatchCommand(2,'revise',form({reason:'Actual correction',status:'dispatched',lr_number:'Old LR'})),{version:2,action:'revise',reason:'Actual correction'});
});
