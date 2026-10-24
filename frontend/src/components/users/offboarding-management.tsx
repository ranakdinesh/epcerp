"use client";
import {useEffect,useState,useRef,type SubmitEvent} from "react";
import {EyeIcon,PlusIcon} from "@heroicons/react/24/outline";
import {useAuthenticatedUser} from "@/components/auth-user-provider";
import {DataGrid} from "@/components/ui/data-grid";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {ValidatedForm,ValidatedLabel,ValidatedSelect,ValidatedTextarea} from "@/components/ui/validated-controls";
import {displayServerErrors} from "@/components/ui/server-field-errors";
type Work={source:string;kind:string;id:string;owner_id:string;title:string;state:string;link:string;version:number;required_permissions:string[]};
type Item={work:Work;replacement_id:string;approved_version:number};
type Case={id:string;user_id:string;manager_id:string;state:string;reason:string;version:number;items:Item[];approval_by_id:string;approved_at:string|null};
type Employee={id:string;username:string;first_name:string;last_name:string;is_active:boolean;manager_id:string;version:number};
type Change={id:string;action:string;reason:string;actor_name:string;version:number;created_at:string};
type Candidate={id:string;name:string;permissions:string[];is_superuser:boolean};
const button="inline-flex min-h-11 min-w-11 items-center justify-center gap-2 rounded-xl border border-slate-200 px-3 text-xs font-bold text-slate-700 focus:ring-2 focus:ring-blue-600 disabled:opacity-50";
const control="mt-2 min-h-11 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900";
export function OffboardingManagement(){
 const actor=useAuthenticatedUser();const admin=actor.permissions.includes("system:admin")||actor.permissions.includes("admin.offboarding.write");const directory=actor.permissions.includes("system:admin")||actor.permissions.includes("admin.organisation.read");
 const [cases,setCases]=useState<Case[]>([]);const [employees,setEmployees]=useState<Employee[]>([]);const [loading,setLoading]=useState(true);const [revision,setRevision]=useState(0);
 const [error,setError]=useState("");const [notice,setNotice]=useState("");const [formError,setFormError]=useState("");const [busy,setBusy]=useState(false);
 const historyRequest=useRef(0);
 const [history,setHistory]=useState<Change[]|null>(null);const [historyError,setHistoryError]=useState("");
 const [create,setCreate]=useState(false);const [selected,setSelected]=useState<Case|null>(null);const [candidates,setCandidates]=useState<Candidate[]>([]);const [choices,setChoices]=useState<Record<string,string>>({});
 const [action,setAction]=useState("plan");
 const key=(w:Work)=>`${w.source}:${w.kind}:${w.id}`;
 const name=(id:string)=>{const e=employees.find(v=>v.id===id);return e?([e.first_name,e.last_name].filter(Boolean).join(" ")||e.username):id;};
 useEffect(()=>{const c=new AbortController();const timer=setTimeout(async()=>{
  setLoading(true);setError("");try{
   const [res,people]=await Promise.all([fetch("/api/administration/organisation/offboarding",{cache:"no-store",signal:c.signal}),fetch(`/api/administration/organisation/${directory?"employees":"team"}`,{cache:"no-store",signal:c.signal})]);
   if(!res.ok||!people.ok)throw Error();setCases(await res.json() as Case[]);setEmployees(await people.json() as Employee[]);
  }catch{if(!c.signal.aborted)setError("Unable to load handover cases. Please refresh and try again.");}finally{if(!c.signal.aborted)setLoading(false);}
 },0);return()=>{clearTimeout(timer);c.abort();};},[revision,directory]);
 async function open(c:Case){
 historyRequest.current++;setHistory(null);setHistoryError("");
  setBusy(true);setFormError("");try{const [r,people]=await Promise.all([fetch(`/api/administration/organisation/offboarding/${c.id}`,{cache:"no-store"}),fetch(`/api/administration/organisation/offboarding/${c.id}/candidates`,{cache:"no-store"})]);if(!r.ok||!people.ok)throw Error();const fresh=await r.json() as Case;setSelected(fresh);setCandidates(await people.json() as Candidate[]);setChoices(Object.fromEntries(fresh.items.map(i=>[key(i.work),i.replacement_id])));setAction(fresh.manager_id===actor.id&&fresh.state==="planning"?"plan":"refresh");}catch{setError("Unable to open this case. Reporting authority may have changed.");}finally{setBusy(false);}
 }
 async function submit(event:SubmitEvent<HTMLFormElement>){
  event.preventDefault();if(busy)return;const form=event.currentTarget;const f=new FormData(form);setBusy(true);setFormError("");
  const reason=String(f.get("reason")??"").trim();const payload=create?{user_id:String(f.get("user_id")??""),reason}:{version:selected?.version,reason,...(action==="plan"?{selections:selected?.items.filter(i=>(choices[key(i.work)]??"")!==i.replacement_id).map(i=>({source:i.work.source,kind:i.work.kind,id:i.work.id,replacement_id:choices[key(i.work)]??""}))}:{})};
  const url=create?"/api/administration/organisation/offboarding":`/api/administration/organisation/offboarding/${selected?.id}/${action}`;
  try{const r=await fetch(url,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});if(!r.ok){const err=await r.json().catch(()=>({detail:"Unable to save handover."}));if(!displayServerErrors(form,err))setFormError(typeof err.detail==="string"?err.detail:"Check the entered values.");return;}
   const updated=await r.json() as Case;historyRequest.current++;setHistory(null);setCreate(false);setAction(updated.manager_id===actor.id&&updated.state==="planning"?"plan":"refresh");setSelected(updated);setChoices(Object.fromEntries(updated.items.map(i=>[key(i.work),i.replacement_id])));setRevision(v=>v+1);setNotice(updated.state==="completed"?"Manager-approved handover completed and account deactivated. Sessions are revoked.":"Handover case saved with immutable change history.");
   if(create){const people=await fetch(`/api/administration/organisation/offboarding/${updated.id}/candidates`,{cache:"no-store"});if(people.ok)setCandidates(await people.json() as Candidate[]);}
  }catch{setFormError("Unable to save. Please try again.");}finally{setBusy(false);}
 }
 const manager=selected?.manager_id===actor.id;const openCase=selected?.state==="planning"||selected?.state==="approved";
 return <section className="mx-auto w-full max-w-[1500px] space-y-5">
  <div className="flex flex-wrap items-end justify-between gap-3 border-b border-slate-200 pb-4"><div><p className="text-xs font-bold uppercase tracking-wider text-blue-700">Administration</p><h2 className="text-2xl font-extrabold text-slate-900">Handovers & deactivation</h2></div>{admin&&<button className={button+" bg-blue-700 text-white"} onClick={()=>{setCreate(true);setSelected(null);setFormError("");}}><PlusIcon className="h-4 w-4"/>Start offboarding</button>}</div>
  <p className="text-sm leading-6 text-slate-600">The reporting manager selects eligible replacements. Complete each handover in its owning workspace, then the manager approves and an administrator deactivates the account.</p>
  {notice&&<p role="status" className="rounded-xl bg-blue-50 p-3 text-sm text-blue-900">{notice}</p>}{error&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{error}</p>}
  <DataGrid label="Offboarding cases" rows={cases} loading={loading} stateKey="offboarding-cases" initialSort="employee" columns={[
   {key:"employee",label:"Employee",value:c=>name(c.user_id)},{key:"manager",label:"Reporting manager",value:c=>c.manager_id===actor.id?"You":name(c.manager_id)},{key:"state",label:"Status",value:c=>c.state},{key:"responsibilities",label:"Responsibility records",value:c=>c.items.length},{key:"approval",label:"Manager approval",value:c=>c.approved_at?new Date(c.approved_at).toLocaleDateString():"Pending"},
  ]} actions={c=><button className={button} disabled={busy} aria-label={`View handover for ${name(c.user_id)}`} onClick={()=>void open(c)}><EyeIcon className="h-4 w-4"/></button>}/>
  {(create||selected)&&<RecordDrawer title={create?"Start offboarding":`Handover · ${name(selected!.user_id)}`} onClose={()=>{if(!busy){historyRequest.current++;setCreate(false);setSelected(null);}}}>
   {selected&&<><dl className="grid gap-3 text-sm sm:grid-cols-2"><div><dt className="font-bold text-slate-500">Status</dt><dd>{selected.state}</dd></div><div><dt className="font-bold text-slate-500">Reporting manager</dt><dd>{selected.manager_id===actor.id?"You":name(selected.manager_id)}</dd></div><div className="sm:col-span-2"><dt className="font-bold text-slate-500">Departure reason</dt><dd>{selected.reason}</dd></div></dl>
    <p className="my-4 rounded-xl bg-blue-50 p-3 text-sm leading-6 text-blue-900">Save the manager’s replacement selections before changing source assignments. Source workspaces enforce their own permissions. Closed or revoked responsibilities retain their historical actors. Approval is blocked while any live work remains with the employee.</p>
    <DataGrid label="Handover responsibilities" rows={selected.items.map(i=>({...i,id:key(i.work)}))} stateKey="handover-items" initialSort="title" columns={[
     {key:"title",label:"Responsibility",value:i=>i.work.title,render:i=><div><span className="block font-bold">{i.work.title}</span><span className="text-xs text-slate-500">{i.work.kind.replaceAll("_"," ")}</span></div>},
     {key:"state",label:"Captured status",value:i=>i.work.state},
     {key:"replacement",label:"Manager-selected replacement",value:i=>choices[key(i.work)]??"",render:i=>manager&&selected.state==="planning"?<ValidatedSelect aria-label={`Replacement for ${i.work.title} (${i.work.kind})`} className={control} value={choices[key(i.work)]??""} onChange={e=>setChoices(v=>({...v,[key(i.work)]:e.target.value}))}><option value="">Select, or leave blank if closed</option>{candidates.filter(c=>c.is_superuser||i.work.required_permissions.every(p=>c.permissions.includes(p))).map(c=><option key={c.id} value={c.id}>{c.name}</option>)}</ValidatedSelect>:<span>{candidates.find(c=>c.id===i.replacement_id)?.name??(i.replacement_id||"Closed / not selected")}</span>},
    ]} actions={i=><a className={button} href={i.work.link} target="_blank" rel="noopener noreferrer">Open source</a>}/>
   </>}
   {selected&&<div className="mt-5 space-y-3"><button type="button" className={button} onClick={async()=>{const request=++historyRequest.current;setHistoryError("");try{const r=await fetch(`/api/administration/organisation/offboarding/${selected.id}/history`,{cache:"no-store"});if(!r.ok)throw Error();const rows=await r.json() as Change[];if(request===historyRequest.current)setHistory(rows);}catch{if(request===historyRequest.current)setHistoryError("Unable to load handover history.");}}}>View change history</button>{historyError&&<p role="alert" className="text-sm text-red-700">{historyError}</p>}{history&&<DataGrid label="Handover change history" rows={history} stateKey="handover-history" initialSort="version" columns={[{key:"version",label:"Version",value:h=>h.version},{key:"action",label:"Action",value:h=>h.action},{key:"actor",label:"Recorded by",value:h=>h.actor_name},{key:"reason",label:"Reason",value:h=>h.reason},{key:"at",label:"Recorded at",value:h=>new Date(h.created_at).toLocaleString()}]}/>}</div>}
   {(create||(openCase&&(admin||manager)))&&<ValidatedForm key={`${selected?.id??"new"}-${selected?.version??0}-${action}`} onSubmit={submit} className="mt-5 space-y-5">
    {create?<><p className="text-sm text-slate-600">Set up the employee’s department and reporting manager in Organisation before starting a handover.</p><ValidatedLabel required className="block text-sm font-bold">Employee<ValidatedSelect name="user_id" required className={control}><option value="">Select employee with a reporting manager</option>{employees.filter(e=>e.is_active&&e.manager_id&&e.id!==actor.id).map(e=><option key={e.id} value={e.id}>{name(e.id)} ({e.username})</option>)}</ValidatedSelect></ValidatedLabel></>:<ValidatedLabel required className="block text-sm font-bold">Next action<ValidatedSelect value={action} onChange={e=>{setAction(e.target.value);setFormError("");}} className={control}>
     {manager&&selected?.state==="planning"&&<><option value="plan">Save replacement selections</option><option value="approve">Approve completed handover</option></>}
     {admin&&<><option value="refresh">Refresh inventory and return for manager review</option>{selected?.state==="approved"&&<option value="deactivate">Finalize handover and deactivate account</option>}<option value="cancel">Cancel offboarding case</option></>}
    </ValidatedSelect></ValidatedLabel>}
    {!create&&action==="approve"&&<p className="text-sm text-slate-600">Confirm that all live responsibilities have been transferred to your selected replacements or properly closed. The server verifies the current assignments.</p>}
    {!create&&action==="deactivate"&&<p className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900">This revokes access immediately after rechecking the manager’s approval, current assignments and replacement eligibility.</p>}
    <ValidatedLabel required className="block text-sm font-bold">{create?"Departure reason":"Decision / change reason"}<ValidatedTextarea name="reason" required minLength={10} maxLength={1000} rows={3} className={control}/></ValidatedLabel>
    {formError&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{formError}</p>}
    <div className="flex justify-end gap-2 border-t border-slate-200 pt-4"><button type="button" disabled={busy} className={button} onClick={()=>{setCreate(false);setSelected(null);}}>Close</button><button type="submit" disabled={busy} className={button+" bg-blue-700 text-white"}>{busy?"Saving…":create?"Start handover":action==="deactivate"?"Deactivate account":action==="approve"?"Approve handover":"Save decision"}</button></div>
   </ValidatedForm>}
  </RecordDrawer>}
 </section>;
}
