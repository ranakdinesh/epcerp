import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/receipt-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;
const {receiptCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=o=>{const f=new FormData();for(const[k,v]of Object.entries(o))f.set(k,String(v));return f;};
test('receipt keeps physical package evidence separate from material acceptance and source',()=>{
const fields={status:'delivered_at_site',arrival_date:'2026-10-11',receiving_officer:'Actual officer',packages_received:0,packages_condition:'quarantine',transit_damage_notes:'Actual damage',gate_entry_ref:'Actual gate entry'};
assert.deepEqual(receiptCommand(1,'record',form({...fields,reason:'Arrival',dispatch_id:'forged',accepted_qty:'999',actor_id:'forged'})),{version:1,action:'record',reason:'Arrival',receipt:fields});
});
test('package count is a bounded integer and draft uses unchecked condition',()=>{
for(const count of ['-1','1.5','1e2','2147483648',''])assert.throws(()=>receiptCommand(0,'record',form({status:'pending',reason:'r',packages_received:count})));
const q=receiptCommand(0,'record',form({status:'in_transit',reason:'r',packages_received:0}));
assert.equal(q.receipt.packages_condition,'not_checked');assert.equal(q.receipt.arrival_date,'');assert.equal(q.receipt.receiving_officer,'');
});
test('receipt correction sends only reason and expected version',()=>{
assert.deepEqual(receiptCommand(2,'revise',form({reason:'Review corrected dispatch',status:'delivered_at_site',packages_received:20})),{version:2,action:'revise',reason:'Review corrected dispatch'});
});
