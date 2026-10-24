import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const {outputText}=ts.transpileModule(readFileSync(new URL("../src/components/purchasing/quotation-costs.ts",import.meta.url),"utf8"),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}});
const {lineEstimate,decimalMoney,chargeEstimate,editPrice,fixedPrice,quotationAmountFields}=await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
const entry={pricing_mode:"unit_price",unit_price:"10.10",line_total:"",discount_pct:"5",tax_pct:"18"};
test("live quote costs match decimal server rounding and ordered quantity",()=>{
 const one=lineEstimate(entry,"3"),two=lineEstimate(entry,"7");
 assert.equal(decimalMoney(one.base),"30.30");
 assert.equal(decimalMoney(one.discount),"1.52");
 assert.equal(decimalMoney(one.total+two.total+chargeEstimate("0.01")),"113.22");
});
test("direct line amount stays exact without derived rate round trip",()=>{
 const one=lineEstimate({...entry,pricing_mode:"line_total",line_total:"30.31"},"3");
 assert.equal(decimalMoney(one.base),"30.31");
 assert.equal(decimalMoney(one.total+lineEstimate(entry,"7").total+1n),"113.23");
});
test("zero prices are quoted; missing and invalid entries do not claim complete totals",()=>{
 assert.equal(lineEstimate({...entry,unit_price:"0"},"3").total,0n);
 for(const unit_price of ["","-1","Infinity","1.23456"]){assert.equal(lineEstimate({...entry,unit_price},"3"),null);}
 assert.equal(lineEstimate({...entry,tax_pct:"101"},"3"),null);
 assert.equal(chargeEstimate("1.001"),null);
 assert.equal(lineEstimate(entry,"0"),null);
});

test("either editable amount updates its counterpart and preserves lump sum basis",()=>{
 let value=editPrice(entry,"line_total","100.00","3");assert.equal(value.unit_price,"33.33");assert.equal(value.pricing_mode,"line_total");assert.equal(decimalMoney(lineEstimate(value,"3").base),"100.00");
 value=editPrice(value,"unit_price","33.33","3");assert.equal(value.line_total,"99.99");assert.equal(value.pricing_mode,"unit_price");
 assert.equal(fixedPrice("1.2"),"1.20");assert.equal(editPrice(value,"line_total","","3").unit_price,"");assert.equal(lineEstimate({...entry,unit_price:"1.234"},"3"),null);
});

test("overall quotation total replaces itemized charges in the request",()=>{
 assert.deepEqual(quotationAmountFields(true,"125000.50",null,null),{amount_mode:"total_only",total_amount:"125000.50",freight:"0",other_charges:"0"});
 assert.deepEqual(quotationAmountFields(false,null,"1200","300"),{amount_mode:"itemized",total_amount:null,freight:"1200",other_charges:"300"});
});
