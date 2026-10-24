"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { assignmentHref } from "@/components/dashboards/dashboard-model";

type Warning = { key: string; label: string; kind: string; reason: string };
type Coverage = { reports: { project_id: string; project_name: string; warnings: Warning[] }[]; next_page: number | null };

export function AssignmentNotifications() {
  const user = useAuthenticatedUser();
  const allowed = user.permissions.includes("system:admin") || user.permissions.includes("projects.assignments.read");
  const [refresh, setRefresh] = useState(0);
  const [result, setResult] = useState<Coverage | null>(null);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const container = useRef<HTMLDivElement>(null);
  const bell = useRef<HTMLButtonElement>(null);
  const close = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    close.current?.focus();
    function outside(event: PointerEvent) {
      if (!container.current?.contains(event.target as Node)) setOpen(false);
    }
    function escape(event: KeyboardEvent) {
      if (event.key === "Escape") { setOpen(false); bell.current?.focus(); }
    }
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", escape);
    return () => { document.removeEventListener("pointerdown", outside); document.removeEventListener("keydown", escape); };
  }, [open]);
  useEffect(() => {
    if (!allowed) return;
    const controller = new AbortController();
    let busy = false;
    async function load() {
      if (busy || document.visibilityState === "hidden") return;
      busy = true;
      try {
        const reports: Coverage["reports"] = [];
        let page: number | null = 1;
        while (page !== null) {
          const response = await fetch(`/api/project-assignment-warnings?page=${page}&page_size=20`, { cache: "no-store", signal: controller.signal });
          if (!response.ok) throw Error("Unable to check assignment notifications. Please retry.");
          const data: Coverage = await response.json();
          reports.push(...data.reports);
          page = data.next_page;
        }
        if (!controller.signal.aborted) { setResult({reports, next_page: null}); setError(""); }
      } catch {
        if (!controller.signal.aborted) setError("Unable to check assignment notifications. Please retry. Previously loaded notifications may be out of date.");
      } finally { busy = false; }
    }
    void load();
    const timer = window.setInterval(() => void load(), 60_000);
    const focus = () => void load();
    window.addEventListener("focus", focus);
    document.addEventListener("visibilitychange", focus);
    return () => { controller.abort(); window.clearInterval(timer); window.removeEventListener("focus", focus); document.removeEventListener("visibilitychange", focus); };
  }, [allowed, refresh, open]);
  if (!allowed) return null;
  const count = result?.reports.reduce((sum, report) => sum + report.warnings.length, 0) ?? 0;
  return <div ref={container} className="relative" onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget as Node)) setOpen(false); }}>
    <button ref={bell} aria-label={`Notifications${error ? ": check unavailable" : result ? `: ${count} assignment warnings` : ": loading"}`} aria-expanded={open} aria-controls="assignment-notifications" onClick={() => setOpen(value => !value)} className="relative flex min-h-11 min-w-11 items-center justify-center rounded-xl border border-slate-200 bg-white px-3 text-slate-700 shadow-sm focus-visible:outline-2 focus-visible:outline-blue-600">
      <svg aria-hidden="true" width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9M10 21h4" /></svg>
      {(count > 0 || error) && <span aria-hidden="true" className="ml-2 rounded-full bg-orange-700 px-2 py-0.5 text-xs font-bold text-white">{error ? "!" : count}</span>}
    </button>
    {open && <section id="assignment-notifications" aria-label="Notifications" className="absolute right-0 z-50 mt-2 w-[min(26rem,calc(100vw-2.5rem))] rounded-2xl border border-slate-200 bg-white text-slate-900 shadow-xl">
      <div className="flex items-center justify-between gap-2 border-b border-slate-200 p-4"><div><h2 className="font-bold">Notifications</h2><p className="mt-1 text-xs text-slate-500">{result ? `${count} unfinished steps need an active assignee` : "Checking assignments…"}</p></div><button ref={close} onClick={() => { setOpen(false); bell.current?.focus(); }} className="min-h-11 min-w-11 rounded-lg px-2 text-sm" aria-label="Close notifications">✕</button></div>
      <div className="max-h-[60vh] overflow-y-auto overscroll-contain">
        {error && <p role="alert" className="p-4 text-sm text-red-800">{error}</p>}
        {!error && result && count === 0 && <p className="p-5 text-sm text-slate-600">No assignment warnings. All unfinished steps have active assignees.</p>}
        <ul className="divide-y divide-slate-100">{result?.reports.flatMap(report => report.warnings.map(warning => <li key={`${report.project_id}:${warning.kind}:${warning.key}`}><Link onClick={() => setOpen(false)} className="block break-words p-4 hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-blue-600" href={assignmentHref(report.project_id, warning)}><p className="text-xs font-semibold text-orange-800">{report.project_name}</p><p className="mt-1 text-sm font-semibold">{warning.label}</p><p className="mt-1 text-xs leading-5 text-slate-600">{warning.reason}</p></Link></li>))}</ul>
      </div>
      <div className="border-t border-slate-200 px-4 py-2"><button onClick={() => setRefresh(value => value + 1)} className="min-h-11 text-sm font-semibold text-blue-700">Refresh notifications</button></div>
    </section>}
  </div>;
}
