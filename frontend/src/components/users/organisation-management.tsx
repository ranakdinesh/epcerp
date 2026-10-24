"use client";
import {useEffect,useState,useRef,type SubmitEvent} from "react";
import {useRouter} from "next/navigation";
import {EyeIcon,PencilSquareIcon,PlusIcon} from "@heroicons/react/24/outline";
import {useAuthenticatedUser} from "@/components/auth-user-provider";
import {DataGrid} from "@/components/ui/data-grid";
import {RecordDrawer} from "@/components/ui/record-drawer";
import {ValidatedForm,ValidatedInput,ValidatedLabel,ValidatedSelect,ValidatedTextarea} from "@/components/ui/validated-controls";
import {displayServerErrors} from "@/components/ui/server-field-errors";
type Employee={id:string;username:string;email:string;first_name:string;last_name:string;mobile:string;department_id:string;manager_id:string;is_active:boolean;is_company_root:boolean;version:number};
type Department={id:string;code:string;name:string;manager_id:string;is_active:boolean;version:number};
const control="mt-2 min-h-11 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-slate-900";
const action="inline-flex min-h-11 min-w-11 items-center justify-center rounded-xl border border-slate-200 px-3 text-xs font-bold text-slate-700 hover:bg-blue-50 focus:ring-2 focus:ring-blue-600 disabled:opacity-50";
type Work={id:string;source:string;kind:string;title:string;state:string;link:string;due_date:string};
const name=(e:Employee)=>[e.first_name,e.last_name].filter(Boolean).join(" ")||e.username;
export function OrganisationManagement({team=false}:{team?:boolean}){
 const actor=useAuthenticatedUser();const router=useRouter();const write=!team&&(actor.permissions.includes("system:admin")||actor.permissions.includes("admin.organisation.write"));
 const [employees,setEmployees]=useState<Employee[]>([]);const [departments,setDepartments]=useState<Department[]>([]);
 const [tab,setTab]=useState<"employees"|"departments">("employees");const [loading,setLoading]=useState(true);const [revision,setRevision]=useState(0);
 const [error,setError]=useState("");const [notice,setNotice]=useState("");const [busy,setBusy]=useState(false);
 const workRequest=useRef(0);
 const [work,setWork]=useState<Work[]>([]);const [workLoading,setWorkLoading]=useState(false);const [workError,setWorkError]=useState("");
 const [dialog,setDialog]=useState<{kind:"employee"|"department"|"work";view:boolean;employee?:Employee;department?:Department}|null>(null);const [formError,setFormError]=useState("");const [root,setRoot]=useState(false);
 useEffect(()=>{const c=new AbortController();const timer=window.setTimeout(async()=>{
  setLoading(true);setError("");try{
   const response=await fetch(`/api/administration/organisation/${team?"team":"employees"}`,{cache:"no-store",signal:c.signal});if(!response.ok)throw Error();
   const values=await response.json() as Employee[];
   let deps:Department[]=[];if(!team){const d=await fetch("/api/administration/organisation/departments",{cache:"no-store",signal:c.signal});if(!d.ok)throw Error();deps=await d.json() as Department[];}
   setEmployees(values);setDepartments(deps);
  }catch{if(!c.signal.aborted)setError("Unable to load the organisation directory. Please refresh and try again.");}finally{if(!c.signal.aborted)setLoading(false);}
 },0);return()=>{clearTimeout(timer);c.abort();};},[revision,team]);
 const byID=new Map(employees.map(e=>[e.id,e]));const departmentByID=new Map(departments.map(d=>[d.id,d]));
 const managerName=(id:string)=>id===actor.id?"You":id?(byID.has(id)?name(byID.get(id)!):"Outside current view"):"Not assigned";
 function open(d:NonNullable<typeof dialog>){setDialog(d);setRoot(d.employee?.is_company_root??false);setFormError("");}
 async function openWork(employee:Employee){const request=++workRequest.current;setDialog({kind:"work",view:true,employee});setWork([]);setWorkLoading(true);setWorkError("");try{const r=await fetch(`/api/administration/organisation/team/${employee.id}/work`,{cache:"no-store"});if(!r.ok)throw Error();const rows=await r.json() as Work[];if(request!==workRequest.current)return;setWork(rows.map(w=>({...w,id:`${w.source}:${w.kind}:${w.id}`})));}catch{if(request===workRequest.current)setWorkError("Unable to load this team member’s work. Reporting lines may have changed; refresh and try again.");}finally{if(request===workRequest.current)setWorkLoading(false);}}
 async function save(event:SubmitEvent<HTMLFormElement>){
  event.preventDefault();if(busy||!dialog)return;const form=event.currentTarget;const f=new FormData(form);const str=(k:string)=>String(f.get(k)??"").trim();
  const dep=dialog.kind==="department";const id=dep?dialog.department?.id:dialog.employee?.id;
  const payload=dep?{code:str("code"),name:str("name"),manager_id:str("manager_id"),is_active:f.get("is_active")==="on",version:dialog.department?.version??0,reason:str("reason")}:{first_name:str("first_name"),last_name:str("last_name"),mobile:str("mobile"),department_id:str("department_id"),manager_id:root?"":str("manager_id"),is_company_root:root,version:dialog.employee?.version??0,reason:str("reason")};
  setBusy(true);setFormError("");try{
   const response=await fetch(`/api/administration/organisation/${dep?"departments":"employees"}${id?`/${id}`:""}`,{method:id?"PATCH":"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
   if(!response.ok){const err=await response.json().catch(()=>({detail:"Unable to save organisation record."}));if(!displayServerErrors(form,err))setFormError(typeof err.detail==="string"?err.detail:"Check the entered values.");return;}
   setDialog(null);if(!dep&&id===actor.id)router.refresh();setNotice(dep?"Department saved with change history.":"Employee profile and reporting line saved with change history.");setRevision(v=>v+1);
  }catch{setFormError("Unable to save. Please try again.");}finally{setBusy(false);}
 }
 const employee=dialog?.employee;const department=dialog?.department;
 return <section className="mx-auto w-full max-w-[1500px] space-y-5">
  <div className="flex flex-wrap items-end justify-between gap-3 border-b border-slate-200 pb-4"><div><p className="text-xs font-bold uppercase tracking-wider text-blue-700">{team?"Reporting chain":"Administration"}</p><h2 className="text-2xl font-extrabold text-slate-900">{team?"My team":"Organisation"}</h2></div><div className="flex flex-wrap gap-2">{!team&&<><button type="button" aria-pressed={tab==="employees"} className={action} onClick={()=>setTab("employees")}>Employee profiles</button><button type="button" aria-pressed={tab==="departments"} className={action} onClick={()=>setTab("departments")}>Departments</button></>}{write&&tab==="departments"&&<button className={action+" gap-2 bg-blue-700 text-white"} onClick={()=>open({kind:"department",view:false})}><PlusIcon className="h-4 w-4"/>Add department</button>}</div></div>
  <p className="text-sm text-slate-600">{team?"Your direct and indirect reports, based on current reporting lines.":"Set up departments, employee contact details and reporting managers. Department membership does not grant permissions."}</p>
  {notice&&<p role="status" className="rounded-xl bg-blue-50 p-3 text-blue-900">{notice}</p>}{error&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-red-800">{error}</p>}
  {tab==="employees"?<DataGrid label={team?"My reporting team":"Employee profiles"} rows={employees} loading={loading} stateKey={team?"reporting-team":"organisation-employees"} initialSort="name" status={e=>e.is_active?"Active":"Inactive"} columns={[
   {key:"name",label:"Employee",value:name,render:e=><div><span className="block font-bold">{name(e)}</span><span className="block text-xs text-slate-500">{e.username} · {e.email}</span></div>},
   ...(!team?[{key:"department",label:"Department",value:(e:Employee)=>departmentByID.get(e.department_id)?.name??"Setup required"}]:[]),
   {key:"manager",label:"Reports to",value:e=>e.is_company_root?"Company root":managerName(e.manager_id)},
   {key:"profile",label:"Profile",value:e=>e.version>0?"Configured":"Setup required"},
   {key:"status",label:"Status",value:e=>e.is_active?"Active":"Inactive"},
  ]} actions={e=><div className="flex justify-end gap-1">{team&&<button className={action} aria-label={`View work for ${e.username}`} onClick={()=>void openWork(e)}>View work</button>}<button className={action} aria-label={`View profile for ${e.username}`} onClick={()=>open({kind:"employee",view:true,employee:e})}><EyeIcon className="h-4 w-4"/></button>{write&&e.is_active&&<button className={action} aria-label={`Edit profile for ${e.username}`} onClick={()=>open({kind:"employee",view:false,employee:e})}><PencilSquareIcon className="h-4 w-4"/></button>}</div>}/>:<DataGrid label="Departments" rows={departments} loading={loading} stateKey="organisation-departments" initialSort="name" status={d=>d.is_active?"Active":"Inactive"} columns={[
   {key:"code",label:"Code",value:d=>d.code},{key:"name",label:"Department",value:d=>d.name},{key:"manager",label:"Department manager",value:d=>managerName(d.manager_id)},{key:"status",label:"Status",value:d=>d.is_active?"Active":"Inactive"},
  ]} actions={d=><div className="flex justify-end gap-1"><button className={action} aria-label={`View ${d.name}`} onClick={()=>open({kind:"department",view:true,department:d})}><EyeIcon className="h-4 w-4"/></button>{write&&<button className={action} aria-label={`Edit ${d.name}`} onClick={()=>open({kind:"department",view:false,department:d})}><PencilSquareIcon className="h-4 w-4"/></button>}</div>}/>}
  {dialog&&<RecordDrawer title={dialog.kind==="department"?(department?department.name:"Add department"):(employee?name(employee):"Employee profile")} onClose={()=>{if(!busy){workRequest.current++;setDialog(null);}}}>
   {dialog.kind==="work"?<div className="space-y-4"><p className="text-sm text-slate-600">Live responsibility status for {employee?name(employee):"team member"}. Source workspaces check their own access permissions.</p>{workError&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-red-800">{workError}</p>}<DataGrid label="Team member work" rows={work} loading={workLoading} stateKey="team-member-work" initialSort="title" columns={[{key:"title",label:"Work",value:w=>w.title},{key:"kind",label:"Responsibility",value:w=>w.kind.replaceAll("_"," ")},{key:"state",label:"Status",value:w=>w.state},{key:"due",label:"Due date",value:w=>w.due_date||"—"}]} actions={w=><a className={action} href={w.link} target="_blank" rel="noopener noreferrer">Open source</a>}/></div>:dialog.view?<dl className="space-y-3 text-sm">{(dialog.kind==="department"?[["Code",department?.code],["Name",department?.name],["Department manager",managerName(department?.manager_id??"")],["Status",department?.is_active?"Active":"Inactive"]]:[["First name",employee?.first_name||"Setup required"],["Last name",employee?.last_name||"Setup required"],["Email",employee?.email],["Mobile",employee?.mobile||"Setup required"],["Department",departmentByID.get(employee?.department_id??"")?.name??"Setup required"],["Reports to",employee?.is_company_root?"Company root":managerName(employee?.manager_id??"")],["Status",employee?.is_active?"Active":"Inactive"]]).map(([k,v])=><div key={k}><dt className="font-bold text-slate-500">{k}</dt><dd>{v}</dd></div>)}</dl>:<ValidatedForm onSubmit={save} className="space-y-5">
    {dialog.kind==="department"?<>
     <ValidatedLabel required className="block text-sm font-bold">Code<ValidatedInput name="code" required minLength={2} maxLength={32} pattern="[A-Za-z0-9_\-]+" defaultValue={department?.code} className={control}/></ValidatedLabel>
     <ValidatedLabel required className="block text-sm font-bold">Department name<ValidatedInput name="name" required minLength={2} maxLength={150} defaultValue={department?.name} className={control}/></ValidatedLabel>
     <ValidatedLabel className="block text-sm font-bold">Department manager<ValidatedSelect name="manager_id" defaultValue={department?.manager_id??""} className={control}><option value="">Pending assignment</option>{employees.filter(e=>e.is_active).map(e=><option key={e.id} value={e.id}>{name(e)} ({e.username})</option>)}</ValidatedSelect></ValidatedLabel>
     <ValidatedLabel className="flex min-h-11 items-center gap-3 text-sm font-bold"><ValidatedInput name="is_active" type="checkbox" defaultChecked={department?.is_active??true}/>Active department</ValidatedLabel>
    </>:<>
     <div className="grid gap-3 sm:grid-cols-2"><ValidatedLabel required className="block text-sm font-bold">First name<ValidatedInput name="first_name" required maxLength={100} defaultValue={employee?.first_name} className={control}/></ValidatedLabel><ValidatedLabel required className="block text-sm font-bold">Last name<ValidatedInput name="last_name" required maxLength={100} defaultValue={employee?.last_name} className={control}/></ValidatedLabel></div>
     <ValidatedLabel required className="block text-sm font-bold">Mobile number<ValidatedInput name="mobile" type="tel" required minLength={8} maxLength={24} defaultValue={employee?.mobile} className={control}/></ValidatedLabel>
     <ValidatedLabel required className="block text-sm font-bold">Department<ValidatedSelect name="department_id" required defaultValue={employee?.department_id??""} className={control}><option value="">Select department</option>{departments.filter(d=>d.is_active).map(d=><option key={d.id} value={d.id}>{d.name}</option>)}</ValidatedSelect></ValidatedLabel>
     <ValidatedLabel className="flex min-h-11 items-center gap-3 text-sm font-bold"><ValidatedInput type="checkbox" checked={root} onChange={e=>setRoot(e.target.checked)}/>Company root (no reporting manager)</ValidatedLabel>
     {!root&&<ValidatedLabel required className="block text-sm font-bold">Reporting manager<ValidatedSelect name="manager_id" required defaultValue={employee?.manager_id??""} className={control}><option value="">Select reporting manager</option>{employees.filter(e=>e.is_active&&e.version>0&&e.id!==employee?.id).map(e=><option key={e.id} value={e.id}>{name(e)} ({e.username})</option>)}</ValidatedSelect><span className="mt-1 block text-xs font-normal text-slate-500">Set up the manager’s organisation profile first.</span></ValidatedLabel>}
    </>}
    <ValidatedLabel required className="block text-sm font-bold">Change reason<ValidatedTextarea name="reason" required minLength={10} maxLength={1000} rows={3} className={control}/></ValidatedLabel>
    {formError&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{formError}</p>}
    <div className="flex justify-end gap-2 border-t border-slate-200 pt-4"><button type="button" disabled={busy} className={action} onClick={()=>setDialog(null)}>Cancel</button><button type="submit" disabled={busy} className={action+" bg-blue-700 text-white"}>{busy?"Saving…":"Save changes"}</button></div>
   </ValidatedForm>}
  </RecordDrawer>}
 </section>;
}
