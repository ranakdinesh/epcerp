import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/purchase-order-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {purchaseOrderCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('release carries only saved version, action and reason, never browser approvals or amounts',()=>{
 assert.deepEqual(purchaseOrderCommand('release',3,form({reason:'Commit approved kit',total:'999',number:'forged',actor_id:'forged',management_approved:'yes',payment_terms:'override',date:'2026-10-11'})),{action:'release',version:3,reason:'Commit approved kit'});
});
test('preparation carries actual issuer and consignee fields without replacing source commercial terms',()=>{
 const q=purchaseOrderCommand('prepare',0,form({reason:'Prepare',date:'2026-10-11',issuer_name:'Actual issuer',issuer_address:'Actual office',bill_to:'Actual invoice address',delivery_address:'Actual site',unit_price:'999',supplier_id:'forged',financial_reference:'forged',source_snapshot:'forged'}));
 assert.equal(q.details.issuer_name,'Actual issuer');assert.equal(q.details.delivery_address,'Actual site');assert.equal(q.details.issuer_tax_reference,'');assert.equal(q.details.issuer_email,'');assert.equal(q.details.unit_price,undefined);assert.equal(q.source_snapshot,undefined);
});
test('client submission binds only reference and date to the saved draft',()=>{
 assert.deepEqual(purchaseOrderCommand('submit_client',4,form({reason:'Sent draft',reference:'SUB-PO-1',date:'2026-10-19',status:'approved',total:'999',file_id:'forged'})),{action:'submit_client',version:4,reason:'Sent draft',submission:{reference:'SUB-PO-1',date:'2026-10-19'}});
});
test('client response keeps external evidence separate from internal Finance approval',()=>{
 const q=purchaseOrderCommand('client_decision',5,form({reason:'Actual client response',status:'approved',reference:'CLIENT-PO-1',date:'2026-10-19',signatory:'Client representative',designation:'Engineer',file_id:'owned-signed-file',actor_id:'forged',source_snapshot:'forged'}));
 assert.deepEqual(q,{action:'client_decision',version:5,reason:'Actual client response',client_decision:{status:'approved',reference:'CLIENT-PO-1',date:'2026-10-19',signatory:'Client representative',designation:'Engineer',file_id:'owned-signed-file'}});
});
