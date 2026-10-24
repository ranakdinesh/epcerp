"use client";

import { DataGrid } from "@/components/ui/data-grid";
import { useId, useLayoutEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { EyeIcon, PencilSquareIcon, ChevronLeftIcon, ChevronRightIcon } from "@heroicons/react/24/outline";
import { controlError, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
import type { VendorIndent } from "./purchase-indent-documents";
import type { Quote } from "./group-quotation-workspace";
import { chargeEstimate, decimalMoney, lineEstimate, editPrice, fixedPrice, type PriceInput } from "./quotation-costs";

const field = "block min-h-11 w-full min-w-20 rounded-md border border-slate-300 bg-white px-2 py-2 text-sm tabular-nums";
const durations = [6, 12, 24, 48];
export function QuotationItemPrices({document, previous, money, taxRate, negotiating=false, canWrite=false, landedCosts}: {canWrite?:boolean; landedCosts?:{line_id:string;landed:string|null}[];document: VendorIndent; previous?: Quote; negotiating?:boolean; taxRate:string; money: (value: string | null | undefined)=>string}) {
  const [entries,setEntries] = useState<Record<string,PriceInput>>(()=>Object.fromEntries(document.data.lines.map(line=>{
    const old=previous?.lines.find(entry=>entry.line_id===line.id);
    const initial:PriceInput={pricing_mode:old?.pricing_mode??"unit_price",unit_price:old?.unit_price??"",line_total:old?.line_total??"",discount_pct:old?.discount_pct??"0",tax_pct:canWrite?taxRate:old?.tax_pct??taxRate};
    return [line.id,editPrice(initial,initial.pricing_mode==="line_total"?"line_total":"unit_price",initial.pricing_mode==="line_total"?initial.line_total:initial.unit_price,line.quantity??"0")];
  })));
  const [activeItem, setActiveItem] = useState<number | null>(null);
  const [itemMode,setItemMode] = useState<"view"|"edit">("view");
  const readOnly = !canWrite || (activeItem !== null && itemMode==="view");
  function closeItem(){setActiveItem(null);setItemMode("edit");}
  const itemDialog = useRef<HTMLDialogElement>(null);
  const itemTitle = useId();
  const itemBody = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    const dialog = itemDialog.current;
    if (itemBody.current) itemBody.current.scrollTop = 0;
    if (activeItem !== null && !dialog?.open) dialog?.showModal();
    if (activeItem === null && dialog?.open) dialog.close();
  }, [activeItem]);
  const [totalOnly,setTotalOnly]=useState(previous?.amount_mode==="total_only");
  const [totalAmount,setTotalAmount]=useState(previous?.total_amount??"");
  const [freight,setFreight]=useState(previous?.freight??"0"),[other,setOther]=useState(previous?.other_charges??"0");
  const estimates=document.data.lines.map(line=>lineEstimate(entries[line.id],line.quantity??"0"));
  const valid=estimates.filter((cost):cost is NonNullable<typeof cost>=>cost!==null);
  const subtotal=valid.reduce((total,cost)=>total+cost.total,BigInt(0));
  const freightCents=chargeEstimate(freight),otherCents=chargeEstimate(other);
  const total=freightCents!==null&&otherCents!==null?subtotal+freightCents+otherCents:null;
  const display=(value:bigint)=>money(decimalMoney(value));
  function update(id:string,patch:Partial<PriceInput>) {setEntries(current=>({...current,[id]:{...current[id],...patch}}));}
  const rows=document.data.lines.map((line,index)=>({...line,position:index+1,sku:line.item_snapshot.sku||line.id,entry:entries[line.id],cost:lineEstimate(entries[line.id],line.quantity??"0"),old:previous?.lines.find(entry=>entry.line_id===line.id),quoted:Boolean(entries[line.id].pricing_mode==="line_total"?entries[line.id].line_total:entries[line.id].unit_price)}));
  type Row = typeof rows[number];
  function numberCell(row: Row, key: keyof Omit<PriceInput,"pricing_mode">, name: string, label: string, step: string, required=false, disabled=false, max?: number) {
    return <ValidatedLabel className="block text-sm font-semibold"><span>{label}</span><ValidatedInput aria-label={`${label} item ${row.position}`} name={`${name}_${row.id}`} title={key==="line_total"?"Total before discount and tax":undefined} type="number" min="0" max={max} step={step} required={required&&!readOnly} disabled={disabled} value={disabled ? "" : row.entry[key]} placeholder={disabled ? "—" : undefined} className={`${field} mt-1 text-right disabled:bg-slate-100 ${negotiating&&key==="discount_pct"?"ring-2 ring-amber-400":""}`} readOnly={key==="tax_pct"} onBlur={event=>{if(key==="unit_price"||key==="line_total")update(row.id,{[key]:fixedPrice(event.target.value)})}} onChange={event=>{if(key==="unit_price"||key==="line_total")setEntries(current=>({...current,[row.id]:editPrice(current[row.id],key,event.target.value,row.quantity??"0")}));else update(row.id,{[key]:event.target.value})}}/></ValidatedLabel>;
  }
  return <div className="min-w-0 space-y-4 [&_table]:min-w-[600px]">
    <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="text-lg font-bold">Quotation pricing</h3><span className="rounded-full bg-blue-50 px-3 py-1 text-sm font-semibold text-blue-800">{valid.length} / {rows.length} items priced</span></div>
    <ValidatedLabel className="indent-selection flex min-h-11 items-center gap-3 rounded-xl border border-slate-300 bg-slate-50 p-4 font-semibold"><ValidatedInput aria-label="Vendor supplied only one overall quotation total" name="total_only" disabled={!canWrite} type="checkbox" checked={totalOnly} onChange={event=>setTotalOnly(event.target.checked)}/>One overall quotation total</ValidatedLabel>
    {totalOnly?<section aria-label="Overall quotation total" className="space-y-3 rounded-xl border border-blue-200 bg-blue-50 p-4"><ValidatedLabel className="block font-semibold">All-inclusive quotation total<ValidatedInput required={canWrite} readOnly={!canWrite} name="total_amount" type="number" min="0.01" step="0.01" value={totalAmount} onBlur={event=>setTotalAmount(fixedPrice(event.target.value))} onChange={event=>setTotalAmount(event.target.value)} className={`${field} mt-1 max-w-md`}/></ValidatedLabel><p className="text-sm text-slate-700">Enter the final amount for the complete vendor indent, including tax, freight and other charges. Item prices and charge allocations are not required. This quotation can be recorded and compared, but it cannot proceed to item-level Accounts approval until the vendor provides an itemized revision.</p></section>:<>
    <DataGrid label="Quotation item pricing" rows={rows} searchable={false} paginate={false} initialSort="position" columns={[
      {key:"position",label:"#",value:row=>row.position},
      {key:"item",label:"Item / specification",value:row=>row.sku,render:row=><div className="min-w-40 max-w-72"><p className="break-all text-sm font-semibold">{row.sku}</p><p className="mt-1 line-clamp-2 text-xs text-slate-600">{row.item_snapshot.description||row.sku}</p></div>},
      {key:"quantity",label:"Quantity",value:row=>row.quantity||"",render:row=><span className="whitespace-nowrap tabular-nums">{Number(row.quantity??0).toFixed(2)} {row.item_snapshot.uom}</span>},
      {key:"total",label:"Total incl. tax",value:row=>row.cost?decimalMoney(row.cost.total):"",render:row=><span className="whitespace-nowrap text-sm font-semibold tabular-nums">{row.cost?display(row.cost.total):row.quoted?"Check price":"Not quoted"}</span>},
      ...(landedCosts?[{key:"landed",label:"Cost incl. charges",value:(row:Row)=>money(landedCosts.find(cost=>cost.line_id===row.id)?.landed)}]:[]),
    ]} actions={row=><div className="flex gap-2"><button type="button" aria-label={`View pricing item ${row.position}`} title={`View pricing item ${row.position}`} className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg border border-slate-300 bg-white text-blue-700" onClick={()=>{setItemMode("view");setActiveItem(row.position-1);}}><EyeIcon className="h-5 w-5"/></button>{canWrite&&<button type="button" aria-label={`Edit pricing item ${row.position}`} title={`Edit pricing item ${row.position}`} className="inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg border border-slate-300 bg-white text-blue-700" onClick={()=>{setItemMode("edit");setActiveItem(row.position-1);}}><PencilSquareIcon className="h-5 w-5"/></button>}</div>}/>
    {/* Keep every item mounted inside the owning form for complete submission and validation. */}
    <dialog ref={itemDialog} aria-labelledby={itemTitle} onKeyDown={event=>{if(event.key==="Enter" && event.target instanceof HTMLInputElement) event.preventDefault();}} onCancel={event=>{event.preventDefault();event.stopPropagation();closeItem();}} className="vtpl-modal quotation-item-editor fixed inset-0 m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-3xl overflow-hidden rounded-xl border-0 bg-slate-50 p-0 text-slate-900 shadow-2xl backdrop:bg-slate-950/45 open:flex open:flex-col">
      <div className="flex shrink-0 items-center justify-between gap-3 border-b border-slate-200 bg-white px-5 py-4"><div><p className="text-xs font-semibold uppercase tracking-wide text-slate-600">{readOnly?"View item · read only":"Edit item pricing"}</p><h2 id={itemTitle} className="text-lg font-bold">Item {(activeItem??0)+1} of {rows.length}</h2></div><button type="button" aria-label="Close item pricing" className="min-h-11 min-w-11 rounded-lg border border-slate-300 text-xl" onClick={closeItem}>×</button></div>
      <div ref={itemBody} className="min-h-0 overflow-y-auto p-4 sm:p-6" onInvalidCapture={event=>{
        const target=event.target as HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement;
        const firstInvalid=Array.from(target.form?.elements??[]).find(control=>(control instanceof HTMLInputElement || control instanceof HTMLSelectElement || control instanceof HTMLTextAreaElement) && controlError(control));
        if (!itemDialog.current?.open && (firstInvalid===target || event.nativeEvent instanceof CustomEvent)) {
          const panel=target.closest<HTMLElement>("[data-quotation-item]");
          if(panel) flushSync(()=>{setItemMode("edit");setActiveItem(Number(panel.dataset.quotationItem));});
        }
      }}>
        {rows.map((row,index)=><section key={row.id} hidden={index!==(activeItem??0)} data-quotation-item={index} className="space-y-5">
          <div className="rounded-xl border border-slate-200 bg-white p-4"><p className="break-all text-sm font-bold">{row.sku}</p><p className="mt-2 text-sm text-slate-600">{row.item_snapshot.description||row.sku}</p><p className="mt-3 text-sm font-semibold">Quantity · <span className="tabular-nums">{Number(row.quantity??0).toFixed(2)} {row.item_snapshot.uom}</span></p></div>
          <fieldset disabled={readOnly} className="space-y-5">
          <ValidatedInput type="hidden" name={`pricing_mode_${row.id}`} value={row.entry.pricing_mode}/>
          <div className="grid gap-4 sm:grid-cols-2">{numberCell(row,"unit_price","price","Unit price","0.01")}{numberCell(row,"line_total","line_total","Line amount","0.01")}{numberCell(row,"discount_pct","discount","Discount %","0.01",true,false,100)}{numberCell(row,"tax_pct","tax","Tax % (fixed)","0.01",true,false,100)}</div>
          <div className="grid gap-4 sm:grid-cols-2">
            <ValidatedLabel className="block text-sm font-semibold">Warranty / guarantee<ValidatedSelect required={row.quoted&&!readOnly} aria-label={`Coverage type item ${row.position}`} name={`coverage_type_${row.id}`} defaultValue={row.old?.coverage_type??"warranty"} className={`${field} mt-1`}><option value="warranty">Warranty</option><option value="guarantee">Guarantee</option><option value="both">Warranty and guarantee</option></ValidatedSelect></ValidatedLabel>
            <ValidatedLabel className="block text-sm font-semibold">Duration (months)<ValidatedSelect required={row.quoted&&!readOnly} aria-label={`Coverage duration item ${row.position}`} name={`warranty_months_${row.id}`} defaultValue={row.old?.warranty_months??""} className={`${field} mt-1`}><option value="">Choose months</option>{durations.map(months=><option key={months} value={months}>{months} months</option>)}</ValidatedSelect></ValidatedLabel>
          </div>
          <ValidatedLabel className="indent-selection flex min-h-11 items-center gap-3 text-sm font-semibold"><ValidatedInput aria-label={`Technical compliance ${row.sku}`} name={`compliant_${row.id}`} type="checkbox" defaultChecked={row.old?.compliant??false}/>Technical compliance confirmed</ValidatedLabel>
          <ValidatedLabel className="block text-sm font-semibold">{row.quoted?"Item notes":"Reason not quoted"}<ValidatedInput required={!row.quoted&&!readOnly} aria-label={`Notes item ${row.position}`} name={`notes_${row.id}`} maxLength={2000} defaultValue={row.old?.notes??""} className={`${field} mt-1`}/></ValidatedLabel>
          </fieldset>
          <div aria-live="polite" className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-blue-200 bg-blue-50 p-4"><span className="text-sm font-semibold">Item total including tax</span><strong className="text-lg tabular-nums">{row.cost?display(row.cost.total):row.quoted?"Check price":"Not quoted"}</strong></div>
        </section>)}
      </div>
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t border-slate-200 bg-white p-4"><div className="flex gap-2"><button type="button" disabled={activeItem===null||activeItem===0} className="inline-flex min-h-11 items-center gap-1 rounded-lg border border-slate-300 px-3 text-sm font-semibold disabled:opacity-40" onClick={()=>setActiveItem(index=>Math.max(0,(index??0)-1))}><ChevronLeftIcon className="h-4 w-4"/>Previous</button><button type="button" disabled={activeItem===null||activeItem===rows.length-1} className="inline-flex min-h-11 items-center gap-1 rounded-lg border border-slate-300 px-3 text-sm font-semibold disabled:opacity-40" onClick={()=>setActiveItem(index=>Math.min(rows.length-1,(index??0)+1))}>Next<ChevronRightIcon className="h-4 w-4"/></button></div><button type="button" className="min-h-11 rounded-lg !bg-blue-700 px-5 text-sm font-semibold !text-white" onClick={closeItem}>Done</button></div>
    </dialog>
    <div className="grid gap-4 sm:grid-cols-2"><ValidatedLabel>Freight (tax inclusive)<ValidatedInput required={canWrite} readOnly={!canWrite} name="freight" type="number" min="0" step="0.01" value={freight} onChange={event=>setFreight(event.target.value)} className={field}/></ValidatedLabel><ValidatedLabel>Other charges (tax inclusive)<ValidatedInput required={canWrite} readOnly={!canWrite} name="other_charges" type="number" min="0" step="0.01" value={other} onChange={event=>setOther(event.target.value)} className={field}/></ValidatedLabel></div>
    <div aria-label="Live quotation total" aria-live="polite" className="rounded-xl border border-blue-200 bg-blue-50 p-4"><p>Items including tax: {display(subtotal)} · {valid.length}/{document.data.lines.length} items priced</p><p className="mt-1 text-lg font-bold">Quotation total including freight and other charges: {total===null?"Check charges":display(total)}{valid.length!==document.data.lines.length?" · partial quotation":""}</p></div>
    </>}
  </div>;
}
