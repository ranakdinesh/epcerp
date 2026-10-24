"use client";
import {useEffect,useState} from "react";
import {useRouter,useSearchParams} from "next/navigation";
import {GroupQuotationWorkspace} from "@/components/purchasing/group-quotation-workspace";
import {ValidatedLabel,ValidatedSelect} from "@/components/ui/validated-controls";
type BOChoice={id:string;name:string;state:string};
export function QuotationExecution({project,step=3}:{project:string;step?:3|4}){
 const params=useSearchParams(),router=useRouter(),requested=params.get("bo");
 const[choices,setChoices]=useState<BOChoice[]|null>(null),[error,setError]=useState(""),[reload,setReload]=useState(0);
 useEffect(()=>{const c=new AbortController();fetch(`/api/projects/${encodeURIComponent(project)}/quotation-bo?section=${step===4?"comparison":"receipt"}`,{cache:"no-store",signal:c.signal}).then(async r=>{const data=await r.json();if(!r.ok)throw Error(typeof data.detail==="string"?data.detail:"Unable to load project BO lists");if(!c.signal.aborted)setChoices(data)}).catch(e=>{if(!c.signal.aborted)setError(e.message)});return()=>c.abort()},[project,reload,step]);
 const selected=choices?.find(b=>b.id===requested)??(!requested?choices?.[0]:undefined);
 function navigate(step:number,bo=selected?.id){const next=new URLSearchParams(params.toString());next.set("screen","purchase-fms");next.set("project",project);next.set("step",`PUR-${String(step).padStart(2,"0")}`);if(bo)next.set("bo",bo);else next.delete("bo");router.push(`/dashboard?${next}`)}
 if(error)return <div><p role="alert">{error}</p><button className="min-h-11 rounded-lg border px-4" onClick={()=>{setError("");setChoices(null);setReload(v=>v+1)}}>Retry</button></div>;
 if(!choices)return <p role="status">Loading project BO lists…</p>;
 return <section className="go-purchase-content space-y-5">{step===4&&choices.length>1&&<ValidatedLabel>BO list<ValidatedSelect className="mt-2 block min-h-11 w-full rounded-lg border bg-white px-3" value={selected?.id??""} onChange={e=>navigate(step,e.target.value)}><option value="" disabled>Choose a saved BO list</option>{choices.map(b=><option key={b.id} value={b.id}>{b.name||b.id} · {b.state}</option>)}</ValidatedSelect></ValidatedLabel>}{requested&&!selected?<p role="alert">This BO list is unavailable in the selected project.</p>:!selected?<p>No saved BO lists are available. Prepare BO items and dispatch vendor indents before recording quotations.</p>:<GroupQuotationWorkspace key={`${project}:${selected.id}`} project={project} boId={selected.id} initialStep={step} onNavigateStep={step=>navigate(step)}/>}</section>
}
