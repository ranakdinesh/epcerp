"use client";
import "./purchase-layout.css";
import {useSearchParams,useRouter} from "next/navigation";
import {PurchaseProjectHeader} from "./purchase-layout";
import {QuotationExecution} from "./quotation-workspace";
import {VendorAwards} from "./vendor-awards";
import {PurchasePackages} from "./work-packages";
import {PurchaseItems} from "./purchase-items";
import {IndentExecution} from "./formal-indents";
import {useEffect,useState} from "react";
import Link from "next/link";
import {ProcessMap} from "@/components/processes/process-map";
import {purchaseProcess} from "@/components/processes/process-directory";
import {read,type Context} from "./data";
export function PurchaseWorkspace({project,map,allowedScreens}:{project:string;map:boolean;allowedScreens:readonly string[]}){
 const params=useSearchParams(),router=useRouter();
 const requestedStep=params.get("step");
 const [data,setData]=useState<Context|null>(null),[error,setError]=useState(""),[reload,setReload]=useState(0);
 useEffect(()=>{const controller=new AbortController();read<Context>(`/api/purchase-fms/projects/${encodeURIComponent(project)}`,controller.signal).then(d=>{if(controller.signal.aborted)return;setData(d);setError("")}).catch(e=>{if(!controller.signal.aborted){setError(e.message);setData(null)}});return()=>controller.abort()},[project,reload]);
 useEffect(()=>{const refresh=()=>setReload(v=>v+1);const timer=window.setInterval(()=>{if(document.visibilityState==="visible")refresh()},30000);window.addEventListener("focus",refresh);window.addEventListener("purchase-progress-saved",refresh);return()=>{window.clearInterval(timer);window.removeEventListener("focus",refresh);window.removeEventListener("purchase-progress-saved",refresh)}},[]);
 useEffect(()=>{if(!data||map||params.get("work")||(requestedStep&&requestedStep!=="workspace"))return;const next=new URLSearchParams(params.toString());next.set("step",data.project.next_step??"PUR-01");router.replace(`/dashboard?${next.toString()}`)},[data,map,params,requestedStep,router]);
 if(error)return <div role="alert"><p>{error}</p><button className="min-h-11 underline" onClick={()=>setReload(v=>v+1)}>Retry</button><Link className="ml-4" href="/dashboard?screen=purchase-fms">Back to purchase dashboard</Link></div>;
 if(!data)return <p role="status">Loading project purchasing access…</p>;
 const p=data.project;
 const step=!requestedStep||requestedStep==="workspace"?(data?.project.next_step??"PUR-01"):requestedStep;

 return <section className="purchase-project-workspace space-y-4">
 <PurchaseProjectHeader project={p} status={p.returned_groups?`${p.returned_groups} group(s) returned by Finance`:p.pending_approval_groups?`${p.pending_approval_groups} group(s) waiting for Finance approval`:p.completed_steps?.includes(4)?`Next: ${purchaseProcess.steps.find(s=>s.code===p.next_step)?.label??"Vendor award & LOI"}`:p.completed_steps?.includes(3)?"Quotations received · Next: Price Comparative Table":p.completed_steps?.includes(2)?"Purchase Indent complete · Next: Receive vendor quotations":p.completed_steps?.includes(1)?"Items to purchase complete · Next: Purchase Indent":"Items to purchase pending"}/> 
 {!p.is_active||["closed","on_hold"].includes(p.lifecycle_status)?<p role="status">This project is inactive, closed or on hold.</p>:null}
 {(!map&&(params.get("work")==="technical"||["PUR-06","PUR-07","PUR-08"].includes(step??""))&&!p.completed_steps?.includes(4))?<p role="status" className="rounded-xl bg-amber-50 p-5">Finance must clear every purchase group before technical documents can begin. <Link className="underline" href={`/dashboard?screen=purchase-fms&project=${project}&step=PUR-04`}>Review quotation status</Link></p>:step==="PUR-20"?<PurchasePackages key={`${project}:accounts-acceptance`} project={project} accounts/>:step==="PUR-19"?<PurchasePackages key={`${project}:engineer-confirmation`} project={project} engineer/>:step==="PUR-18"?<PurchasePackages key={`${project}:grn`} project={project} grn/>:step==="PUR-17"?<PurchasePackages key={`${project}:site-receipt`} project={project} receipt/>:step==="PUR-16"?<PurchasePackages key={`${project}:dispatch`} project={project} dispatch/>:step==="PUR-15"?<PurchasePackages key={`${project}:client-clearance`} project={project} clientClearance/>:step==="PUR-14"?<PurchasePackages key={`${project}:quality`} project={project} quality/>:step==="PUR-13"?<PurchasePackages key={`${project}:inspection-report`} project={project} inspectionReport/>:step==="PUR-12"?<PurchasePackages key={`${project}:inspection`} project={project} inspection/>:step==="PUR-11"?<PurchasePackages key={`${project}:advances`} project={project} advances/>:step==="PUR-10"?<PurchasePackages key={`${project}:acceptance`} project={project} acceptance/>:step==="PUR-PO-CLIENT"?<PurchasePackages key={`${project}:client-po`} project={project} clientPO/>:step==="PUR-09"?<PurchasePackages key={`${project}:orders`} project={project} orders/>:(params.get("work")==="technical"||["PUR-06","PUR-07","PUR-08"].includes(step??""))?<PurchasePackages key={`${project}:technical:${step}`} project={project} technicalStage={step==="PUR-07"?7:step==="PUR-08"?8:6}/>:params.get("work")==="sourcing"||step==="PUR-05"?<VendorAwards key={`${project}:awards`} project={project}/>:params.get("work")==="packages"?<PurchasePackages key={project} project={project}/>:map?<ProcessMap process={purchaseProcess} projectId={project} allowedScreens={allowedScreens}/>: (step==="PUR-03"||step==="PUR-04")?<QuotationExecution key={`${project}:${step}`} project={project} step={step==="PUR-04"?4:3}/>: step==="PUR-02"?<IndentExecution key={project} project={project} projectName={p.name}/>:step==="PUR-01"?<PurchaseItems key={project} project={project} projectName={p.name}/>:<PurchaseItems key={project} project={project} projectName={p.name}/>}

 </section>;
}
