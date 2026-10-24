"use client";

import { useEffect, useState, type SubmitEvent } from "react";
import { UserIdentity } from "@/components/personal-profile";
import { DataGrid, type GridColumn } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { displayServerErrors } from "@/components/ui/server-field-errors";

type Profile = { id: string; project_id: string; user_id: string; user_name: string; bundle_id: string; bundle_name: string; responsibility: string; permission_codes: string[]; is_active: boolean; version: number; created_at: string; updated_at: string; created_by_id: string | null; updated_by_id: string | null };
type Workspace = { project: { id: string; name: string; project_manager_id: string; procurement_executive_id: string; is_active: boolean; lifecycle: string }; profiles: Profile[]; can_write: boolean };
type User = { id: string; name: string; is_active: boolean; bundle_ids: string[] };
type Bundle = { id: string; name: string; is_active: boolean; permissions: string[] };
type Options = { users: User[]; bundles: Bundle[] };
type History = { id: string; action: string; reason: string; before: Profile | null; after: Profile; created_at: string; created_by_id: string | null };
function permissionLabel(code: string) { const parts = code.split("."); const resource = (parts.at(-2) ?? code).replaceAll("_", " "); const action = parts.at(-1) ?? ""; return `${resource.charAt(0).toUpperCase()}${resource.slice(1)} · ${action.charAt(0).toUpperCase()}${action.slice(1)}`; }
const emptyOptions: Options = { users: [], bundles: [] };
const control = "mt-1 block min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm";
const button = "min-h-11 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold disabled:opacity-50";
const primary = "min-h-11 rounded-lg bg-blue-700 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50";
function detail(body: unknown, fallback: string) { return body && typeof body === "object" && "detail" in body && typeof body.detail === "string" ? body.detail : fallback; }
async function get<T>(path: string, signal?: AbortSignal): Promise<T> { const response = await fetch(path, { cache: "no-store", signal }); const body: unknown = await response.json(); if (!response.ok) throw new Error(detail(body, "Unable to load project responsibilities.")); return body as T; }

export function ProjectResponsibilities({ projectId, onClose }: { projectId: string; onClose: () => void }) {
  const [data, setData] = useState<Workspace | null>(null);
  const [options, setOptions] = useState<Options>(emptyOptions);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [reload, setReload] = useState(0);
  const [editor, setEditor] = useState<{ profile: Profile | null; readOnly: boolean } | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    get<Workspace>(`/api/projects/${projectId}/responsibilities`, controller.signal).then(async workspace => {
      // Options require stronger management authority than the list itself.
      const choices = workspace.can_write ? await get<Options>(`/api/projects/${projectId}/responsibilities/options`, controller.signal) : emptyOptions;
      if (!controller.signal.aborted) { setData(workspace); setOptions(choices); setError(""); }
    }).catch(e => { if (!controller.signal.aborted) { setData(null); setError(e instanceof Error ? e.message : "Unable to load responsibilities."); } }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [projectId, reload]);
  const columns: GridColumn<Profile>[] = [
    { key: "user", label: "User", value: row => row.user_name || row.user_id, render: row => <UserIdentity id={row.user_id} name={row.user_name || row.user_id} /> },
    { key: "responsibility", label: "Responsibility", value: row => row.responsibility },
    { key: "role", label: "Role", value: row => row.bundle_name || row.bundle_id },
    { key: "permissions", label: "Scoped permissions", value: row => row.permission_codes.length },
    { key: "status", label: "Status", value: row => row.is_active ? "Active" : "Inactive" },
  ];
  return <RecordDrawer title={data ? `${data.project.name} · Team` : "Project team"} onClose={onClose}>
    <div className="flex flex-wrap items-center justify-between gap-3"><h3 className="text-lg font-semibold">Project responsibilities</h3><div className="flex gap-2"><button className={button} disabled={loading} onClick={() => { setLoading(true); setReload(n => n + 1); }}>Refresh team</button>{data?.can_write && <button className={primary} disabled={!data.project.is_active || ["closed", "on_hold"].includes(data.project.lifecycle)} onClick={() => setEditor({ profile: null, readOnly: false })}>Add responsibility</button>}</div></div>
    {error && <p role="alert" className="text-red-700">{error}</p>}{notice && <p role="status" className="text-green-700">{notice}</p>}
    {data && <p className="text-sm text-slate-600">Role permissions apply only to this project. Project Manager and Procurement Executive handovers use their ownership controls.</p>}
    <DataGrid label="Project responsibilities" rows={data?.profiles ?? []} columns={columns} loading={loading} status={row => row.is_active ? "Active" : "Inactive"} actions={row => <div className="flex gap-2"><button className={button} aria-label={`View ${row.responsibility} for ${row.user_name}`} onClick={() => setEditor({ profile: row, readOnly: true })}>View & history</button>{data?.can_write && <button className={button} aria-label={`Edit ${row.responsibility} for ${row.user_name}`} onClick={() => setEditor({ profile: row, readOnly: false })}>Edit</button>}</div>} />
    {editor && <ProfileEditor key={`${editor.profile?.id ?? "new"}:${editor.readOnly}`} projectId={projectId} profile={editor.profile} options={options} readOnly={editor.readOnly} projectOpen={!!data?.project.is_active && !["closed", "on_hold"].includes(data.project.lifecycle)} onClose={() => setEditor(null)} onSaved={() => { setEditor(null); setNotice("Project responsibility saved."); setLoading(true); setReload(n => n + 1); }} />}
  </RecordDrawer>;
}

function ProfileEditor({ projectId, profile, options, readOnly, projectOpen, onClose, onSaved }: { projectId: string; profile: Profile | null; options: Options; readOnly: boolean; projectOpen: boolean; onClose: () => void; onSaved: () => void }) {
  const [userId, setUserId] = useState(profile?.user_id ?? "");
  const [bundleId, setBundleId] = useState(profile?.bundle_id ?? "");
  const [name, setName] = useState(profile?.responsibility ?? "");
  const [codes, setCodes] = useState<string[]>(profile?.permission_codes ?? []);
  const [active, setActive] = useState(profile?.is_active ?? true);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [history, setHistory] = useState<History[] | null>(profile ? null : []);
  const candidate = options.users.find(user => user.id === userId);
  const bundles = options.bundles.filter(bundle => candidate?.bundle_ids.includes(bundle.id));
  const bundle = bundles.find(row => row.id === bundleId);
  const availableCodes = [...new Set([...(bundle?.permissions ?? []), ...(profile?.permission_codes ?? [])])].sort();
  useEffect(() => {
    if (!profile) return;
    const controller = new AbortController();
    get<History[]>(`/api/projects/${projectId}/responsibilities/${profile.id}/history`, controller.signal).then(result => { if (!controller.signal.aborted) setHistory(result); }).catch(e => { if (!controller.signal.aborted) setHistory([]); setError(e instanceof Error ? e.message : "Unable to load history."); });
    return () => controller.abort();
  }, [profile, projectId]);
  async function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault(); const form = event.currentTarget;
    if (!codes.length) { setError("Select at least one project permission."); return; }
    setBusy(true); setError("");
    try {
      const response = await fetch(`/api/projects/${projectId}/responsibilities${profile ? `/${profile.id}` : ""}`, { method: profile ? "PATCH" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ user_id: userId, bundle_id: bundleId, responsibility: name, permission_codes: codes, is_active: active, version: profile?.version ?? 0, reason }) });
      const body: unknown = await response.json();
      if (!response.ok) { displayServerErrors(form, body); setError(detail(body, response.status === 409 ? "This responsibility changed. Close and refresh the team list before editing again." : "Please correct the highlighted fields or refresh the role choices.")); return; }
      onSaved();
    } catch { setError("Unable to confirm the save. Refresh the team list before retrying."); } finally { setBusy(false); }
  }
  return <RecordDrawer title={readOnly ? "View responsibility" : profile ? "Edit responsibility" : "Add responsibility"} onClose={onClose} footer={readOnly ? undefined : <div className="flex justify-end gap-3"><button type="button" className={button} disabled={busy} onClick={onClose}>Cancel</button><button type="submit" form="project-responsibility-form" className={primary} disabled={busy || (!projectOpen && active)}>{busy ? "Saving…" : "Save responsibility"}</button></div>}>
    <ValidatedForm id="project-responsibility-form" onSubmit={submit} className="space-y-4">
      <fieldset disabled={readOnly || busy} className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <ValidatedLabel className="text-sm font-semibold">User<ValidatedSelect name="user_id" required className={control} disabled={!!profile} value={userId} onChange={event => { setUserId(event.target.value); setBundleId(""); setCodes([]); }}><option value="">Select active user</option>{profile ? <option value={profile.user_id}>{profile.user_name || profile.user_id}</option> : options.users.map(user => <option key={user.id} value={user.id}>{user.name}</option>)}</ValidatedSelect></ValidatedLabel>
          <ValidatedLabel className="text-sm font-semibold">Role<ValidatedSelect name="bundle_id" required className={control} disabled={!!profile || !userId} value={bundleId} onChange={event => { setBundleId(event.target.value); setCodes([]); }}><option value="">Select assigned role</option>{profile ? <option value={profile.bundle_id}>{profile.bundle_name || profile.bundle_id}</option> : bundles.map(row => <option key={row.id} value={row.id}>{row.name}</option>)}</ValidatedSelect></ValidatedLabel>
        </div>
        <ValidatedLabel className="block text-sm font-semibold">Responsibility<ValidatedInput name="responsibility" required maxLength={80} className={control} value={name} onChange={event => setName(event.target.value)} placeholder="For example, Finance" /></ValidatedLabel>
        <div className="space-y-2"><div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-semibold">Project permissions</h3>{!readOnly && <div className="flex gap-2"><button type="button" className={button} disabled={!bundle} onClick={() => setCodes(bundle?.permissions ?? [])}>Select all</button><button type="button" className={button} onClick={() => setCodes([])}>Clear</button></div>}</div>
          {!bundleId ? <p className="text-sm text-slate-600">Select a user and their assigned role first.</p> : <div className="grid gap-2 sm:grid-cols-2">{availableCodes.map(code => <ValidatedLabel key={code} className="flex min-h-11 items-start gap-2 rounded-lg border border-slate-200 p-3 text-sm"><ValidatedInput type="checkbox" name="permission_codes" checked={codes.includes(code)} onChange={event => setCodes(current => event.target.checked ? [...current, code] : current.filter(value => value !== code))} /><span className="min-w-0 break-all">{permissionLabel(code)}{!readOnly && bundle && !bundle.permissions.includes(code) ? " · Removed from current role" : ""}</span></ValidatedLabel>)}</div>}
        </div>
        {profile && <ValidatedLabel className="flex min-h-11 items-center gap-2 text-sm"><ValidatedInput type="checkbox" name="is_active" checked={active} onChange={event => setActive(event.target.checked)} />Active responsibility</ValidatedLabel>}
        {!readOnly && !projectOpen && <p role="status" className="text-amber-800">The project is inactive, held or closed. You can deactivate an existing responsibility.</p>}
        {!readOnly && <ValidatedLabel className="block text-sm font-semibold">Reason<ValidatedTextarea name="reason" required maxLength={2000} rows={3} className={control} value={reason} onChange={event => setReason(event.target.value)} /></ValidatedLabel>}
      </fieldset>
      {error && <p role="alert" className="text-red-700">{error}</p>}
    </ValidatedForm>
    {profile && <section className="space-y-3"><h3 className="font-semibold">History</h3>{history === null ? <p role="status">Loading history…</p> : history.map(event => <article key={event.id} className="rounded-lg border border-slate-200 p-3 text-sm"><p className="font-semibold">{event.action} · Version {event.after.version}</p><p>{event.reason}</p><p className="text-slate-600">{new Date(event.created_at).toLocaleString()}</p>{event.created_by_id && <UserIdentity id={event.created_by_id} name={options.users.find(user => user.id === event.created_by_id)?.name ?? event.created_by_id} />}<p className="mt-2 break-words">{event.before ? `${event.before.is_active ? "Active" : "Inactive"} → ` : ""}{event.after.is_active ? "Active" : "Inactive"} · {event.after.permission_codes.length} scoped permissions</p><details className="mt-2"><summary className="min-h-11 cursor-pointer font-semibold">Changes</summary><p>{event.before ? `${event.before.responsibility} → ` : ""}{event.after.responsibility}</p>{event.before && <p className="mt-2">Before: {event.before.permission_codes.map(permissionLabel).join(", ")}</p>}<p className="mt-2">After: {event.after.permission_codes.map(permissionLabel).join(", ")}</p></details></article>)}</section>}
  </RecordDrawer>;
}
