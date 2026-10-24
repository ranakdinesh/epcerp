import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import ts from "typescript";
const {outputText}=ts.transpileModule(readFileSync(new URL("../src/components/projects/purchase-budget-calculations.ts",import.meta.url),"utf8"),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}});
const {decimalUnits,unitBudget,moneyText}=await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
test("fractional quantity rounds half up in cents, zero remains distinct from blank",()=>{assert.equal(unitBudget('2.5000','10.01'),2503n);assert.equal(unitBudget('3','0'),0n);assert.equal(unitBudget('3',''),null);assert.equal(unitBudget('0','10'),null);assert.equal(unitBudget('3','1.001'),null);});
test("large unit totals and running remainder preserve every cent",()=>{const main=decimalUnits('99999999999999.99',2);const total=unitBudget('3.0000','33333333333333.33');assert.equal(main-total,0n);assert.equal(moneyText(main-total-1n,'INR'),'−INR 0.01');assert.equal(moneyText(2503n,'INR'),'INR 25.03');});
