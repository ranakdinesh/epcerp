"use client";
import styles from "./supplier-workflow.module.css";
import {useEffect,useState,type SubmitEvent} from "react";
import {DataGrid} from "@/components/ui/data-grid";
import {SupplierPDFViewer} from "./supplier-pdf-viewer";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {ValidatedForm,ValidatedLabel as Label,ValidatedTextarea as Textarea} from "@/components/ui/validated-controls";
import {displayServerErrors} from "@/components/ui/server-field-errors";
import {supplierRequest,failureMessage,type SupplierDocument,type Verification} from "./supplier-types";
import {SupplierDocumentActions} from "./supplier-document-actions";
import {SupplierDocumentUpload} from "./supplier-document-upload";
import {documentReviewRows,documentDecisionUpdate,documentTypeNames} from "./supplier-document-review";

const action="min-h-11 rounded-lg px-3 font-semibold disabled:opacity-50";
const badge="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold";
export function SupplierEvidence({supplierId,supplierName,canReadDocuments,canWriteDocuments,canReview,focusedDocumentId}:{focusedDocumentId?:string|null;supplierId:string;supplierName:string;canReadDocuments:boolean;canWriteDocuments:boolean;canReview:boolean}) {
 const [documents,setDocuments]=useState<SupplierDocument[]>([]),[status,setStatus]=useState<Verification|null>(null),[error,setError]=useState(""),[loading,setLoading]=useState(true),[revision,setRevision]=useState(0),[preview,setPreview]=useState<SupplierDocument|null>(null),[upload,setUpload]=useState<string|null>(null),[replacement,setReplacement]=useState<SupplierDocument|null>(null),[archive,setArchive]=useState<SupplierDocument|null>(null),[reject,setReject]=useState<SupplierDocument|null>(null),[busy,setBusy]=useState(false),[documentError,setDocumentError]=useState<{id:string;message:string}|null>(null),[notice,setNotice]=useState(""),[showArchived,setShowArchived]=useState(false);
 useEffect(()=>{const controller=new AbortController();Promise.all([supplierRequest<Verification>(`/api/settings/suppliers/${supplierId}/verification`,{signal:controller.signal}),canReadDocuments?supplierRequest<SupplierDocument[]>(`/api/settings/suppliers/${supplierId}/documents`,{signal:controller.signal}):Promise.resolve([])]).then(([next,docs])=>{if(controller.signal.aborted)return;setStatus(next);setDocuments(docs);}).catch(e=>{if(!controller.signal.aborted)setError(failureMessage(e));}).finally(()=>{if(!controller.signal.aborted)setLoading(false);});return()=>controller.abort();},[supplierId,canReadDocuments,revision]);
 function refresh(){setLoading(true);setError("");setRevision(v=>v+1);}
 async function decide(document:SupplierDocument,decision:"accept"|"reject",comment:string){
  if(!status||busy||loading)return false;
  setBusy(true);setError("");setDocumentError(null);
  try{const next=await supplierRequest<Verification>(`/api/settings/suppliers/${supplierId}/verification`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(documentDecisionUpdate(status,document.id,decision,comment))});setStatus(next);setNotice(decision==="accept"?"Document approved.":"Document rejected. The uploader will receive a correction task.");return true;}
  catch(e){if(decision==="reject")setError(failureMessage(e));else setDocumentError({id:document.id,message:failureMessage(e)});return false;}finally{setBusy(false);}
 }
 const rows=documentReviewRows(documents,status,showArchived||documents.some(d=>d.id===focusedDocumentId&&!d.is_active)),activeRows=documentReviewRows(documents,status),approved=activeRows.filter(r=>r.status==="Approved").length,archivedCount=documents.filter(d=>!d.is_active).length;
 return <section className="space-y-3" aria-label="Supplier document review"><header className="flex flex-wrap items-center justify-between gap-3"><div className="flex flex-wrap items-center gap-3"><h3 className="text-lg font-bold">Documents</h3>{status&&<span role="status" className={`${badge} ${status.can_generate_purchase_order?"bg-green-100 text-green-800":"bg-slate-100 text-slate-700"}`}>{status.can_generate_purchase_order?"✓ Approved":status.can_invite_quotation?"Pending":"Inactive"}</span>}</div>{canWriteDocuments&&<button type="button" className={`${action} bg-blue-700 text-white`} disabled={upload!==null||busy} onClick={()=>{setReplacement(null);setUpload("");}}>Upload document</button>}</header>
 {status&&canReadDocuments&&<p className="text-sm text-slate-500" aria-live="polite">{approved} of {activeRows.length} documents approved</p>}
 {notice&&<p role="status" className="text-sm text-green-800">{notice}</p>}
 {upload!==null&&<SupplierDocumentUpload key={`${upload}:${replacement?.id??"new"}`} supplierId={supplierId} supplierName={supplierName} initialType={upload} replacement={replacement??undefined} onCancel={()=>setUpload(null)} onSaved={()=>{setNotice(replacement?"Document replaced. Previous file kept in history.":"Document uploaded.");setUpload(null);setDocumentError(null);refresh();}}/>}
 {error&&<p role="alert" className="text-sm text-red-700">{error}</p>}
 {status&&!status.can_invite_quotation&&<p role="alert" className="text-sm text-red-700">{status.message}</p>}
 {!canReadDocuments?<p>Document read permission is required.</p>:<DataGrid label="Supplier documents" rows={rows} loading={loading} searchable={false} paginate={false} initialSort="type" columns={[
 {key:"document",label:"File",value:r=>r.title,render:r=><div className="whitespace-normal break-words">{r.title}{r.id===focusedDocumentId&&<span className="ml-2 text-xs text-blue-700">Task document</span>}{documentError?.id===r.id&&<p role="alert" className="mt-1 max-w-80 text-xs text-red-700">{documentError.message}</p>}</div>},
 {key:"type",label:"Document",value:r=>documentTypeNames[r.type]??r.type},
 {key:"status",label:"Status",value:r=>r.status,render:r=><div><span className={`${badge} ${r.status==="Approved"?"bg-green-100 text-green-800":r.status==="Rejected"?"bg-red-50 text-red-700":"bg-slate-100 text-slate-600"}`}>{r.status==="Approved"?"✓ ":""}{r.status}</span>{r.status==="Rejected"&&r.comment&&<p className="mt-1 max-w-48 whitespace-normal text-xs text-slate-500">{r.comment}</p>}</div>}
 ]} actions={r=><SupplierDocumentActions row={r} canWrite={canWriteDocuments} canReview={canReview} busy={busy||loading||upload!==null} supplierActive={Boolean(status?.can_invite_quotation)} hasError={documentError?.id===r.id&&/cannot be previewed|readable PDF/i.test(documentError.message)} onUpload={()=>{setReplacement(r.document??null);setUpload(r.type);setNotice("");}} onView={()=>setPreview(r.document!)} onApprove={()=>void decide(r.document!,"accept","Document reviewed and approved")} onReject={()=>setReject(r.document!)}/>}/>}
 {archivedCount>0&&<button type="button" className={`${action} text-xs text-slate-500`} onClick={()=>setShowArchived(v=>!v)}>{showArchived?"Hide archived":`Show archived (${archivedCount})`}</button>}
 {preview&&<DocumentViewer key={preview.id} supplierId={supplierId} document={preview} onClose={()=>setPreview(null)} onArchive={canWriteDocuments&&preview.is_active?()=>{setArchive(preview);setPreview(null);}:undefined}/>}

 {archive&&<RecordDrawer centered title="Archive supplier document" onClose={()=>setArchive(null)}><ArchiveForm supplierId={supplierId} document={archive} onSaved={()=>{setArchive(null);refresh();}}/></RecordDrawer>}
 {reject&&<RecordDrawer centered title="Reject document" onClose={()=>setReject(null)}><RejectForm document={reject} busy={busy} error={error} onReject={async comment=>{if(await decide(reject,"reject",comment))setReject(null);}}/></RecordDrawer>}
 </section>;
}
function RejectForm({document,busy,error,onReject}:{document:SupplierDocument;busy:boolean;error:string;onReject:(comment:string)=>Promise<void>}){
 return <ValidatedForm className={styles.form} onSubmit={async event=>{event.preventDefault();await onReject(String(new FormData(event.currentTarget).get("comment")));}}><fieldset disabled={busy} className="space-y-4"><p className="text-sm">{document.title||document.original_filename}</p><Label>Reason<Textarea name="comment" required maxLength={1800}/></Label>{error&&<p role="alert" className="text-sm text-red-700">{error}</p>}<button className={`${action} bg-red-700 text-white`} type="submit">{busy?"Saving…":"Reject document"}</button></fieldset></ValidatedForm>;
}
function DocumentViewer({supplierId,document:doc,onClose,onArchive}:{supplierId:string;document:SupplierDocument;onClose:()=>void;onArchive?:()=>void}) {
 const [url,setUrl]=useState(""),[kind,setKind]=useState(""),[error,setError]=useState("");
 useEffect(()=>{const controller=new AbortController();let objectUrl="";fetch(`/api/settings/suppliers/${supplierId}/documents/${doc.id}/preview`,{signal:controller.signal,cache:"no-store"}).then(async response=>{if(!response.ok)throw await response.json();const blob=await response.blob();if(controller.signal.aborted)return;objectUrl=URL.createObjectURL(blob);setKind(blob.type);setUrl(objectUrl);}).catch(e=>{if(!controller.signal.aborted)setError(failureMessage(e));});return()=>{controller.abort();if(objectUrl)URL.revokeObjectURL(objectUrl);};},[supplierId,doc.id]);
 // Authenticated blob preview must stay in this browser; it cannot use the Next image proxy.
 // eslint-disable-next-line @next/next/no-img-element
 return <RecordDrawer title={doc.title||doc.original_filename} centered wide onClose={onClose}><div className="flex justify-end gap-2"><a className={`${action} py-3 text-blue-700`} href={`/api/settings/suppliers/${supplierId}/documents/${doc.id}/download`} download>Download</a>{onArchive&&<button type="button" className={`${action} text-slate-500`} onClick={onArchive}>Archive document</button>}</div>{error?<p role="alert" className="text-red-700">{error}</p>:!url?<p role="status">Opening document…</p>:kind==="application/pdf"?<SupplierPDFViewer url={url} filename={doc.original_filename}/>:<img src={url} alt={doc.title||doc.original_filename} className="mx-auto max-h-[65dvh] max-w-full object-contain"/>}</RecordDrawer>;
}
function ArchiveForm({supplierId,document:doc,onSaved}:{supplierId:string;document:SupplierDocument;onSaved:()=>void}) {
 const [error,setError]=useState(""),[busy,setBusy]=useState(false);
 async function submit(event:SubmitEvent<HTMLFormElement>){event.preventDefault();const form=event.currentTarget;setBusy(true);try{await supplierRequest(`/api/settings/suppliers/${supplierId}/documents/${doc.id}/archive`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({version:doc.version,reason:new FormData(form).get("reason")})});onSaved();}catch(e){if(!displayServerErrors(form,e))setError(failureMessage(e));}finally{setBusy(false);}}
 return <ValidatedForm className={styles.form} onSubmit={submit}><fieldset disabled={busy} className="space-y-4"><p>Archiving {doc.title||doc.original_filename} removes it from active evidence. Other documents keep their current approval decisions.</p><Label>Archive reason<Textarea name="reason" required maxLength={2000}/></Label>{error&&<p role="alert" className="text-red-700">{error}</p>}<button type="submit" className="rounded-lg bg-red-700 px-4 py-3 text-white">{busy?"Archiving…":"Archive document"}</button></fieldset></ValidatedForm>;
}
