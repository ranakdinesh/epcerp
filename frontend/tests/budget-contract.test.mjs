import assert from "node:assert/strict";
import test from "node:test";
import {readFileSync} from "node:fs";
import ts from "typescript";
const {outputText}=ts.transpileModule(readFileSync(new URL("../src/components/projects/budget-contract.ts",import.meta.url),"utf8"),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}});
const {budgetInput}=await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
test("budget correction preserves identity and exact decimals without sending audit/source authority",()=>{
 const source={total:"99999999999999.99",currency:"INR",owner_id:"owner",notes:"reason",pricing_mode:"unit_rates",created_by_id:"server",heads:[{id:"head",parent_id:null,name:"Purchase",kind:"purchase",amount:"10.01",owner_id:"owner",is_active:false,created_at:"server"}],allocations:[{id:"allocation",bo_id:"bo",line_id:"line",head_id:"head",amount:"10.01",owner_id:"owner",is_active:false,unit_price:"1.00",pricing_quantity:"10.0050",source:{sku:"retained"},updated_by_id:"server"}]};
 const input=budgetInput(source);assert.equal(input.total,source.total);assert.equal(input.allocations[0].pricing_quantity,"10.0050");assert.equal(input.allocations[0].id,"allocation");assert.equal(input.heads[0].is_active,false);
 assert.equal(JSON.stringify(input).includes("server"),false);assert.equal(Object.hasOwn(input.allocations[0],"source"),false);
 input.heads[0].name="Correction";assert.equal(source.heads[0].name,"Purchase");
});
