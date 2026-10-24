"use client";
import {QuotationWorkTasks,type QuotationTask} from "./quotation-work-tasks";
import {useRouter} from "next/navigation";
import {useCallback,useEffect,useState} from "react";
import dynamic from "next/dynamic";
import {DataGrid,GridAction,type GridQuery} from "@/components/ui/data-grid";
import {ValidatedLabel,ValidatedSelect} from "@/components/ui/validated-controls";
import {RecordDrawer} from "@/components/ui/record-drawer";
const TaskWork=dynamic(()=>import("./task-work").then(m=>m.TaskWork),{loading:()=> <p role="status">Loading assigned work…</p>});
export type PersonalTask={id:string;project_id:string;project_name:string;package_id:string;package_code:string;step_number:number;title:string;phase:string;relation:string;coordination_state:string;planned_date:string;resolution_due:string;block_reason:string;available:boolean;blockers:string[];package_version:number;is_overdue:boolean};
type SupplierReviewTask={id:string;document_id:string;title:string;kind:string;outcome:string;completed:boolean;supplier_id:string;code:string;name:string;state:string;message:string;link:string};
type Page={supplier_review_tasks?:SupplierReviewTask[];quotation_tasks?:QuotationTask[];rows:PersonalTask[];total:number;page:number;page_size:number};
export function PersonalTasks(){
 const router=useRouter();
 const [query,setQuery]=useState<GridQuery>({search:"",status:"",sort:"",direction:"asc",page:1,size:20});
 const [tab,setTab]=useState<"pending"|"completed">("pending");
 const [role,setRole]=useState("all"),[state,setState]=useState("all"),[data,setData]=useState<Page|null>(null),[error,setError]=useState(""),[loading,setLoading]=useState(true),[revision,setRevision]=useState(0),[selected,setSelected]=useState<PersonalTask|null>(null);
 const changed=useCallback((q:GridQuery)=>{setLoading(true);setQuery(q);},[]);
 useEffect(()=>{const abort=new AbortController();const p=new URLSearchParams({page:String(query.page),page_size:String(query.size),search:query.search,role,state});
 fetch(`/api/my-tasks?${p}`,{signal:abort.signal}).then(async r=>{const v=await r.json();if(!r.ok)throw new Error(typeof v.detail==="string"?v.detail:"Cannot load personal tasks");return v as Page;}).then(v=>{setData(v);setError("");}).catch(e=>{if(e.name!=="AbortError"){setError(e.message);setData(null);}}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});return()=>abort.abort();},[query,role,state,revision]);
 function refresh(){setLoading(true);setRevision(v=>v+1);}
 const filterClass="mt-1 block min-h-11 rounded-lg border border-slate-200 bg-white px-3 text-sm";
 return <section className="space-y-5"><div className="flex flex-wrap items-center justify-between gap-3"><div><p className="go-eyebrow">Personal workspace</p><h1 className="text-2xl font-bold">My tasks</h1><p className="text-sm text-slate-600">Your assigned supplier reviews and purchasing work across projects.</p></div><button type="button" className="min-h-11 rounded-xl border border-slate-200 bg-white px-4" onClick={refresh}>Refresh tasks</button></div>
 <nav aria-label="Task status" className="flex gap-2 border-b border-slate-200 pb-2">{(["pending","completed"] as const).map(value=><button type="button" key={value} aria-current={tab===value?"page":undefined} className={`min-h-11 rounded-lg px-4 font-bold ${tab===value?"bg-blue-700 text-white":"text-slate-600"}`} onClick={()=>setTab(value)}>{value==="pending"?"Pending":"Completed"}</button>)}</nav>
 {error&&<div role="alert" className="rounded-xl border border-red-200 bg-red-50 p-4">{error} <button type="button" className="underline" onClick={refresh}>Retry</button></div>}
 {tab==="pending"&&<QuotationWorkTasks rows={data?.quotation_tasks??[]}/>}
 <section aria-label="Supplier document tasks" className="space-y-3"><h2 className="text-lg font-bold">Supplier document tasks</h2><DataGrid key={tab} label="Supplier document tasks" loading={loading} rows={(data?.supplier_review_tasks??[]).filter(r=>r.completed===(tab==="completed"))} columns={[{key:"supplier",label:"Supplier",value:r=>`${r.code} · ${r.name}`},{key:"document",label:"Document",value:r=>r.title},{key:"task",label:"Task",value:r=>r.kind==="correction"?"Replace document":"Review document"},{key:"status",label:"Status",value:r=>r.completed?"Completed":"Pending",render:r=><span className={r.completed?"font-semibold text-green-700":"text-slate-600"}>{r.completed?"Completed":"Pending"}</span>},{key:"result",label:"Outcome",value:r=>r.completed?r.outcome:"—"}]} actions={r=><GridAction kind="view" label={`Open document task ${r.title}`} onClick={()=>router.push(r.link)}/>}/></section>
 {tab==="pending"&&<>
 <DataGrid key={`${role}:${state}`} label="Purchasing task assignments" rows={data?.rows??[]} loading={loading} remote={{total:data?.total??0,onChange:changed}} toolbar={<><ValidatedLabel className="text-xs font-semibold text-slate-600">Assignment role<ValidatedSelect aria-label="Assignment role" className={filterClass} value={role} onChange={e=>{setLoading(true);setRole(e.target.value);}}><option value="all">All assignments</option><option value="doer">Doer</option><option value="resolver">Issue resolver</option></ValidatedSelect></ValidatedLabel><ValidatedLabel className="text-xs font-semibold text-slate-600">Coordination state<ValidatedSelect aria-label="Coordination state" className={filterClass} value={state} onChange={e=>{setLoading(true);setState(e.target.value);}}><option value="all">All states</option><option value="not_started">Not started</option><option value="preparing">Preparing</option><option value="blocked">Blocked</option></ValidatedSelect></ValidatedLabel></>} columns={[
 {key:"project",label:"Project",value:r=>r.project_name,sortable:false},
 {key:"task",label:"Assigned task",value:r=>r.title,sortable:false,render:r=><div><strong>{r.step_number}. {r.title}</strong><p className="text-xs text-slate-500">{r.package_code} · {r.phase}</p></div>},
 {key:"role",label:"Your role",value:r=>r.relation==="resolver"?"Issue resolver":"Doer",sortable:false},
 {key:"state",label:"Coordination",value:r=>r.coordination_state,sortable:false,render:r=><div><span className="capitalize">{r.coordination_state.replaceAll("_"," ")}</span>{r.block_reason&&<p className="text-xs text-amber-700">{r.block_reason}</p>}{!r.available&&<p className="text-xs text-amber-700">Source blocked — open for details</p>}</div>},
 {key:"due",label:"Due date",value:r=>(r.relation==="resolver"?r.resolution_due:r.planned_date)||"—",sortable:false,render:r=><span>{(r.relation==="resolver"?r.resolution_due:r.planned_date)||"—"}{r.is_overdue&&<strong className="ml-2 text-red-700">Overdue</strong>}</span>},
 ]} actions={r=><GridAction kind="view" label={`Open task ${r.title}`} onClick={()=>setSelected(r)}/>}/>
 </>}
 {selected&&<RecordDrawer title={`${selected.project_name} · ${selected.title}`} wide extraWide onClose={()=>{setSelected(null);refresh();}}><TaskWork task={selected}/></RecordDrawer>}
 </section>;
}
