"use client";
import {useState} from "react";
import dynamic from "next/dynamic";
import type {PersonalTask} from "./personal-tasks";
const loading=()=> <p role="status">Loading task workspace…</p>;
const Coordination=dynamic(()=>import("@/components/purchase-fms/work-packages").then(m=>m.PurchasePackageDetail),{loading});
const Technical=dynamic(()=>import("@/components/purchase-fms/technical-workspace").then(m=>m.TechnicalWorkspace),{loading});
const Sourcing=dynamic(()=>import("@/components/purchase-fms/sourcing-workspace").then(m=>m.SourcingWorkspace),{loading});
const PurchaseOrder=dynamic(()=>import("@/components/purchase-fms/purchase-order-workspace").then(m=>m.PurchaseOrderWorkspace),{loading});
const Acceptance=dynamic(()=>import("@/components/purchase-fms/acceptance-workspace").then(m=>m.AcceptanceWorkspace),{loading});
const Advance=dynamic(()=>import("@/components/purchase-fms/advance-workspace").then(m=>m.AdvanceWorkspace),{loading});
const Inspection=dynamic(()=>import("@/components/purchase-fms/inspection-workspace").then(m=>m.InspectionWorkspace),{loading});
const InspectionReport=dynamic(()=>import("@/components/purchase-fms/inspection-report-workspace").then(m=>m.InspectionReportWorkspace),{loading});
const Quality=dynamic(()=>import("@/components/purchase-fms/quality-workspace").then(m=>m.QualityWorkspace),{loading});
const ClientClearance=dynamic(()=>import("@/components/purchase-fms/client-clearance-workspace").then(m=>m.ClientClearanceWorkspace),{loading});
const Dispatch=dynamic(()=>import("@/components/purchase-fms/dispatch-workspace").then(m=>m.DispatchWorkspace),{loading});
const Receipt=dynamic(()=>import("@/components/purchase-fms/receipt-workspace").then(m=>m.ReceiptWorkspace),{loading});
const GRN=dynamic(()=>import("@/components/purchase-fms/grn-workspace").then(m=>m.GRNWorkspace),{loading});
const Engineer=dynamic(()=>import("@/components/purchase-fms/engineer-workspace").then(m=>m.EngineerWorkspace),{loading});
const Accounts=dynamic(()=>import("@/components/purchase-fms/accounts-workspace").then(m=>m.AccountsWorkspace),{loading});
export function TaskWork({task}:{task:PersonalTask}){
 const [mode,setMode]=useState("coordination");const props={project:task.project_id,packageId:task.package_id};
 const specialist=task.step_number===5 || task.step_number===9 || task.step_number===10 || task.step_number===11 || task.step_number===12 || task.step_number===13 || task.step_number===14 || task.step_number===15 || task.step_number===16 || task.step_number===17 || task.step_number===18 || task.step_number===19 || task.step_number===20 || task.step_number===6 || task.step_number===7 || task.step_number===8;
 return <div className="space-y-4">{task.blockers.length>0&&<div role="status" className="rounded-xl border border-amber-200 bg-amber-50 p-3">{task.blockers.map((b,i)=><p key={i}>{b}</p>)}</div>}
 <div className="flex flex-wrap gap-2" role="group" aria-label="Task workspace"><button type="button" aria-pressed={mode==="coordination"} className="min-h-11 rounded-lg border border-slate-200 px-4" onClick={()=>setMode("coordination")}>Coordination & blockers</button>{specialist&&<button type="button" aria-pressed={mode==="evidence"} className="min-h-11 rounded-lg border border-slate-200 px-4" onClick={()=>setMode("evidence")}>Step evidence & decisions</button>}</div>
 {mode==="coordination"?<Coordination project={task.project_id} id={task.package_id}/>:task.step_number===5?<Sourcing {...props}/>:task.step_number===9?<PurchaseOrder {...props}/>:task.step_number===10?<Acceptance {...props}/>:task.step_number===11?<Advance {...props}/>:task.step_number===12?<Inspection {...props}/>:task.step_number===13?<InspectionReport {...props}/>:task.step_number===14?<Quality {...props}/>:task.step_number===15?<ClientClearance {...props}/>:task.step_number===16?<Dispatch {...props}/>:task.step_number===17?<Receipt {...props}/>:task.step_number===18?<GRN {...props}/>:task.step_number===19?<Engineer {...props}/>:task.step_number===20?<Accounts {...props}/>:[6,7,8].includes(task.step_number)?<Technical {...props} stage={task.step_number}/>:null}
 </div>;
}
