"use client";
import {GridAction} from "@/components/ui/data-grid";
import {documentTypeNames,type DocumentReviewRow} from "./supplier-document-review";
const action="min-h-11 rounded-lg px-3 font-semibold disabled:opacity-50";
export function SupplierDocumentActions({row:r,canWrite,canReview,busy,supplierActive,hasError,onUpload,onView,onApprove,onReject}:{row:DocumentReviewRow;canWrite:boolean;canReview:boolean;busy:boolean;supplierActive:boolean;hasError:boolean;onUpload:()=>void;onView:()=>void;onApprove:()=>void;onReject:()=>void}){
 return <div className="flex flex-wrap items-center justify-end gap-1">
 {r.status==="Missing"?canWrite?<button type="button" className={`${action} text-blue-700`} disabled={busy} onClick={onUpload} aria-label={`Upload ${documentTypeNames[r.type]??r.type}`}>Upload</button>:<span className="text-xs text-slate-500">Awaiting upload</span>:r.document&&<>
  <GridAction kind="view" label={`View ${r.title}`} onClick={onView}/>
  {canWrite&&r.document.is_active&&<button type="button" className={`${action} text-blue-700`} disabled={busy} onClick={onUpload} aria-label={`Replace ${r.title}`}>Replace</button>}
  {canReview&&r.document.is_active&&<>{r.status!=="Approved"&&!hasError&&<button type="button" disabled={busy||!supplierActive} className={`${action} text-blue-700`} aria-label={`Approve ${r.title}`} onClick={onApprove}>Approve</button>}{r.status!=="Rejected"&&<button type="button" disabled={busy||!supplierActive} className={`${action} text-slate-500`} aria-label={`Reject ${r.title}`} onClick={onReject}>Reject</button>}</>}
 </>}
 </div>;
}
