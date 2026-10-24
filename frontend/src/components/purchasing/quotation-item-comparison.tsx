"use client";
import { useId, useRef, type ReactNode } from "react";
import { ValidatedSelect } from "@/components/ui/validated-controls";
import { quoteKey } from "./quotation-comparison-selection";
import { DataGrid } from "@/components/ui/data-grid";
import type { Comparison } from "./group-quotation-workspace";

export function ItemComparisonMatrix({comparisons,revisions,money,onRevision,onDetails,groupFooter,quoteStatus}:{comparisons:Comparison[];revisions?:Comparison[];money:(v:string|null|undefined)=>string;onRevision?:(document:string,key:string)=>void;onDetails?:(quote:Comparison)=>void;groupFooter?:(quotes:Comparison[])=>ReactNode;quoteStatus?:(quote:Comparison)=>string}) {
 const navigationId=useId();
 const sections=useRef<Record<string,HTMLElement|null>>({});
 const groupButton="min-h-11 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-blue-800";
 function goToGroup(group:string){const section=sections.current[group];section?.scrollIntoView({block:"start",behavior:"auto"});section?.focus({preventScroll:true});}
 const groupName=(group:string)=>comparisons.find(quote=>quote.group_id===group)?.group_name??group;
 const groups=[...new Set(comparisons.map(quote=>quote.group_id))].sort((a,b)=>(comparisons.find(quote=>quote.group_id===a)?.group_name??"").localeCompare(comparisons.find(quote=>quote.group_id===b)?.group_name??""));
 return <div className="min-w-0 space-y-6">
  {groups.length>1&&<nav aria-label="Comparison item groups" className="flex flex-wrap items-center gap-2"><span className="mr-2 text-xs font-semibold text-slate-600">Item groups</span><div className="flex flex-wrap gap-2">{groups.map(group=><button key={group} type="button" className={groupButton} aria-controls={`${navigationId}-${group}`} onClick={()=>goToGroup(group)}>{groupName(group)}</button>)}</div></nav>}
  {groups.map((group,groupIndex)=>{
  const quotes=comparisons.filter(quote=>quote.group_id===group).sort((a,b)=>a.vendor.localeCompare(b.vendor));
  const items=Array.from(new Map(quotes.flatMap(quote=>quote.lines.map(line=>[line.line_id,line] as const))).values());
  type Row={id:string;label:string;kind:"item"|"quote"|"total"|"delivery"|"validity"|"payment"|"terms";line?:typeof items[number]};
  const rows:Row[]=[{id:"quote",label:"Quotation being compared",kind:"quote"},...items.map((line,index)=>({id:`item-${line.line_id}`,label:`${index+1}. ${line.source.item_snapshot.description||line.source.item_snapshot.sku||"Item"}`,kind:"item" as const,line})),{id:"total",label:"Group total · tax & charges included",kind:"total"},{id:"delivery",label:"Delivery from PO",kind:"delivery"},{id:"validity",label:"Valid until",kind:"validity"},{id:"payment",label:"Payment terms",kind:"payment"},{id:"terms",label:"Delivery basis / location",kind:"terms"}];
  const recommendations=quotes.filter(quote=>quote.eligible&&quote.suggested);
  const cellValue=(row:Row,quote:Comparison)=>{
   if(!quote.eligible)return `Excluded: ${quote.exclusions.join("; ")}`;
   if(row.kind==="quote")return `${quote.reference} · revision ${quote.revision??1}`;
   if(row.kind==="total")return `${money(quote.total)} · L${quote.rank}`;
   if(row.kind==="delivery")return `${quote.delivery_days} days`;
   if(row.kind==="validity")return quote.valid_until??"Not recorded";
   if(row.kind==="payment")return quote.payment_terms||"Not recorded";
   if(row.kind==="terms")return quote.delivery_terms||"Not recorded";
   const line=quote.lines.find(line=>line.line_id===row.line?.line_id);
   return `Unit ${money(line?.unit_price)} · Item total ${money(line?.landed)}${line?.item_lowest?" · lowest item price":""}`;
  };
  return <section key={group} id={`${navigationId}-${group}`} ref={element=>{sections.current[group]=element;}} tabIndex={-1} aria-label={`Price comparison ${quotes[0].group_name}`} className="min-w-0 scroll-mt-24 space-y-3 focus-visible:outline-2 focus-visible:outline-blue-500">
   <div className="flex flex-wrap items-end justify-between gap-2"><h3 className="text-base font-bold">{quotes[0].group_name} · side-by-side prices</h3><span className="text-sm text-slate-600">{items.length} {items.length===1?"item":"items"} · {quotes.length} {quotes.length===1?"vendor":"vendors"}</span></div>
   {groups.length>1&&<div className="flex flex-wrap items-center justify-between gap-2"><span className="text-sm font-semibold text-slate-600">Group {groupIndex+1} of {groups.length}</span>{groups[groupIndex+1]&&<button type="button" className={groupButton} onClick={()=>goToGroup(groups[groupIndex+1])}>Next group: {groupName(groups[groupIndex+1])} →</button>}</div>}
   {recommendations.length>0&&<p className="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-950"><strong>Best group price: </strong>{recommendations.map(quote=>`${quote.vendor} · ${money(quote.total)}`).join(" / ")}{recommendations.length>1?" · tied L1":" · L1"}</p>}
   <DataGrid label={`Item comparison ${quotes[0].group_name}`} rows={rows} searchable={false} paginate={false} initialSort="order" columns={[
    {key:"order",label:"Item / comparison detail",value:row=>rows.indexOf(row),className:"min-w-64",render:row=><div className="max-w-80"><p className={`break-words ${row.kind!=="item"?"font-bold":"font-semibold"}`}>{row.label}</p>{row.line?.source.item_snapshot.sku&&<p className="mt-1 break-words text-xs text-slate-500">{row.line.source.item_snapshot.sku}</p>}</div>},
    {key:"quantity",label:"Quantity / unit",value:row=>row.line?.quantity??"",className:"min-w-28",render:row=><span className="tabular-nums">{row.line?`${row.line.quantity} ${row.line.source.item_snapshot.uom??""}`:"—"}</span>},
    ...quotes.map(quote=>({key:quote.document_id,label:quote.vendor,className:"min-w-64",value:(row:Row)=>cellValue(row,quote),render:(row:Row)=>{
      const line=row.kind==="item"?quote.lines.find(line=>line.line_id===row.line?.line_id):undefined;
      const best=quote.eligible&&(row.kind==="total"?quote.suggested:line?.item_lowest);
      return <div className={`max-w-80 space-y-1 rounded-lg p-2 ${best?"border border-emerald-200 bg-emerald-50 text-emerald-950":""}`}>
       {row.kind==="quote"?<><p className="break-words font-semibold">{quote.reference}</p>{quoteStatus&&<p role="status" className="rounded bg-blue-50 p-2 text-xs text-blue-900">{quoteStatus(quote)}</p>}<p className="text-xs text-slate-600">Revision {quote.revision??1}{quote.eligible?" · eligible":` · ${quote.exclusions.join("; ")}`}</p><div className="flex flex-wrap gap-2 pt-2">{onRevision&&<ValidatedSelect aria-label={`Quotation revision ${quote.vendor} ${quote.group_name}`} className="min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" value={quoteKey(quote)} onChange={event=>onRevision(quote.document_id,event.target.value)}>{(revisions??comparisons).filter(option=>option.document_id===quote.document_id).sort((a,b)=>(b.revision??1)-(a.revision??1)).map(option=><option key={quoteKey(option)} value={quoteKey(option)} disabled={!option.eligible}>R{option.revision??1} · {money(option.total)}{option.eligible?"":` · excluded: ${option.exclusions.join("; ")}`}</option>)}</ValidatedSelect>}{onDetails&&<button type="button" aria-label={`View prices ${quote.vendor} ${quote.group_name}`} className="min-h-11 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" onClick={()=>onDetails(quote)}>View prices</button>}</div></>:
       row.kind==="item"&&quote.eligible&&line?<><p className="text-sm text-slate-600">{line.pricing_mode==="line_total"?"Derived unit price":"Unit price"} <span className="font-semibold tabular-nums text-slate-900">{money(line.unit_price)}</span></p><p className="text-xs text-slate-600">Discount {line.discount_pct}% · tax {line.tax_pct}%</p><p className="text-xs text-slate-600">Tax {money(line.tax_amount)} · charges {money(line.allocated_charges)}</p><p className="font-bold tabular-nums">{money(line.landed)}</p>{best&&<p className="text-xs font-semibold">Lowest item price</p>}</>:
       <><p className={`break-words ${row.kind==="total"?"text-lg font-bold tabular-nums":"text-sm"}`}>{cellValue(row,quote)}</p>{best&&<p className="text-xs font-semibold">Lowest group total</p>}</>}
      </div>;
    }}))
   ]}/>
   {groupFooter?.(quotes)}
   {groups.length>1&&<div className="flex flex-wrap justify-between gap-2 border-t border-slate-200 pt-3">{groups[groupIndex-1]?<button type="button" className={groupButton} onClick={()=>goToGroup(groups[groupIndex-1])}>← Previous group: {groupName(groups[groupIndex-1])}</button>:<span/>}{groups[groupIndex+1]?<button type="button" className={groupButton} onClick={()=>goToGroup(groups[groupIndex+1])}>Next group: {groupName(groups[groupIndex+1])} →</button>:<button type="button" className={groupButton} onClick={()=>goToGroup(groups[0])}>Back to first group</button>}</div>}
  </section>;
 })}</div>;
}
