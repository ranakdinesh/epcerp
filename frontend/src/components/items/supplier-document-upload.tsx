"use client";
import {useEffect,useRef,useState,type SubmitEvent} from "react";
import {ValidatedForm,ValidatedLabel,ValidatedInput,ValidatedSelect} from "@/components/ui/validated-controls";
import {displayServerErrors} from "@/components/ui/server-field-errors";
import {supplierRequest,failureMessage,type SupplierDocument} from "./supplier-types";
import {documentTypeNames} from "./supplier-document-review";

export function SupplierDocumentUpload({supplierId,supplierName,initialType,replacement,onCancel,onSaved}:{supplierId:string;supplierName:string;initialType:string;replacement?:SupplierDocument;onCancel:()=>void;onSaved:()=>void}){
 const [busy,setBusy]=useState(false),[error,setError]=useState(""),[fileError,setFileError]=useState(""),[type,setType]=useState(initialType);
 const fileInput=useRef<HTMLInputElement>(null);
 useEffect(()=>{fileInput.current?.focus();},[]);
 const fixedType=Boolean(initialType),name=documentTypeNames[type]??"document";
 async function submit(event:SubmitEvent<HTMLFormElement>){
  event.preventDefault();if(busy)return;
  const form=event.currentTarget,data=new FormData(form),file=data.get("file");
  if(!(file instanceof File)||file.size===0||file.size>15*1024*1024){setFileError("Choose a PDF or image up to 15 MiB.");return;}
  setBusy(true);setError("");
  try{await supplierRequest(`/api/settings/suppliers/${supplierId}/documents`,{method:"POST",body:data});onSaved();}
  catch(e){if(!displayServerErrors(form,e))setError(failureMessage(e));}
  finally{setBusy(false);}
 }
 return <ValidatedForm onSubmit={submit} className="space-y-3 rounded-xl border border-blue-200 bg-blue-50/40 p-4" aria-label={replacement?`Replace ${name}`:`Upload ${name}`}>
  <p className="break-words text-sm font-semibold">{supplierName}</p>
  <h4 className="font-semibold">{replacement?`Replace ${name}`:type?`Upload ${name}`:"Upload document"}</h4>
  <fieldset disabled={busy} className="space-y-3">
   {replacement&&<ValidatedInput type="hidden" name="replaces_document_id" value={replacement.id}/>}
   {fixedType?<ValidatedInput type="hidden" name="doc_type" value={initialType}/>:<ValidatedLabel required>Document type<ValidatedSelect name="doc_type" value={type} required onChange={e=>setType(e.target.value)} className="mt-1 block min-h-11 w-full rounded-lg border bg-white px-3"><option value="">Choose type</option>{Object.entries(documentTypeNames).map(([key,label])=><option key={key} value={key}>{label}</option>)}</ValidatedSelect></ValidatedLabel>}
   {replacement&&<p className="text-xs text-slate-600">Replacing {replacement.title||replacement.original_filename}. The previous file stays in history; other documents keep their approvals.</p>}
   {!replacement&&!fixedType&&type&&type!=="other"&&<p className="text-xs text-slate-600">An existing {name} file will be replaced automatically and kept in history.</p>}
   <ValidatedLabel required>PDF or image<ValidatedInput ref={fileInput} name="file" type="file" required accept=".pdf,.png,.jpg,.jpeg,.gif" error={fileError} onChange={()=>{setFileError("");setError("");}} className="mt-1 block min-h-11 w-full min-w-0 rounded-lg border bg-white p-2"/></ValidatedLabel>
   <p className="text-xs text-slate-500">PDF, PNG, JPEG or GIF · Up to 15 MiB</p>
   <details><summary className="min-h-11 cursor-pointer py-3 text-xs text-slate-600">Optional title</summary><ValidatedLabel>Title<ValidatedInput name="title" maxLength={255} defaultValue={replacement?.title??""} className="mt-1 block min-h-11 w-full rounded-lg border bg-white px-3"/></ValidatedLabel></details>
   {error&&<p role="alert" className="text-sm text-red-700">{error}</p>}
   {/* Keep Cancel from blurring the file control and moving the button as its error appears. */}
   <div className="flex flex-wrap justify-end gap-2"><button type="button" className="min-h-11 rounded-lg border px-4 font-semibold" onPointerDown={event=>event.preventDefault()} onClick={onCancel}>Cancel</button><button type="submit" className="min-h-11 rounded-lg bg-blue-700 px-4 font-semibold text-white">{busy?"Uploading…":replacement?"Upload replacement":"Upload document"}</button></div>
  </fieldset>
 </ValidatedForm>;
}
