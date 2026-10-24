import assert from "node:assert/strict";
import test from "node:test";
import {readFileSync} from "node:fs";
import {createElement} from "react";
import {renderToStaticMarkup} from "react-dom/server";
import ts from "typescript";
const data=source=>`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`;
const compile=source=>ts.transpileModule(source,{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022,jsx:ts.JsxEmit.ReactJSX}}).outputText;
const termsModule=data(compile(readFileSync(new URL('../src/components/purchasing/quotation-terms.ts',import.meta.url),'utf8')));
const component=compile(readFileSync(new URL('../src/components/purchasing/purchase-indent-terms.tsx',import.meta.url),'utf8')).replaceAll('"./quotation-terms"',JSON.stringify(termsModule)).replaceAll('"react/jsx-runtime"',JSON.stringify(import.meta.resolve('react/jsx-runtime')));
const {vendorIndentDefaults,PurchaseIndentTerms}=await import(data(component));
const {indentDeliveryTerms}=await import(termsModule);
const vendor=(notes,conditions,delivery)=>({data:{indent_notes:notes,indent_terms:conditions,delivery_terms:delivery,payment_terms:{description:'Agreed payment'}}});
test('different vendor masters provide different clauses without shared commercial defaults',()=>{
 const a=vendorIndentDefaults(vendor('Note A\r\n\r\nSecond note','Terms A','Site A'));
 const b=vendorIndentDefaults(vendor('Note B','Terms B','Site B'));
 assert.deepEqual(a.notes,['Note A','Second note']);
 assert.deepEqual(a.conditions,['Terms A']);
 assert.deepEqual(b.conditions,['Terms B']);
 assert.equal(a.delivery_basis,'Site A');
 assert.deepEqual(vendorIndentDefaults(),{source:'vendor_master',notes:[],conditions:[],delivery_basis:''});
});
test('an issued snapshot renders its own clauses after master defaults change',()=>{
 const supplier=vendor('Old note','Original agreed condition','Original site');
 const snapshot=vendorIndentDefaults(supplier);
 supplier.data.indent_terms='New master condition';supplier.data.delivery_terms='New site';
 const html=renderToStaticMarkup(createElement(PurchaseIndentTerms,{terms:snapshot,supplier}));
 assert.match(html,/Original agreed condition/);assert.match(html,/Original site/);
 assert.doesNotMatch(html,/New master condition|New site/);
 assert.equal(indentDeliveryTerms({data:{terms:snapshot}}),'Original site');
 assert.equal(indentDeliveryTerms({data:{}}),'');
});
test('vendor clauses remain escaped plain text and missing snapshots are explicit',()=>{
 const html=renderToStaticMarkup(createElement(PurchaseIndentTerms,{terms:vendorIndentDefaults(vendor('','<script>bad()</script>','Vendor site'))}));
 assert.match(html,/&lt;script&gt;/);assert.doesNotMatch(html,/<script>/);
 const missing=renderToStaticMarkup(createElement(PurchaseIndentTerms,{}));
 assert.match(missing,/no saved terms snapshot/);
 assert.doesNotMatch(missing,/5 years|arbitration|F\.O\.R\./);
});
