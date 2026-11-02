"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { RequiredSelection, ValidatedForm, ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";

import { FormEvent, useEffect, useState } from "react";

import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { actionButtonClass } from "@/components/ui/action-button";
import { SecurityHistory } from "@/components/users/security-history";
import { PermissionCatalogue } from "./permission-catalogue";
import { groupPermissions, type Permission } from "./permission-groups";
type Role = { id: string; name: string; description: string; permissions: string[]; is_active: boolean; edit_version: number };

export function RoleManagement() {
 const actor=useAuthenticatedUser();
 const canWrite=actor.is_superuser===true||actor.permissions.includes("admin.roles.write");
 const [editing,setEditing]=useState<Role|null>(null);
 const [busy,setBusy]=useState(false);
 const[conflict,setConflict]=useState(false);
  const [roles, setRoles] = useState<Role[]>([]);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [selectedCodes, setSelectedCodes] = useState<string[]>([]);
  const [permissionQuery, setPermissionQuery] = useState("");
  const [registryQuery, setRegistryQuery] = useState("");
  const [notice, setNotice] = useState("");
  const [permissionError, setPermissionError] = useState("");
  const [isLoading, setIsLoading] = useState(true);

  async function load() {
    setIsLoading(true);
    try {
      const [rolesResponse, permissionsResponse] = await Promise.all([
        fetch("/api/administration/roles", { cache: "no-store" }),
        fetch("/api/administration/roles/permissions", { cache: "no-store" }),
      ]);
      if (!rolesResponse.ok || !permissionsResponse.ok) throw new Error("Unable to load roles");
      setRoles(await rolesResponse.json() as Role[]);
      setPermissions((await permissionsResponse.json() as Permission[]).filter(p=>p.code!=="system:admin"));
    } catch {
      setNotice("Unable to load roles. Please refresh and try again.");
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    const timer = window.setTimeout(() => { void load(); }, 0);
    return () => window.clearTimeout(timer);
  }, []);

  function togglePermission(code: string) { setPermissionError("");
    setSelectedCodes((current) => current.includes(code)
      ? current.filter((item) => item !== code)
      : [...current, code]);
  }

  async function installFinanceRoles() {
    setIsLoading(true);
    try {
      const response = await fetch("/api/administration/roles/finance-presets", { method: "POST" });
      if (!response.ok) throw Error("Unable to create finance roles");
      setNotice("Finance Viewer, Budget Editor and Approver roles are ready. Assign staff in Users, then grant their project team capabilities. Existing bundles were preserved.");
      await load();
    } catch {
      setNotice("Unable to create finance roles. Check role administration access and retry.");
    } finally { setIsLoading(false); }
  }

  async function reloadSavedRole(){
    if(!editing||busy)return;setBusy(true);try{const r=await fetch("/api/administration/roles",{cache:"no-store"});if(!r.ok)throw Error();const saved=await r.json() as Role[];const role=saved.find(v=>v.id===editing.id);if(!role)throw Error();setRoles(saved);setEditing(role);setSelectedCodes(role.permissions.filter(code=>code!=="system:admin"));setPermissionError("");setConflict(false);}catch{setPermissionError("Unable to reload this role. Please retry.");}finally{setBusy(false);}
  }

  async function createRole(event: FormEvent<HTMLFormElement>) {
    const submittedForm = event.currentTarget;
    event.preventDefault();
    if (!editing&&!selectedCodes.length) { setPermissionError("Select at least one permission."); return; }
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    if(busy||!canWrite)return;setBusy(true);
    try {
    const response = await fetch(editing?`/api/administration/roles/${editing.id}`:"/api/administration/roles", {
      method: editing?"PATCH":"POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        ...(editing?{expected_version:editing.edit_version,is_active:form.get("is_active")==="on"}:{name: String(form.get("name") ?? "").trim().toLowerCase()}),
        description: String(form.get("description") ?? "").trim(),
        permission_codes: selectedCodes,
      }),
    });
    if (!response.ok) {
      const error = await response.json().catch(() => ({ detail: "Unable to create role" }));
      if(response.status===409)setConflict(true);
      if (displayServerErrors(submittedForm, error)) return; setPermissionError(typeof error.detail === "string" ? error.detail : "Unable to save role.");
      return;
    }
    setIsDialogOpen(false);
    setSelectedCodes([]);
    setNotice(editing?"Role updated. Its permissions apply to subsequent requests immediately.":"Role created with its selected permissions.");
    await load();
    }catch{setPermissionError("Unable to save this role. Please retry.");}finally{setBusy(false);}
  }

  return (
    <section className="mx-auto w-full max-w-[1500px] space-y-5">
      <div className="flex flex-col gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-end sm:justify-between">
        <div><p className="text-xs font-bold uppercase tracking-[0.16em] text-blue-700">Administration</p><h2 className="mt-1 text-2xl font-extrabold tracking-tight text-slate-900">Roles & permissions</h2></div>
        {canWrite&&<button type="button" onClick={() => { setConflict(false);setEditing(null);setSelectedCodes([]); setPermissionQuery(""); setPermissionError(""); setIsDialogOpen(true); }} className={actionButtonClass("primary")}>Create role</button>}
      </div>
      <div className="rounded-xl border border-slate-200 bg-white p-5"><h3 className="font-bold">Finance access levels</h3><p className="mt-2 text-sm text-slate-600">Viewer reads budgets and quote differences. Budget Editor allocates costs. Approver approves budgets and Accounts decisions. Each user also needs project team access.</p>{canWrite&&<button type="button" disabled={isLoading} onClick={() => void installFinanceRoles()} className={actionButtonClass("secondary")+" mt-3"}>Create finance roles</button>}</div>
      {isLoading ? <p className="rounded-xl border border-blue-100 bg-blue-50 px-4 py-3 text-sm font-medium text-blue-800">Loading registered permissions...</p> : null}
      {notice ? <p role="status" className="rounded-xl border border-blue-100 bg-blue-50 px-4 py-3 text-sm font-medium text-blue-800">{notice}</p> : null}
      <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-[0_14px_34px_-26px_rgba(15,23,42,0.35)]"><div className="border-b border-slate-100 px-5 py-4"><h3 className="font-bold text-slate-900">Existing roles</h3></div><div className="divide-y divide-slate-100">{roles.map((role) => <article key={role.id} className="p-5"><div className="flex items-start justify-between gap-3"><div><h4 className="font-bold text-slate-900">{role.name} <span className="text-sm font-normal">({role.is_active?"Active":"Inactive"})</span></h4>{canWrite&&(role.name!=="superadmin"||actor.is_superuser===true)&&<button type="button" className={actionButtonClass("secondary")} onClick={()=>{setConflict(false);setEditing(role);setSelectedCodes(role.permissions.filter(p=>p!=="system:admin"));setPermissionQuery("");setPermissionError("");setIsDialogOpen(true);}}>Edit role</button>}<p className="mt-1 text-sm text-slate-500">{role.description || "No description provided."}</p></div><span className="rounded-full bg-blue-50 px-2.5 py-1 text-xs font-bold text-blue-700">{role.permissions.length} permissions</span></div><div className="mt-3 space-y-3">{groupPermissions(role.permissions.map((code) => (permissions.find((permission) => permission.code === code) ?? { code, description: "" }))).map((group) => <div key={group.key}><p className="text-xs font-bold text-slate-700">{group.area} / {group.label}</p><div className="mt-1 flex flex-wrap gap-2">{group.permissions.map(({ code }) => <span key={code} className="max-w-full break-all rounded-lg bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{code}</span>)}</div></div>)}</div></article>)}</div></div>
        <aside className="rounded-2xl border border-slate-200 bg-slate-50 p-5"><p className="text-xs font-bold uppercase tracking-[0.15em] text-blue-700">Permission registry</p><h3 className="mt-1 text-lg font-bold text-slate-900">{permissions.length} permissions available</h3><p className="mt-2 text-sm leading-6 text-slate-600">Browse by area and capability. View opens a workspace, Read retrieves its records, and other actions allow only their described operations.</p><ValidatedLabel className="mt-4 block text-sm font-bold text-slate-700">Search registry<ValidatedInput type="search" value={registryQuery} onChange={(event) => setRegistryQuery(event.target.value)} placeholder="Search area, capability or description" className="mt-1 w-full rounded-xl border border-slate-200 bg-white px-3 py-2 font-normal" /></ValidatedLabel><div className="mt-4"><PermissionCatalogue permissions={permissions} query={registryQuery} /></div></aside>
      </div>
      <SecurityHistory/>
      {isDialogOpen ? <RecordDrawer title={editing?"Edit role":"Create role"} onClose={()=>{if(!busy)setIsDialogOpen(false);}}><ValidatedForm key={editing?`${editing.id}-${editing.edit_version}`:"create"} onSubmit={createRole} className="space-y-5"><div className="min-h-0 flex-1 overflow-y-auto pr-1"><div className="mt-6 grid gap-4 sm:grid-cols-2"><ValidatedLabel className="text-sm font-bold text-slate-700">Role name<ValidatedInput name="name" defaultValue={editing?.name} readOnly={!!editing} required pattern={"[a-z0-9._\\-]+"} placeholder="project-manager" className="mt-1.5 min-h-11 w-full rounded-xl border border-slate-200 px-3 font-normal outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/15" /></ValidatedLabel><ValidatedLabel className="text-sm font-bold text-slate-700">Description<ValidatedInput name="description" defaultValue={editing?.description} placeholder="Manages project delivery" className="mt-1.5 min-h-11 w-full rounded-xl border border-slate-200 px-3 font-normal outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/15" /></ValidatedLabel></div>{editing&&<ValidatedLabel className="user-access-card flex items-center gap-3 rounded-xl border p-3"><ValidatedInput type="checkbox" name="is_active" defaultChecked={editing.is_active}/>Active role (inactive roles grant no permissions)</ValidatedLabel>}<RequiredSelection selectionCount={editing?Math.max(1,selectedCodes.length):selectedCodes.length} onInvalid={() => setPermissionError("Select at least one permission.")} className="mt-5"><legend className="text-sm font-bold text-slate-700">Assign permissions<span aria-hidden="true" className="text-red-600"> *</span></legend><p className="mt-2 text-xs leading-5 text-slate-600">Expand an area to choose permissions by capability. Selecting View does not automatically grant Read or Write.</p><ValidatedLabel className="mt-3 block text-sm font-bold text-slate-700">Find permissions<ValidatedInput type="search" value={permissionQuery} onChange={(event) => setPermissionQuery(event.target.value)} placeholder="Search area, capability, code or description" className="mt-1 min-h-11 w-full rounded-xl border border-slate-200 px-3 font-normal" /></ValidatedLabel><p role="status" className="my-3 text-sm font-semibold text-blue-700">{selectedCodes.length} of {permissions.length} permissions selected</p><PermissionCatalogue permissions={permissions} query={permissionQuery} selectedCodes={selectedCodes} onToggle={togglePermission} />{permissionError ? <p role="alert" className="mt-1 text-sm text-red-600">{permissionError}</p> : null}{conflict&&<button type="button" className={actionButtonClass("secondary")} onClick={()=>void reloadSavedRole()}>Reload saved role (discard unsaved changes)</button>}</RequiredSelection></div><div className="record-drawer-actions sticky bottom-0 flex flex-col-reverse gap-3 py-4 sm:flex-row sm:justify-end"><button type="button" onClick={() => setIsDialogOpen(false)} disabled={busy} className={actionButtonClass("secondary")}>Cancel</button><button type="submit" disabled={busy} className={actionButtonClass("primary")}>{busy?"Saving…":editing?"Save role":"Create role"}</button></div></ValidatedForm></RecordDrawer> : null}
    </section>
  );
}
