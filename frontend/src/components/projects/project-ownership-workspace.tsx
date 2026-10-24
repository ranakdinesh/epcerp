"use client";
import {ProjectQuotationStatus} from "./project-quotation-status";
import {ProjectProcurementOwnership} from "./project-procurement-ownership";
import {ProjectBudgetPanel} from "./project-budget-panel";
import {ProjectResponsibilities} from "./project-responsibilities";
import {ProjectMetadataEditor} from "./project-metadata-editor";
import {ProjectCaptureFields,captureNames} from "./project-capture-fields";
import { UserIdentity } from "@/components/personal-profile";

import { useEffect, useState, type SubmitEvent } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid, type GridColumn } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { displayServerErrors } from "@/components/ui/server-field-errors";

type Project = { purchase_enabled?:boolean; schedule_calculation?: Record<string,number|string|null>; id: string; name: string; project_number: string; client_id: string; project_manager_id: string; lifecycle_status: string; version: number; created_at: string } & Partial<Record<typeof captureNames[number],string>>;
type Assignment = { id: string; previous_manager_id: string | null; project_manager_id: string; reason: string; project_version: number; created_at: string; created_by_id: string | null };
type Choice = { id: string; name: string };
type Options = { clients: Choice[]; managers: Choice[]; procurement_executives:Choice[] };
const emptyOptions: Options = { clients: [], managers: [], procurement_executives:[] };
const inputClass = "mt-1 block min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm";
const buttonClass = "min-h-11 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold disabled:opacity-50";
function message(body: unknown, fallback: string): string {
  if (body && typeof body === "object") {
    if ("detail" in body && typeof body.detail === "string") return body.detail;
    if ("message" in body && typeof body.message === "string") return body.message;
  }
  return fallback;
}
async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, { cache: "no-store", signal });
  const body: unknown = await response.json();
  if (!response.ok) throw new Error(message(body, "Unable to load project records."));
  return body as T;
}
export function ProjectOwnershipWorkspace() {
  const user = useAuthenticatedUser();
  const allows = (code: string) => user.permissions.includes("system:admin") || user.permissions.includes(code);
  const canRead = allows("projects.master.read");
  const canCreate = canRead && allows("projects.master.write") && allows("projects.portfolio.write") && allows("projects.purchase_assignments.write");
  const canTransfer = canRead && allows("projects.manager_assignments.write");
  const [rows, setRows] = useState<Project[]>([]);
  const [options, setOptions] = useState<Options>(emptyOptions);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [search, setSearch] = useState("");
  const [total, setTotal] = useState(0);
  const [editID,setEditID]=useState<string|null>(null);
  const [notice,setNotice]=useState("");
  const [budgetID,setBudgetID]=useState<string|null>(null);
  const [teamID,setTeamID]=useState<string|null>(null);
  const canEdit=(p:Project)=>allows("projects.master.write")&&(p.project_manager_id===user.id||allows("projects.portfolio.write"))&&p.lifecycle_status==="draft";
  const [panel, setPanel] = useState<string | null>(null);
  useEffect(() => {
    if (!canRead) return;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      setLoading(true); setError("");
      const query = new URLSearchParams({ page: String(page), page_size: String(size), search });
      Promise.all([
        get<{ items: Project[]; total: number }>(`/api/projects?${query}`, controller.signal),
        canCreate || canTransfer ? get<Options>("/api/projects/assignment-options", controller.signal) : Promise.resolve(emptyOptions),
      ]).then(([result, choices]) => { setRows(result.items); setTotal(result.total); setOptions(choices); })
        .catch(e => { if (!controller.signal.aborted) { setError(e instanceof Error ? e.message : "Unable to load projects."); setRows([]); } })
        .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    }, search ? 250 : 0);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [canRead, canCreate, canTransfer, page, size, search, reload]);
  const manager = (id: string) => options.managers.find(v => v.id === id)?.name ?? (id === user.id ? user.username : id);
  const client = (id: string) => options.clients.find(v => v.id === id)?.name ?? id;
  const columns: GridColumn<Project>[] = [
    { key: "number", label: "Project number", value: p => p.project_number },
    { key: "name", label: "Project", value: p => p.name },
    { key: "client", label: "Client", value: p => client(p.client_id) },
    { key: "manager", label: "Current manager", value: p => manager(p.project_manager_id), render: p => <UserIdentity id={p.project_manager_id} name={manager(p.project_manager_id)} /> },
    { key: "state", label: "Status", value: p => p.lifecycle_status, render: () => <span className="rounded bg-slate-100 px-2 py-1 text-xs font-semibold">Draft</span> },
  ];
  return <section className="space-y-5">
    <div className="flex flex-wrap items-start justify-between gap-3"><div><p className="go-eyebrow">Project delivery</p><h1 className="text-2xl font-bold">Projects</h1></div>{canCreate && <button className="min-h-11 rounded-xl bg-blue-700 px-4 py-2 text-sm font-semibold text-white" onClick={() => setPanel("create")}>Create project</button>}</div>
    <p className="rounded-lg border border-blue-100 bg-blue-50 p-3 text-sm text-blue-950">Project activation requires contract, schedule and finance readiness.</p>
    {!canRead ? <p role="alert">Project read permission is required. Contact your administrator.</p> : <>
      <div className="flex flex-wrap items-end justify-between gap-3"><ValidatedLabel className="text-sm font-semibold">Search projects<ValidatedInput className={inputClass} value={search} maxLength={255} onChange={e => { setSearch(e.target.value); setPage(1); }} placeholder="Name or project number" /></ValidatedLabel><button className={buttonClass} onClick={() => setReload(v => v + 1)}>Refresh projects</button></div>
      {notice&&<p role="status" className="rounded-lg bg-green-50 p-3 text-green-800">{notice}</p>}
      {error && <p role="alert" className="text-red-700">{error}</p>}
      <DataGrid label="Projects" rows={rows} columns={columns} loading={loading} searchable={false} paginate={false} actions={p => <div className="flex gap-2"><button className={buttonClass} aria-label={`View ${p.name}`} onClick={() => setPanel(p.id)}>View & history</button>{allows("projects.budgets.read")&&<button className={buttonClass} aria-label={`Budget ${p.name}`} onClick={()=>setBudgetID(p.id)}>Budget</button>}{allows("projects.team.read")&&<button className={buttonClass} aria-label={`Team ${p.name}`} onClick={()=>setTeamID(p.id)}>Team</button>}{canEdit(p)&&<button className={buttonClass} aria-label={`Edit ${p.name}`} onClick={()=>{setNotice("");setEditID(p.id)}}>Edit</button>}</div>} />
      <div className="flex flex-wrap items-center justify-between gap-3 text-sm"><p>{total ? `${(page - 1) * size + 1}–${Math.min(page * size, total)} of ${total} projects` : "No registered projects in your scope."}</p><div className="flex items-center gap-2"><ValidatedLabel>Rows <ValidatedSelect className="min-h-11 rounded border p-2" value={size} onChange={e => { setSize(Number(e.target.value)); setPage(1); }}>{[20,50,100].map(n => <option key={n}>{n}</option>)}</ValidatedSelect></ValidatedLabel><button className={buttonClass} disabled={page === 1 || loading} onClick={() => setPage(p => p - 1)}>Previous</button><button className={buttonClass} disabled={page * size >= total || loading} onClick={() => setPage(p => p + 1)}>Next</button></div></div>
    </>}
    {budgetID&&<RecordDrawer title="Project budget" onClose={()=>setBudgetID(null)} wide><ProjectBudgetPanel key={budgetID} projectId={budgetID}/></RecordDrawer>}
    {teamID&&<ProjectResponsibilities key={teamID} projectId={teamID} onClose={()=>setTeamID(null)}/>}
    {editID&&<ProjectMetadataEditor id={editID} onClose={()=>setEditID(null)} onSaved={()=>{setReload(v=>v+1);setNotice("Project details saved.")}}/>}
    {panel && <OwnershipPanel key={panel} id={panel} onEdit={(p)=>{setPanel(null);setEditID(p.id)}} canEdit={canEdit} canTransfer={canTransfer} canProcurement={allows("projects.purchase_workflow.view")&&allows("projects.purchase_workflow.read")} options={options} managerName={manager} onClose={() => setPanel(null)} onSaved={() => setReload(v => v + 1)} />}
  </section>;
}
function OwnershipPanel({ id, onEdit,canEdit,canTransfer, canProcurement, options, managerName, onClose, onSaved }: { id: string; onEdit:(p:Project)=>void;canEdit:(p:Project)=>boolean;canTransfer: boolean; canProcurement:boolean; options: Options; managerName: (id: string) => string; onClose: () => void; onSaved: () => void }) {
  const user=useAuthenticatedUser();
 const finance=user.permissions.includes("system:admin")||(user.permissions.includes("accounts.quotation_approvals.read")&&user.permissions.includes("accounts.quotation_approvals.view"));
 const creating = id === "create";
  const [project, setProject] = useState<Project | null>(null);
  const [metadataHistory,setMetadataHistory]=useState<{id:string;reason:string;project_version:number;created_at:string;created_by_id:string;before:Record<string,unknown>;after:Record<string,unknown>}[]>([]);
  const [history, setHistory] = useState<Assignment[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(!creating);
  const [saving, setSaving] = useState(false);
  const [stale, setStale] = useState(false);
  const [reload, setReload] = useState(0);
  const [handover, setHandover] = useState(false);
  useEffect(() => {
    if (creating) return;
    const controller = new AbortController();
    Promise.all([get<Project>(`/api/projects/${id}`, controller.signal), get<Assignment[]>(`/api/projects/${id}/manager-history`, controller.signal),get<typeof metadataHistory>(`/api/projects/${id}/metadata-history`,controller.signal)])
      .then(([p, h,m]) => { setMetadataHistory(m);setProject(p); setHistory(h); setStale(false); })
      .catch(e => { if (!controller.signal.aborted) { setProject(null); setHistory([]); setError(e instanceof Error ? e.message : "Unable to load project."); } })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [creating, id, reload]);
  async function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const body = creating ? { purchase_enabled:data.get("purchase_enabled")==="on",name: data.get("name"), client_id: data.get("client_id"), project_manager_id: data.get("project_manager_id"), ...Object.fromEntries(captureNames.map(n=>[n,data.get(n)??""])) }
      : { project_manager_id: data.get("project_manager_id"), reason: data.get("reason"), expected_version: project?.version };
    setSaving(true); setError("");
    try {
      const response = await fetch(creating ? "/api/projects" : `/api/projects/${id}/manager`, { method: creating ? "POST" : "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
      const result: unknown = await response.json();
      if (!response.ok) {
        if (displayServerErrors(form, result)) return;
        if (response.status === 409 && !creating) setStale(true);
        setError(message(result, "Unable to save project ownership."));
        return;
      }
      onSaved();
      if (creating) onClose();
      else { setHandover(false); setLoading(true); setReload(v => v + 1); }
    } catch { setError("Service unavailable. Check the connection, then refresh before retrying."); }
    finally { setSaving(false); }
  }
  function refresh() { setError(""); setLoading(true); setHandover(false); setReload(v => v + 1); }
  return <RecordDrawer title={creating ? "Create project" : project?.name ?? "Project ownership"} footer={creating?<div className="flex justify-end"><button form="project-capture-form" type="submit" disabled={saving} className="min-h-11 rounded-xl bg-blue-700 px-5 py-2 font-semibold text-white disabled:opacity-50">{saving?"Saving…":"Create project"}</button></div>:undefined} onClose={() => { if (!saving) onClose(); }}>
    {loading && <p role="status">Loading project ownership…</p>}
    {error && <p role="alert" className="rounded-lg bg-red-50 p-3 text-sm text-red-700">{error}</p>}
    {!creating && <button className={buttonClass} disabled={saving || loading} onClick={refresh}>Refresh record</button>}
    {project && !loading && <><dl className="grid gap-3 text-sm sm:grid-cols-2"><div><dt className="text-slate-500">Project number</dt><dd className="font-semibold">{project.project_number}</dd></div><div><dt className="text-slate-500">Current project manager</dt><dd className="break-words font-semibold"><UserIdentity id={project.project_manager_id} name={managerName(project.project_manager_id)} /></dd></div><div><dt className="text-slate-500">Status</dt><dd>{project.lifecycle_status}</dd></div><div><dt className="text-slate-500">Purchase FMS</dt><dd>{project.purchase_enabled?"Enabled":"Disabled"}</dd></div><div><dt className="text-slate-500">Record version</dt><dd>{project.version}</dd></div></dl><details className="my-4"><summary className="min-h-11 cursor-pointer font-semibold">Project details</summary><dl className="grid gap-3 text-sm sm:grid-cols-2">{captureNames.map(n=><div key={n}><dt className="text-slate-500">{n.replaceAll("_"," ")}</dt><dd className="break-words whitespace-pre-wrap">{n==="procurement_executive_id" && project[n]?<UserIdentity id={project[n]!} name="Procurement Executive"/>:project[n]||"Not recorded"}</dd></div>)}</dl>{project.schedule_calculation&&<dl className="mt-4 grid gap-3 text-sm sm:grid-cols-2">{Object.entries(project.schedule_calculation).map(([k,v])=><div key={k}><dt>{k.replaceAll("_"," ")}</dt><dd>{v??"Pending actual start"}</dd></div>)}</dl>}</details>{canEdit(project)&&<button className={buttonClass} onClick={()=>onEdit(project)}>Edit project details</button>}{canTransfer && !handover && <button className={buttonClass} onClick={() => setHandover(true)}>Change project manager</button>}</>}
    {!creating&&project&&!loading&&<ProjectQuotationStatus project={project.id} finance={finance}/>}
    {!creating&&project&&!loading&&canProcurement&&<ProjectProcurementOwnership project={project.id} onSaved={()=>{setReload(v=>v+1);onSaved()}}/>}
    {(creating || (handover && project && !loading)) && <ValidatedForm id="project-capture-form" onSubmit={submit} className="space-y-4">
      <fieldset disabled={saving || stale} className="space-y-4">
        {creating && <ProjectCaptureFields procurement={options.procurement_executives??[]} clients={options.clients} managers={options.managers}/>}
        {!creating && <ValidatedLabel className="block text-sm font-semibold">{creating ? "Project manager" : "New project manager"}<ValidatedSelect className={inputClass} name="project_manager_id" required defaultValue=""><option value="">Select eligible manager</option>{options.managers.filter(m => creating || m.id !== project?.project_manager_id).map(m => <option key={m.id} value={m.id}>{m.name}</option>)}</ValidatedSelect></ValidatedLabel>}
        {!creating && <ValidatedLabel className="block text-sm font-semibold">Handover reason<ValidatedTextarea className={inputClass} name="reason" required maxLength={2000} rows={4} /></ValidatedLabel>}
        <p className="text-sm text-slate-600">{creating ? "Only active users with project read and write permissions can be assigned." : "The old manager loses current ownership access. Existing task assignments and past records remain attributed to their original actors."}</p>
        {!creating&&<button className={buttonClass} type="submit">{saving ? "Saving…" : "Save handover"}</button>}
      </fieldset>
    </ValidatedForm>}
    {!creating&&project&&!loading&&<section className="space-y-3"><h3 className="font-bold">Project details history</h3>{metadataHistory.length===0?<p className="text-sm text-slate-600">No metadata corrections recorded.</p>:metadataHistory.map(h=><details key={h.id} className="rounded-lg border p-3"><summary className="cursor-pointer font-semibold">Version {h.project_version} · {h.reason}</summary><p className="text-xs">{new Date(h.created_at).toLocaleString()} · Recorded by <UserIdentity id={h.created_by_id} name={h.created_by_id}/></p><dl className="mt-3 space-y-2">{["name","client_id",...captureNames].filter(n=>JSON.stringify(h.before[n])!==JSON.stringify(h.after[n])).map(n=><div key={n}><dt className="font-semibold">{n.replaceAll("_"," ")}</dt><dd>{String(h.before[n]??"Not recorded")} → {String(h.after[n]??"Not recorded")}</dd></div>)}</dl></details>)}</section>}
    {!creating && project && !loading && <section className="space-y-3"><h3 className="font-bold">Manager assignment history</h3><p className="text-sm text-slate-600">Permanent handover records, newest first.</p><ol className="space-y-3">{history.map(h => <li key={h.id} className="rounded-lg border border-slate-200 bg-white p-4 text-sm"><div className="go-assignment-people">{h.previous_manager_id && <><UserIdentity id={h.previous_manager_id} name={managerName(h.previous_manager_id)} /><span aria-hidden="true">→</span></>}<UserIdentity id={h.project_manager_id} name={managerName(h.project_manager_id)} /></div><p className="mt-2 whitespace-pre-wrap break-words">{h.reason}</p><p className="mt-2 text-xs text-slate-500">{new Date(h.created_at).toLocaleString()} · Version {h.project_version}</p><p className="mt-1 break-words text-xs text-slate-500">Recorded by {h.created_by_id ? <UserIdentity id={h.created_by_id} name={h.created_by_id} /> : "Legacy / bootstrap"}</p></li>)}</ol></section>}
  </RecordDrawer>;
}
