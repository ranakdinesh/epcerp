import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const compile=s=>ts.transpileModule(s,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const dependency=`data:text/javascript;base64,${Buffer.from(compile(readFileSync(new URL('../src/components/projects/purchase-budget-calculations.ts',import.meta.url),'utf8'))).toString('base64')}`;
const source=readFileSync(new URL('../src/components/purchasing/quotation-money.ts',import.meta.url),'utf8').replace('@/components/projects/purchase-budget-calculations',dependency);
const {quotationMoney,positiveDifference,quotationPremium}=await import(`data:text/javascript;base64,${Buffer.from(compile(source)).toString('base64')}`);
test('quotation displays retain cents above floating point precision and signed differences',()=>{
 assert.equal(quotationMoney('99999999999999999999.98','INR'),'INR 9,99,99,99,99,99,99,99,99,999.98');
 assert.equal(quotationMoney('-54.31','INR'),'−INR 54.31');
 assert.equal(quotationMoney('0.01','INR'),'INR 0.01');
 assert.equal(quotationMoney('12.3456','INR'),'INR 12.3456');
 assert.equal(quotationMoney('0.0001','INR'),'INR 0.0001');
 assert.equal(positiveDifference('-0.01'),false);assert.equal(positiveDifference('0.00'),false);assert.equal(positiveDifference('0.01'),true);
 assert.equal(quotationMoney(null,'INR'),'Not set');assert.equal(quotationMoney('NaN','INR'),'Invalid amount');
});

test('preferred offer premium uses exact cents and handles dual selections and zero totals',()=>{
 assert.equal(quotationPremium('248791.98','247779.28','INR'),'INR 1,012.70 (0.41%) above lowest');
 assert.equal(quotationPremium('100.00','100.00','INR'),'');
 assert.equal(quotationPremium('10.00','0.00','INR'),'INR 10.00 above lowest');
 assert.equal(quotationPremium('100.00',undefined,'INR'),'');
 assert.equal(quotationPremium('99999999999999999999.99','99999999999999999999.98','INR'),'INR 0.01 (0.00%) above lowest');
});
