import assert from "node:assert/strict";
import test from "node:test";
import {readFileSync} from "node:fs";
import ts from "typescript";
const {outputText}=ts.transpileModule(readFileSync(new URL('../src/components/purchasing/quotation-supplier-status.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}});
const {quotationSupplierSummary}=await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
test('supplier status names the actual missing documents without confusing another approved supplier',()=>{
 assert.deepEqual(quotationSupplierSummary({state:'blocked',message:'Required evidence missing: pan, cheque'}),{label:'Missing documents',detail:'PAN, cancelled cheque',approved:false});
 assert.deepEqual(quotationSupplierSummary({state:'verified',message:'Required evidence missing: pan'}),{label:'Approved',detail:'',approved:true});
});
test('unknown, pending and rejected statuses cannot silently qualify for submission',()=>{
 assert.equal(quotationSupplierSummary().approved,false);
 assert.equal(quotationSupplierSummary({state:'unverified',message:'Finance review in progress'}).label,'Review pending');
 assert.equal(quotationSupplierSummary({state:'rejected',message:'Rejected'}).label,'Rejected');
});
