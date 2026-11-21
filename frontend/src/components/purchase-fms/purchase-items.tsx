"use client";
import "./choose-vendors.css";
import { CategoryThumbnail } from "@/components/items/category-thumbnail";
import {PurchaseSectionHeader} from "./purchase-layout";
import {useEffect,useRef,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";
import {PlusIcon,ArrowPathIcon,DocumentArrowDownIcon} from "@heroicons/react/24/outline";
import {useAuthenticatedUser} from "@/components/auth-user-provider";
import {DataGrid,GridAction,GridStatus} from "@/components/ui/data-grid";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {UserIdentity} from "@/components/personal-profile";
import {ValidatedForm,ValidatedLabel,ValidatedTextarea} from "@/components/ui/validated-controls";
import {purchaseStepAllowed} from "@/components/projects/workspace-permissions";
import {createRecordId} from "@/lib/record-id";
import type {Item,Line} from "@/components/purchasing/purchase-bo-workspace";
import {PurchaseItemEditor} from "./purchase-item-editor";
import {nextItemEntry,itemRows,canEditPreparation,editablePreparation,purchaseCategory,usedItems,type ItemPreparation,type ItemListing,type PurchaseItemRow} from "./purchase-items-model";
const button="app-action inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-transparent px-4 py-2 text-sm font-semibold disabled:opacity-50";
async function read<T>(path:string,signal?:AbortSignal):Promise<T>{const response=await fetch(path,{cache:"no-store",signal});const data=await response.json();if(!response.ok)throw Error(typeof data.detail==="string"?data.detail:"Purchase items could not be loaded.");return data as T;}
export function PurchaseItems({project}:{project:string;projectName:string}){
 const router=useRouter();
 const user=useAuthenticatedUser(),superuser=user.permissions.includes("system:admin");
 const visible=purchaseStepAllowed(user.permissions,1),catalogueAllowed=superuser||user.permissions.includes("projects.items.read");
 const [listing,setListing]=useState<ItemListing|null>(null),[items,setItems]=useState<Item[]>([]),[error,setError]=useState(""),[catalogueError,setCatalogueError]=useState(""),[loading,setLoading]=useState(true),[revision,setRevision]=useState(0),[notice,setNotice]=useState("");
 const [editor,setEditor]=useState<{record:ItemPreparation;initial:Line;adding:boolean;row?:PurchaseItemRow}|null>(null),[view,setView]=useState<PurchaseItemRow|null>(null),[action,setAction]=useState<{record:ItemPreparation;command:"complete"|"reopen"}|null>(null);
 const [registerAction,setRegisterAction]=useState<"complete"|"reopen"|null>(null);
 const finalised=!!listing?.lists.length&&listing.lists.every(r=>r.state==="completed");
 const canChangeState=(record:ItemPreparation)=>!!listing?.can_complete&&(listing.can_manage_lists||record.preparer_id===user.id);
 const canFinalise=!!listing?.lists.filter(r=>r.state===(finalised?"completed":"draft")).every(canChangeState);
 const base=`/api/projects/${encodeURIComponent(project)}/purchase-bo`;
 useEffect(()=>{if(!visible)return;const controller=new AbortController();Promise.allSettled([read<ItemListing>(base,controller.signal),catalogueAllowed?read<Item[]>(`/api/projects/${encodeURIComponent(project)}/items/catalogue`,controller.signal):Promise.resolve([] as Item[])]).then(([records,catalogue])=>{if(controller.signal.aborted)return;if(records.status==="fulfilled"){setListing(records.value);setError("")}else{setListing(null);setError(records.reason instanceof Error?records.reason.message:"Items could not be loaded.")}if(catalogue.status==="fulfilled"){setItems(catalogue.value);setCatalogueError("")}else{setItems([]);setCatalogueError(catalogue.reason instanceof Error?catalogue.reason.message:"Eligible items could not be loaded.")}setLoading(false)});return()=>controller.abort()},[base,project,visible,catalogueAllowed,revision]);
 const visit=useRef<{project:string;id:string;recorded:boolean}|null>(null);
 useEffect(()=>{
  if(!visible||loading||!listing)return;
  if(visit.current?.project!==project)visit.current={project,id:createRecordId(),recorded:false};
  const current=visit.current;
  if(current.recorded)return;
  const record=()=>fetch(`${base}/activity`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({visit_id:current.id}),keepalive:true});
  async function logOpen(){
   for(let attempt=0;attempt<2;attempt++){
    try{
     const response=await record();
     if(response.ok){current.recorded=true;return;}
     if(response.status<500)return;
    }catch{/* Retry transient network failures with the same visit ID. */}
   }
  }
  void logOpen();
 },[base,project,visible,loading,listing]);
 function refresh(){setLoading(true);setRevision(v=>v+1)}
 function saved(record:ItemPreparation,message:string){setListing(old=>old?{...old,lists:[record,...old.lists.filter(r=>r.id!==record.id)]}:old);setEditor(null);setView(null);setAction(null);setNotice(message);window.dispatchEvent(new CustomEvent("purchase-progress-saved",{detail:project}))}
 const records=listing?.lists??[];
 const mode=listing?.item_list_mode??"single";
 const allRows=itemRows(records);
 const existing=editablePreparation(records,user.id,superuser,mode==="single"?200:100);
 const canAdd=!!listing?.can_write&&catalogueAllowed&&!catalogueError&&!loading&&!finalised&&(mode==="multiple"||!records.length||!!existing);
 function add(){if(!canAdd)return;const record=existing??{id:createRecordId(),preparer_id:user.id,version:0,state:"draft",lines:[]};setEditor({record,adding:true,initial:{id:createRecordId(),item_id:null,quantity:null,required_date:null,reference:"",notes:"",item_snapshot:{},demand_id:null}});setNotice("")}
 const itemLabel=(row:PurchaseItemRow)=>row.line.item_snapshot.description?.split(/[·\n]/)[0].trim()||"Item selection pending";
 if(!visible)return <p role="alert">You do not have access to purchase items.</p>;
 return <section className="purchase-item-register choose-vendors space-y-4"><PurchaseSectionHeader title="Items to purchase"><div className="flex flex-wrap items-center justify-end gap-2"><a href={loading?undefined:`${base}/pdf`} download aria-disabled={loading} aria-label="Download purchase items PDF" title="Download landscape PDF" className={button}><DocumentArrowDownIcon className="h-5 w-5" aria-hidden="true"/></a>{listing?.can_write&&!finalised&&<button type="button" className={`${button} w-full sm:w-auto`} disabled={!canAdd} onClick={add}><PlusIcon className="h-5 w-5" aria-hidden="true"/>Add item</button>}</div></PurchaseSectionHeader>{finalised&&<p role="status" className="rounded-lg border-l-4 border-[#588368] bg-green-50 p-3 text-sm text-green-900">List finalised.{canFinalise?" Reopen with a comment to add or change items.":""}</p>}
 {error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p>}{catalogueError&&<p role="alert" className="rounded-lg bg-yellow-50 p-3 text-yellow-900">The saved items remain available. Item selection could not be loaded: {catalogueError}</p>}{listing?.can_write&&!catalogueAllowed&&<p role="status">Item selection requires project material read permission.</p>}{notice&&<p role="status" className="rounded-lg bg-green-50 p-3 text-green-800">{notice}</p>}
 <div className="purchase-items-grid"><DataGrid<PurchaseItemRow> label="Items to purchase" rows={allRows} loading={loading} initialSort="item"
 grouping={{columnKey:"category",key:row=>row.line.item_snapshot.category_code||"",label:row=>purchaseCategory(row.line.item_snapshot.category_code),spacerRow:true}}
 toolbar={<button type="button" className={button} disabled={loading} onClick={refresh}><ArrowPathIcon className="h-4 w-4" aria-hidden="true"/>Refresh items</button>} columns={[
 {key:"item",label:"Item / SKU",value:r=>`${itemLabel(r)} ${r.line.item_snapshot.sku??""}`,render:r=><div className="flex items-start gap-2"><CategoryThumbnail code={r.line.item_snapshot.category_code}/><div className="min-w-0 [overflow-wrap:anywhere]"><strong>{itemLabel(r)}</strong><p className="mt-1 text-xs text-slate-500">{r.line.item_snapshot.sku??"Not selected"}</p></div></div>},
 {key:"specifications",label:"Technical specifications",value:r=>r.line.item_snapshot.description??"",render:r=><p className="text-sm" title={r.line.item_snapshot.description}>{r.line.item_snapshot.description??"Pending item selection"}</p>},
 {key:"quantity",label:"Quantity",value:r=>r.line.quantity??"—"},
 {key:"unit",label:"Unit",value:r=>r.line.item_snapshot.uom??"—"},
 {key:"date",label:"Required at site",value:r=>r.line.required_date??"—"},
 {key:"status",label:"Status",value:r=>r.preparation.state==="completed"?"Finalised":"Draft",render:r=><GridStatus value={r.preparation.state==="completed"?"Finalised":"Draft"}/>},
 ]} actions={row=><div className="flex justify-end gap-1"><GridAction label={`View ${itemLabel(row)}`} onClick={()=>setView(row)}/>{canEditPreparation(row.preparation,user.id,superuser,!!listing?.can_write)&&catalogueAllowed&&!catalogueError&&<GridAction kind="edit" label={`Edit ${itemLabel(row)}`} onClick={()=>setEditor({record:row.preparation,initial:row.line,adding:false,row})}/>}</div>}/></div>
 {canFinalise && allRows.length > 0 && (
   <div className="flex justify-end">
     <button
       type="button"
       className={finalised ? button : `${button} focus-visible:outline-2 focus-visible:outline-offset-2`}
       disabled={loading}
       onClick={() => setRegisterAction(finalised ? "reopen" : "complete")}
     >
       {finalised ? "Reopen list" : "Finalise list"}
     </button>
   </div>
 )}
 {!loading&&listing&&allRows.length>0&&!canFinalise&&<p className="text-sm text-slate-600">{user.permissions.includes("projects.purchase_bo.complete")||superuser
   ?"The list preparer or the project’s assigned Purchase Manager can change this list’s status when purchasing is open."
   :"Ask an administrator for the Finalise and reopen purchase item lists permission."}</p>}
 {!loading&&!error&&!allRows.length&&<p className="text-sm text-slate-600">No purchase items added yet.{canAdd?" Use Add item to enter the first requirement.":""}</p>}
 {editor&&<PurchaseItemEditor key={editor.initial.id} savedNotice={notice} project={project} record={editor.record} initial={editor.initial} adding={editor.adding} items={items} used={usedItems(listing?.lists??[],editor.row)} onClose={()=>setEditor(null)} onSaved={(record,addAnother)=>{saved(record,editor.adding?(addAnother?"Item saved. Add the next item.":"Item added."):"Item changes saved.");if(addAnother&&(mode==="multiple"||record.lines.length<200))setEditor(nextItemEntry(record,user.id,createRecordId(),createRecordId(),mode))}}/>}
 {view&&<RecordDrawer title="Purchase item" onClose={()=>setView(null)} footer={<div className="flex flex-wrap justify-end gap-3">{canChangeState(view.preparation)&&<button type="button" className={`${button}`} onClick={()=>{setAction({record:view.preparation,command:view.preparation.state==="completed"?"reopen":"complete"});setView(null)}}>{view.preparation.state==="completed"?"Reopen for changes":"Publish requirements"}</button>}<Link className={button} href={`/dashboard?screen=purchase-fms&project=${encodeURIComponent(project)}&step=PUR-VENDORS`}>Choose vendors</Link></div>}><dl className="grid gap-5 text-sm sm:grid-cols-2">{[["Item",view.line.item_snapshot.description],["SKU",view.line.item_snapshot.sku],["Quantity",view.line.quantity],["Unit",view.line.item_snapshot.uom],["Required at site",view.line.required_date],["Reference",view.line.reference],["Notes",view.line.notes],["Status",view.preparation.state==="completed"?"Finalised":"Draft"]].map(([label,value])=><div key={label}><dt className="text-slate-500">{label}</dt><dd className="mt-1 whitespace-pre-wrap break-words font-semibold">{value||"Not recorded"}</dd></div>)}<div><dt className="text-slate-500">Prepared by</dt><dd className="mt-1"><UserIdentity id={view.preparation.preparer_id} name="Preparer"/></dd></div></dl></RecordDrawer>}
 {action&&<PreparationAction base={base} record={action.record} command={action.command} onClose={()=>setAction(null)} onSaved={record=>saved(record,action.command==="complete"?"Requirements published for purchasing.":"Items reopened for changes.")}/>}
 {registerAction&&listing&&<RegisterAction base={base} records={listing.lists} command={registerAction} onClose={()=>setRegisterAction(null)} onSaved={value=>{setListing(value);setRegisterAction(null);window.dispatchEvent(new CustomEvent("purchase-progress-saved",{detail:project}));if(registerAction==="complete")router.push(`/dashboard?screen=purchase-fms&project=${encodeURIComponent(project)}&step=PUR-VENDORS`);setNotice(registerAction==="complete"?"List finalised. Material requirements published.":"List reopened for changes.")}}/>}
 </section>;
}
function PreparationAction({base,record,command,onClose,onSaved}:{base:string;record:ItemPreparation;command:"complete"|"reopen";onClose:()=>void;onSaved:(record:ItemPreparation)=>void}){
 const [reason,setReason]=useState(""),[busy,setBusy]=useState(false),[error,setError]=useState("");
 return <RecordDrawer title={command==="complete"?"Publish requirements":"Reopen requirements"} onClose={()=>{if(!busy)onClose()}} footer={<div className="flex justify-end gap-3"><button type="button" className={button} disabled={busy} onClick={onClose}>Cancel</button><button form="publish-purchase-items" type="submit" disabled={busy} className={`${button}`}>{busy?"Saving…":command==="complete"?"Publish items":"Reopen for changes"}</button></div>}><p className="text-sm">This {command==="complete"?"publishes":"reopens"} the {record.lines.length} items prepared together in this submission. Their previous history and released procurement links are retained.</p><ul className="list-disc space-y-2 pl-5 text-sm">{record.lines.map(l=><li key={l.id}>{l.item_snapshot.description||"Incomplete item"} · {l.quantity||"Quantity pending"} {l.item_snapshot.uom}</li>)}</ul>{error&&<p role="alert" className="text-red-700">{error}</p>}<ValidatedForm id="publish-purchase-items" onSubmit={async event=>{event.preventDefault();setBusy(true);setError("");try{const r=await fetch(`${base}/${record.id}/${command}`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({version:record.version,reason:reason.trim()})});const value=await r.json();if(!r.ok){setError(typeof value.detail==="string"?value.detail:"This operation could not be completed. Refresh the saved items before retrying.");return;}onSaved(value);}catch{setError("The operation could not be confirmed. Refresh saved items before retrying.")}finally{setBusy(false)}}}><ValidatedLabel className="block text-sm font-semibold">Reason<ValidatedTextarea name="reason" required maxLength={2000} value={reason} disabled={busy} onChange={e=>setReason(e.target.value)} className="mt-2 block w-full rounded-lg border border-slate-300 bg-white p-3"/></ValidatedLabel></ValidatedForm></RecordDrawer>;
}

function RegisterAction({base,records,command,onClose,onSaved}:{base:string;records:ItemPreparation[];command:"complete"|"reopen";onClose:()=>void;onSaved:(value:ItemListing)=>void}){
 const[comment,setComment]=useState(""),[busy,setBusy]=useState(false),[error,setError]=useState("");
 return <RecordDrawer title={command==="complete"?"Finalise list":"Reopen list"} onClose={()=>{if(!busy)onClose()}} footer={<div className="flex justify-end gap-3"><button type="button" className={button} disabled={busy} onClick={onClose}>Cancel</button><button type="submit" form="purchase-register-action" className={`${button}`} disabled={busy}>{busy?"Saving…":command==="complete"?"Finalise list":"Reopen list"}</button></div>}><p className="text-sm">{command==="complete"?"Finalise all saved items and publish the material requirements. The list becomes read-only until reopened.":"Reopen the finalised items to add or change requirements. Earlier comments, published identities and procurement history remain recorded."}</p><p className="text-sm font-semibold">{records.reduce((n,r)=>n+r.lines.length,0)} saved items</p>{error&&<p role="alert" className="text-red-700">{error}</p>}<ValidatedForm id="purchase-register-action" onSubmit={async e=>{e.preventDefault();if(busy)return;setBusy(true);setError("");try{const response=await fetch(`${base}/register-${command}`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({records:records.map(r=>({id:r.id,version:r.version})),reason:comment.trim()})});const value=await response.json();if(!response.ok){setError(response.status===409?"The list changed. Close and refresh before retrying; nothing was finalised or reopened.":typeof value.detail==="string"?value.detail:"The list could not be changed. Check the saved items and comment.");return;}onSaved(value)}catch{setError("The result could not be confirmed. Close and refresh the saved list before retrying.")}finally{setBusy(false)}}}><ValidatedLabel className="block text-sm font-semibold">Comment<ValidatedTextarea name="reason" required maxLength={2000} rows={4} value={comment} disabled={busy} onChange={e=>setComment(e.target.value)} className="mt-2 block w-full rounded-lg border border-slate-300 bg-white p-3"/></ValidatedLabel></ValidatedForm></RecordDrawer>;
}
