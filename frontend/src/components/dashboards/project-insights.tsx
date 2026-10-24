"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { ProjectActionQueue } from "./project-action-queue";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import {
  type DeliveryBoard,
  type TaskIssue,
} from "@/components/projects/delivery-plan-data";
import { type ProcessBoard } from "@/components/projects/project-process-data";
import { ProjectControlTimeline } from "@/components/projects/project-control-timeline";
import {
  type ControlRow,
  controlModel,
} from "@/components/projects/project-control-model";
import {
  Bars,
  CompositionChart,
  ChartPanel,
  Metric,
} from "./dashboard-charts";

type ProjectSummary = {
  id: string;
  planned_start_date: string | null;
  planned_completion_date: string | null;
};
export function ProjectInsights({
  project,
  readiness,
}: {
  project: ProjectSummary;
  readiness: { total: number; completed: number; attention: number };
}) {
  const user = useAuthenticatedUser();
  const can = (area: string) =>
    user.permissions.includes("system:admin") ||
    user.permissions.includes(`projects.${area}.read`);
  const readEscalations = can("escalations");
  const [issues, setIssues] = useState<TaskIssue[] | null>(null);
  const readDelivery = can("delivery"),
    readProcesses = can("processes");
  const [board, setBoard] = useState<DeliveryBoard | null>(null),
    [processes, setProcesses] = useState<ProcessBoard | null>(null),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [refresh, setRefresh] = useState(0),
    [updated, setUpdated] = useState("");
  const [selected, setSelected] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      setLoading(true);
      const errors: string[] = [];
      async function read<T>(
        path: string,
        allowed: boolean,
      ): Promise<T | null> {
        if (!allowed) return null;
        try {
          const r = await fetch(`/api/projects/${project.id}/${path}`, {
            cache: "no-store",
            signal: controller.signal,
          });
          if (!r.ok)
            throw Error(
              r.status === 403
                ? `${path}: access is not available.`
                : `${path}: unable to load records.`,
            );
          return await r.json();
        } catch (e) {
          if (!controller.signal.aborted) errors.push((e as Error).message);
          return null;
        }
      }
      const [delivery, process, escalations] = await Promise.all([
        read<DeliveryBoard>("delivery-plan", readDelivery),
        read<ProcessBoard>("processes", readProcesses),
        read<TaskIssue[]>("task-escalations", readEscalations),
      ]);
      if (!controller.signal.aborted) {
        setBoard(delivery);
        setIssues(escalations);
        setProcesses(process);
        setError(errors.join(" "));
        setUpdated(new Date().toLocaleTimeString());
        setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [project.id, readDelivery, readProcesses, readEscalations, refresh]);
  const nodes = board?.nodes.filter((n) => n.leaf) ?? [];
  const roots = board?.nodes.filter((n) => !n.parent_code) ?? [];
  const processRows = processes ? controlModel(processes, [], [], []).rows : [];
  const rows: ControlRow[] = board?.plan
    ? roots.map((n) => ({
        id: n.code,
        parent: null,
        kind: "process",
        name: `${n.name} · ${n.progress.toFixed(1)}%`,
        owner:
          board.members.find((m) => m.id === n.effective_doer_id)?.label ??
          "Unassigned",
        start: n.start_date,
        finish: n.finish_date,
        status: n.status,
        milestone: false,
        actualStart: n.actual_start,
        actualFinish: n.actual_finish,
        confirmed: board.plan?.state === "active",
      }))
    : processRows;
  const today = new Date().toISOString().slice(0, 10);
  const timelineRows = rows;
  const stateCounts = board?.plan
    ? ["not_started", "in_progress", "blocked", "completed"].map((label) => ({
        label: label.replaceAll("_", " "),
        value: nodes.filter(
          (n) => (n.progress === 100 ? "completed" : n.status) === label,
        ).length,
      }))
    : ["not_started", "in_progress", "blocked", "completed"].map((label) => ({
        label: label.replaceAll("_", " "),
        value:
          processes?.processes.filter((p) => p.status === label).length ?? 0,
      }));
  const href = `/dashboard?screen=projects&project=${project.id}&tab=control`;
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3 className="text-2xl font-bold text-slate-900">
            Project dashboard
          </h3>
          <p className="mt-1 text-xs text-slate-500">
            {updated ? `Last refreshed ${updated}` : "Loading sources…"} ·{" "}
            {project.planned_start_date ?? "Start not set"} →{" "}
            {project.planned_completion_date ?? "Finish not set"}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-4">

          <button
            className="min-h-11 rounded-lg border border-slate-300 bg-white px-4 text-sm font-semibold"
            disabled={loading}
            onClick={() => setRefresh((n) => n + 1)}
          >
            Refresh
          </button>
        </div>
      </div>
      {error && (
        <p
          role="alert"
          className="rounded-xl bg-red-50 p-4 text-sm text-red-800"
        >
          {error}
        </p>
      )}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Metric
          label="Weighted task progress"
          value={
            loading
              ? "—"
              : board?.plan
                ? `${board.progress.toFixed(1)}%`
                : "Not available"
          }
          note={
            board?.plan
              ? `${board.plan.state} plan · declared task progress`
              : "Requires a readable delivery plan"
          }
        />
        <Metric
          tone="sage"
          label="Completed subtasks"
          value={
            loading
              ? "—"
              : board?.plan
                ? `${nodes.filter((n) => n.progress === 100).length} / ${nodes.length}`
                : "Not available"
          }
          note="Leaf tasks in the delivery plan"
        />
        <Metric
          tone="ice"
          label="Readiness completed"
          value={`${readiness.completed} / ${readiness.total}`}
          note="Recorded O2TE checklist"
        />
        <Metric
          tone="orange"
          label="Readiness needs action"
          value={readiness.attention}
          note="Recorded O2TE attention items"
        />
      </div>
      <ProjectActionQueue
        board={board}
        issues={issues}
        userId={user.id}
        projectId={project.id}
        loading={loading}
      />
      {loading ? (
        <p role="status">Loading project charts…</p>
      ) : (
        <div className="grid gap-5 lg:grid-cols-2">
          <ChartPanel
            title={board?.plan ? "Subtask status" : "Process status"}
            subtitle={
              board?.plan
                ? "Declared state of leaf tasks"
                : "Recorded process states"
            }
            source={board?.plan || processes ? "Recorded" : "Unavailable"}
          >
            {board?.plan || processes ? (
              <CompositionChart
                label="Project work status distribution"
                data={stateCounts}
              />
            ) : (
              <p className="text-sm text-slate-500">
                No authorised status source is available.
              </p>
            )}
          </ChartPanel>
          <ChartPanel
            title="Progress by major step"
            subtitle="Weighted completion · 0–100%"
            source={board?.plan ? "Recorded" : "Unavailable"}
          >
            {board?.plan ? (
              <div className="max-h-80 overflow-y-auto pr-2">
                <Bars
                  data={roots.map((n) => ({
                    label: n.name,
                    value: Number(n.progress.toFixed(1)),
                  }))}
                  unit="%"
                  max={100}
                />
              </div>
            ) : (
              <p className="text-sm text-slate-500">
                Generate a delivery plan in Project Control to track weighted
                progress.
              </p>
            )}
          </ChartPanel>
        </div>
      )}
      <ChartPanel
        title="Project Gantt · planned process timeline"
        subtitle="Saved planned dates · select a row to inspect ownership and open full dependency control"
        source={board?.plan?.state === "draft" ? "Draft plan" : "Recorded"}
      >
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <p className="text-xs text-slate-500">
            Recorded schedule ·{" "}
            {timelineRows.length} major steps ·{" "}
            {timelineRows.filter((r) => !r.start || !r.finish).length} without
            complete dates
          </p>
          <Link
            className="min-h-11 rounded-lg bg-blue-700 px-4 py-3 text-sm font-semibold text-white"
            href={href}
          >
            Open full Gantt & dependencies
          </Link>
        </div>
        {loading ? (
          <p>Loading timeline…</p>
        ) : timelineRows.length ? (
          <div className="max-h-[350px] overflow-auto rounded-lg">
            <ProjectControlTimeline
              rows={timelineRows}
              allRows={timelineRows}
              links={[]}
              selected={selected}
              impacted={new Set()}
              today={today}
              expanded={new Set()}
              onSelect={setSelected}
              onToggle={() => {}}
            />
          </div>
        ) : (
          <p className="py-5 text-sm text-slate-500">
            No accessible process or task plan is available yet. The full Gantt
            will appear here when planning records are ready.
          </p>
        )}
        {selected && timelineRows.some((r) => r.id === selected) && (
          <p className="mt-3 rounded-lg bg-blue-50 p-3 text-sm text-blue-900">
            {timelineRows.find((r) => r.id === selected)?.name} · Owner:{" "}
            {timelineRows.find((r) => r.id === selected)?.owner} ·{" "}
            <Link className="underline" href={href}>
              Inspect tasks and dependencies
            </Link>
          </p>
        )}
      </ChartPanel>

    </div>
  );
}
