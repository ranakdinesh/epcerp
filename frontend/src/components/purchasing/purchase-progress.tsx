"use client";

import { useEffect, useState } from "react";
import Link from "next/link";

export type ProgressSignal = "complete" | "delayed" | "blocked" | "due_soon" | "unplanned" | "on_track" | "attention";
export type StepProgress = {
  number: number; title: string; contribution_percent: number; state: string; details?: string;
  due_date: string | null; signal: ProgressSignal; missing_dates: boolean; days_overdue: number;
};
export type BOProgress = {
  bo_id: string; progress_percent: number; signal: ProgressSignal;
  completion_date: string | null; issues: string[]; steps: StepProgress[];
};
export type ProgressReport = {
  as_of: string; project_start: string | null; project_end: string | null;
  progress_percent: number | null; packages: BOProgress[];
};
const labels: Record<ProgressSignal, string> = {
  complete: "Complete", delayed: "Delayed", blocked: "Blocked", due_soon: "Due soon",
  unplanned: "Not fully planned", on_track: "On track", attention: "Review dates",
};
export function ProgressFlag({ signal }: { signal: ProgressSignal }) {
  const color = signal === "delayed" ? "bg-red-50 text-red-800 border-red-200"
    : ["blocked", "due_soon", "attention"].includes(signal) ? "bg-amber-50 text-amber-900 border-amber-200"
    : ["complete", "on_track"].includes(signal) ? "bg-emerald-50 text-emerald-800 border-emerald-200"
    : "bg-slate-50 text-slate-600 border-slate-200";
  return <span className={`inline-flex rounded-md border px-2 py-1 text-xs font-semibold ${color}`}>{labels[signal]}</span>;
}
export function CompletionTick() {
  return <svg aria-label="Completed" role="img" width="20" height="20" viewBox="0 0 20 20" className="inline-block shrink-0 text-emerald-700"><circle cx="10" cy="10" r="9" fill="currentColor" /><path d="m5.5 10 3 3 6-6" stroke="white" strokeWidth="2" fill="none" /></svg>;
}
export function usePurchaseProgress(project: string, revision: string | number = 0) {
  const [snapshot, setSnapshot] = useState<{ key: string; data: ProgressReport | null; error: string } | null>(null);
  const [savedRevision, setSavedRevision] = useState(0);
  useEffect(() => {
    const saved = (event: Event) => { if ((event as CustomEvent).detail === project) setSavedRevision(n => n + 1); };
    window.addEventListener("purchase-progress-saved", saved);
    return () => window.removeEventListener("purchase-progress-saved", saved);
  }, [project]);
  const key = `${project}:${revision}:${savedRevision}`;
  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const response = await fetch(`/api/projects/${project}/purchase-progress`, { cache: "no-store", signal: controller.signal });
        if (!response.ok) throw Error(response.status === 403 ? "Purchase progress access is unavailable." : "Purchase progress could not be loaded.");
        const data: ProgressReport = await response.json();
        if (!controller.signal.aborted) setSnapshot({ key, data, error: "" });
      } catch (e) {
        if (!controller.signal.aborted) setSnapshot({ key, data: null, error: (e as Error).message });
      }
    }
    void load();
    return () => controller.abort();
  }, [project, key]);
  return snapshot?.key === key ? snapshot : { data: null, error: "" };
}
export function PurchaseProgressPanel({ project }: { project: string }) {
  const [revision, setRevision] = useState(0);
  const { data, error } = usePurchaseProgress(project, revision);
  return <section aria-label="Purchase FMS progress" className="rounded-xl border border-slate-200 bg-white p-4">
    <div className="flex flex-wrap items-center justify-between gap-3"><h3 className="font-bold">Purchase FMS progress {data?.progress_percent != null ? `· ${data.progress_percent.toFixed(1)}%` : ""}</h3><button type="button" className="min-h-11 rounded-lg border border-slate-300 px-3 text-sm font-semibold" onClick={() => setRevision(n => n + 1)}>Refresh progress</button></div>
    {error ? <p role="alert" className="text-sm text-red-800">{error}</p> : !data ? <p role="status" className="text-sm text-slate-500">Loading saved purchase progress…</p> : <>
      <p className="mt-2 text-xs text-slate-600">Project: {data.project_start || "Start not set"} → {data.project_end || "End not set"} · As of {data.as_of} (UTC)</p>
      <p className="mt-2 text-xs text-slate-500">Weighted completion from saved records · Red: overdue · Yellow: blocked, due within 2 days or date conflict · Green: complete / on track.</p>
      {!data.packages.length ? <p className="mt-4 text-sm text-slate-500">No saved BO lists. Purchase progress is not yet available.</p> : data.packages.map((bo, i) => <details key={bo.bo_id} className="mt-3 rounded-lg border border-slate-200 p-3" open={data.packages.length === 1}>
        <summary className="cursor-pointer text-sm font-semibold">BO {i + 1} · {bo.progress_percent.toFixed(1)}% <ProgressFlag signal={bo.signal} /> <span className="ml-2 text-xs font-normal">Purchase completion: {bo.completion_date || "Not planned"}</span></summary>
        <progress className="mt-3 h-2 w-full accent-emerald-600" max={100} value={bo.progress_percent} aria-label={`BO ${i + 1} weighted completion`} />
        {bo.issues.length > 0 ? <p className="mt-2 text-xs text-amber-900">{bo.issues.join(" · ")}</p> : null}
        <div className="mt-3 overflow-x-auto"><table className="vtpl-data-table w-full min-w-[570px] text-left text-xs"><caption className="sr-only">Step progress and deadlines for BO {i + 1}</caption><thead className="bg-slate-50 text-slate-600"><tr>{["Step", "Weight", "Status", "Deadline"].map(label => <th key={label} className="p-2">{label}</th>)}</tr></thead><tbody>{bo.steps.map(step => <tr key={step.number} className="border-t border-slate-100"><td className="p-2"><Link className="inline-flex min-h-11 items-center gap-2 font-semibold text-blue-800 underline" href={`/dashboard?screen=projects&project=${project}&tab=purchasing&purchaseView=bo&bo=${bo.bo_id}&purchaseStep=${step.number}`}>{step.state === "completed" ? <CompletionTick /> : null}{step.number}. {step.title}</Link>{step.details && <p className="text-slate-600">{step.details}</p>}</td><td className="p-2">{step.contribution_percent.toFixed(2)}%</td><td className="p-2"><ProgressFlag signal={step.signal} /><span className="mt-1 block text-slate-600">{step.state === "not_applicable" ? "Not applicable (recorded)" : step.state.replaceAll("_", " ")}</span></td><td className="p-2">{step.due_date || "Not planned"}{step.days_overdue > 0 ? <span className="block text-red-800">{step.days_overdue} days overdue</span> : null}{step.missing_dates && step.state !== "completed" ? <span className="block text-slate-500">Dates missing</span> : null}</td></tr>)}</tbody></table></div>
      </details>)}
      <p className="mt-3 text-xs text-slate-500">Project percentage averages active BO lists. Browser-only drafts and example approvals earn no completion credit. Set step deadlines in Purchase workspaces → task assignment; Step 20 is the purchase completion target.</p>
      <Link className="mt-2 inline-flex min-h-11 items-center text-sm font-semibold text-blue-800 underline" href={`/dashboard?screen=projects&project=${project}&tab=purchasing&purchaseView=workflow`}>Open purchase workspaces and deadlines</Link>
    </>}
  </section>;
}
