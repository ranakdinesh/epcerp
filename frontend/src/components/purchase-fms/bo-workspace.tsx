"use client";
import {PurchaseSectionHeader} from "./purchase-layout";
import {useEffect,useState} from "react";
import {useRouter,useSearchParams} from "next/navigation";
import Link from "next/link";
import {useAuthenticatedUser} from "@/components/auth-user-provider";
import {BOLineGrid} from "@/components/purchasing/bo-line-grid";
import {PurchaseIndentWorkspace} from "@/components/purchasing/purchase-indent-workspace";
import type {Item,Line} from "@/components/purchasing/purchase-bo-workspace";
import {DataGrid} from "@/components/ui/data-grid";
import {ValidatedInput,ValidatedLabel,ValidatedSelect,ValidatedTextarea} from "@/components/ui/validated-controls";
import {purchaseStepAllowed} from "@/components/projects/workspace-permissions";
import {itemRows,purchaseItemType,type ItemListMode} from "./purchase-items-model";
import {createRecordId} from "@/lib/record-id";
type BO={id:string;name?:string;preparer_id:string;version:number;state:string;lines:Line[];completed_at:string|null;completed_by_id:string|null};
type Listing={item_list_mode:ItemListMode;merged_lists?:Record<string,string>;lists:BO[];can_write:boolean;can_complete:boolean;can_manage_lists:boolean};
const button="min-h-11 rounded-xl border border-slate-300 px-4 py-2 text-sm font-semibold disabled:opacity-50";
const primary=`primary ${button} border-blue-700 bg-blue-700 text-white`;
function blank(user:string):BO{return{id:createRecordId(),preparer_id:user,version:0,state:"draft",lines:[{id:createRecordId(),item_id:null,quantity:null,required_date:null,reference:"",notes:"",item_snapshot:{},demand_id:null}],completed_at:null,completed_by_id:null}}
async function read<T>(url:string,signal?:AbortSignal,body?:unknown):Promise<T>{const response=await fetch(url,{cache:"no-store",signal,...(body?{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)}:{})});const data=await response.json();if(!response.ok)throw Error(typeof data.detail==="string"?data.detail:Array.isArray(data.detail)?data.detail.map((e:{msg:string})=>e.msg).join("; "):"Check your input and retry. Your draft is retained.");return data}
export function BOExecution({project,projectName,step}:{project:string;projectName:string;step:string}){
 const user=useAuthenticatedUser(),params=useSearchParams(),router=useRouter(),requested=params.get("bo");
 const [listing,setListing]=useState<Listing|null>(null),[items,setItems]=useState<Item[]>([]),[bo,setBO]=useState<BO|null>(null),[dirty,setDirty]=useState(false),[busy,setBusy]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState(""),[reason,setReason]=useState(""),[reload,setReload]=useState(0);
 const visible=purchaseStepAllowed(user.permissions,step==="PUR-01"?1:2);
 const base=`/api/projects/${encodeURIComponent(project)}/purchase-bo`;
 useEffect(()=>{if(!visible)return;const c=new AbortController();Promise.all([read<Listing>(base,c.signal),step==="PUR-01"?read<Item[]>(`/api/projects/${encodeURIComponent(project)}/items/catalogue`,c.signal):Promise.resolve([] as Item[])]).then(([data,catalogue])=>{if(c.signal.aborted)return;setListing(data);setItems(catalogue);const selected=requested?data.lists.find(b=>b.id===(data.merged_lists?.[requested]??requested)):data.lists[0];if(requested&&!selected){setError("This BO list is unavailable in the selected project.");setBO(null);return}setBO(selected??(data.can_write?blank(user.id):null));setDirty(false);setError("")}).catch(e=>{if(!c.signal.aborted)setError(e.message)});return()=>c.abort()},[base,project,user.id,requested,reload,step,visible]);
 useEffect(()=>{if(!dirty)return;const unload=(e:BeforeUnloadEvent)=>{e.preventDefault();e.returnValue=""};const guard=(e:MouseEvent)=>{const target=e.target;if(!(target instanceof Element)||target.closest("[data-bo-editor]"))return;if(target.closest("a,button")&&!window.confirm("Leave this page and discard unsaved BO changes?")){e.preventDefault();e.stopImmediatePropagation()}};window.addEventListener("beforeunload",unload);document.addEventListener("click",guard,true);return()=>{window.removeEventListener("beforeunload",unload);document.removeEventListener("click",guard,true)}},[dirty]);
 const [itemType,setItemType]=useState("");
 const owner=bo?.preparer_id===user.id||user.permissions.includes("system:admin");const editable=!!listing?.can_write&&owner&&bo?.state==="draft";
 function choose(id:string){if(dirty&&!window.confirm("Discard unsaved changes and open another BO list?"))return;const next=new URLSearchParams(params.toString());if(id)next.set("bo",id);else next.delete("bo");router.push(`/dashboard?${next}`);setDirty(false)}
 async function save(action:"save"|"complete"|"reopen"){
  if(!bo)return;if(action!=="save"&&(dirty||bo.version===0)){setError("Save the current draft before changing its completion state.");return}if(action!=="save"&&!reason.trim()){setError("Record a reason for this completion or reopening.");return}
  setBusy(true);setError("");setNotice("");try{const body=action==="save"?{version:bo.version,name:bo.name?.trim()||`BO ${bo.id.slice(0,8)}`,reason:reason.trim()||"BO list draft saved",lines:bo.lines.filter(l=>l.item_id||l.quantity||l.required_date||l.reference.trim()||l.notes.trim()).map(({id,item_id,quantity,required_date,reference,notes})=>({id,item_id,quantity:quantity||null,required_date:required_date||null,reference,notes}))}:{version:bo.version,reason:reason.trim()};const saved=await read<BO>(`${base}/${bo.id}${action==="save"?"":`/${action}`}`,undefined,body);setBO(saved);setListing(current=>current?{...current,lists:[saved,...current.lists.filter(r=>r.id!==saved.id)]}:current);setDirty(false);setReason("");setNotice(action==="save"?"Draft saved.":action==="complete"?"BO requirements published.":"BO reopened.")}catch(e){setError(e instanceof Error?e.message:"Unable to save the BO. Your draft is retained.")}finally{setBusy(false)}
 }
 const href=(target:string)=>`/dashboard?screen=purchase-fms&project=${encodeURIComponent(project)}&step=${target}${bo?.version?`&bo=${bo.id}`:""}`;
 if(!visible)return <p role="alert">You do not have access to this purchasing step.</p>;
 if(!listing)return <div>{error?<><p role="alert">{error}</p><button className={button} onClick={()=>setReload(v=>v+1)}>Retry</button></>:<p role="status">Loading BO lists and eligible project items…</p>}</div>;
 const multiple=listing.item_list_mode==="multiple";
 const consolidated=step!=="PUR-01"&&!multiple&&listing.lists.length>1;
 if(consolidated&&!requested){
  const items=itemRows(listing.lists);
  const types=[...new Set(items.map(row=>purchaseItemType(row.line)))].sort();
  return <section className="go-purchase-content space-y-3">
   <PurchaseSectionHeader title="Purchase Indent"/>
   <DataGrid label="Project purchase items" rows={itemType?items.filter(row=>purchaseItemType(row.line)===itemType):items}
    toolbar={types.length>1?<ValidatedLabel>Item type<ValidatedSelect className="ml-2 min-h-11 rounded-lg border px-3" value={itemType} onChange={event=>setItemType(event.target.value)}><option value="">All types</option>{types.map(type=><option key={type}>{type}</option>)}</ValidatedSelect></ValidatedLabel>:null}
    columns={[
     {key:"item",label:"Item / SKU",value:row=>row.line.item_snapshot.description??"",render:row=><div><p className="font-semibold">{row.line.item_snapshot.description}</p><p className="text-xs text-slate-500">{row.line.item_snapshot.sku}</p></div>},
     {key:"quantity",label:"Quantity",value:row=>row.line.quantity??"—"},
     {key:"date",label:"Required at site",value:row=>row.line.required_date??"—"},
    ]}
    actions={row=><button type="button" className={button} onClick={()=>choose(row.preparation.id)}>Open grouping & indents</button>}/>
  </section>;
 }
 return <section className="go-purchase-content space-y-3" data-bo-editor><PurchaseSectionHeader title={step==="PUR-01"?"Bought-out item lists":"Groups & vendor documents"}>{listing.can_write&&step==="PUR-01"&&<button className={primary} disabled={busy} onClick={()=>{if(dirty&&!window.confirm("Discard unsaved changes and create a BO list?"))return;setBO(blank(user.id));setDirty(false);setError("");setNotice("")}}>Add BO list</button>}{multiple&&listing.lists.length>0&&<ValidatedLabel className="w-full sm:max-w-sm">Prepared items<ValidatedSelect className="mt-2 block min-h-11 w-full rounded-lg border bg-white px-3" value={bo?.version?bo.id:""} disabled={busy} onChange={e=>choose(e.target.value)}><option value="" disabled>Choose prepared items</option>{listing.lists.map(b=><option key={b.id} value={b.id}>{b.name||b.id} · {b.lines.length} {b.lines.length===1?"item":"items"} · {b.state}</option>)}</ValidatedSelect></ValidatedLabel>}</PurchaseSectionHeader>
 {consolidated&&requested&&<button type="button" className={button} onClick={()=>choose("")}>Back to all purchase items</button>}
 {error&&<p role="alert" className="rounded-lg border border-red-300 p-3">{error}</p>}{notice&&<p role="status" className="rounded-lg border border-green-300 p-3">{notice}</p>}
 
 {!bo?<p>No prepared purchase items are available in this project.</p>:step!=="PUR-01"?bo.version===0?<p>Add and save purchase items before grouping them and generating vendor indents. <Link className="underline" href={href("PUR-01")}>Open items to purchase</Link></p>:<PurchaseIndentWorkspace key={bo.id} project={project} projectName={projectName} lines={bo.lines} boId={bo.id} boVersion={bo.version} onBackToBO={()=>router.push(href("PUR-01"))}/>:<><div className="flex flex-wrap items-center justify-between gap-3"><span className="rounded-lg border px-3 py-2 text-sm">{bo.state==="completed"?"Completed":"Draft"} · version {bo.version}{dirty?" · Unsaved changes":""}</span></div>
 <ValidatedLabel>BO list name<ValidatedInput className="mt-2 block min-h-11 w-full rounded-lg border bg-white px-3" maxLength={128} value={bo.name??""} disabled={!editable||busy} onChange={e=>{setBO({...bo,name:e.target.value});setDirty(true)}}/></ValidatedLabel>
 <BOLineGrid key={`${bo.id}:${bo.state}`} lines={bo.lines} items={items} editable={editable} busy={busy} reserved={new Set(listing.lists.filter(b=>b.id!==bo.id).flatMap(b=>b.lines.flatMap(l=>l.item_id?[l.item_id]:[])))} onChange={lines=>{setBO({...bo,lines});setDirty(true)}}/>
 {(editable||listing.can_complete&&(owner||listing.can_manage_lists)&&bo.version>0)&&<><ValidatedLabel>Change reason<ValidatedTextarea className="mt-2 block w-full rounded-lg border bg-white p-3" maxLength={2000} value={reason} disabled={busy} onChange={e=>setReason(e.target.value)}/></ValidatedLabel><div className="flex flex-wrap justify-end gap-3">{editable&&<button className={primary} disabled={busy} onClick={()=>void save("save")}>{busy?"Saving…":"Save draft"}</button>}{listing.can_complete&&(owner||listing.can_manage_lists)&&bo.version>0&&<button className={button} disabled={busy||dirty||!reason.trim()} onClick={()=>void save(bo.state==="completed"?"reopen":"complete")}>{bo.state==="completed"?"Reopen BO":"Complete BO"}</button>}</div></>}
 {bo.version>0&&<Link className="inline-flex min-h-11 items-center text-blue-700 underline" href={href("PUR-02")}>Open purchase indent and vendor documents →</Link>}
 </>}
 </section>
}
