import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {createElement} from 'react';
import {renderToStaticMarkup} from 'react-dom/server';
import ts from 'typescript';
const data=s=>`data:text/javascript;base64,${Buffer.from(s).toString('base64')}`;
const compile=s=>ts.transpileModule(s,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022,jsx:ts.JsxEmit.ReactJSX}}).outputText;
const base=new URL('../src/components/purchasing/',import.meta.url);
const status=data(compile(readFileSync(new URL('quotation-supplier-status.ts',base),'utf8')));
const stub=data(`import {createElement as h} from ${JSON.stringify(import.meta.resolve('react'))};export const DocumentArrowDownIcon=()=>null;export const quoteKey=q=>q.document_id+':'+(q.revision??1);export const quotationMoney=(v,c)=>c+' '+v;export const ValidatedForm=({children,...p})=>h('form',p,children);export const ValidatedLabel=({children,...p})=>h('label',p,children);export const ValidatedInput=p=>h('input',p);export const ValidatedTextarea=p=>h('textarea',p);export const GridAction=({label})=>h('button',{'aria-label':label},'Documents');export const DataGrid=({rows,columns,actions})=>h('table',null,h('tbody',null,...rows.map(r=>h('tr',{key:r.id},...columns.map(c=>h('td',{key:c.key},c.render?c.render(r):c.value(r))),h('td',null,actions?.(r))))));`);
let source=compile(readFileSync(new URL('quotation-shortlist.tsx',base),'utf8')).replaceAll('"react/jsx-runtime"',JSON.stringify(import.meta.resolve('react/jsx-runtime'))).replaceAll('"react"',JSON.stringify(import.meta.resolve('react'))).replace('"./quotation-supplier-status"',JSON.stringify(status)).replace(/from "(?:@[^"]*|\.[^"]*)"/g,`from ${JSON.stringify(stub)}`);
const {DraftShortlist}=await import(data(source));
const quotes=[{document_id:'a',revision:1,group_id:'g',group_name:'Group',vendor_id:'havells',vendor:'Havells',reference:'Q-1',total:'100.00'},{document_id:'b',revision:1,group_id:'g',group_name:'Group',vendor_id:'polycab',vendor:'Polycab',reference:'Q-2',total:'110.00'}];
const rows=[{quote_key:'a:1',best_price:true,best_price_comment:'Lowest landed cost'},{quote_key:'b:1',preferred:true,preferred_comment:'Earlier delivery'}];
const workspace={version:1,currency:'INR',can_submit:true,vendor_statuses:{havells:{state:'verified',message:'All approved'},polycab:{state:'blocked',message:'Required evidence missing: pan, cheque'}}};
const render=(extra={})=>renderToStaticMarkup(createElement(DraftShortlist,{rows,quotes,workspace,url:'/api/test',busy:false,onSave:async()=>true,onSubmit:async()=>true,onRefresh:()=>{},onDocuments:()=>{},...extra}));
test('shortlist distinguishes approved and missing suppliers and keeps document access beside their offer',()=>{
 const html=render();assert.match(html,/✓ Approved/);assert.match(html,/Missing documents/);assert.match(html,/PAN, cancelled cheque/);assert.match(html,/View documents for Havells/);assert.match(html,/View documents for Polycab/);assert.doesNotMatch(html,/Upload missing documents in Vendors/);
 assert.match(html,/<button[^>]*disabled=""[^>]*>Submit for approval/);assert.doesNotMatch(html,/<button[^>]*disabled=""[^>]*>Save draft/);
});
test('a single offer can hold both selections and approved documents permit submission',()=>{
 const html=render({quotes:[quotes[0]],rows:[{...rows[0],preferred:true,preferred_comment:'Best delivery'}]});
 assert.equal((html.match(/<tr/g)??[]).length,1);assert.match(html,/Lowest price/);assert.match(html,/Preferred/);assert.doesNotMatch(html,/<button[^>]*disabled=""[^>]*>Submit for approval/);assert.match(html,/Refresh status/);assert.match(html,/Print PDF/);
});
