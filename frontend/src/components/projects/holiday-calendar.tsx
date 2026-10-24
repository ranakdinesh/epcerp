"use client";
import { useEffect, useState, type FormEvent } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
import { displayServerErrors } from "@/components/ui/server-field-errors";

type Holiday = {id:string; name:string; holiday_date:string; is_active:boolean};
export function HolidayCalendar() {
  const user = useAuthenticatedUser();
  const canWrite = user.permissions.includes("system:admin") || user.permissions.includes("settings.holidays.write");
  const [rows,setRows] = useState<Holiday[]>([]);
  const [year,setYear] = useState(String(new Date().getFullYear()));
  const [reload,setReload] = useState(0);
  const [notice,setNotice] = useState("");
  const [error,setError] = useState("");
  const [loading,setLoading] = useState(true);
  const [saving,setSaving] = useState(false);
  const [edit,setEdit] = useState<Partial<Holiday> | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    void fetch(`/api/work-calendar/holidays${year ? `?year=${year}` : ""}`,{cache:"no-store",signal:controller.signal})
      .then(async response => {if (!response.ok) throw new Error("Unable to load holidays."); return response.json();})
      .then(setRows).catch(e=>{if (!controller.signal.aborted) setError(e.message);})
      .finally(()=>{if (!controller.signal.aborted) setLoading(false);});
    return ()=>controller.abort();
  },[year,reload]);
  async function save(event:FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!edit || saving) return;
    const form=event.currentTarget; setSaving(true); setError("");
    try {
      const response=await fetch(`/api/work-calendar/holidays${edit.id ? `/${edit.id}` : ""}`,{
        method:edit.id ? "PATCH" : "POST",headers:{"Content-Type":"application/json"},
        body:JSON.stringify({name:edit.name,holiday_date:edit.holiday_date,is_active:edit.is_active}),
      });
      const result=await response.json();
      if (!response.ok) { if (!displayServerErrors(form,result)) setError(typeof result.detail === "string" ? result.detail : "Unable to save holiday."); return; }
      setNotice(`Holiday saved. ${result.recalculated_projects} project working-day counts recalculated.`);
      setEdit(null); setReload(v=>v+1);
    } catch {setError("Unable to reach the server. Your holiday changes are retained.");}
    finally {setSaving(false);}
  }
  return <section className="mx-auto max-w-[1500px] space-y-5">
    <div className="flex flex-wrap items-center justify-between gap-3"><div><h2 className="text-2xl font-extrabold">Holidays</h2><p className="mt-1 text-sm text-slate-500">Active holidays reduce project working days. Sundays and second/fourth Saturdays are already excluded.</p></div>{canWrite ? <button className="min-h-11 rounded-xl bg-blue-700 px-4 font-bold text-white" onClick={()=>{setError("");setEdit({name:"",holiday_date:"",is_active:true});}}>Add holiday</button>:null}</div>
    {notice ? <p role="status" className="rounded-xl bg-blue-50 p-4 text-blue-800">{notice}</p>:null}
    {error ? <p role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">{error}</p>:null}
    <DataGrid label="Holidays" rows={rows} loading={loading} status={row=>row.is_active ? "Active":"Inactive"} initialSort="date"
      toolbar={<ValidatedLabel className="text-sm font-bold text-slate-700">Calendar year<ValidatedSelect aria-label="Holiday year" className="field" value={year} onChange={event=>{setLoading(true);setYear(event.target.value);}}><option value="">All years</option>{Array.from(new Set([...rows.map(row=>row.holiday_date.slice(0,4)),...Array.from({length:11},(_,i)=>String(new Date().getFullYear()-5+i))])).sort().map(value=><option key={value}>{value}</option>)}</ValidatedSelect></ValidatedLabel>}
      columns={[{key:"date",label:"Date",value:row=>row.holiday_date},{key:"name",label:"Holiday",value:row=>row.name},{key:"active",label:"Status",value:row=>row.is_active ? "Active":"Inactive"}]}
      actions={row=>canWrite ? <button className="min-h-11 rounded-lg px-3 font-bold text-blue-700" aria-label={`Edit holiday ${row.name}`} onClick={()=>{setError("");setEdit(row);}}>Edit</button>:null}/>
    {edit ? <RecordDrawer title={edit.id ? "Edit holiday":"Add holiday"} onClose={()=>{if(!saving)setEdit(null);}}
      footer={<div className="flex justify-end gap-2"><button disabled={saving} className="min-h-11 rounded-xl border px-4" onClick={()=>setEdit(null)}>Cancel</button><button disabled={saving} type="submit" form="holiday-form" className="min-h-11 rounded-xl bg-blue-700 px-4 font-bold text-white">{saving ? "Saving...":"Save holiday"}</button></div>}>
      <ValidatedForm id="holiday-form" onSubmit={event=>void save(event)}><fieldset disabled={saving} className="grid gap-4">
        <ValidatedLabel>Holiday name<ValidatedInput name="name" required minLength={2} maxLength={128} className="field" value={edit.name??""} onChange={event=>setEdit({...edit,name:event.target.value})}/></ValidatedLabel>
        <ValidatedLabel>Holiday date<ValidatedInput name="holiday_date" required type="date" className="field" value={edit.holiday_date??""} onChange={event=>setEdit({...edit,holiday_date:event.target.value})}/></ValidatedLabel>
        <ValidatedLabel className="flex min-h-11 items-center gap-2"><ValidatedInput type="checkbox" checked={edit.is_active??true} onChange={event=>setEdit({...edit,is_active:event.target.checked})}/>Active</ValidatedLabel>
        <p className="text-sm text-slate-500">Saving recalculates every project whose LOI-to-completion dates include the changed holiday. Inactive holidays do not exclude a working day.</p>
      </fieldset></ValidatedForm>
    </RecordDrawer>:null}
  </section>;
}
