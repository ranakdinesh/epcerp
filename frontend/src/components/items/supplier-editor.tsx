"use client";
import { useState } from "react";
import { SupplierForm } from "./supplier-form";
import { SupplierEvidence } from "./supplier-evidence";
import { type Supplier } from "./supplier-types";

export function SupplierEditor({record,onSaved,canReadDocuments,canWriteDocuments,canReview}:{record?:Supplier;onSaved:(record:Supplier)=>void;canReadDocuments:boolean;canWriteDocuments:boolean;canReview:boolean}) {
 const [current,setCurrent]=useState(record);
 const [step,setStep]=useState<"details"|"documents">("details");
 const [message,setMessage]=useState("");
 function saved(next:Supplier){setCurrent(next);onSaved(next);setMessage("Supplier details saved. Upload PAN and cancelled cheque, plus GST evidence when a GSTIN is recorded.");setStep("documents");}
 return <section className="space-y-4">
  {current&&<p className="text-sm font-semibold">{current.code} · {current.name}</p>}
  <nav aria-label="Supplier setup" className="flex flex-wrap gap-2">
   <button type="button" aria-pressed={step==="details"} className={`min-h-11 rounded-lg border px-4 ${step==="details"?"bg-blue-700 text-white":""}`} onClick={()=>setStep("details")}>1. Supplier details</button>
   <button type="button" aria-pressed={step==="documents"} disabled={!current} className={`min-h-11 rounded-lg border px-4 disabled:opacity-50 ${step==="documents"?"bg-blue-700 text-white":""}`} onClick={()=>setStep("documents")}>2. Documents</button>
  </nav>
  {!current&&<p className="rounded-lg border border-blue-200 p-3 text-sm">Save supplier details first, then upload PAN, cancelled cheque and applicable GST evidence in Documents.</p>}
  {message&&<p role="status" className="text-sm text-green-700">{message}</p>}
  <div hidden={step!=="details"}><SupplierForm key={current?.version??"new"} record={current} onSaved={saved}/></div>
  {current&&step==="documents"&&<SupplierEvidence key={`${current.id}-${current.version}`} supplierId={current.id} supplierName={current.name} canReadDocuments={canReadDocuments} canWriteDocuments={canWriteDocuments} canReview={canReview}/>}
 </section>;
}
