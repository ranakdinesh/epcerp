import assert from "node:assert/strict";
import test from "node:test";
import {readFileSync} from "node:fs";
import {dirname,resolve} from "node:path";
import {fileURLToPath} from "node:url";
import {createElement} from "react";
import {renderToStaticMarkup} from "react-dom/server";
import ts from "typescript";

// Render the real component and shared controls; only Next's route context is stubbed.
const src=fileURLToPath(new URL("../src/",import.meta.url));
const compiled=new Map();
const data=source=>`data:text/javascript;base64,${Buffer.from(source).toString("base64")}`;
function component(path){
 if(compiled.has(path))return compiled.get(path);
 let {outputText}=ts.transpileModule(readFileSync(path,"utf8"),{compilerOptions:{module:ts.ModuleKind.ESNext,target:ts.ScriptTarget.ES2022,jsx:ts.JsxEmit.ReactJSX}});
 outputText=outputText.replace(/from ["']([^"']+)["']/g,(_,specifier)=>{
  let target;
  if(specifier==="next/navigation")target=data('export const useSearchParams=()=>new URLSearchParams();');
  else if(specifier.startsWith("@/")||specifier.startsWith(".")){
   const base=specifier.startsWith("@/")?resolve(src,specifier.slice(2)):resolve(dirname(path),specifier);
   let child;for(const extension of [".tsx",".ts"]){try{readFileSync(base+extension);child=base+extension;break;}catch{}}
   if(!child)throw Error(`Unresolved component ${specifier}`);
   target=component(child);
  }else target=import.meta.resolve(specifier);
  return `from ${JSON.stringify(target)}`;
 });
 const result=data(outputText);compiled.set(path,result);return result;
}
const {QuotationItemPrices}=await import(component(resolve(src,"components/purchasing/quotation-item-prices.tsx")));
const document={data:{lines:[{id:"line-1",quantity:"50.000",item_snapshot:{sku:"CABLE-001",description:"Cable",uom:"Meter"}}]}};
const props={document,taxRate:"18.00",money:value=>value??"Not set"};

test("quotation readers can view every item without receiving edit controls",()=>{
 const html=renderToStaticMarkup(createElement(QuotationItemPrices,{...props,canWrite:false}));
 assert.match(html,/aria-label="View pricing item 1"/);
 assert.doesNotMatch(html,/aria-label="Edit pricing/);
 assert.match(html,/<fieldset disabled=""/);
 assert.match(html,/readonly=""[^>]*name="freight"/i);
 assert.match(html,/50\.00/);
});
test("quotation editing fails closed unless write access is explicitly supplied",()=>{
 const omitted=renderToStaticMarkup(createElement(QuotationItemPrices,props));
 assert.doesNotMatch(omitted,/aria-label="Edit pricing/);
 const writer=renderToStaticMarkup(createElement(QuotationItemPrices,{...props,canWrite:true}));
 assert.match(writer,/aria-label="View pricing item 1"/);
 assert.match(writer,/aria-label="Edit pricing item 1"/);
 assert.doesNotMatch(writer,/<fieldset disabled=""/);
});
test("read-only revisions retain recorded tax and costs including charge allocation",()=>{
 const previous={lines:[{line_id:"line-1",unit_price:"10.00",discount_pct:"0",tax_pct:"5.00",warranty_months:12,coverage_type:"guarantee",compliant:true,notes:"Recorded offer"}],freight:"25.00",other_charges:"0"};
 const html=renderToStaticMarkup(createElement(QuotationItemPrices,{...props,previous,canWrite:false,landedCosts:[{line_id:"line-1",landed:"550.00"}]}));
 assert.match(html,/value="5\.00"/);
 assert.match(html,/525\.00/);
 assert.match(html,/Cost incl\. charges/);
 assert.match(html,/550\.00/);
 assert.match(html,/Recorded offer/);
});
