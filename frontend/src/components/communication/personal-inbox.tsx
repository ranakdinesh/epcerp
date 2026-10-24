"use client";
import {useCallback,useEffect,useState} from "react";
import {DataGrid,GridAction,type GridQuery} from "@/components/ui/data-grid";
import {ValidatedLabel,ValidatedSelect} from "@/components/ui/validated-controls";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {UserIdentity} from "@/components/personal-profile";
type Notice={notice_kind?:string;title?:string;link?:string;id:string;source_event_id:string;recipient_id:string;director_designation_id:string;director_version:number;decision_id:string;project_id:string;source_version:number;decision_action:string;version:number;read_at:string|null;acknowledged_at:string|null;created_at:string;updated_at:string;created_by_id:string|null;updated_by_id:string|null};
type Event={id:string;action:string;before:Notice|null;after:Notice;created_at:string;created_by_id:string};
type Workspace={notice:Notice;history:Event[];can_write:boolean};
type Listing={items:Notice[];total:number;page:number;size:number;can_write:boolean};
const button="min-h-11 rounded-xl border border-slate-300 bg-white px-4 py-2 text-sm font-semibold disabled:opacity-50";
const primary="min-h-11 rounded-xl bg-blue-700 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50";
function message(v:unknown,fallback:string){if(v&&typeof v==="object"&&"detail"in v){if(typeof v.detail==="string")return v.detail;if(Array.isArray(v.detail))return v.detail.map(i=>typeof i?.msg==="string"?i.msg:fallback).join("; ")}return fallback}
async function load<T>(path:string,signal?:AbortSignal):Promise<T>{const r=await fetch(path,{cache:"no-store",signal}),v:unknown=await r.json();if(!r.ok)throw Error(message(v,"Unable to load personal inbox"));return v as T}
function status(n:Notice){return n.acknowledged_at?"Acknowledged":n.read_at?"Read":"Unread"}
function date(v:string|null){return v?new Date(v).toLocaleString():"Pending"}
export function PersonalInbox(){
 const [listing,setListing]=useState<Listing|null>(null),[error,setError]=useState(""),[reload,setReload]=useState(0),[selected,setSelected]=useState<string|null>(null),[filter,setFilter]=useState(""),[paging,setPaging]=useState({page:1,size:20});
 const pageChanged=useCallback((q:GridQuery)=>{setListing(null);setPaging(p=>p.page===q.page&&p.size===q.size?p:{page:q.page,size:q.size})},[]);
 useEffect(()=>{const c=new AbortController();load<Listing>(`/api/inbox?page=${paging.page}&size=${paging.size}&status=${filter}`,c.signal).then(v=>{setListing(v);setError("")}).catch(e=>{if(!c.signal.aborted)setError(e instanceof Error?e.message:"Unable to load inbox")});return()=>c.abort()},[paging,filter,reload]);
 function refresh(){setListing(null);setError("");setReload(v=>v+1)}
 return <section className="min-w-0 space-y-5"><div className="flex flex-wrap items-start justify-between gap-3"><div><p className="go-eyebrow">Communication</p><h1 className="text-2xl font-bold">My inbox</h1><p className="mt-2 text-sm text-slate-600">Your retained assignment and financial decision notices. Reading or acknowledging a notice does not approve or complete its source work.</p></div><button className={button} onClick={refresh}>Refresh inbox</button></div>
 {error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p>}
 <DataGrid key={filter} label="Personal financial notices" rows={listing?.items ?? []} loading={!listing&&!error} searchable={false} remote={{total:listing?.total ?? 0,onChange:pageChanged}} columns={[{key:"action",label:"Notice",value:n=>n.title||n.decision_action,sortable:false,render:n=><span>{n.title||n.decision_action}</span>},{key:"status",label:"Receipt",value:status,sortable:false},{key:"project",label:"Project reference",value:n=>n.project_id,sortable:false},{key:"created",label:"Received",value:n=>date(n.created_at),sortable:false}]} toolbar={<ValidatedLabel className="text-sm font-semibold">Receipt status<ValidatedSelect aria-label="Receipt status" className="mt-1 block min-h-11 rounded-lg border border-slate-300 bg-white px-3 py-2 font-normal" value={filter} onChange={e=>{setFilter(e.target.value);setPaging({page:1,size:20});setListing(null)}}><option value="">All notices</option><option value="unread">Unread</option><option value="read">Read</option><option value="acknowledged">Acknowledged</option></ValidatedSelect></ValidatedLabel>} actions={n=><GridAction label="View notice" onClick={()=>setSelected(n.id)}/>}/>
 {selected&&<NoticePanel key={selected} id={selected} close={()=>setSelected(null)} onSaved={refresh}/>}
 </section>;
}
function NoticePanel({id,close,onSaved}:{id:string;close:()=>void;onSaved:()=>void}){
 const [ws,setWs]=useState<Workspace|null>(null),[error,setError]=useState(""),[notice,setNotice]=useState(""),[reload,setReload]=useState(0),[busy,setBusy]=useState(false);
 useEffect(()=>{const c=new AbortController();load<Workspace>(`/api/inbox/${id}`,c.signal).then(v=>{setWs(v);setError("")}).catch(e=>{if(!c.signal.aborted)setError(e instanceof Error?e.message:"Unable to load notice")});return()=>c.abort()},[id,reload]);
 async function receipt(action:"read"|"acknowledge"){
  if(!ws)return;setBusy(true);setError("");setNotice("");
  try{const r=await fetch(`/api/inbox/${id}/${action}`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({version:ws.notice.version})}),v=await r.json();if(!r.ok){setError(message(v,"Unable to record receipt"));return}setWs(v);setNotice(action==="read"?"Notice marked read.":"Notice acknowledged.");onSaved()}
  catch{setError("Unable to reach the inbox. Reload the notice to check whether the receipt committed before trying again.")}
  finally{setBusy(false)}
 }
 const n=ws?.notice;
 return <RecordDrawer title="View notice" onClose={()=>{if(!busy)close()}} footer={<div className="flex flex-wrap justify-end gap-2"><button className={button} disabled={busy} onClick={close}>Close</button>{ws?.can_write&&<><button className={button} disabled={busy||!!n?.read_at} onClick={()=>receipt("read")}>Mark read</button><button className={primary} disabled={busy||!!n?.acknowledged_at} onClick={()=>receipt("acknowledge")}>{busy?"Saving…":"Acknowledge"}</button></>}</div>}>
 {error&&<div className="space-y-2"><p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p><button className={button} disabled={busy} onClick={()=>{setWs(null);setError("");setReload(v=>v+1)}}>Reload notice</button></div>}{notice&&<p role="status" className="rounded-lg bg-green-50 p-3 text-green-800">{notice}</p>}
 {!n&&!error&&<p role="status">Loading notice…</p>}
 {n&&<><p className="rounded-lg bg-blue-50 p-3 text-sm text-blue-900">{n.title||"Mandatory Director notice. Commercial details remain in the independently authorized source record."}</p>{n.link&&<a className="inline-flex min-h-11 items-center rounded-lg border px-4" href={n.link}>{n.notice_kind==="supplier_review"?"Open supplier review":"Open quotation work"}</a>}<dl className="grid min-w-0 grid-cols-1 gap-4 text-sm sm:grid-cols-2">{[["Decision",n.decision_action],["Receipt",status(n)],["Received",date(n.created_at)],["Read",date(n.read_at)],["Acknowledged",date(n.acknowledged_at)],["Receipt version",n.version],["Project reference",n.project_id],["Decision reference",n.decision_id],["Source version",n.source_version],["Director designation version",n.director_version]].map(([label,value])=><div key={label} className="min-w-0"><dt className="font-semibold">{label}</dt><dd className="mt-1 break-words">{value}</dd></div>)}</dl>
 <details className="rounded-xl border bg-white p-4"><summary className="cursor-pointer font-semibold">Receipt history ({ws?.history.length ?? 0})</summary>{ws?.history.map(e=><article className="mt-3 border-t pt-3 text-sm" key={e.id}><div className="flex flex-wrap items-center gap-2"><UserIdentity id={e.created_by_id} name={e.created_by_id}/><span>{date(e.created_at)} · {e.action}</span></div><details className="mt-2"><summary>View before and after</summary><div className="grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2">{[["Before",e.before],["After",e.after]].map(([label,value])=><div key={String(label)} className="min-w-0"><h3 className="font-semibold">{String(label)}</h3><pre className="max-h-80 overflow-auto rounded border p-2 text-xs">{JSON.stringify(value,null,2)}</pre></div>)}</div></details></article>)}</details></>}
 </RecordDrawer>;
}
