"use client";

import {useSearchParams} from "next/navigation";
import { useEffect, useState } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid } from "@/components/ui/data-grid";
import { EyeIcon, PencilSquareIcon, PlusIcon } from "@heroicons/react/24/outline";
import {useSupplierStatuses} from "./use-supplier-statuses";
import {quotationSupplierSummary} from "../purchasing/quotation-supplier-status";
import { SupplierEditor } from "./supplier-editor";
import { SupplierReviewAssignment } from "./supplier-review-assignment";
import { SupplierEvidence } from "./supplier-evidence";
import { type Supplier } from "./supplier-types";
import { RecordDrawer } from "@/components/ui/record-drawer";

const labels: Record<string,string> = { registration_reference: "Registration reference", contact: "Contact", location: "Location", qualification: "Qualification", contact_person_name: "Contact person", email: "Email", phone: "Phone", aadhar_number: "Aadhaar number", pan_number: "PAN", gst_number: "GSTIN", cancelled_cheque_ref: "Cancelled cheque reference", notes: "Review notes", advance_pct: "Advance (%)", qc_pct: "Quality clearance (%)", delivery_pct: "Delivery (%)", retention_pct: "Retention (%)", credit_days: "Credit days", description: "Description" };
function text(value: unknown): string { return value === undefined || value === null || value === "" ? "Pending" : typeof value === "boolean" ? value ? "Yes" : "No" : String(value); }
export function SupplierDirectory() {
  const user=useAuthenticatedUser();
  const allows=(p:string)=>user.permissions.includes("system:admin")||user.permissions.includes(p);
  const canRead=allows("settings.suppliers.read")||allows("accounts.vendor_verification.read");
  const canWrite=allows("settings.suppliers.write"), canWriteDocuments=allows("settings.supplier_documents.write"), canReview=allows("accounts.vendor_verification.read")&&allows("accounts.vendor_verification.write");
  const canReadDocuments=allows("settings.supplier_documents.read")||allows("accounts.vendor_verification.read");
  const [rows,setRows]=useState<Supplier[]>([]),[loading,setLoading]=useState(true),[error,setError]=useState("");
  const readiness=useSupplierStatuses(rows.map(row=>row.id));
  const requested=useSearchParams().get("supplier_id");
 const requestedDocument=useSearchParams().get("document_id");
  const [selection,setSelected]=useState<Supplier|null>(null),[requestedOpen,setRequestedOpen]=useState(true);
  const selected=selection??(requestedOpen?rows.find(r=>r.id===requested)??null:null);
  const fromReviewTask=requestedOpen&&selected?.id===requested;
  const [editing,setEditing]=useState<Supplier|null>(null),[creating,setCreating]=useState(false);
  function saved(record:Supplier){setRows(values=>values.some(v=>v.id===record.id)?values.map(v=>v.id===record.id?record:v):[...values,record]);}
  useEffect(()=>{if(!canRead)return;const controller=new AbortController();fetch("/api/settings/suppliers",{signal:controller.signal,cache:"no-store"}).then(async response=>{if(!response.ok)throw new Error();setRows(await response.json());}).catch(()=>{if(!controller.signal.aborted)setError("Unable to load suppliers. Please refresh or check your access.");}).finally(()=>{if(!controller.signal.aborted)setLoading(false);});return()=>controller.abort();},[canRead]);

  if(!canRead)return <p role="alert">Supplier read permission is required.</p>;
  return <section className="space-y-5"><header className="flex flex-col gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-end sm:justify-between"><div><h1 className="text-2xl font-bold">Vendors / Suppliers</h1></div>{canWrite&&<button type="button" className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white shadow-lg shadow-blue-900/15 hover:bg-blue-800" onClick={()=>setCreating(true)}><PlusIcon className="h-5 w-5"/>Add supplier</button>}</header>{error&&<p role="alert" className="text-red-700">{error}</p>}<DataGrid label="Suppliers" stateKey="suppliers" rows={rows} loading={loading} status={row=>row.is_active?"Active":"Inactive"} initialSort="name" columns={[
    {key:"code",label:"Code",value:row=>row.code},
    {key:"name",label:"Supplier",value:row=>row.name},
    {key:"location",label:"Location",value:row=>text(row.data.location)},
    {key:"contact",label:"Contact person",value:row=>text(row.data.contact_person_name)},
    {key:"documents",label:"Documents",value:row=>readiness.statuses[row.id]===undefined?"Checking…":quotationSupplierSummary(readiness.statuses[row.id]??undefined).label,render:row=>{const status=readiness.statuses[row.id],summary=quotationSupplierSummary(status??undefined);return <span title={summary.detail} className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${summary.approved?"bg-green-50 text-green-800":"bg-amber-50 text-amber-900"}`}>{status===undefined?"Checking…":`${summary.approved?"✓ ":""}${summary.label}`}</span>;}},
    {key:"indent_terms",label:"Indent terms",value:row=>String(row.data.indent_terms??"").trim()&&String(row.data.delivery_terms??"").trim()?"Configured":"Pending"},
    {key:"status",label:"Status",value:row=>row.is_active?"Active":"Inactive"},
  ]} actions={row=><div className="flex gap-1"><button type="button" title="View supplier" aria-label={`View supplier ${row.code}`} onClick={()=>setSelected(row)} className="flex min-h-11 min-w-11 items-center justify-center text-blue-700"><EyeIcon className="h-5 w-5"/></button>{canWrite&&<button type="button" title="Edit supplier" aria-label={`Edit supplier ${row.code}`} onClick={()=>setEditing(row)} className="flex min-h-11 min-w-11 items-center justify-center text-blue-700"><PencilSquareIcon className="h-5 w-5"/></button>}</div>} />
  {selected&&<RecordDrawer title={selected.name} onClose={()=>{setSelected(null);setRequestedOpen(false);readiness.refresh();}}><p className="text-sm text-slate-600">{selected.code} · {selected.is_active?"Active":"Inactive"}</p>{<SupplierEvidence focusedDocumentId={requestedDocument} key={`${selected.id}-${selected.version}`} supplierId={selected.id} supplierName={selected.name} canReadDocuments={canReadDocuments} canWriteDocuments={canWriteDocuments} canReview={canReview}/>}<details className="rounded-lg border border-slate-200 p-3"><summary className="min-h-11 cursor-pointer py-3 text-sm font-semibold">Supplier details</summary><div className="space-y-4"><dl className="grid gap-4 sm:grid-cols-2">{Object.entries(labels).filter(([key])=>key in selected.data).map(([key,label])=><div key={key}><dt className="text-xs font-semibold text-slate-500">{label}</dt><dd className="mt-1 whitespace-pre-wrap break-words text-sm">{text(selected.data[key])}</dd></div>)}</dl><h3 className="text-lg font-bold">Payment terms</h3><dl className="grid gap-3 sm:grid-cols-2">{Object.entries((selected.data.payment_terms??{}) as Record<string,unknown>).map(([key,value])=><div key={key}><dt className="text-xs font-semibold text-slate-500">{labels[key]??key.replaceAll("_"," ")}</dt><dd className="text-sm">{text(value)}</dd></div>)}</dl><section aria-label="Vendor purchase indent terms" className="space-y-3"><h3 className="text-lg font-bold">Purchase indent terms</h3><dl className="space-y-3">{[["indent_notes","Important notes"],["indent_terms","Terms and conditions"],["delivery_terms","Delivery basis"]].map(([key,label])=><div key={key}><dt className="text-xs font-semibold text-slate-500">{label}</dt><dd className="mt-1 whitespace-pre-wrap break-words text-sm">{text(selected.data[key])}</dd></div>)}</dl></section></div></details>{!fromReviewTask&&<details className="rounded-lg border border-slate-200 p-3"><summary className="min-h-11 cursor-pointer py-3 text-sm font-semibold">Reviewer assignment</summary><SupplierReviewAssignment supplierId={selected.id}/></details>}</RecordDrawer>}
  {(creating||editing)&&<RecordDrawer title={editing?"Edit supplier":"Add supplier"} onClose={()=>{setEditing(null);setCreating(false);readiness.refresh();}}><SupplierEditor key={editing?.id??"new"} record={editing??undefined} onSaved={saved} canReadDocuments={canReadDocuments} canWriteDocuments={canWriteDocuments} canReview={canReview}/></RecordDrawer>}
  </section>;
}
