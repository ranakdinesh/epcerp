"use client";
import {useEffect,useState,type SubmitEvent} from "react";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {ValidatedForm,ValidatedLabel,ValidatedTextarea} from "@/components/ui/validated-controls";
import {displayServerErrors} from "@/components/ui/server-field-errors";
import {ProjectCaptureFields} from "./project-capture-fields";
import {metadataPayload} from "./project-metadata";
type Project=Record<string,unknown>&{version:number;name:string;client_id:string};
export function ProjectMetadataEditor({id,onClose,onSaved}:{id:string;onClose:()=>void;onSaved:()=>void}){
 const [project,setProject]=useState<Project|null>(null),[clients,setClients]=useState<{id:string;name:string}[]>([]),[error,setError]=useState(""),[saving,setSaving]=useState(false),[stale,setStale]=useState(false),[revision,setRevision]=useState(0);
 useEffect(()=>{const c=new AbortController();async function read(path:string){const r=await fetch(path,{signal:c.signal,cache:"no-store"});if(!r.ok)throw Error("Project could not be loaded for editing. Check project write access.");return r.json()}
 Promise.all([read(`/api/projects/${id}`),read(`/api/projects/${id}/metadata-options`)]).then(([p,o])=>{if(!c.signal.aborted){setProject(p);setClients(o.clients);setStale(false);setError("")}}).catch(e=>{if(!c.signal.aborted)setError(e.message)});return()=>c.abort()},[id,revision]);
 async function submit(e:SubmitEvent<HTMLFormElement>){e.preventDefault();if(!project)return;const form=e.currentTarget;setSaving(true);setError("");try{
 const r=await fetch(`/api/projects/${id}`,{method:"PATCH",headers:{"Content-Type":"application/json"},body:JSON.stringify(metadataPayload(new FormData(form),project.version))});const result=await r.json();
 if(!r.ok){if(r.status===409)setStale(true);if(displayServerErrors(form,result))return;setError(typeof result.detail==="string"?result.detail:"Unable to save project details");return}
 onSaved();onClose();
 }catch{setError("Service unavailable. Refresh the record before retrying.")}finally{setSaving(false)}}
 return <RecordDrawer title={project?`Edit ${project.name}`:"Edit project"} onClose={()=>{if(!saving)onClose()}} footer={<div className="flex justify-end gap-3"><button type="button" className="min-h-11 rounded-lg border px-4" disabled={saving} onClick={onClose}>Cancel</button><button form="project-metadata-form" type="submit" disabled={!project||saving||stale} className="min-h-11 rounded-xl bg-blue-700 px-5 py-2 font-semibold text-white disabled:opacity-50">{saving?"Saving…":"Save changes"}</button></div>}>
 {error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-sm text-red-700">{error}</p>}
 {stale&&<button className="min-h-11 rounded-lg border px-4" onClick={()=>{setProject(null);setRevision(v=>v+1)}}>Reload latest project</button>}
 {!project&&!error&&<p role="status">Loading saved project details…</p>}
 {project&&<ValidatedForm id="project-metadata-form" onSubmit={submit}><fieldset disabled={saving||stale} className="space-y-4">
 <p className="text-sm">Draft fields may remain pending. Correct entered values and record a reason. Ownership changes use their dedicated handover actions.</p>
 <ProjectCaptureFields key={revision} editing initial={Object.fromEntries(Object.entries(project).filter(([k,v])=>typeof v==="string"||(k==="purchase_enabled"&&typeof v==="boolean")).map(([k,v])=>[k,String(v)]))} clients={clients} managers={[]} procurement={[]}/>
 <ValidatedLabel className="block text-sm font-semibold">Correction reason<ValidatedTextarea name="reason" required maxLength={2000} rows={3} className="mt-1 block min-h-11 w-full rounded-lg border px-3 py-2"/></ValidatedLabel>
 </fieldset></ValidatedForm>}
 </RecordDrawer>
}
