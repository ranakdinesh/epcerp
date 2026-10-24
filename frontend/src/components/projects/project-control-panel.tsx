"use client";
import { ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useProjectUser } from "@/components/projects/project-permissions";
import {
  ProcessBoard,
  processRequest,
  processSecondary,
  processStatus,
} from "./project-process-data";
import {
  ControlTask,
  TaskEdge,
  controlModel,
  downstream,
  pendingDependencies,
  overdue,
  day,
  ControlRow,
} from "./project-control-model";
import { ProjectControlTimeline } from "./project-control-timeline";

type ScheduleOptions = { owners: { id: string; label: string }[] };
type Data = {
  board: ProcessBoard;
  tasks: ControlTask[];
  edges: TaskEdge[];
  owners: ScheduleOptions["owners"];
  scheduleError: string;
  loaded: string;
};
const filters = ["all", "blocked", "overdue", "next14"] as const;
type Filter = (typeof filters)[number];
export function ProjectControlPanel({
  projectId,
  targetFinish,
}: {
  projectId: string;
  targetFinish: string | null;
}) {
  const user = useProjectUser();
  const params = useSearchParams();
  const router = useRouter();
  const canReadSchedule =
    user.permissions.includes("system:admin") ||
    (user.permissions.includes("projects.schedule.view") &&
      user.permissions.includes("projects.schedule.read"));
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const selected = params.get("focus") ?? "";
  const filter: Filter =
    filters.find((f) => f === params.get("filter")) ?? "all";
  const expanded = new Set(
    (params.get("expanded") ?? "").split(",").filter(Boolean),
  );
  function change(values: Record<string, string>) {
    const next = new URLSearchParams(params);
    next.set("tab", "control");
    for (const [key, value] of Object.entries(values)) {
      if (value) next.set(key, value);
      else next.delete(key);
    }
    router.push(`/dashboard?${next}`, { scroll: false });
  }
  useEffect(() => {
    const c = new AbortController();
    const init = { signal: c.signal };
    const base = `/api/projects/${projectId}`;
    const schedule = canReadSchedule
      ? Promise.all([
          processRequest<ControlTask[]>(`${base}/schedule/activities`, init),
          processRequest<TaskEdge[]>(`${base}/schedule/dependencies`, init),
          processRequest<ScheduleOptions>(
            `${base}/schedule/editor-options`,
            init,
          ),
        ])
          .then(([tasks, edges, options]) => ({
            tasks,
            edges,
            owners: options.owners,
            scheduleError: "",
          }))
          .catch((e: Error) => ({
            tasks: [],
            edges: [],
            owners: [],
            scheduleError: `Task schedule unavailable: ${e.message}`,
          }))
      : Promise.resolve({
          tasks: [],
          edges: [],
          owners: [],
          scheduleError:
            "Task schedule is outside your access. This view covers process records only.",
        });
    void Promise.all([
      processRequest<ProcessBoard>(`${base}/processes`, init),
      schedule,
    ])
      .then(([board, s]) => {
        if (!c.signal.aborted) {
          setData({ board, ...s, loaded: new Date().toISOString() });
          setError("");
        }
      })
      .catch((e: Error) => {
        if (!c.signal.aborted) setError(e.message);
      });
    return () => c.abort();
  }, [projectId, canReadSchedule, reload]);
  const source = (row: ControlRow) =>
    `/dashboard?screen=projects&project=${projectId}&tab=${row.kind === "process" ? "processes" : "schedule"}&${row.kind === "process" ? "process" : "activity"}=${row.id}`;
  if (!data)
    return (
      <div
        className="rounded-2xl bg-white p-6"
        role={error ? "alert" : "status"}
      >
        {error || "Loading project control…"}
        {error ? (
          <button
            className={processSecondary + " ml-3"}
            onClick={() => setReload((v) => v + 1)}
          >
            Retry
          </button>
        ) : null}
      </div>
    );
  const { rows, links } = controlModel(
    data.board,
    data.tasks,
    data.edges,
    data.owners,
  );
  const byId = new Map(rows.map((r) => [r.id, r]));
  const today = data.loaded.slice(0, 10);
  const processes = rows.filter((r) => r.kind === "process");
  const tasks = rows.filter((r) => r.kind === "task");
  const selectedRow = byId.get(selected);
  const pendingLinks = pendingDependencies(rows, links);
  const impacted = downstream(selected, pendingLinks);
  const pendingImpact = [...impacted]
    .map((id) => byId.get(id)!)
    .filter((r) => !["completed", "bypassed"].includes(r.status));
  const predecessors = links.filter((e) => e.to === selected);
  const successors = links.filter((e) => e.from === selected);
  const issues = rows
    .filter((r) => r.status === "blocked" || overdue(r, today) > 0)
    .map((row) => ({
      row,
      impact: [...downstream(row.id, pendingLinks)].filter(
        (id) => !["completed", "bypassed"].includes(byId.get(id)?.status ?? ""),
      ).length,
    }))
    .sort(
      (a, b) =>
        b.impact - a.impact || overdue(b.row, today) - overdue(a.row, today),
    );
  const matches = (r: ControlRow) =>
    filter === "all" ||
    (filter === "blocked" && r.status === "blocked") ||
    (filter === "overdue" && overdue(r, today) > 0) ||
    (filter === "next14" &&
      !["completed", "bypassed"].includes(r.status) &&
      !!r.finish &&
      day(r.finish) >= day(today) &&
      day(r.finish) <= day(today) + 14);
  const visible: ControlRow[] = [];
  for (const p of processes) {
    const children = tasks.filter((t) => t.parent === p.id);
    if (matches(p) || children.some(matches)) {
      visible.push(p);
      if (expanded.has(p.id) || filter !== "all")
        visible.push(...children.filter(matches));
    }
  }
  visible.push(
    ...tasks.filter((t) => !t.parent || !byId.has(t.parent)).filter(matches),
  );
  const milestones = tasks
    .filter(
      (t) =>
        t.milestone &&
        !["completed", "bypassed"].includes(t.status) &&
        t.finish,
    )
    .sort((a, b) => a.finish!.localeCompare(b.finish!));
  function choose(id: string) {
    const row = byId.get(id);
    const open = new Set(expanded);
    if (row?.parent) open.add(row.parent);
    for (const target of downstream(id, pendingLinks)) {
      const parent = byId.get(target)?.parent;
      if (parent) open.add(parent);
    }
    change({ focus: id, expanded: [...open].join(",") });
  }
  const missing = processes.filter((p) => !p.start || !p.finish).length;
  const unreviewed = processes.filter((p) => !p.confirmed).length;
  return (
    <section className="space-y-5">
      <header className="flex flex-wrap items-start justify-between gap-4 rounded-2xl border border-slate-200 bg-white p-5">
        <div>
          <p className="text-xs font-bold uppercase tracking-widest text-blue-700">
            Project control · draft plan
          </p>
          <h3 className="mt-1 text-2xl font-extrabold text-slate-900">
            See what needs to move next.
          </h3>
          <p className="mt-2 max-w-3xl text-sm text-slate-600">
            Trace processes and their tasks across time. Select a row to inspect
            its prerequisites and dependent work.
          </p>
          <p className="mt-2 text-xs text-slate-500">
            Updated {new Date(data.loaded).toLocaleString()} · Reporting date{" "}
            {today} (UTC)
          </p>
        </div>
        <button
          className={processSecondary}
          onClick={() => setReload((v) => v + 1)}
        >
          Refresh
        </button>
      </header>
      {error ? (
        <p role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">
          Refresh failed: {error}. Showing data from {data.loaded}.
        </p>
      ) : null}
      {data.scheduleError ? (
        <p
          role="status"
          className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900"
        >
          {data.scheduleError} Task counts and impact are incomplete.
        </p>
      ) : null}
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {[
          [
            "Observed attention",
            `${issues.length} blocked / overdue records`,
            issues.length
              ? "Review the action list below."
              : "No observed blockers; this does not establish that the project is on track.",
          ],
          [
            "Declared completion",
            `${processes.filter((p) => p.status === "completed").length} / ${processes.length} processes`,
            data.scheduleError
              ? "Task completion unavailable."
              : `${tasks.filter((t) => t.status === "completed").length} / ${tasks.length} tasks completed · not physical progress`,
          ],
          [
            "Project target",
            targetFinish ?? "Not recorded",
            "Forecast delay: not yet calculable. No approved schedule baseline.",
          ],
          [
            "Next draft milestone",
            milestones[0]?.finish ?? "Not scheduled",
            milestones[0]?.name ?? "Add dated milestones in the schedule.",
          ],
        ].map(([title, value, hint]) => (
          <div
            key={title}
            className="rounded-2xl border border-slate-200 bg-white p-4"
          >
            <p className="text-xs font-bold uppercase tracking-wide text-slate-500">
              {title}
            </p>
            <p className="mt-2 text-lg font-extrabold text-slate-900">
              {value}
            </p>
            <p className="mt-2 text-xs leading-5 text-slate-600">{hint}</p>
          </div>
        ))}
      </div>
      <div className="grid items-start gap-5 xl:grid-cols-[minmax(0,1fr)_320px]">
        <aside className="order-first rounded-2xl border border-slate-200 bg-white p-5 xl:order-last">
          <h4 className="text-lg font-bold">Needs action</h4>
          <p className="mt-1 text-xs leading-5 text-slate-500">
            Ranked by reachable incomplete dependencies, then overdue days. This
            is exposure, not critical-path ranking.
          </p>
          <div className="mt-4 max-h-96 space-y-2 overflow-y-auto">
            {issues.map(({ row, impact }) => (
              <button
                key={row.id}
                onClick={() => choose(row.id)}
                className={`w-full rounded-xl border p-3 text-left ${selected === row.id ? "border-blue-500 bg-blue-50" : "border-slate-200 hover:bg-slate-50"}`}
              >
                <span className="block text-sm font-bold">{row.name}</span>
                <span className="mt-1 block text-xs text-red-700">
                  {row.status === "blocked" ? "Blocked" : "Overdue"}
                  {overdue(row, today) > 0
                    ? ` · ${overdue(row, today)} calendar days overdue`
                    : ""}
                </span>
                <span className="mt-1 block text-xs text-slate-600">
                  {row.owner} · {impact} dependent{" "}
                  {row.kind === "process" ? "processes" : "tasks"}
                </span>
              </button>
            ))}
            {!issues.length ? (
              <p className="rounded-xl bg-slate-50 p-3 text-sm text-slate-600">
                No blocked or overdue records in the available data.
              </p>
            ) : null}
          </div>
          <div className="mt-5 border-t border-slate-200 pt-4">
            <h5 className="font-bold text-slate-800">Planning gaps</h5>
            <p className="mt-2 text-sm text-slate-600">
              {missing} processes need complete dates.
              <br />
              {unreviewed} dependency maps need review.
              <br />
              {tasks.filter((t) => !t.parent).length} tasks have no process
              link.
            </p>
            <Link
              className="mt-3 inline-flex min-h-11 items-center text-sm font-bold text-blue-700"
              href={`/dashboard?screen=projects&project=${projectId}&tab=processes`}
            >
              Review process setup →
            </Link>
          </div>
        </aside>
        <div className="min-w-0 space-y-4">
          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white">
            <div className="flex flex-wrap items-center justify-between gap-3 p-4">
              <div>
                <h4 className="font-bold">Process & task timeline</h4>
                <p className="mt-1 text-xs text-slate-500">
                  Draft dates · process status and task status are tracked
                  separately
                </p>
              </div>
              <ValidatedLabel className="text-sm font-semibold">
                Show{" "}
                <ValidatedSelect name="filter"
                  aria-label="Timeline filter"
                  value={filter}
                  onChange={(e) => change({ filter: e.target.value })}
                  className="ml-2 min-h-11 rounded-lg border border-slate-300 px-2"
                >
                  <option value="all">All work</option>
                  <option value="blocked">Blocked</option>
                  <option value="overdue">Overdue</option>
                  <option value="next14">Due in next 14 days</option>
                </ValidatedSelect>
              </ValidatedLabel>
            </div>
            <p className="px-4 pb-3 text-xs text-slate-500 sm:hidden">
              Scroll the timeline horizontally to see dates and dependency links
              →
            </p>
            <ProjectControlTimeline
              links={links}
              rows={visible}
              allRows={rows}
              selected={selected}
              impacted={impacted}
              today={today}
              expanded={expanded}
              onSelect={choose}
              onToggle={(id) => {
                const next = new Set(expanded);
                if (next.has(id)) next.delete(id);
                else next.add(id);
                change({ expanded: [...next].join(",") });
              }}
            />
            <p className="border-t border-slate-200 p-3 text-xs leading-5 text-slate-500">
              Bars represent entered planned dates, not approved baselines or
              calculated forecasts. Incomplete dates stay visible without an
              invented bar. Process rows do not automatically roll up task
              progress.
            </p>
          </div>
          <div
            className="rounded-2xl border border-slate-200 bg-white p-5"
            aria-live="polite"
          >
            {selectedRow ? (
              <>
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-xs font-bold uppercase tracking-wide text-blue-700">
                      Selected {selectedRow.kind}
                    </p>
                    <h4 className="mt-1 text-lg font-bold">
                      {selectedRow.name}
                    </h4>
                    <p className="mt-2 text-sm text-slate-600">
                      {processStatus(selectedRow.status)} · {selectedRow.owner}
                    </p>
                  </div>
                  <Link
                    href={source(selectedRow)}
                    className={processSecondary + " inline-flex items-center"}
                  >
                    Open {selectedRow.kind === "task" ? "task" : "process"}{" "}
                    details →
                  </Link>
                </div>
                <p className="mt-3 text-sm text-slate-600">
                  Actual start: {selectedRow.actualStart ?? "Not recorded"} ·
                  Actual finish: {selectedRow.actualFinish ?? "Not recorded"}
                </p>
                <div className="mt-5 grid gap-5 sm:grid-cols-2">
                  <div>
                    <h5 className="font-bold">Direct prerequisites</h5>
                    {predecessors.map((e) => {
                      const p = byId.get(e.from)!;
                      return (
                        <button
                          onClick={() => choose(p.id)}
                          key={e.from}
                          className="mt-2 block min-h-11 text-left text-sm text-blue-700 underline"
                        >
                          {p.name}
                          <span className="block text-xs text-slate-600">
                            {e.relationship} · {processStatus(p.status)}
                          </span>
                        </button>
                      );
                    })}
                    {!predecessors.length ? (
                      <p className="mt-2 text-sm text-slate-600">
                        {selectedRow.kind === "process" &&
                        !selectedRow.confirmed
                          ? "Map not reviewed. No conclusion about readiness."
                          : "No prerequisites mapped."}
                      </p>
                    ) : null}
                  </div>
                  <div>
                    <h5 className="font-bold">Potential downstream exposure</h5>
                    <p className="mt-2 text-sm text-slate-600">
                      {pendingImpact.length} incomplete dependent{" "}
                      {selectedRow.kind === "task" ? "tasks" : "processes"};{" "}
                      {successors.length} direct links.
                    </p>
                    {pendingImpact.map((p) => (
                      <button
                        key={p.id}
                        onClick={() => choose(p.id)}
                        className="mt-2 block min-h-11 text-left text-sm text-blue-700 underline"
                      >
                        {p.name}
                        {p.milestone ? " · Milestone" : ""}
                      </button>
                    ))}
                    <p className="mt-3 text-xs leading-5 text-slate-500">
                      Links show potential exposure. Completed work is excluded
                      from this list. Delay days and project-finish impact
                      require calendars, remaining durations and an approved
                      baseline. Process gates and task dependencies are separate
                      maps.
                    </p>
                  </div>
                </div>
                {selectedRow.parent ? (
                  <p className="mt-4 text-sm">
                    Process context:{" "}
                    <button
                      className="min-h-11 text-blue-700 underline"
                      onClick={() => choose(selectedRow.parent!)}
                    >
                      {byId.get(selectedRow.parent)?.name ?? "Unavailable"}
                    </button>
                  </p>
                ) : null}
              </>
            ) : (
              <>
                <h4 className="font-bold">
                  Trace a blocker through the project
                </h4>
                <p className="mt-2 text-sm text-slate-600">
                  Select a process or task in the timeline or action list. Its
                  prerequisites and reachable dependent work will appear here.
                </p>
                {selected ? (
                  <p className="mt-2 text-sm text-amber-800">
                    The selected record is not available in this view.
                  </p>
                ) : null}
              </>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
