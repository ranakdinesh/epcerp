"use client";
import type { IndentTerms } from "./purchase-indent-terms";
import { useState } from "react";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import type { BOLine, Supplier } from "./purchase-indent-workspace";
import type { IndentGroup } from "./purchase-indent-groups";
export type VendorIndent = {id:string;group_id:string;vendor_id:string;version:number;data:{terms?:IndentTerms;scope_revision?:number;is_current?:boolean;project_name?:string;number:string;group_name:string;vendor:Supplier;lines:BOLine[];generated_at:string;generated_by_id:string;generated_by_name?:string;print_requests:{requested_at:string;actor_id:string}[];sent_date:string|null;channel?:string;dispatch_reference?:string;sent_recorded_at?:string;sent_by_id?:string}};
export type DocumentWorkspace = {documents:VendorIndent[];history:{document_id:string;action:string;created_at:string;created_by_id:string}[];manager_assigned:boolean;can_generate:boolean;can_print:boolean;can_dispatch:boolean};
export type DocumentCommand = {action:"generate"|"print"|"send";group_id?:string;vendor_id?:string;document_id?:string;version?:number;sent_date?:string;channel?:string;dispatch_reference?:string};
const button="min-h-11 rounded-lg border border-slate-300 px-4 py-2 text-sm font-semibold disabled:opacity-50";
export function VendorIndentRegister({groups,suppliers,workspace,canWrite,canGenerate,canOpenDocument,busy,notice,onAction,onOpen}: {groups:IndentGroup[];suppliers:Supplier[];workspace:DocumentWorkspace|null;canWrite:boolean;canGenerate:boolean;canOpenDocument:boolean;busy:boolean;notice:string;onAction:(command:DocumentCommand,form?:HTMLFormElement)=>Promise<boolean>;onOpen:(group:string,vendor:string,documentId?:string,opener?:HTMLElement)=>void}) {
 const [sending,setSending]=useState<VendorIndent|null>(null);
 const today=new Date().toLocaleDateString("en-CA",{timeZone:"Asia/Kolkata"});
 const rows=groups.flatMap(group=>group.vendor_ids.map(vendor_id=>({id:group.id+vendor_id,group,vendor_id,vendor:suppliers.find(v=>v.id===vendor_id),document:workspace?.documents.find(d=>d.group_id===group.id&&d.vendor_id===vendor_id&&(d.data.scope_revision??1)===(group.scope_revision??1))})));
 return <section className="no-print space-y-4">
  <h2 className="text-lg font-bold">Vendor indent register</h2>
  <p className="text-sm text-slate-600">Generate each group/vendor indent to open a popup. Review it, print or save PDF, then close to return here and continue with the next vendor. After dispatch, record the actual sent date. Print requests without dispatch dates notify the project manager and superadmins in the app.</p>
  {workspace && !workspace.manager_assigned && <p role="status" className="rounded-lg bg-amber-50 p-3 text-sm text-amber-900">No active project manager is assigned in the team or task plan. Missing-dispatch notifications will still reach active superadmins.</p>}
  <DataGrid label="Vendor indent register" rows={rows} grouping={{columnKey:"group",key:row=>row.group.id,label:row=>row.group.name}} columns={[
    {key:"group",label:"Group / scope",value:row=>`${row.group.name} · revision ${row.group.scope_revision??1}`},
    {key:"vendor",label:"Vendor",value:row=>row.vendor?.name||"Unavailable vendor"},
    {key:"number",label:"Indent number",value:row=>row.document?.data.number||"—",render:row=><span className="block max-w-44 break-all text-xs font-mono">{row.document?.data.number||"—"}</span>},
    {key:"generated",label:"Generated",value:row=>row.document?new Date(row.document.data.generated_at).toLocaleString():"Not generated"},
    {key:"printed",label:"Last print request",value:row=>row.document?.data.print_requests.length?`${new Date(row.document.data.print_requests.at(-1)!.requested_at).toLocaleString()} (${row.document.data.print_requests.length})`:"Not requested"},
    {key:"sent",label:"Actual sent date",value:row=>row.document?.data.sent_date||"Not recorded"},
  ]} actions={row=><div className="flex flex-wrap gap-2"><button type="button" className={button} disabled={busy || (row.document ? !canOpenDocument : (!canGenerate || !row.vendor?.is_active))} onClick={async(event)=>{const opener=event.currentTarget;if(row.document || await onAction({action:"generate",group_id:row.group.id,vendor_id:row.vendor_id})){if(canOpenDocument)onOpen(row.group.id,row.vendor_id,row.document?.id,opener);}}}>{row.document?"Open indent":"Generate indent"}</button>{row.document&&canWrite&&!row.document.data.sent_date&&<button type="button" className={button} disabled={busy || !workspace?.can_dispatch} onClick={()=>setSending(row.document!)}>Record sent date</button>}</div>}/>
  {workspace?.documents.some(d=>d.data.is_current===false) && <details className="rounded-lg border bg-white p-3"><summary className="cursor-pointer font-semibold">Previous scope indents</summary><DataGrid label="Previous vendor indent revisions" rows={workspace.documents.filter(d=>d.data.is_current===false)} columns={[{key:"group",label:"Group / revision",value:d=>`${d.data.group_name} · revision ${d.data.scope_revision??1}`},{key:"vendor",label:"Vendor",value:d=>d.data.vendor.name},{key:"items",label:"Items",value:d=>d.data.lines.length},{key:"sent",label:"Sent date",value:d=>d.data.sent_date||"Not recorded"}]} actions={d=><div className="flex gap-2"><button className={button} disabled={!canOpenDocument} onClick={event=>onOpen(d.group_id,d.vendor_id,d.id,event.currentTarget)}>Open previous indent</button>{canWrite&&!d.data.sent_date&&<button className={button} disabled={busy || !workspace?.can_dispatch} onClick={()=>setSending(d)}>Record sent date</button>}</div>}/></details>}
  {workspace?.history.length ? <details className="rounded-lg border border-slate-200 bg-white p-3"><summary className="cursor-pointer font-semibold">Generation, print and dispatch log</summary><ul className="mt-3 space-y-2 text-sm">{workspace.history.map((event,index)=><li key={index}>{new Date(event.created_at).toLocaleString()} · {event.action === "print" ? "Print requested" : event.action === "send" ? "Dispatch recorded" : "Indent generated"} · {workspace.documents.find(d=>d.id===event.document_id)?.data.vendor.name} · {workspace.documents.find(d=>d.id===event.document_id)?.data.group_name}</li>)}</ul></details>:null}
  {sending&&<RecordDrawer centered title={`Record sent date — ${sending.data.vendor.name}`} onClose={()=>{if(!busy)setSending(null);}}>
   {notice&&<p role="alert" className="text-sm text-red-700">{notice}</p>}
   <p className="text-sm">{sending.data.group_name}. Record when this generated indent was actually sent. This resolves its missing-dispatch notifications.</p>
   <ValidatedForm className="space-y-4" onSubmit={async event=>{event.preventDefault();const data=new FormData(event.currentTarget);if(await onAction({action:"send",document_id:sending.id,version:sending.version,sent_date:String(data.get("sent_date")),channel:String(data.get("channel")),dispatch_reference:String(data.get("dispatch_reference"))},event.currentTarget))setSending(null);}}>
    <ValidatedLabel className="block font-semibold">Actual sent date<ValidatedInput required name="sent_date" type="date" min={sending.data.generated_at.slice(0,10)} max={today} defaultValue={today} className="mt-1 block w-full rounded-lg border border-slate-300 p-3"/></ValidatedLabel>
    <ValidatedLabel className="block font-semibold">Dispatch channel<ValidatedInput required name="channel" maxLength={80} placeholder="Email, courier, hand delivery…" className="mt-1 block w-full rounded-lg border border-slate-300 p-3"/></ValidatedLabel>
    <ValidatedLabel className="block font-semibold">Dispatch reference / evidence<ValidatedInput required name="dispatch_reference" maxLength={500} className="mt-1 block w-full rounded-lg border border-slate-300 p-3"/></ValidatedLabel>
    <button type="submit" disabled={busy} className={button}>{busy?"Saving…":"Save sent date"}</button>
   </ValidatedForm>
  </RecordDrawer>}
 </section>;
}
