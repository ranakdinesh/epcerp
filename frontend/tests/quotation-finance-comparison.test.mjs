import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import ts from 'typescript';
const compile=s=>ts.transpileModule(s,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const data=s=>`data:text/javascript;base64,${Buffer.from(s).toString('base64')}`;
const dependency=data(compile(readFileSync(new URL('../src/components/projects/purchase-budget-calculations.ts',import.meta.url),'utf8')));
const {financeComparison}=await import(data(compile(readFileSync(new URL('../src/components/purchasing/quotation-finance-comparison.ts',import.meta.url),'utf8').replace('@/components/projects/purchase-budget-calculations',dependency))));
const lines=[{line_id:'a',budget:'100.00'},{line_id:'b',budget:'200.00'}];
const offers=[{best_price:true,preferred:false,snapshot:{total:'310.00',lines:[{line_id:'b',quoted:'220.00'},{line_id:'a',quoted:'90.00'}]}},{best_price:false,preferred:true,snapshot:{total:'320.00',lines:[{line_id:'a',quoted:'110.00'},{line_id:'b',quoted:'210.00'}]}}];
test('both item variances and group totals match line identities rather than array order',()=>{
 const c=financeComparison(lines,offers);
 assert.deepEqual(c.rows.map(r=>[r.lowestVariance,r.preferredVariance,r.premium]),[['-10.00','10.00','20.00'],['20.00','10.00','-10.00']]);
 assert.deepEqual(c.totals,{budgetAmount:'300.00',lowestAmount:'310.00',preferredAmount:'320.00',lowestVariance:'10.00',preferredVariance:'20.00',premium:'10.00'});assert.deepEqual(c.issues,[]);
});
test('one offer with both nominations has identical columns and zero item and total premium',()=>{
 const c=financeComparison(lines,[{...offers[0],preferred:true}]);
 assert.equal(c.totals.premium,'0.00');assert.deepEqual(c.rows.map(r=>r.premium),['0.00','0.00']);assert.equal(c.totals.lowestVariance,c.totals.preferredVariance);
});
test('missing budgets do not become zero or misleading partial totals',()=>{
 const c=financeComparison([{line_id:'a',budget:'100.00'},{line_id:'b',budget:null}],offers);
 assert.equal(c.rows[0].lowestVariance,'-10.00');assert.equal(c.rows[1].lowestVariance,null);assert.equal(c.totals.budgetAmount,null);assert.equal(c.totals.lowestVariance,null);assert.equal(c.totals.preferredVariance,null);assert.equal(c.totals.premium,'10.00');
});
test('missing quotes remain unavailable and source total mismatches are disclosed',()=>{
 const c=financeComparison(lines,[{...offers[0],snapshot:{total:'310.00',lines:[{line_id:'a',quoted:'90.00'}]}},offers[1]]);
 assert.equal(c.rows[1].lowestAmount,null);assert.equal(c.rows[1].premium,null);assert.equal(c.totals.lowestVariance,null);
 const mismatched=financeComparison(lines,[{...offers[0],snapshot:{...offers[0].snapshot,total:'300.00'}},offers[1]]);
 assert.equal(mismatched.totals.lowestAmount,'300.00');assert.match(mismatched.issues[0],/differs from its item amounts/);
});
test('financial comparison retains exact cents above floating-point precision',()=>{
 const c=financeComparison([{line_id:'a',budget:'99999999999999999999.98'}],[{best_price:true,preferred:false,snapshot:{total:'99999999999999999999.99',lines:[{line_id:'a',quoted:'99999999999999999999.99'}]}},{best_price:false,preferred:true,snapshot:{total:'100000000000000000000.01',lines:[{line_id:'a',quoted:'100000000000000000000.01'}]}}]);
 assert.equal(c.totals.lowestVariance,'0.01');assert.equal(c.totals.preferredVariance,'0.03');assert.equal(c.totals.premium,'0.02');
});
