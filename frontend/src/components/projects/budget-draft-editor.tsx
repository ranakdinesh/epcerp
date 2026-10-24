"use client";
import {createRecordId} from "@/lib/record-id";
import {useId,useState} from "react";
import {DataGrid,GridAction} from "@/components/ui/data-grid";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {ValidatedForm,ValidatedInput,ValidatedLabel,ValidatedSelect,ValidatedTextarea} from "@/components/ui/validated-controls";
import {budgetInput,type BudgetData,type BudgetHead,type BudgetOwner} from "./budget-contract";
import {decimalUnits,moneyText} from "./purchase-budget-calculations";
const control="mt-1 min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm";
const button="min-h-11 rounded-xl border border-slate-300 bg-white px-4 py-2 text-sm font-semibold disabled:opacity-50";
const kinds=["purchase","labour","subcontract","equipment","engineering","logistics","overheads","testing","contingency","other"];
export function BudgetDraftEditor({initial,owners,contractValue,error,onClose,onReload,onSave}:{initial:BudgetData;owners:BudgetOwner[];contractValue:string;error:string;onClose:()=>void;onReload:()=>void;onSave:(data:BudgetData,reason:string,form:HTMLFormElement)=>Promise<boolean>}) {
 const [data,setData]=useState(()=>budgetInput(initial)),[head,setHead]=useState<BudgetHead|null>(null),[busy,setBusy]=useState(false),[headReadOnly,setHeadReadOnly]=useState(false);
 const formID=useId();
 const owner=(id:string)=>owners.find(o=>o.id===id)?.name ?? id;
 const total=decimalUnits(data.total,2),limit=decimalUnits(contractValue,2);
 const rootTotal=data.heads.filter(h=>h.is_active&&!h.parent_id).reduce((sum,h)=>sum+(decimalUnits(h.amount,2) ?? BigInt(0)),BigInt(0));
 const invalid=total===null||limit===null||total>limit||rootTotal>total;
 const activeAllocations=new Map<string,number>();for(const a of data.allocations)if(a.is_active)activeAllocations.set(a.head_id,(activeAllocations.get(a.head_id) ?? 0)+1);
 function parents(h:BudgetHead){const descendants=new Set([h.id]);for(let n=0;n<data.heads.length;n++)for(const row of data.heads)if(row.parent_id&&descendants.has(row.parent_id))descendants.add(row.id);return data.heads.filter(row=>row.is_active&&row.kind===h.kind&&!descendants.has(row.id));}
 return <RecordDrawer title="Edit budget draft" wide onClose={()=>{if(!busy)onClose()}} footer={<div className="flex justify-end gap-2"><button className={button} disabled={busy} onClick={onClose}>Cancel</button><button form={formID} className="min-h-11 rounded-xl bg-blue-700 px-4 py-2 font-semibold text-white disabled:opacity-50" disabled={busy||invalid}>{busy?"Saving…":"Save budget draft"}</button></div>}>
  <ValidatedForm id={formID} className="space-y-5" onSubmit={async e=>{e.preventDefault();setBusy(true);try{if(await onSave(budgetInput(data),String(new FormData(e.currentTarget).get("reason")),e.currentTarget))onClose()}finally{setBusy(false)}}}>
   {error&&<div className="space-y-2"><p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p><button type="button" className={button} disabled={busy} onClick={onReload}>Discard changes and reload saved budget</button></div>}
   <fieldset disabled={busy} className="min-w-0 space-y-5">
   <p className="text-sm text-slate-600">Contract value {moneyText(limit ?? BigInt(0),data.currency)}. Allocate the cost ceiling to process heads; item prices are managed in Item purchase. Saved records are deactivated rather than deleted.</p>
   <div className="grid gap-4 sm:grid-cols-2"><ValidatedLabel required>Budget cost ceiling<ValidatedInput name="data.total" required type="number" min="0" max={contractValue} step="0.01" className={control} value={data.total} onChange={e=>setData({...data,total:e.target.value})}/></ValidatedLabel><ValidatedLabel required>Responsible budget owner<ValidatedSelect name="data.owner_id" required className={control} value={data.owner_id} onChange={e=>setData({...data,owner_id:e.target.value})}><option value="">Select project owner</option>{owners.map(o=><option key={o.id} value={o.id}>{o.name}</option>)}</ValidatedSelect></ValidatedLabel></div>
   <ValidatedLabel>Notes<ValidatedTextarea name="data.notes" maxLength={2000} className={control} value={data.notes} onChange={e=>setData({...data,notes:e.target.value})}/></ValidatedLabel>
   <div className="flex flex-wrap justify-between gap-3"><h3 className="font-semibold">Process budget heads</h3><button type="button" className={button} disabled={busy||data.heads.length>=100} onClick={()=>{setHeadReadOnly(false);setHead({id:createRecordId(),parent_id:null,name:"",kind:"purchase",amount:"0.00",owner_id:data.owner_id,is_active:true})}}>+ Add budget head</button></div>
   <DataGrid label="Editable budget heads" rows={data.heads} columns={[{key:"name",label:"Head",value:h=>h.name},{key:"parent",label:"Parent",value:h=>data.heads.find(p=>p.id===h.parent_id)?.name ?? "Top level"},{key:"kind",label:"Cost category",value:h=>h.kind},{key:"amount",label:"Allocation",value:h=>moneyText(decimalUnits(h.amount,2) ?? BigInt(0),data.currency)},{key:"owner",label:"Owner",value:h=>owner(h.owner_id)},{key:"active",label:"Status",value:h=>h.is_active?"Active":"Inactive"}]} actions={h=><div className="flex gap-1"><GridAction label={`View budget head ${h.name}`} onClick={()=>{setHeadReadOnly(true);setHead({...h})}}/><GridAction kind="edit" label={`Edit budget head ${h.name}`} onClick={()=>{setHeadReadOnly(false);setHead({...h})}}/></div>}/>
   <p role="status" className={invalid?"text-sm text-red-700":"text-sm text-slate-600"}>{invalid?"Enter a valid ceiling within the contract value and at least the total of active top-level heads.":`Head allocations ${moneyText(rootTotal,data.currency)} · Unallocated ${moneyText((total ?? BigInt(0))-rootTotal,data.currency)}`}</p>
   <ValidatedLabel required>Change reason<ValidatedInput name="reason" required maxLength={2000} className={control}/></ValidatedLabel>
   </fieldset>
  </ValidatedForm>
  {head&&<RecordDrawer title={headReadOnly?"View budget head":data.heads.some(h=>h.id===head.id)?"Edit budget head":"Add budget head"} onClose={()=>setHead(null)}><ValidatedForm className="space-y-4" onSubmit={e=>{e.preventDefault();setData({...data,heads:data.heads.some(h=>h.id===head.id)?data.heads.map(h=>h.id===head.id?head:h):[...data.heads,head]});setHead(null)}}>
   <fieldset disabled={headReadOnly} className="min-w-0 space-y-4">
   <ValidatedLabel required>Head name<ValidatedInput name="name" required maxLength={128} className={control} value={head.name} onChange={e=>setHead({...head,name:e.target.value})}/></ValidatedLabel>
   <ValidatedLabel required>Cost category<ValidatedSelect name="kind" required className={control} value={head.kind} onChange={e=>setHead({...head,kind:e.target.value,parent_id:null})}>{kinds.map(k=><option key={k} value={k}>{k.charAt(0).toUpperCase()+k.slice(1)}</option>)}</ValidatedSelect></ValidatedLabel>
   <ValidatedLabel>Parent head<ValidatedSelect name="parent_id" className={control} value={head.parent_id ?? ""} onChange={e=>setHead({...head,parent_id:e.target.value||null})}><option value="">Top level</option>{parents(head).map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</ValidatedSelect></ValidatedLabel>
   <ValidatedLabel required>Head allocation<ValidatedInput name="amount" required type="number" min="0" max="99999999999999.99" step="0.01" className={control} value={head.amount} onChange={e=>setHead({...head,amount:e.target.value})}/></ValidatedLabel>
   <ValidatedLabel required>Responsible owner<ValidatedSelect name="owner_id" required className={control} value={head.owner_id} onChange={e=>setHead({...head,owner_id:e.target.value})}><option value="">Select project owner</option>{!owners.some(o=>o.id===head.owner_id)&&head.owner_id&&<option value={head.owner_id}>{head.owner_id} (unavailable)</option>}{owners.map(o=><option key={o.id} value={o.id}>{o.name}</option>)}</ValidatedSelect></ValidatedLabel>
   <ValidatedLabel className="flex gap-2 text-sm"><ValidatedInput name="is_active" type="checkbox" checked={head.is_active} disabled={(activeAllocations.get(head.id) ?? 0)>0||data.heads.some(h=>h.is_active&&h.parent_id===head.id)} onChange={e=>setHead({...head,is_active:e.target.checked})}/>Active</ValidatedLabel>
   {(activeAllocations.get(head.id) ?? 0)>0&&<p className="text-sm">This head has active item allocations. Clear their unit prices in Item purchase before deactivating it.</p>}
   </fieldset>
   {!headReadOnly&&<p className="text-sm text-slate-600">Apply updates this form. Save budget draft persists all changes with a reason.</p>}<div className="flex justify-end gap-2"><button type="button" className={button} onClick={()=>setHead(null)}>Cancel</button>{!headReadOnly&&<button className={button}>Apply head</button>}</div>
  </ValidatedForm></RecordDrawer>}
 </RecordDrawer>;
}
