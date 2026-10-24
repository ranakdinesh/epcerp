"use client";
import {useState,type SubmitEvent} from "react";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {ValidatedForm,ValidatedInput,ValidatedLabel,ValidatedTextarea} from "@/components/ui/validated-controls";
import {displayServerErrors} from "@/components/ui/server-field-errors";
import {ItemSearch} from "@/components/purchasing/bo-line-grid";
import type {Item,Line} from "@/components/purchasing/purchase-bo-workspace";
import {itemInputErrors,itemSavePayload,type ItemPreparation} from "./purchase-items-model";
const input="mt-2 block min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm";
const button="min-h-11 rounded-xl border border-slate-300 px-4 py-2 text-sm font-semibold disabled:opacity-50";
export function PurchaseItemEditor({project,record,initial,adding,items,used,savedNotice,onClose,onSaved}:{project:string;record:ItemPreparation;initial:Line;adding:boolean;items:Item[];used:Set<string>;savedNotice?:string;onClose:()=>void;onSaved:(record:ItemPreparation,addAnother:boolean)=>void}){
 const[line,setLine]=useState<Line>(()=>({...initial,item_snapshot:{...initial.item_snapshot}})),[reason,setReason]=useState(""),[busy,setBusy]=useState(false),[error,setError]=useState(""),[fields,setFields]=useState<Record<string,string>>({});
 const selected=line.item_snapshot;
 function patch(value:Partial<Line>){setLine(old=>({...old,...value}));setFields({});setError("")}
 async function submit(event:SubmitEvent<HTMLFormElement>){
  event.preventDefault();if(busy)return;const form=event.currentTarget;const addAnother=adding&&(event.nativeEvent.submitter as HTMLButtonElement|null)?.value==="another";
  const errors=itemInputErrors(line,selected,used);if(!adding&&!reason.trim())errors.reason="Explain the correction.";
  setFields(errors);if(Object.keys(errors).length){const field=Object.keys(errors)[0];(form.elements.namedItem(field) as HTMLElement|null)?.focus();return;}
  setBusy(true);setError("");
  try{const body=itemSavePayload(record,line,adding,adding?"Purchase item added":reason.trim());
   const response=await fetch(`/api/projects/${encodeURIComponent(project)}/purchase-bo/${record.id}`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});const value=await response.json();
   if(!response.ok){if(response.status===409){setError("The saved items changed. Close this popup and refresh items before retrying; your changes have not overwritten anyone’s work.");return;}if(!displayServerErrors(form,value))setError(typeof value.detail==="string"?value.detail:"The item could not be saved. Check the entered values.");return;}
   onSaved(value as ItemPreparation,addAnother);
  }catch(e){setError(e instanceof Error?e.message:"Save could not be confirmed. Refresh the saved items before trying again.");}finally{setBusy(false);}
 }
 return <RecordDrawer title={adding?"Add item":"Edit item"} onClose={()=>{if(!busy)onClose()}} footer={<div className="flex flex-wrap justify-end gap-3"><button type="button" className={button} disabled={busy} onClick={onClose}>Cancel</button>{adding&&<button type="submit" form="purchase-item-form" name="save_mode" value="another" disabled={busy} className={button}>Save &amp; add another</button>}<button type="submit" form="purchase-item-form" name="save_mode" value="done" disabled={busy} className={`${button} border-transparent bg-blue-700 text-white`}>{busy?"Saving…":adding?"Done":"Save item"}</button></div>}>
 <div className="go-purchase-content">{savedNotice&&<p role="status" className="rounded-lg bg-green-50 p-3 text-sm text-green-800">{savedNotice}</p>}{error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-sm text-red-800">{error}</p>}
 <ValidatedForm id="purchase-item-form" onSubmit={submit} className="space-y-5"><fieldset disabled={busy} className="space-y-5"><ValidatedLabel required className="block text-sm font-semibold">Item name / SKU{line.demand_id?<ValidatedInput name="item_id" required readOnly value={`${selected.description??""} · ${selected.sku??""}`} className={input}/>:<ItemSearch required inputName="item_id" error={fields.item_id} value={selected.description?`${selected.description} · ${selected.sku??""}`:""} items={items} used={used} onSelect={item=>patch({item_id:item?.id??null,item_snapshot:item??{},quantity:null})} onError={message=>setFields(old=>({...old,item_id:message}))}/>}</ValidatedLabel>
 <ValidatedLabel className="block text-sm font-semibold">Technical specifications<ValidatedTextarea name="specifications" readOnly value={selected.description??""} rows={4} className={input}/></ValidatedLabel>
 <div className="grid gap-5 sm:grid-cols-2"><ValidatedLabel className="block text-sm font-semibold">Quantity<ValidatedInput name="quantity" required inputMode="decimal" value={line.quantity??""} error={fields.quantity} onChange={e=>patch({quantity:e.target.value})} className={input}/></ValidatedLabel><ValidatedLabel className="block text-sm font-semibold">Unit<ValidatedInput name="unit" readOnly value={selected.uom??""} placeholder="From selected item" className={input}/></ValidatedLabel><ValidatedLabel className="block text-sm font-semibold">Required at site<ValidatedInput name="required_date" required type="date" value={line.required_date??""} error={fields.required_date} onChange={e=>patch({required_date:e.target.value})} className={input}/></ValidatedLabel><ValidatedLabel className="block text-sm font-semibold">BOQ / drawing reference<ValidatedInput name="reference" maxLength={500} value={line.reference} onChange={e=>patch({reference:e.target.value})} className={input}/></ValidatedLabel></div>
 <ValidatedLabel className="block text-sm font-semibold">Notes<ValidatedTextarea name="notes" rows={3} maxLength={4000} value={line.notes} onChange={e=>patch({notes:e.target.value})} className={input}/></ValidatedLabel>
 {!adding&&<ValidatedLabel className="block text-sm font-semibold">Change reason<ValidatedTextarea name="reason" required maxLength={2000} value={reason} error={fields.reason} onChange={e=>setReason(e.target.value)} className={input}/></ValidatedLabel>}
 </fieldset></ValidatedForm></div></RecordDrawer>;
}
