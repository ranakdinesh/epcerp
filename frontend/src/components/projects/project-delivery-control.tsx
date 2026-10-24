"use client";
import { ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";
import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { useProjectUser } from "@/components/projects/project-permissions";
import {
  processRequest,
  processButton,
  processSecondary,
} from "./project-process-data";
import { ProjectControlPanel } from "./project-control-panel";
import {
  DeliveryBoard,
  TaskTemplate,
  TaskIssue,
  outline,
  deliveryEdges,
  blockedMinutes,
  hours,
  deliveryField as field,
} from "./delivery-plan-data";
import { DeliveryPlanEditor } from "./delivery-plan-editor";
import { DeliveryTaskActions, EscalationQueue } from "./delivery-task-actions";
import { ProjectControlTimeline } from "./project-control-timeline";
import {
  ControlRow,
  downstream,
  pendingDependencies,
} from "./project-control-model";

export function ProjectDeliveryControl({
  projectId,
  startDate,
  finishDate,
}: {
  projectId: string;
  startDate: string | null;
  finishDate: string | null;
}) {
  const user = useProjectUser();
  const params = useSearchParams();
  const allowed =
    user.permissions.includes("system:admin") ||
    (user.permissions.includes("projects.delivery.view") &&
      user.permissions.includes("projects.delivery.read"));
  if (!allowed || params.get("mode") === "manual")
    return (
      <div className="space-y-4">
        {allowed ? (
          <Link
            className="text-sm font-bold text-blue-700 underline"
            href={`/dashboard?screen=projects&project=${projectId}&tab=control`}
          >
            ← Template task plan
          </Link>
        ) : null}
        <ProjectControlPanel projectId={projectId} targetFinish={finishDate} />
      </div>
    );
  return (
    <DeliveryControl
      key={projectId}
      projectId={projectId}
      startDate={startDate}
      finishDate={finishDate}
    />
  );
}
function DeliveryControl({
  projectId,
  startDate,
  finishDate,
}: {
  projectId: string;
  startDate: string | null;
  finishDate: string | null;
}) {
  const user = useProjectUser();
  const router = useRouter();
  const params = useSearchParams();
  const [board, setBoard] = useState<DeliveryBoard | null>(null);
  const [issues, setIssues] = useState<TaskIssue[]>([]);
  const [issueError, setIssueError] = useState("");
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [editing, setEditing] = useState<DeliveryBoard | null>(null);
  const [templates, setTemplates] = useState<TaskTemplate[]>([]);
  const [template, setTemplate] = useState("");
  const [start, setStart] = useState(startDate ?? "");
  const [finish, setFinish] = useState(finishDate ?? "");
  const [saving, setSaving] = useState(false);
  const [history, setHistory] = useState<
    | {
        id: string;
        action: string;
        reason: string;
        created_at: string;
        created_by_id: string | null;
      }[]
    | null
  >(null);
  const focus = params.get("task") ?? "";
  const filter = params.get("tasks") ?? "all";
  const expanded = new Set(
    (params.get("open") ?? "").split(",").filter(Boolean),
  );
  const canIssues =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("projects.escalations.read");
  const canTemplates =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.task_templates.read");
  useEffect(() => {
    const c = new AbortController();
    void processRequest<DeliveryBoard>(
      `/api/projects/${projectId}/delivery-plan`,
      { signal: c.signal },
    )
      .then((value) => {
        setBoard(value);
        setError("");
      })
      .catch((e) => {
        if (!c.signal.aborted) setError(e.message);
      });
    return () => c.abort();
  }, [projectId, reload]);
  useEffect(() => {
    if (!canIssues) return;
    const c = new AbortController();
    void processRequest<TaskIssue[]>(
      `/api/projects/${projectId}/task-escalations`,
      { signal: c.signal },
    )
      .then((value) => {
        setIssues(value);
        setIssueError("");
      })
      .catch((e) => {
        if (!c.signal.aborted) setIssueError(e.message);
      });
    return () => c.abort();
  }, [projectId, reload, canIssues]);
  useEffect(() => {
    if (!canTemplates) return;
    const c = new AbortController();
    void processRequest<TaskTemplate[]>("/api/settings/task-templates", {
      signal: c.signal,
    })
      .then(setTemplates)
      .catch(() => {});
    return () => c.abort();
  }, [canTemplates]);
  useEffect(() => {
    const timer = setInterval(() => setReload((v) => v + 1), 60000);
    return () => clearInterval(timer);
  }, []);
  function change(values: Record<string, string>) {
    const next = new URLSearchParams(params);
    next.set("tab", "control");
    for (const [k, v] of Object.entries(values)) {
      if (v) next.set(k, v);
      else next.delete(k);
    }
    router.push(`/dashboard?${next}`, { scroll: false });
  }
  function refresh() {
    setReload((v) => v + 1);
    setHistory(null);
  }
  function choose(code: string) {
    const next = new Set(expanded);
    let n = board?.nodes.find((n) => n.code === code);
    while (n?.parent_code) {
      next.add(n.parent_code);
      n = board?.nodes.find((p) => p.code === n!.parent_code);
    }
    change({ task: code, open: [...next].join(",") });
  }
  async function generate() {
    setError("");
    setSaving(true);
    try {
      await processRequest(`/api/projects/${projectId}/delivery-plan`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          template_id: template || null,
          start_date: start,
          finish_date: finish,
        }),
      });
      refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  const manual = (
    <Link
      className="min-h-11 text-sm font-bold text-blue-700 underline"
      href={`/dashboard?screen=projects&project=${projectId}&tab=control&mode=manual`}
    >
      Manual process / schedule records →
    </Link>
  );
  if (!board)
    return (
      <div
        className="rounded-2xl bg-white p-5"
        role={error ? "alert" : "status"}
      >
        {error || "Loading project task plan…"}
        {error ? (
          <button onClick={refresh} className={processSecondary + " ml-3"}>
            Retry
          </button>
        ) : null}
      </div>
    );
  if (!board.plan)
    return (
      <section className="space-y-5 rounded-2xl border border-slate-200 bg-white p-5">
        <h3 className="text-2xl font-bold">
          Build this project’s complete task map
        </h3>
        <p className="text-sm text-slate-600">
          Generate an independent draft from a standard task template, then
          review weights, dates, manager and doer/resolver assignments. Existing
          process and manual schedule records remain separate.
        </p>
        {error ? (
          <p role="alert" className="text-red-700">
            {error}
          </p>
        ) : null}
        {board.can_generate ? (
          <>
            <div className="grid gap-4 sm:grid-cols-3">
              <ValidatedLabel className="text-sm font-bold">
                Task template
                <ValidatedSelect name="template"
                  value={template}
                  onChange={(e) => setTemplate(e.target.value)}
                  className={field}
                >
                  <option value="">Latest standard EPC template</option>
                  {templates
                    .filter((t) => t.is_active)
                    .map((t) => (
                      <option value={t.id} key={t.id}>
                        {t.name} · revision {t.revision}
                      </option>
                    ))}
                </ValidatedSelect>
              </ValidatedLabel>
              <ValidatedLabel className="text-sm font-bold">
                Project start date
                <ValidatedInput name="start"
                  type="date"
                  value={start}
                  onChange={(e) => setStart(e.target.value)}
                  className={field}
                />
              </ValidatedLabel>
              <ValidatedLabel className="text-sm font-bold">
                Project finish date
                <ValidatedInput name="finish"
                  type="date"
                  value={finish}
                  onChange={(e) => setFinish(e.target.value)}
                  className={field}
                />
              </ValidatedLabel>
            </div>
            <button
              disabled={saving || !start || !finish}
              onClick={() => void generate()}
              className={processButton}
            >
              {saving ? "Generating…" : "Generate draft task plan"}
            </button>
          </>
        ) : (
          <p className="text-sm text-slate-600">
            A project manager with delivery-plan write access can generate this
            plan.
          </p>
        )}
        {manual}
      </section>
    );
  const plan = board.plan;
  const byCode = new Map(board.nodes.map((n) => [n.code, n]));
  const selected = byCode.get(focus);
  const openIssues = issues.filter((i) => !i.resolved_at);
  const blockedCodes = new Set(
    openIssues
      .map((i) => board.nodes.find((n) => n.id === i.node_id)?.code)
      .filter((c): c is string => !!c),
  );
  const graph = deliveryEdges(board.nodes);
  const allRows: ControlRow[] = board.nodes.map((n) => ({
    id: n.code,
    parent: n.parent_code,
    kind: n.parent_code ? "task" : "process",
    name: `${n.code} · ${n.name} · ${n.progress.toFixed(1)}%`,
    owner:
      board.members.find((m) => m.id === n.effective_doer_id)?.label ??
      "Unassigned doer",
    start: n.start_date,
    finish: n.finish_date,
    status: graph.leaves(n.code).some((c) => blockedCodes.has(c))
      ? "blocked"
      : n.progress === 100
        ? "completed"
        : n.progress > 0 || n.status === "in_progress"
          ? "in_progress"
          : "not_started",
    milestone: false,
    actualStart: n.actual_start?.slice(0, 10) ?? null,
    actualFinish: n.actual_finish?.slice(0, 10) ?? null,
    confirmed: true,
  }));
  const openGraph = pendingDependencies(allRows, graph.edges);
  const impact = new Set<string>();
  if (selected)
    for (const leaf of graph.leaves(selected.code))
      for (const c of downstream(leaf, openGraph)) impact.add(c);
  const matches = (n: (typeof board.nodes)[number]) =>
    filter === "all" ||
    (filter === "mine" && n.effective_doer_id === user.id) ||
    (filter === "blocked" &&
      graph.leaves(n.code).some((c) => blockedCodes.has(c))) ||
    (filter === "overdue" &&
      n.progress < 100 &&
      !!n.finish_date &&
      n.finish_date < board.as_of.slice(0, 10));
  const ordered = outline(board.nodes).filter((n) => {
    if (filter !== "all") return matches(n);
    let parent = n.parent_code;
    while (parent) {
      if (!expanded.has(parent)) return false;
      parent = byCode.get(parent)?.parent_code ?? null;
    }
    return true;
  });
  const visible = ordered.map((n) => allRows.find((r) => r.id === n.code)!);
  return (
    <section className="space-y-5">
      <header className="flex flex-wrap items-start justify-between gap-4 rounded-2xl border border-slate-200 bg-white p-5">
        <div>
          <p className="text-xs font-bold uppercase tracking-wide text-blue-700">
            Project control · {plan.state} task plan
          </p>
          <h3 className="mt-1 text-2xl font-bold">
            A complete map. Clear accountability.
          </h3>
          <p className="mt-2 text-sm text-slate-600">
            {plan.template_name} · template revision {plan.template_revision} ·
            plan version {plan.version}
          </p>
          <p className="mt-1 text-xs text-slate-500">
            Manager:{" "}
            {board.members.find((m) => m.id === plan.manager_id)?.label ??
              "Not assigned"}{" "}
            · {plan.start_date ?? "Start needed"} →{" "}
            {plan.finish_date ?? "Finish needed"} · Updated{" "}
            {new Date(board.as_of).toLocaleString()}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button onClick={refresh} className={processSecondary}>
            Refresh
          </button>
          {board.can_manage && plan.state === "draft" ? (
            <button
              onClick={() => setEditing(structuredClone(board))}
              className={processButton}
            >
              Review & assign plan
            </button>
          ) : null}
        </div>
      </header>
      {error ? (
        <p role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">
          {error} Showing the last loaded plan.
        </p>
      ) : null}
      {plan.state === "draft" ? (
        <p className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">
          Draft plan: confirm scope, 100% allocations, project dates, manager
          and doer/resolver pairs before activating execution. Template defaults
          are proposals.
        </p>
      ) : null}
      {editing ? (
        <DeliveryPlanEditor
          key={editing.plan?.version}
          projectId={projectId}
          board={editing}
          onSaved={() => {
            setEditing(null);
            refresh();
          }}
          onClose={() => setEditing(null)}
        />
      ) : null}
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div className="rounded-xl border border-blue-200 bg-white p-4">
          <p className="text-xs font-bold text-slate-500">
            WEIGHTED PROJECT COMPLETION
          </p>
          <p className="mt-2 text-3xl font-extrabold text-blue-800">
            {board.progress.toFixed(2)}%
          </p>
          <progress
            aria-label="Weighted project completion"
            value={board.progress}
            max={100}
            className="mt-3 h-2 w-full accent-blue-700"
          />
          <p className="mt-2 text-xs text-slate-500">
            Evidence-backed leaf outcomes; no double counting.
          </p>
        </div>
        <div className="rounded-xl border border-slate-200 bg-white p-4">
          <p className="text-xs font-bold text-slate-500">EXECUTION OUTCOMES</p>
          <p className="mt-2 text-2xl font-bold">
            {board.nodes.filter((n) => n.leaf && n.progress === 100).length} /{" "}
            {board.nodes.filter((n) => n.leaf).length}
          </p>
          <p className="mt-2 text-xs text-slate-500">
            Completed subtasks ·{" "}
            {board.nodes.filter((n) => !n.parent_code).length} processes
          </p>
        </div>
        <div className="rounded-xl border border-slate-200 bg-white p-4">
          <p className="text-xs font-bold text-slate-500">OPEN BLOCKERS</p>
          <p className="mt-2 text-2xl font-bold">
            {canIssues && !issueError ? openIssues.length : "Unavailable"}
          </p>
          <p className="mt-2 text-xs text-slate-500">
            {openIssues.filter((i) => i.overdue_minutes > 0).length} beyond
            resolver deadline
          </p>
        </div>
        <div className="rounded-xl border border-slate-200 bg-white p-4">
          <p className="text-xs font-bold text-slate-500">
            COMPLETION FORECAST
          </p>
          <p className="mt-2 text-lg font-bold">Not calculated</p>
          <p className="mt-2 text-xs text-slate-500">
            Task blocked time is recorded separately from project-finish impact.
          </p>
        </div>
      </div>
      <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white">
        <div className="flex flex-wrap items-center justify-between gap-3 p-4">
          <h4 className="font-bold">
            Weighted process, task & subtask timeline
          </h4>
          <ValidatedLabel className="text-sm font-bold">
            Show{" "}
            <ValidatedSelect name="filter"
              aria-label="Task map filter"
              value={filter}
              onChange={(e) => change({ tasks: e.target.value })}
              className="ml-2 min-h-11 rounded-lg border border-slate-300 px-3"
            >
              <option value="all">All work</option>
              <option value="mine">My tasks</option>
              <option value="blocked">Blocked</option>
              <option value="overdue">Overdue</option>
            </ValidatedSelect>
          </ValidatedLabel>
        </div>
        <p className="px-4 pb-3 text-xs text-slate-500">
          Expand each row to its children. Percentages are calculated
          completion, not elapsed time. On mobile, scroll horizontally for
          dates.
        </p>
        <ProjectControlTimeline
          rows={visible}
          allRows={allRows}
          links={graph.edges}
          selected={focus}
          impacted={impact}
          today={board.as_of.slice(0, 10)}
          expanded={expanded}
          onSelect={choose}
          onToggle={(code) => {
            const next = new Set(expanded);
            if (next.has(code)) next.delete(code);
            else next.add(code);
            change({ open: [...next].join(",") });
          }}
        />
      </div>
      {selected ? (
        <section className="space-y-4 rounded-2xl border border-blue-200 bg-white p-5">
          <div>
            <p className="text-xs font-bold uppercase text-blue-700">
              Selected {selected.leaf ? "subtask" : "summary"}
            </p>
            <h4 className="mt-1 text-xl font-bold">
              {selected.code} · {selected.name}
            </h4>
            <p className="mt-2 text-sm text-slate-600">
              {selected.weight_bp / 100}% of {selected.parent_code ?? "project"}{" "}
              · {selected.progress.toFixed(2)}% complete · Recorded blocked time
              (overlaps counted once){" "}
              {hours(
                blockedMinutes(
                  issues,
                  new Set(
                    graph
                      .leaves(selected.code)
                      .map((code) => byCode.get(code)!.id),
                  ),
                  board.as_of,
                ),
              )}
            </p>
          </div>
          {selected.process_code === "material_delivery" &&
            (user.permissions.includes("system:admin") ||
              user.permissions.includes("projects.purchase_workflow.view")) && (
              <Link
                className="inline-flex min-h-11 items-center rounded-lg bg-blue-700 px-4 text-sm font-bold text-white"
                href={`/dashboard?screen=projects&project=${projectId}&tab=purchasing&purchaseView=workflow`}
              >
                Open material purchase workspaces →
              </Link>
            )}
          <DeliveryTaskActions
            key={`${selected.id}:${plan.version}`}
            projectId={projectId}
            board={board}
            node={selected}
            issues={issues}
            onSaved={refresh}
          />
          <div className="grid gap-4 border-t border-slate-200 pt-4 sm:grid-cols-2">
            <div>
              <h5 className="font-bold">Required outcomes not yet complete</h5>
              {selected.unmet_predecessors.map((code) => (
                <button
                  key={code}
                  onClick={() => choose(code)}
                  className="block min-h-11 text-left text-sm text-blue-700 underline"
                >
                  {code} · {byCode.get(code)?.name}
                </button>
              ))}
              {!selected.unmet_predecessors.length ? (
                <p className="mt-2 text-sm text-slate-500">
                  No unmet mapped prerequisites.
                </p>
              ) : null}
            </div>
            <div>
              <h5 className="font-bold">Potential dependent work</h5>
              <p className="mt-2 text-xs text-slate-500">
                {impact.size} unfinished dependent subtasks. This is dependency
                exposure, not calculated delay.
              </p>
              <div className="max-h-48 overflow-y-auto">
                {[...impact].map((code) => (
                  <button
                    key={code}
                    onClick={() => choose(code)}
                    className="block min-h-11 text-left text-sm text-blue-700 underline"
                  >
                    {code} · {byCode.get(code)?.name}
                  </button>
                ))}
              </div>
            </div>
          </div>
        </section>
      ) : (
        <p className="text-sm text-slate-600">
          Select any row to see assignments, completion weight, dates, blockers
          and dependent tasks.
        </p>
      )}
      {canIssues && !issueError ? (
        <EscalationQueue
          projectId={projectId}
          board={board}
          issues={issues}
          onSaved={refresh}
          onSelect={choose}
        />
      ) : (
        <p className="rounded-xl bg-amber-50 p-4 text-sm text-amber-900">
          Escalation information is unavailable:{" "}
          {issueError || "outside your access"}. Blocker indicators may be
          incomplete.
        </p>
      )}
      <div className="rounded-2xl border border-slate-200 bg-white p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          {manual}
          <button
            className={processSecondary}
            onClick={() => {
              void processRequest<typeof history>(
                `/api/projects/${projectId}/delivery-plan/history`,
              )
                .then(setHistory)
                .catch((e) => setError(e.message));
            }}
          >
            View plan audit history
          </button>
        </div>
        {history ? (
          <ol className="mt-4 max-h-80 space-y-3 overflow-y-auto">
            {history.map((e) => (
              <li key={e.id} className="rounded-lg bg-slate-50 p-3 text-sm">
                <p className="font-bold">
                  {e.action.replaceAll("_", " ")} ·{" "}
                  {new Date(e.created_at).toLocaleString()}
                </p>
                <p>{e.reason}</p>
                <p className="text-xs text-slate-500">
                  {board.members.find((m) => m.id === e.created_by_id)?.label ??
                    e.created_by_id ??
                    "System"}
                </p>
              </li>
            ))}
          </ol>
        ) : null}
      </div>
    </section>
  );
}
