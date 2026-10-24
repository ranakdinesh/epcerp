"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { ChartPanel } from "./dashboard-charts";
import { type DashboardProject } from "./dashboard-model";
import { type DeliveryBoard } from "@/components/projects/delivery-plan-data";

type Progress = { value: number | null; note: string };
export function RecordedProgress({ projects, visible, revision }: { projects: DashboardProject[]; visible: DashboardProject[]; revision: number }) {
  const user = useAuthenticatedUser();
  const allowed = user.permissions.includes("system:admin") || user.permissions.includes("projects.delivery.read");
  const [values, setValues] = useState<Record<string, Progress>>({});
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      setLoading(true);
      const next: Record<string, Progress> = {};
      if (allowed) for (let i = 0; i < projects.length; i += 4) {
        await Promise.all(projects.slice(i, i + 4).map(async project => {
          try {
            const response = await fetch(`/api/projects/${project.id}/delivery-plan`, {cache: "no-store", signal: controller.signal});
            if (!response.ok) throw Error(response.status === 403 ? "Access unavailable" : "Unable to load progress");
            const board: DeliveryBoard = await response.json();
            next[project.id] = board.plan ? {value: board.progress, note: `${board.plan.state} plan`} : {value: null, note: "No delivery plan"};
          } catch (error) { next[project.id] = {value: null, note: (error as Error).message}; }
        }));
        if (controller.signal.aborted) return;
      }
      if (!controller.signal.aborted) { setValues(next); setLoading(false); }
    }
    void load();
    return () => controller.abort();
  }, [projects, revision, allowed]);
  return <div className="grid gap-5 xl:grid-cols-2">
    <ChartPanel title="Planned vs actual progress" subtitle="Current weighted task completion from saved delivery plans. A planned progress baseline has not been recorded.">
      {!allowed ? <p className="text-sm text-slate-500">Delivery read permission is required to view progress.</p> : loading ? <p role="status" className="text-sm text-slate-500">Loading recorded progress…</p> : !visible.length ? <p className="text-sm text-slate-500">No projects match this selection.</p> : <div className="max-h-96 overflow-auto"><table className="vtpl-data-table w-full text-left text-sm"><thead className="text-xs text-slate-500"><tr><th className="py-3 pr-3">Project</th><th className="px-2 py-3">Planned</th><th className="px-2 py-3">Actual</th></tr></thead><tbody className="divide-y divide-slate-100">{visible.map(project => { const progress = values[project.id]; return <tr key={project.id}><th className="py-3 pr-3 font-medium"><Link className="inline-flex min-h-11 items-center text-blue-700 underline" href={`/dashboard?screen=projects&project=${project.id}&tab=control`}>{project.name}</Link></th><td className="px-2 py-3 text-slate-500">Not recorded</td><td className="px-2 py-3">{progress?.value != null ? <><span className="font-semibold">{progress.value.toFixed(1)}%</span><span className="mt-1 block text-xs text-slate-500">{progress.note}</span></> : <span className="text-slate-500">{progress?.note ?? "Unavailable"}</span>}</td></tr>; })}</tbody></table></div>}
    </ChartPanel>
    <ChartPanel title="Cash-flow outlook" source="No data" subtitle="Recorded receipts and payments">
      <p className="text-sm text-slate-500">No cash-flow records are available. Receipts, payments and forecasts have not been recorded in this system.</p>
    </ChartPanel>
  </div>;
}
