import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/accounts-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;
const {accountsCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=o=>{const f=new FormData();for(const[k,v]of Object.entries(o))f.set(k,String(v));return f;};
test('Accounts acceptance keeps exact booked amount and excludes caller currency/source/actor',()=>{
const fields={status:'accepted',acceptance_date:'2026-10-11',officer_name:'Actual officer',voucher_ref:'Actual voucher',booked_amount:'999999999999.99',notes:'Actual Finance review'};
assert.deepEqual(accountsCommand(1,'record',form({...fields,reason:'Finance accepts',currency:'USD',engineer_id:'forged',actor_id:'forged'})),{version:1,action:'record',reason:'Finance accepts',acceptance:fields});
});
test('pending and zero booking retain optional references; invalid decimal formats rejected',()=>{
const q=accountsCommand(0,'record',form({status:'pending',reason:'Prepare'}));assert.deepEqual(q.acceptance,{status:'pending',acceptance_date:'',officer_name:'',voucher_ref:'',booked_amount:'0',notes:''});
for(const amount of ['-1','1.001','1e3','1000000000000','NaN','+1','01'])assert.throws(()=>accountsCommand(0,'record',form({status:'pending',reason:'Prepare',booked_amount:amount})));
});
test('Accounts correction carries expected version and reason only',()=>{assert.deepEqual(accountsCommand(2,'revise',form({reason:'Correct Finance evidence',status:'accepted',booked_amount:'999'})),{version:2,action:'revise',reason:'Correct Finance evidence'});});
