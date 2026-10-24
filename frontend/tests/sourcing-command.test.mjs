import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/sourcing-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const {sourcingCommand}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=values=>{const f=new FormData();for(const[k,v]of Object.entries(values))f.set(k,v);return f};
test('received quote retains exact decimal strings, four-place unit rate and explicit noncompliance',()=>{
 const q=sourcingCommand('quote',7,form({reason:'Receive',unit_price:'12.3456',discount:'0.00',tax:'22.22',freight:'0.01',other:'0',delivery_days:'10',compliant:'no',quote_reference:'Q-1',evidence_reference:'document:2',valid_until:'2026-10-11',payment_terms:'Receipt',warranty:'24 months',compliance_notes:'Not compliant',approve:'yes',supplier_id:'forged'}),{id:'offer'});
 assert.equal(q.unit_price,'12.3456');assert.equal(q.tax,'22.22');assert.equal(q.freight,'0.01');assert.equal(q.compliant,false);assert.equal(q.delivery_days,10);assert.equal(q.offer_id,'offer');assert.equal(q.evidence_reference,'document:2');assert.equal(q.approve,undefined);assert.equal(q.supplier_id,undefined);
});
test('selection rejection remains explicit false and does not include captured source/client authority',()=>{
 assert.deepEqual(sourcingCommand('decide',9,form({reason:'Reject',approve:'no',source:'forged',approver_id:'forged',financial_approval:'forged'})),{action:'decide',version:9,reason:'Reject',approve:false});
});
test('LOI dispatch command includes only durable recorded dispatch values',()=>{
 assert.deepEqual(sourcingCommand('release',10,form({reason:'Release',sent_date:'2026-10-11',channel:'manual',dispatch_reference:'Delivery receipt',loi_reference:'forged',approve:'yes'})),{action:'release',version:10,reason:'Release',sent_date:'2026-10-11',channel:'manual',dispatch_reference:'Delivery receipt'});
});
test('direct total sends exact amount and never a derived or stale unit rate',()=>{
 const q=sourcingCommand('quote',8,form({pricing_mode:'line_total',line_total:'1.00',unit_price:'0.0001',discount:'0',tax:'0.18',freight:'0.01',other:'0',delivery_days:'1',compliant:'yes'}),{id:'offer'});
 assert.equal(q.pricing_mode,'line_total');assert.equal(q.line_total,'1.00');assert.equal(q.unit_price,undefined);assert.equal(q.tax,'0.18');
 const rate=sourcingCommand('quote',8,form({pricing_mode:'unit_price',line_total:'1.00',unit_price:'12.3456',compliant:'yes'}),{id:'offer'});
 assert.equal(rate.unit_price,'12.3456');assert.equal(rate.line_total,undefined);
});
