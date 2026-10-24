"use client";

import {useEffect, useState} from "react";
import {ValidatedLabel, ValidatedTextarea} from "@/components/ui/validated-controls";
import {useProjectUser} from "@/components/projects/project-permissions";

export type ProjectFreezeState = {project_id:string; is_frozen:boolean; version:number};
type Event = {id:string; action:string; reason:string; actor_username:string; created_by_id:string|null; created_at:string; version:number};
const message = (payload:{detail?:unknown}, fallback:string) => typeof payload.detail === "string" ? payload.detail : fallback;

export function ProjectFreezePanel({projectId,onStateChange,dirty=false,busy=false}: {projectId:string; onStateChange?:(state:ProjectFreezeState|null)=>void; dirty?:boolean; busy?:boolean}) {
  const user = useProjectUser();
  const has = (code:string) => user.permissions.includes("system:admin") || user.permissions.includes(code);
  const canReadHistory = has("projects.freeze.read");
  const canReadStatus = canReadHistory || has("projects.master.read");
  const [state,setState] = useState<ProjectFreezeState|null>(null);
  const [history,setHistory] = useState<Event[]>([]);
  const [reason,setReason] = useState("");
  const [error,setError] = useState("");
  const [historyError,setHistoryError] = useState("");
  const [saving,setSaving] = useState(false);
  const [revision,setRevision] = useState(0);
  useEffect(() => {
    if (!canReadStatus) return;
    const controller = new AbortController();
    const load = async () => {
      try {
        const response = await fetch(`/api/projects/${projectId}/freeze`,{cache:"no-store",signal:controller.signal});
        const payload = await response.json();
        if (!response.ok) throw new Error(message(payload,"Unable to read freeze status. Editing is blocked until status is available."));
        if (!controller.signal.aborted) {setState(payload);onStateChange?.(payload);setError("");}
      } catch (failure) {if (!controller.signal.aborted) {setState(null);onStateChange?.(null);setError(failure instanceof Error ? failure.message : "Unable to read freeze status.");}}
    };
    void load();
    window.addEventListener("focus",load);
    const timer = window.setInterval(() => void load(),30_000);
    if (canReadHistory) void fetch(`/api/projects/${projectId}/freeze/history`,{cache:"no-store",signal:controller.signal})
      .then(async response => {const payload = await response.json();if (!response.ok) throw new Error("Unable to read freeze history.");return payload;})
      .then(payload => {if (!controller.signal.aborted) {setHistory(payload);setHistoryError("");}})
      .catch(() => {if (!controller.signal.aborted) setHistoryError("Unable to read freeze history.");});
    return () => {controller.abort();window.clearInterval(timer);window.removeEventListener("focus",load);};
  },[projectId,revision,canReadHistory,canReadStatus,onStateChange]);
  async function change() {
    if (!state || saving || busy || (!state.is_frozen && dirty)) return;
    if (state.is_frozen && !reason.trim()) {setError("Enter a reason for unfreezing this project.");return;}
    setSaving(true);setError("");
    try {
      const response = await fetch(`/api/projects/${projectId}/freeze`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({freeze:!state.is_frozen,expected_version:state.version,reason:reason.trim()})});
      const payload = await response.json();
      if (!response.ok) throw new Error(message(payload,"Unable to change freeze status."));
      setState(payload);onStateChange?.(payload);setReason("");setRevision(value => value + 1);
    } catch (failure) {setError(failure instanceof Error ? failure.message : "Unable to change freeze status.");}
    finally {setSaving(false);}
  }
  const allowed = has("projects.master.write") && has(state?.is_frozen ? "projects.freeze.unfreeze" : "projects.freeze.write");
  if (!canReadStatus) return null;
  return <section aria-label="Project freeze control" className="mb-4 rounded-xl border border-slate-200 bg-slate-50 p-4">
    <h3 className="font-bold text-slate-900">{state ? state.is_frozen ? "Project frozen" : "Project editable" : "Checking project freeze status…"}</h3>
    <p className="mt-1 text-sm text-slate-600">Freezing locks project details, dates and configuration. Execution updates continue.</p>
    {error ? <p role="alert" className="mt-2 text-sm text-red-700">{error}</p> : null}
    {!state || error ? <button type="button" onClick={() => setRevision(value => value + 1)} className="min-h-11 font-bold text-blue-700">Reload freeze status</button> : null}
    {state && allowed ? <div className="mt-3 space-y-2">
      {state.is_frozen ? <ValidatedLabel className="block text-sm font-bold text-slate-700">Reason for unfreezing<ValidatedTextarea name="unfreeze_reason" required maxLength={2000} disabled={saving || busy} value={reason} onChange={event => setReason(event.target.value)} className="field" /></ValidatedLabel> : dirty ? <p className="text-sm text-slate-600">Save or discard your edits before freezing the project.</p> : null}
      <button type="button" disabled={saving || busy || (!state.is_frozen && dirty) || (state.is_frozen && !reason.trim())} onClick={() => void change()} className="min-h-11 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white disabled:opacity-50">{saving ? "Updating…" : state.is_frozen ? "Unfreeze project" : "Freeze project"}</button>
    </div> : state?.is_frozen ? <p className="mt-2 text-sm text-slate-600">Unfreezing requires the Unfreeze project permission.</p> : null}
    {canReadHistory ? <details className="mt-3 text-sm"><summary className="min-h-11 cursor-pointer font-bold text-slate-700">Freeze history</summary>{historyError ? <p role="alert">{historyError}</p> : history.length ? <ul className="space-y-3">{history.map(event => <li key={event.id} className="border-t border-slate-200 pt-2"><p><strong>{event.action === "unfreeze" ? "Unfrozen" : "Frozen"}</strong> by {event.actor_username} · {new Intl.DateTimeFormat("en-IN",{dateStyle:"medium",timeStyle:"short",timeZone:"Asia/Kolkata"}).format(new Date(event.created_at))}</p>{event.reason ? <p className="whitespace-pre-wrap text-slate-600">Reason: {event.reason}</p> : null}<p className="break-all text-xs text-slate-500">User ID: {event.created_by_id ?? "Historical user removed"} · Revision {event.version}</p></li>)}</ul> : <p className="text-slate-600">No freeze transitions recorded.</p>}</details> : null}
  </section>;
}
