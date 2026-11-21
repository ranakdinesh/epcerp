import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {createElement} from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import ts from 'typescript';
const data=s=>`data:text/javascript;base64,${Buffer.from(s).toString('base64')}`;
const compile=s=>ts.transpileModule(s,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022,jsx:ts.JsxEmit.ReactJSX}}).outputText;
const base=new URL('../src/components/purchasing/',import.meta.url);
const stub=data(`import {createElement as h} from ${JSON.stringify(import.meta.resolve('react'))};export const DocumentArrowDownIcon=()=>null;export const quoteKey=q=>q.document_id+':'+(q.revision??1);export const quotationMoney=(v,c)=>c+' '+v;export const ValidatedForm=({children,...p})=>h('form',p,children);export const ValidatedLabel=({children,...p})=>h('label',p,children);export const ValidatedInput=p=>h('input',p);export const ValidatedTextarea=p=>h('textarea',p);export const GridAction=({label})=>h('button',{'aria-label':label},'Documents');export const DataGrid=({rows,columns,actions})=>h('table',null,h('tbody',null,...rows.map(r=>h('tr',{key:r.id},...columns.map(c=>h('td',{key:c.key},c.render?c.render(r):c.value(r))),h('td',null,actions?.(r))))));`);
const selection=data(compile(readFileSync(new URL('quotation-comparison-selection.ts',base),'utf8')));
let source=compile(readFileSync(new URL('quotation-shortlist.tsx',base),'utf8')).replaceAll('"react/jsx-runtime"',JSON.stringify(import.meta.resolve('react/jsx-runtime'))).replaceAll('"react"',JSON.stringify(import.meta.resolve('react'))).replace(/from "(?:@[^"]*|\.[^"]*)"/g,(path)=>`from ${JSON.stringify(path.includes("quotation-comparison-selection")?selection:stub)}`);
const {DraftShortlist,GroupNominations,FinanceSubmissionStatus}=await import(data(source));
const quotes=[{document_id:'a',revision:1,group_id:'g',group_name:'Group',vendor_id:'havells',vendor:'Havells',reference:'Q-1',total:'100.00',eligible:true,exclusions:[]},{document_id:'b',revision:1,group_id:'g',group_name:'Group',vendor_id:'polycab',vendor:'Polycab',reference:'Q-2',total:'110.00',eligible:true,exclusions:[]}];
const rows=[{quote_key:'a:1',best_price:true,best_price_comment:'Lowest landed cost'},{quote_key:'b:1',preferred:true,preferred_comment:'Earlier delivery'}];
const workspace={version:1,currency:'INR',can_submit:true};
const render=(extra={})=>renderToStaticMarkup(createElement(DraftShortlist,{rows,quotes,workspace,url:'/api/test',busy:false,onSave:async()=>true,onSubmit:async()=>true,onRefresh:()=>{},onDocuments:()=>{},...extra}));
test('shortlist can be submitted before supplier document verification',()=>{
 const html=render();assert.match(html,/View documents for Havells/);assert.match(html,/View documents for Polycab/);
 assert.match(html,/Supplier documents are verified before PO release/);
 assert.doesNotMatch(html,/<button[^>]*disabled=""[^>]*>Send for approval/);
});
test('an ineligible quote blocks submission even though supplier verification is deferred',()=>{
 const html=render({quotes:[quotes[0],{...quotes[1],eligible:false}]});
 assert.match(html,/<button[^>]*disabled=""[^>]*>Send for approval/);
 assert.match(html,/select eligible quotations before submitting/);
});
test('a single offer can hold both selections',()=>{
 const html=render({quotes:[quotes[0]],rows:[{...rows[0],preferred:true,preferred_comment:'Best delivery'}]});
 assert.equal((html.match(/<tr/g)??[]).length,1);assert.match(html,/Lowest price/);assert.match(html,/Preferred/);assert.doesNotMatch(html,/<button[^>]*disabled=""[^>]*>Send for approval/);assert.match(html,/Refresh status/);assert.match(html,/Print PDF/);
});

const renderNominations=(compared,revisions=compared,initial=[])=>renderToStaticMarkup(createElement(GroupNominations,{
 quotes:compared,revisions,initial,busy:false,canSubmit:true,onSave:()=>{},onSend:()=>{},
 renderGrid:footer=>createElement('table',null,createElement('tfoot',null,footer)),
}));
const checkbox=(html,label)=>html.match(new RegExp(`<input[^>]*aria-label="${label}"[^>]*>`))?.[0]??'';

test('the comparison marks the lowest recorded overall total when all offers are excluded',()=>{
 const html=renderNominations(quotes.map((quote,index)=>({...quote,eligible:false,amount_mode:'total_only',
  total_amount:index?'80.00':'100.00',exclusions:['Partial quotation','Technical compliance not confirmed for every item'],
 })));
 assert.match(checkbox(html,'Lowest quote · Polycab'),/checked=""/);
 assert.doesNotMatch(checkbox(html,'Lowest quote · Polycab'),/disabled=""/);
 for(const quote of quotes)assert.doesNotMatch(checkbox(html,`Preferred partner · ${quote.vendor}`),/disabled=""|checked=""/);
 assert.match(html,/Approval is blocked for this group/);
 assert.match(html,/Lowest quote · Polycab<\/strong> — Partial quotation\. Technical compliance not confirmed for every item/);
 assert.match(html,/<button[^>]*disabled=""[^>]*>Send for approval/);
});

test('the lowest eligible quote is checked automatically and can also be preferred',()=>{
 const compared=[{...quotes[0],suggested:true},quotes[1]];
 const html=renderNominations(compared);
 assert.match(checkbox(html,'Lowest quote · Havells'),/checked=""/);
 assert.doesNotMatch(checkbox(html,'Preferred partner · Havells'),/checked=""|disabled=""/);
 const saved=renderNominations(compared,compared,[{...rows[0],preferred:true,preferred_comment:'Earlier delivery'}]);
 assert.match(checkbox(saved,'Lowest quote · Havells'),/checked=""/);
 assert.match(checkbox(saved,'Preferred partner · Havells'),/checked=""/);
 assert.doesNotMatch(saved,/<button[^>]*disabled=""[^>]*>Send for approval/);
});

test('a higher displayed revision cannot replace the lowest eligible revision',()=>{
 const lowest={...quotes[0],suggested:true};
 const displayed={...lowest,revision:2,suggested:true,total:'105.00'};
 const html=renderNominations([displayed,quotes[1]],[lowest,{...displayed,suggested:false},quotes[1]]);
 assert.doesNotMatch(checkbox(html,'Lowest quote · Havells'),/checked=""/);
 assert.match(html,/Select the lowest eligible quotation revision/);
});

test('a saved Finance submission displays the group and submission time',()=>{
 const submitted_at='2026-11-16T07:30:00Z';
 const html=renderToStaticMarkup(createElement(FinanceSubmissionStatus,{attempt:{group_name:'LED Lighting',submitted_at}}));
 assert.match(html,/LED Lighting was sent to Finance for approval/);
 assert.match(html,/Awaiting Finance review/);
 assert.match(html,/<time dateTime="2026-11-16T07:30:00Z">/i);
});
