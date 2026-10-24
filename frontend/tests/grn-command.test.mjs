import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const source=readFileSync(new URL('../src/components/purchase-fms/grn-model.ts',import.meta.url),'utf8');
const compiled=ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext}}).outputText;
const {grnCommand,grnQuantityErrors}=await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);
const form=o=>{const f=new FormData();for(const[k,v]of Object.entries(o))f.set(k,String(v));return f;};
test('GRN retains exact quantities and all warranty metadata without caller unit/source/actors',()=>{
const fields={status:'issued',grn_number:'GRN-1',grn_date:'2026-10-11',received_qty:'1.000001',accepted_qty:'1',rejected_qty:'0.000001',store_keeper_name:'Actual keeper',remarks:'Actual inwarding',warranty_ref:'Actual warranty',warranty_valid_until:'2000-01-01',file_name:'Actual reference'};
assert.deepEqual(grnCommand(1,'record',form({...fields,reason:'Inwarding',receipt_id:'forged',uom:'forged',actor_id:'forged'})),{version:1,action:'record',reason:'Inwarding',grn:fields});
});
test('pending quantities normalize blank to zero without inventing warranty evidence',()=>{
const q=grnCommand(0,'record',form({status:'pending',reason:'Prepare'}));assert.equal(q.grn.received_qty,'0');assert.equal(q.grn.accepted_qty,'0');assert.equal(q.grn.rejected_qty,'0');assert.equal(q.grn.warranty_ref,'');assert.equal(q.grn.warranty_valid_until,'');
assert.deepEqual(grnQuantityErrors('pending','0','1','2','3'),{});
});
test('GRN limits and aggregate checks remain exact at six decimal places and large quantities',()=>{
assert.deepEqual(grnQuantityErrors('issued','1.000001','1','0.000001','1.000001'),{});
assert.ok(grnQuantityErrors('issued','1.000001','1.000001','0.000001','1.000001').accepted_qty);
assert.ok(grnQuantityErrors('issued','1.000002','0','0','1.000001').received_qty);
assert.ok(grnQuantityErrors('issued','999999999999.999999','999999999999.999999','0.000001','999999999999.999999').accepted_qty);
assert.deepEqual(grnQuantityErrors('issued','1','0','0','1.000001'),{});
for(const value of ['-1','1e-6','1.0000001','1000000000000'])assert.ok(grnQuantityErrors('pending',value,'0','0','1').received_qty);
assert.ok(grnQuantityErrors('issued','0','0','0','1').received_qty);
});
test('GRN revision carries only expected version and reason',()=>{
assert.deepEqual(grnCommand(2,'revise',form({reason:'Review corrected receipt',status:'issued',received_qty:'1'})),{version:2,action:'revise',reason:'Review corrected receipt'});
});
