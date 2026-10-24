"use client";
import {PurchaseSectionHeader} from "@/components/purchase-fms/purchase-layout";

import { EyeIcon, PencilSquareIcon, PlusIcon } from "@heroicons/react/24/outline";
import {useQuotationApprovals} from "./quotation-approval-controls";
import { QuotationCommonTerms } from "./quotation-common-terms";
import { QuotationItemPrices } from "./quotation-item-prices";
import {useSearchParams} from "next/navigation";
import { useId, useState } from "react";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
import type { VendorIndent } from "./purchase-indent-documents";
import type { Quote, Workspace } from "./group-quotation-workspace";

export type QuotationFile = { id: string; document_id: string; filename: string; created_at: string; byte_size: number };
export type QuotationRevision = Quote & { revision: number; recorded_at: string; recorded_by_id: string | null; reason: string; attachment?: QuotationFile; calculation: { total: string; complete: boolean; lines: {line_id: string; landed: string | null; unit_price?: string | null}[] } };
const button = "min-h-11 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold disabled:opacity-50";
const iconButton = "inline-flex min-h-11 min-w-11 items-center justify-center rounded-lg border border-slate-300 bg-white text-blue-700 hover:bg-blue-50 focus-visible:outline-2 focus-visible:outline-blue-600 disabled:opacity-50";
const field = "mt-1 block min-h-11 w-full rounded-lg border border-slate-300 p-3";

export function GroupQuotationReceipt({ workspace, busy, error, url, onSave, onUpload, money }: {
  workspace: Workspace; busy: boolean; error: string; url: string;
  onSave: (form: HTMLFormElement, document: VendorIndent) => Promise<boolean>;
  onUpload: (document: VendorIndent, file: File) => Promise<QuotationFile | null>;
  money: (value: string | null | undefined) => string;
}) {
 const params=useSearchParams();
 const approval=useQuotationApprovals(url.replace("/group-quotations","/quotation-approvals"),url.split("/")[3]);
 const groupStatus=(id:string)=>approval.workspace?.attempts.filter(a=>a.group_id===id).at(-1);
 const formId = useId();
  const [editor,setEditor]=useState<VendorIndent|null>(null),[resumeHandled,setResumeHandled]=useState(false);
 const key=params.get("quote");const resumeDoc=workspace.documents.find(d=>d.id===key?.split(":")[0]&&d.data.is_current!==false);
 const resumeAttempt=resumeDoc?groupStatus(resumeDoc.group_id):undefined;
 const editing=editor??(!resumeHandled&&workspace.can_write&&resumeAttempt?.state==="rejected"&&resumeAttempt.return_action==="negotiate"&&resumeAttempt.quote_key===key?resumeDoc??null:null);
 function setEditing(value:VendorIndent|null){setResumeHandled(true);setEditor(value)}
  const [tab, setTab] = useState<"file" | "prices">("file");
  const [attachment, setAttachment] = useState("");
  const [noDocument, setNoDocument] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [viewing, setViewing] = useState<VendorIndent | null>(null);
  const [selectedRevision, setSelectedRevision] = useState(0);
  const dispatched = workspace.documents.filter(document => document.data.sent_date);
  const latest = (id: string) => workspace.record?.quotes.find(quote => quote.document_id === id);
  const revisions = (id: string) => workspace.revisions.filter(quote => quote.document_id === id).sort((a,b)=>b.revision-a.revision);
  const vendorDetails = (document: VendorIndent) => {
    const vendor=document.data.vendor;
    return [vendor.name, `Code: ${vendor.code}`, ...([ ["Contact",vendor.data.contact_person_name], ["Email",vendor.data.email], ["Phone",vendor.data.phone], ["Location",vendor.data.location] ] as const).filter(([,value])=>value).map(([label,value])=>`${label}: ${value}`)].join("\n");
  };
  const receiptDetails = (document: VendorIndent) => {
    const quote=latest(document.id);
    return quote ? `Revision ${revisions(document.id)[0]?.revision??1} · ${quote.reference} · received ${quote.received_date}${quote.document_not_provided ? " · document not provided" : ""}` : workspace.attachments.some(file=>file.document_id===document.id) ? "File uploaded — item prices pending" : "Awaiting quotation";
  };
  const receiptLabel = (document: VendorIndent) => latest(document.id)?`Revision ${revisions(document.id)[0]?.revision??1}`:receiptDetails(document);
  const editLabel = (document: VendorIndent) => groupStatus(document.group_id)?.effective_state==="pending_accounts"?"Waiting for approval":groupStatus(document.group_id)?.effective_state==="approved"?"Group approved":latest(document.id)?"Add negotiated revision":"Receive quotation";
  const uploadedFiles = editing ? workspace.attachments.filter(file=>file.document_id===editing.id) : [];
  const canEnterPrices = noDocument || Boolean(attachment);
 const returned=editing?groupStatus(editing.group_id):undefined;
 const negotiating=returned?.effective_state==="rejected"&&returned.return_action==="negotiate"&&returned.quote_key.split(":")[0]===editing?.id;
 const previous = editing ? (negotiating?revisions(editing.id).find(q=>q.revision===Number(returned?.quote_key.split(":")[1])):undefined)??latest(editing.id) : undefined;
  const revision = viewing ? revisions(viewing.id).find(revision=>revision.revision===selectedRevision) : undefined;
  const today = new Date().toLocaleDateString("en-CA", {timeZone:"Asia/Kolkata"});
  function open(document: VendorIndent) {
    setEditing(document); setTab("file"); setAttachment(""); setSelectedFile(null); setNoDocument(false);
  }

 const download = (documentId: string, file: Pick<QuotationFile,"id"|"filename">) => <a className="text-blue-700 underline" href={`${url}/${documentId}/attachments/${file.id}/download`}>{file.filename}</a>;
  const input = (name: string, label: string, type = "text", value = "", extra: Record<string,string|number> = {}) => <ValidatedLabel className="block font-semibold">{label}<ValidatedInput required name={name} type={type} defaultValue={value} className={field} {...extra}/></ValidatedLabel>;
  return <div className="quotation-receipt-workspace space-y-4">
    <PurchaseSectionHeader title="Receive vendor quotations"/>
    
    {!workspace.can_write && <p role="status" className="text-sm text-slate-600">Read-only access. Uploading or changing quotations requires permission.</p>}
    <DataGrid label="Dispatched group vendor quotations" grouping={{columnKey:"group",key:document=>document.group_id,label:document=>document.data.group_name}} rows={dispatched} columns={[
      {key:"group",label:"Item group",value:document=>`${document.data.group_name} · scope ${document.data.scope_revision??1}${document.data.is_current===false?" · previous scope":""}`},
      {key:"vendor",label:"Vendor",value:document=>`${document.data.vendor.code} ${document.data.vendor.name}`,render:document=><span tabIndex={0} title={vendorDetails(document)} aria-label={vendorDetails(document)} className="whitespace-nowrap font-semibold">{document.data.vendor.code||document.data.vendor.name}</span>},
      {key:"indent",label:"Indent / sent date",value:document=>`${document.data.number} · ${document.data.sent_date}`,render:document=><div className="max-w-44"><span className="block break-all text-xs">{document.data.number}</span><span className="mt-1 block whitespace-nowrap">{document.data.sent_date}</span></div>},
      {key:"items",label:"Items",value:document=>document.data.lines.length},
      {key:"status",label:"Receipt status",value:receiptDetails,render:document=><span tabIndex={0} title={receiptDetails(document)} aria-label={receiptDetails(document)} className="whitespace-nowrap">{receiptLabel(document)}</span>},
      {key:"total",label:"Latest quotation total",value:document=>money(latest(document.id)?.calculation?.total)},
    ]} actions={document=><div className="flex gap-1">
      <button type="button" className={iconButton} title={`View quotation / history (${revisions(document.id).length})`} aria-label={`View quotation / history for ${document.data.vendor.code}`} disabled={busy} onClick={()=>{setViewing(document);setSelectedRevision(revisions(document.id)[0]?.revision??0);}}><EyeIcon aria-hidden="true" className="h-5 w-5"/></button>
      {workspace.can_write && document.data.is_current!==false && <button type="button" className={iconButton} title={editLabel(document)} aria-label={`${editLabel(document)} for ${document.data.vendor.code}`} disabled={busy||["pending_accounts","approved"].includes(groupStatus(document.group_id)?.effective_state??"")} onClick={()=>open(document)}>{latest(document.id)?<PencilSquareIcon aria-hidden="true" className="h-5 w-5"/>:<PlusIcon aria-hidden="true" className="h-5 w-5"/>}</button>}
    </div>}/>
    {!dispatched.length && <p role="status" className="rounded-lg bg-amber-50 p-4">No indents have a recorded sent date yet. Record actual dispatch in the vendor indent register to receive quotations here.</p>}
    {workspace.documents.some(document=>!document.data.sent_date) && <p className="text-sm text-slate-600">{workspace.documents.filter(document=>!document.data.sent_date).length} generated indents are awaiting dispatch and are not included in this receipt register.</p>}
    {editing && <RecordDrawer centered wide footer={<div className="flex flex-wrap items-center justify-between gap-3"><p className="text-sm text-slate-600">{tab === "file" ? "Choose a quotation document or record manual receipt to continue." : "Review item prices and terms, then save this revision."}</p><div className="flex gap-2"><button type="button" className={button} disabled={busy} onClick={()=>setEditing(null)}>Cancel</button>{tab === "prices" && <button type="submit" form={formId} className={`${button} !border-blue-700 !bg-blue-700 !text-white`} disabled={busy||!workspace.can_write||!canEnterPrices}>{busy?"Saving…":previous?"Save new quotation revision":"Save item quotation"}</button>}</div></div>} title={`${previous?"New quotation revision":"Receive quotation"} — ${editing.data.group_name} / ${editing.data.vendor.name}`} onClose={()=>{if(!busy)setEditing(null);}}>
      {negotiating&&<p className="mb-3 rounded-lg bg-amber-50 p-3 text-sm">Finance requested negotiation. Enter revised discounts, attach the revised offer and save a new quotation revision. {returned?.review_comment}</p>}
      <p className="mb-3 break-words text-xs text-slate-600">Indent {editing.data.number} · sent {editing.data.sent_date} · {editing.data.lines.length} items.{previous && ` Previous quotation ${previous.reference}; its prices are copied for review and remain saved separately.`}</p>
      <nav aria-label="Quotation receipt steps" className="mb-4 flex flex-wrap gap-2"><button type="button" className={`${button} ${tab=== "file" ? "!border-blue-700 !bg-blue-50 !text-blue-800" : ""}`} aria-pressed={tab==="file"} disabled={busy} onClick={()=>setTab("file")}>1. Quotation document</button><button type="button" className={`${button} ${tab==="prices" ? "!border-blue-700 !bg-blue-50 !text-blue-800" : ""}`} aria-pressed={tab==="prices"} disabled={busy||!canEnterPrices} onClick={()=>setTab("prices")}>2. Enter item prices</button></nav>
      {error && <p role="alert" className="mb-3 text-red-800">{error}</p>}
      {/* Keep both panels mounted so revisiting the file never loses entered prices. */}
      <div hidden={tab!=="file"} className="space-y-4">
        <ValidatedLabel className="indent-selection flex min-h-11 items-center gap-2"><ValidatedInput aria-label="Quotation document not provided" type="checkbox" checked={noDocument} disabled={busy} onChange={event=>{setNoDocument(event.target.checked);if(event.target.checked){setAttachment("");setSelectedFile(null);setTab("prices");}}}/>Quotation document not provided — enter prices manually</ValidatedLabel>
        {!noDocument && <ValidatedForm className="space-y-3" onSubmit={async event=>{event.preventDefault();if(!selectedFile)return;const file=await onUpload(editing,selectedFile);if(file){setAttachment(file.id);setSelectedFile(null);setTab("prices");}}}>
          <ValidatedLabel className="block font-semibold">Quotation file<ValidatedInput required type="file" name="quotation_file" accept=".pdf,.png,.jpg,.jpeg,.xlsx" className={field} disabled={busy} onChange={event=>setSelectedFile(event.target.files?.[0]??null)}/></ValidatedLabel>
          <p className="text-sm text-slate-600">PDF, PNG, JPEG or XLSX, at most 10 MiB. Uploaded files are saved immediately. Upload the revised vendor document for each negotiation round.</p>
          <button type="submit" className={button} disabled={busy||!selectedFile}>{busy?"Uploading…":"Upload and continue to prices"}</button>
        </ValidatedForm>}
        {uploadedFiles.length>0 && <section aria-label="Uploaded quotation files" className="space-y-3 rounded-lg border border-slate-200 p-4"><h3 className="font-semibold">Uploaded quotation files</h3><p className="text-sm text-slate-600">Open a file to review it. Files already recorded in a revision remain in history; unrecorded files can be used for this quotation.</p><ul className="space-y-3">{uploadedFiles.map(file=>{const recorded=workspace.revisions.find(revision=>revision.attachment_id===file.id);return <li key={file.id} className="flex flex-wrap items-center gap-3 border-t pt-3"><span className="min-w-0 flex-1 break-words">{file.filename}<small className="block text-slate-600">{new Date(file.created_at).toLocaleString()} · {recorded?`Recorded in revision ${recorded.revision}`:"Item prices pending"}</small></span><a className={button} target="_blank" rel="noopener noreferrer" href={`${url}/${editing.id}/attachments/${file.id}/download?inline=true`}>Open {file.filename}</a>{!recorded && <button type="button" className={button} disabled={busy} onClick={()=>{setNoDocument(false);setAttachment(file.id);setTab("prices");}}>Use {file.filename}</button>}</li>;})}</ul></section>}

      </div>
      <div hidden={tab!=="prices"}>
        <ValidatedForm id={formId} key={editing.id} className="space-y-5" onSubmit={async event=>{event.preventDefault();if(workspace.can_write && canEnterPrices && await onSave(event.currentTarget,editing))setEditing(null);}}>
          <ValidatedInput type="hidden" name="document_not_provided" value={String(noDocument)}/><ValidatedInput type="hidden" name="attachment_id" value={noDocument?"":attachment}/><ValidatedInput type="hidden" name="evidence_reference" value={workspace.attachments.find(file=>file.id===attachment)?.filename??""}/>
          <fieldset disabled={busy||!canEnterPrices} className="min-w-0 space-y-5">
            <section aria-label="Quotation details" className="space-y-3"><h3 className="font-bold">Quotation details</h3><div className="grid gap-4 sm:grid-cols-2">{input("reference","Vendor quote reference","text","",{maxLength:128})}{input("received_date","Quotation received date","date",today,{min:editing.data.sent_date!,max:today})}{input("valid_until","Valid until","date",previous?.valid_until||today)}{input("delivery_days","Delivery timeline (days from PO)","number",String(previous?.delivery_days??0),{min:0,max:3650,step:1})}</div></section>
            <QuotationItemPrices canWrite={workspace.can_write} negotiating={negotiating} key={editing.id} document={editing} previous={previous} money={money} taxRate={workspace.tax_rate}/>
            <QuotationCommonTerms key={`terms-${editing.id}`} document={editing} previous={previous}/>
            {noDocument && <div className="rounded-lg border border-amber-200 bg-amber-50 p-4"><p className="mb-2 font-semibold">Quotation document not provided</p>{input("no_document_reason","How were prices received?","text","",{maxLength:2000})}<p className="mt-2 text-sm">Record the source, such as a vendor call, message or email. This declaration is saved with this revision.</p></div>}
            {input("reason",previous?"Negotiation / revision reason":"Receipt notes","text","",{maxLength:2000})}
            <p className="text-sm text-slate-600">Saving calculates item costs and the quotation total. Every saved revision remains available in history; comparison is a separate step.</p>
          </fieldset>
        </ValidatedForm>
      </div>
    </RecordDrawer>}
    {viewing && <RecordDrawer centered wide title={`View quotation — ${viewing.data.group_name} / ${viewing.data.vendor.name}`} onClose={()=>setViewing(null)}>
      {revisions(viewing.id).length>0 && <ValidatedLabel className="block font-semibold">Quotation revision<ValidatedSelect className={field} value={selectedRevision} onChange={event=>setSelectedRevision(Number(event.target.value))}>{revisions(viewing.id).map(quote=><option key={quote.revision} value={quote.revision}>Revision {quote.revision} · {quote.reference} · {money(quote.calculation.total)}</option>)}</ValidatedSelect></ValidatedLabel>}
      {!revision && <div className="space-y-4"><p role="status" className="text-sm text-slate-600">No quotation recorded. Showing dispatched indent items.</p><QuotationItemPrices key={viewing.id} document={viewing} canWrite={false} money={money} taxRate={workspace.tax_rate}/></div>}
      {revision && <div className="mt-4 space-y-4"><p>Received {revision.received_date} · valid until {revision.valid_until} · recorded {new Date(revision.recorded_at).toLocaleString()} · actor {revision.recorded_by_id || "Legacy import"}</p><p>{revision.reason}</p><p>Quotation file: {revision.document_not_provided ? `Document not provided · ${revision.no_document_reason}` : revision.attachment ? download(viewing.id,revision.attachment) : `Legacy evidence reference: ${revision.evidence_reference}`}</p><p className="font-semibold">Quotation total: {money(revision.calculation.total)}{!revision.calculation.complete && " · partial quotation"}</p><p>{revision.amount_mode==="total_only"?"One all-inclusive vendor total; item pricing and separate charges were not recorded.":`Freight ${money(revision.freight)} · other charges ${money(revision.other_charges)}`} · delivery {revision.delivery_days} days · Payment terms: {revision.payment_terms} · Delivery terms: {revision.delivery_terms||"Not recorded"}</p>{revision.amount_mode==="total_only"?<p className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm">This revision records the complete indent total only. Record an itemized revision before submitting it for Accounts approval.</p>:<QuotationItemPrices key={`${viewing.id}-${revision.revision}`} document={viewing} previous={revision} canWrite={false} money={money} taxRate={workspace.tax_rate} landedCosts={revision.calculation.lines}/>}</div>}
    </RecordDrawer>}
  </div>;
}
