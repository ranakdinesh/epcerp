"use client";
import {useEffect,useState} from "react";
import {useRouter,useSearchParams} from "next/navigation";
import {PurchaseIndents,type IndentOverview} from "@/components/purchasing/purchase-indents";
import type {Overview,RecordRow,IndentData} from "@/components/purchasing/purchase-data";
import {useProjectUser} from "@/components/projects/project-permissions";
import {canOpen,indentTabs} from "@/components/projects/workspace-permissions";
import {IndentDispatchNotifications} from "@/components/purchasing/indent-dispatch-notifications";
import {BOExecution} from "./bo-workspace";
type Workspace=Pick<Overview,"lines"|"members"|"permissions">&{currency:string;rows:RecordRow<IndentData>[]};
export function IndentExecution({project,projectName}:{project:string;projectName:string}){
 const params=useSearchParams(),router=useRouter(),user=useProjectUser();
 const allowed=canOpen(user.permissions,"projects.purchase_indents");
 const groupsAllowed=indentTabs(user.permissions).length>0;
 const formal=params.get("indentMode")==="formal"||allowed&&!groupsAllowed;
 function choose(mode:string){const next=new URLSearchParams(params.toString());if(mode==="formal")next.set("indentMode",mode);else next.delete("indentMode");router.push(`/dashboard?${next}`)}
 return <section className="space-y-3"><div className="purchase-indent-switcher"><div role="tablist" aria-label="Purchase indent views">{groupsAllowed&&<button type="button" role="tab" aria-selected={!formal} className="min-h-11 rounded-lg border px-4" onClick={()=>choose("groups")}>Groups & vendors</button>}{allowed&&<button type="button" role="tab" aria-selected={formal} className="min-h-11 rounded-lg border px-4" onClick={()=>choose("formal")}>Formal indents</button>}</div><IndentDispatchNotifications/></div>{formal?allowed?<FormalIndents key={project} project={project}/>:<p role="alert">You do not have access to formal purchase indents.</p>:<BOExecution key={project} project={project} projectName={projectName} step="PUR-02"/>}</section>
}
function FormalIndents({project}:{project:string}){
 const[data,setData]=useState<Workspace|null>(null),[error,setError]=useState(""),[reload,setReload]=useState(0);
 useEffect(()=>{const c=new AbortController();fetch(`/api/projects/${encodeURIComponent(project)}/purchase-indents/workspace`,{cache:"no-store",signal:c.signal}).then(async r=>{const body=await r.json();if(!r.ok)throw Error(typeof body.detail==="string"?body.detail:"Unable to load formal purchase indents");if(!c.signal.aborted){setData(body);setError("")}}).catch(e=>{if(!c.signal.aborted)setError(e instanceof Error?e.message:"Unable to load formal purchase indents")});return()=>c.abort()},[project,reload]);
 function refresh(){setData(null);setError("");setReload(v=>v+1)}
 if(error)return <div><p role="alert">{error}</p><button type="button" className="min-h-11 rounded-lg border px-4" onClick={refresh}>Retry indents</button></div>;
 if(!data)return <p role="status">Loading formal purchase indents…</p>;
 const overview:IndentOverview={lines:data.lines,members:data.members,permissions:data.permissions};
 return <div className="space-y-5"><PurchaseIndents project={project} overview={overview} rows={data.rows} currency={data.currency} reload={refresh}/></div>
}
