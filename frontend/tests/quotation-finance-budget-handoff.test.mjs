import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {createElement} from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import ts from 'typescript';
const data=source=>`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const stub=data(`import {createElement as h} from ${JSON.stringify(import.meta.resolve('react'))};
export const useProjectUser=()=>({id:'finance-reviewer'});export const useAuthenticatedUser=()=>({permissions:[]});
export const DataGrid=({rows,columns,footer})=>h('table',null,h('thead',null,h('tr',null,...columns.map(c=>h('th',{key:c.key},c.label,c.headerContent)))),h('tbody',null,...rows.map(r=>h('tr',{key:r.id},...columns.map(c=>h('td',{key:c.key},c.render?c.render(r):c.value(r)))))),footer?h('tfoot',null,footer):null);
export const ValidatedForm=({children,...props})=>h('form',props,children);export const ValidatedLabel=({children})=>h('label',null,children);export const ValidatedInput=props=>h('input',props);export const ValidatedTextarea=props=>h('textarea',props);export const ValidatedSelect=props=>h('select',props);export const ValidatedRadioGroup=({label,options})=>h('fieldset',null,h('legend',null,label),...options.map(o=>h('span',{key:o.value},o.label)));
export const RecordDrawer=()=>null;export const SupplierEvidence=()=>null;export const SelectedQuoteBudget=()=>null;export const ComparisonPrint=()=>null;export const quoteKey=()=>'';export const quotationPremium=()=>'';`);
const compile=s=>ts.transpileModule(s,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022}}).outputText;
const calculation=data(compile(readFileSync(new URL('../src/components/projects/purchase-budget-calculations.ts',import.meta.url),'utf8')));
const comparison=data(compile(readFileSync(new URL('../src/components/purchasing/quotation-finance-comparison.ts',import.meta.url),'utf8').replace('@/components/projects/purchase-budget-calculations',calculation)));
let source=ts.transpileModule(readFileSync(new URL('../src/components/purchasing/quotation-approval-controls.tsx',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022,jsx:ts.JsxEmit.ReactJSX}}).outputText;
source=source.replace('"./quotation-finance-comparison"',JSON.stringify(comparison)).replaceAll('"react/jsx-runtime"',JSON.stringify(import.meta.resolve('react/jsx-runtime'))).replaceAll('"react"',JSON.stringify(import.meta.resolve('react'))).replace(/from "(?:@[^\"]*|\.[^\"]*)"/g,`from ${JSON.stringify(stub)}`);
const {AccountsReview}=await import(data(source));
const attempt={id:'request',group_name:'Cables',quote_key:'quote:1',group_id:'group',vendor:'Vendor',reference:'Q-1',revision:1,total:'145.69',effective_state:'pending_accounts',submitted_by_id:'purchaser',message:'Awaiting Accounts item approval',reason:'Please review the items',budget_revision:0,budget_issue:'Approve the project item budget before approving this vendor.',lines:[{line_id:'line',item:{description:'Cable'},quoted:'145.69',budget:null,difference:null}]};
const render=(a,extra={})=>renderToStaticMarkup(createElement(AccountsReview,{attempt:a,workspace:{can_review:true,version:1},busy:false,onReview:async()=>true,money:v=>v??'Not allocated',...extra}));
test('Finance receives the submitted items and owns the pending budget warning and rejection option',()=>{
 const html=render(attempt);assert.match(html,/Approve the project item budget/);assert.match(html,/Please review the items/);assert.match(html,/Cable/);assert.match(html,/145.69/);assert.match(html,/Reject group/);assert.match(html,/Not allocated/);
});
test('Finance sees resolved current allocations on the same pending request',()=>{
 const html=render({...attempt,budget_revision:2,budget_issue:'',lines:[{...attempt.lines[0],budget:'180.00',difference:'-34.31'}]});
 assert.doesNotMatch(html,/Approve the project item budget/);assert.match(html,/180.00/);assert.match(html,/-34.31/);assert.match(html,/Approve selected vendor/);
});

test('comparison comes before the vendor choice and decision notes, with a fixed external action',()=>{
 const html=render(attempt,{formId:'finance-decision-request',externalSubmit:true});
 assert.match(html,/<form id="finance-decision-request"/);assert.ok(html.indexOf('<table')<html.indexOf('type="checkbox"'));assert.ok(html.indexOf('Group total')<html.indexOf('type="checkbox"'));assert.ok(html.indexOf('Decision notes')>html.indexOf('</table>'));assert.doesNotMatch(html,/Submitted supplier offers/);assert.doesNotMatch(html,/Proposal for decision/);assert.doesNotMatch(html,/>Record Finance decision</);assert.match(html,/Specifications/);
});

test('the Finance total row contains budget, both offer totals and all three signed variances',()=>{
 const low={...attempt,total:'145.69',budget_revision:1,budget_issue:'',lines:[{...attempt.lines[0],budget:'180.00'}]};
 const preferred={...low,total:'160.00',lines:[{...low.lines[0],quoted:'160.00'}]};
 const html=render({...low,proposals:[{quote_key:attempt.quote_key,best_price:true,preferred:false,snapshot:low},{quote_key:'other:1',best_price:false,preferred:true,snapshot:preferred}]});
 const footer=html.match(/<tfoot><tr>[\s\S]*?<\/tr>/)?.[0]??'';
 assert.match(footer,/Group total/);for(const value of ['180.00','145.69','160.00','-34.31','-20.00','+14.31'])assert.ok(footer.includes(value),`missing total ${value}`);
 assert.equal((footer.match(/<td/g)??[]).length,7);assert.ok(footer.indexOf('145.69')<footer.indexOf('160.00'));assert.ok(footer.indexOf('160.00')<footer.indexOf('-34.31'));
});


test('one mutually exclusive vendor choice per proposal, with no default winner',()=>{
 const low={...attempt,vendor:'Lowest supplier'},preferred={...attempt,vendor:'Preferred supplier'};
 const html=render({...attempt,proposals:[{quote_key:'low:1',best_price:true,preferred:false,snapshot:low},{quote_key:'preferred:1',best_price:false,preferred:true,snapshot:preferred}]});
 assert.equal((html.match(/type="checkbox"/g)??[]).length,2);
 assert.equal((html.match(/name="vendor_request"/g)??[]).length,2);
 assert.doesNotMatch(html,/checked=""/);assert.doesNotMatch(html,/<select/);
 assert.match(html,/Select Lowest supplier for the whole group/);assert.match(html,/Select Preferred supplier for the whole group/);
 assert.match(html,/Lowest price/);assert.match(html,/Preferred price/);assert.match(html,/No vendor selected/);
});
test('a vendor nominated as both lowest and preferred is still one choice',()=>{
 const html=render({...attempt,proposals:[{quote_key:attempt.quote_key,best_price:true,preferred:true,snapshot:attempt}]});
 assert.equal((html.match(/type="checkbox"/g)??[]).length,1);assert.match(html,/Lowest &amp; Preferred/);assert.match(html,/Lowest price/);assert.match(html,/Preferred/);
});
test('completed or unauthorized reviews offer no vendor selection or decision editor',()=>{
 for(const html of [render({...attempt,effective_state:'approved'}),render(attempt,{workspace:{can_review:false,version:1}}),render({...attempt,submitted_by_id:'finance-reviewer'})]){
 assert.doesNotMatch(html,/type="checkbox"/);assert.doesNotMatch(html,/Decision notes/);assert.doesNotMatch(html,/Select vendor/);
 }
});

test('completed approval identifies the single saved winning vendor',()=>{
 const html=render({...attempt,effective_state:'approved',proposals:[{quote_key:attempt.quote_key,best_price:true,preferred:false,snapshot:attempt},{quote_key:'other:1',best_price:false,preferred:true,snapshot:{...attempt,vendor:'Other supplier'}}]});
 assert.equal((html.match(/Approved vendor for this group/g)??[]).length,1);assert.doesNotMatch(html,/type="checkbox"/);
});

test('missing budgets expose a budget action and explain the approval prerequisite',()=>{
 const html=render(attempt,{onResolveBudget:()=>{}});
 const action=html.match(/<button[^>]*>View project budget<\/button>/)?.[0]??'';
 assert.ok(action);assert.match(action,/type="button"/);assert.doesNotMatch(action,/\sdisabled(?:=|[ >])/);
 assert.match(html,/Approved budget required/);assert.doesNotMatch(html,/This does not block your Finance decision/);
 const resolved=render({...attempt,budget_revision:1,budget_issue:'',lines:[{...attempt.lines[0],budget:'180.00'}]},{onResolveBudget:()=>{}});
 assert.doesNotMatch(resolved,/>View project budget</);
 const saving=render(attempt,{onResolveBudget:()=>{},busy:true});
 assert.match(saving,/<button[^>]*\sdisabled(?:="")?[^>]*>View project budget<\/button>/);
});


test('completed comparison uses the audited reviewed budget while submitted offers remain frozen',()=>{
 const frozen={...attempt,budget_revision:0,lines:[{...attempt.lines[0],budget:null}]};
 const approved={...attempt,effective_state:'approved',budget_revision:2,budget_issue:'',lines:[{...attempt.lines[0],budget:'180.00'}],proposals:[{quote_key:attempt.quote_key,best_price:true,preferred:true,snapshot:frozen}]};
 const html=render(approved);const footer=html.match(/<tfoot>[\s\S]*?<\/tfoot>/)?.[0]??'';
 assert.match(footer,/180.00/);assert.doesNotMatch(footer,/Item budgets not set/);assert.doesNotMatch(html,/type="checkbox"/);
});


test('item annotations are optional while the overall Finance reason stays required',()=>{
 const html=render(attempt,{onResolveBudget:()=>{}});
 assert.match(html,/Item notes \(optional\)/);assert.doesNotMatch(html,/Finance item notes \*/);
 const itemInputs=html.match(/<textarea[^>]*aria-label="Finance notes item [^"]*"[^>]*>/g)??[];
 assert.ok(itemInputs.length);for(const input of itemInputs)assert.doesNotMatch(input,/\srequired(?:=|[ >])/);
 assert.match(html,/<textarea[^>]*name="reason"[^>]*required/);
 const budgetAction=html.match(/<button[^>]*>View project budget<\/button>/)?.[0]??'';assert.match(budgetAction,/type="button"/);
});


test('an approved project budget does not require individual item allocations',()=>{
 const html=render({...attempt,budget_revision:3,budget_issue:'',lines:[{...attempt.lines[0],budget:null}]},{onResolveBudget:()=>{}});
 assert.doesNotMatch(html,/Approved budget required/);assert.doesNotMatch(html,/>View project budget</);
 assert.match(html,/Item budgets not set/);assert.match(html,/Not available/);assert.match(html,/Approve selected vendor/);
});
