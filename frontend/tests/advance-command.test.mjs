import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const compiled=ts.transpileModule(readFileSync(new URL('../src/components/purchase-fms/advance-model.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {advanceCommand,advancePreview,advanceSubmissionMissing}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f;};
test('request commands preserve exact percentage without source, amount or approval flags',()=>{
 const q=advanceCommand(4,'prepare',form({kind:'payment',request_no:'ADV1',request_date:'2026-10-11',advance_pct:'30',proforma_invoice_ref:'Actual PI',proforma_invoice_date:'2026-10-09',bank_account_ref:'Actual bank reference',notes:'Agreed terms',reason:'Correction',amount:'999',order_id:'forged',decision_id:'forged',actor_id:'forged'}));
 assert.deepEqual(q,{version:4,action:'prepare',reason:'Correction',details:{kind:'payment',request_no:'ADV1',request_date:'2026-10-11',advance_pct:'30',proforma_invoice_ref:'Actual PI',proforma_invoice_date:'2026-10-09',bank_account_ref:'Actual bank reference',notes:'Agreed terms'}});
});
test('no advance clears old payment instructions and sends exact zero',()=>{
 const q=advanceCommand(0,'prepare',form({kind:'not_applicable',advance_pct:'30',proforma_invoice_ref:'old',proforma_invoice_date:'2026-10-09',bank_account_ref:'old',notes:'No advance agreed',reason:'Disposition'}));
 assert.equal(q.details.advance_pct,'0');assert.equal(q.details.proforma_invoice_ref,'');assert.equal(q.details.proforma_invoice_date,'');assert.equal(q.details.bank_account_ref,'');
});
test('review, revision and submission cannot change saved amount or requester',()=>{
 for(const a of ['approve','reject','revise','submit'])assert.deepEqual(advanceCommand(2,a,form({reason:'Independent review',amount:'999',kind:'payment',request_no:'forged',decision_id:'forged'})),{version:2,action:a,reason:'Independent review'});
});
test('payment carries manual evidence and cannot change approved amount',()=>{
 assert.deepEqual(advanceCommand(7,'disburse',form({voucher_ref:'Actual UTR',disbursed_date:'2026-10-11',notes:'Actual payment',reason:'Recorded evidence',amount:'999',status:'approved'})),{version:7,action:'disburse',reason:'Recorded evidence',payment:{voucher_ref:'Actual UTR',disbursed_date:'2026-10-11',notes:'Actual payment'}});
});
test('preview uses exact half-up arithmetic and rejects invalid percentages',()=>{
 assert.equal(advancePreview('145.69','30'),'43.71');assert.equal(advancePreview('0.01','50'),'0.01');assert.equal(advancePreview('99999999999999.99','100'),'99999999999999.99');assert.equal(advancePreview('145.69','0'),'0.00');
 for(const p of ['','101','100.01','1e2','-1','0.001'])assert.equal(advancePreview('145.69',p),'Pending valid percentage');
});
test('submission readiness lists missing fields, including no-advance reason',()=>{
 const d={kind:'payment',request_no:'',request_date:'',advance_pct:'',proforma_invoice_ref:'',proforma_invoice_date:'',bank_account_ref:'',notes:''};
 assert.equal(advanceSubmissionMissing({details:d,amount:''}).length,6);
 assert.deepEqual(advanceSubmissionMissing({details:{...d,kind:'not_applicable',request_no:'ADV1',request_date:'2026-10-11',advance_pct:'0'},amount:'0.00'}),['No-advance reason']);
});
