"use client";
import {
  ValidatedInput,
  ValidatedLabel,
  ValidatedSelect,
} from "@/components/ui/validated-controls";
import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import {
  ProgressFlag,
  type ProgressReport,
  type ProgressSignal,
} from "./purchase-progress";
import { purchaseStepAllowed } from "@/components/projects/workspace-permissions";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { Metric } from "@/components/dashboards/dashboard-charts";
import { type WorkflowList } from "./workflow-data";
import {
  pendingPurchaseSteps,
  matchesPurchaseProject,
  purchaseProjectSignal,
  purchasePhases,
  purchaseTasks,
  purchaseTaskHref,
  purchaseProjectAction,
  type PurchaseSource,
  type PurchaseProject,
  type PurchaseTask,
} from "./purchase-dashboard-model";

export function PurchaseDashboard() {
  const user = useAuthenticatedUser(),
    router = useRouter(),
    params = useSearchParams();
  const allowed =
    user.permissions.includes("system:admin") ||
    (user.permissions.includes("projects.purchase_workflow.view") &&
      user.permissions.includes("projects.purchase_workflow.read"));
  const [sources, setSources] = useState<PurchaseSource[]>([]);
  const [issues, setIssues] = useState<string[]>([]);
  const [loading, setLoading] = useState(true),
    [revision, setRevision] = useState(0),
    [updated, setUpdated] = useState("");
  const projectFilter = params.get("purchaseProject") ?? "";
  const lifecycle = params.get("purchaseLifecycle") ?? "";
  const completion = params.get("purchaseCompletion") ?? "";
  const phase = params.get("phase") ?? "";
  const queue = params.get("queue") ?? "all";
  const mine = params.get("mine") === "true";
  function change(key: string, value: string) {
    const query = new URLSearchParams(params);
    if (value) query.set(key, value);
    else query.delete(key);
    window.history.replaceState(null, "", `/dashboard?${query}`);
  }
  useEffect(() => {
    if (!allowed) return;
    const controller = new AbortController();
    async function load() {
      setLoading(true);
      const problems: string[] = [];
      const next: PurchaseSource[] = [];
      try {
        let page = 1;
        while (true) {
          const response = await fetch(
            `/api/projects?page=${page}&page_size=100&sort=name_asc&status=active&purchase_enabled=true`,
            { cache: "no-store", signal: controller.signal },
          );
          if (!response.ok)
            throw Error("Purchase projects could not be loaded. Please retry.");
          const result: { items: PurchaseProject[]; total_pages: number } =
            await response.json();
          const projects = result.items.filter((p) => p.is_active && p.purchase_enabled);
          for (let i = 0; i < projects.length; i += 4) {
            await Promise.all(
              projects.slice(i, i + 4).map(async (project) => {
                try {
                  const response = await fetch(
                    `/api/projects/${project.id}/purchase-workflow`,
                    { cache: "no-store", signal: controller.signal },
                  );
                  if (response.status === 403 || response.status === 404) {
                    problems.push(
                      `${project.name}: purchase access is unavailable. Ask the project manager to check your project capabilities.`,
                    );
                    return;
                  }
                  if (!response.ok) throw Error();
                  const data = (await response.json()) as WorkflowList;
                  let progress: ProgressReport | null = null;
                  let progressError = "";
                  try {
                    const progressResponse = await fetch(
                      `/api/projects/${project.id}/purchase-progress`,
                      { cache: "no-store", signal: controller.signal },
                    );
                    if (!progressResponse.ok) throw Error();
                    progress =
                      (await progressResponse.json()) as ProgressReport;
                  } catch {
                    if (!controller.signal.aborted)
                      progressError = "Progress unavailable";
                  }
                  next.push({ project, data, progress, progressError });
                } catch {
                  if (!controller.signal.aborted)
                    problems.push(
                      `${project.name}: purchase records could not be loaded.`,
                    );
                }
              }),
            );
            if (controller.signal.aborted) return;
          }
          if (page >= result.total_pages) break;
          if (page >= 50) {
            problems.push(
              "Partial dashboard: the first 5,000 projects were checked.",
            );
            break;
          }
          page++;
        }
      } catch (error) {
        if (!controller.signal.aborted) problems.push((error as Error).message);
      }
      if (!controller.signal.aborted) {
        setSources(
          next.sort((a, b) => a.project.name.localeCompare(b.project.name)),
        );
        setIssues(problems);
        setUpdated(new Date().toLocaleTimeString());
        setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [allowed, revision]);
  useEffect(() => {
    if (!allowed || loading) return;
    const refresh = () => { if (document.visibilityState === "visible") setRevision(value => value + 1); };
    window.addEventListener("purchase-progress-saved", refresh);
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    const timer = window.setInterval(refresh, 30_000);
    return () => {
      window.removeEventListener("purchase-progress-saved", refresh);
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refresh);
      window.clearInterval(timer);
    };
  }, [allowed, loading]);
  const projectAction = (source: PurchaseSource) => purchaseProjectAction(source, number => purchaseStepAllowed(user.permissions, number));
  if (!allowed)
    return (
      <p role="alert">
        Purchase workflow view and read permissions are required.
      </p>
    );
  const selected = sources.filter(
    (source) =>
      (!projectFilter || source.project.id === projectFilter) &&
      matchesPurchaseProject(source, lifecycle, completion),
  );
  const tasks = purchaseTasks(selected, user.id);
  const filtered = tasks.filter(
    (task) =>
      (!phase || task.step.phase === phase) &&
      (!mine || task.mine) &&
      (queue === "overdue"
        ? task.overdue
        : queue === "blocked"
          ? task.step.state === "blocked"
          : queue === "unassigned"
            ? !task.step.doer_id
            : true),
  );
  const packages = selected.flatMap((source) =>
    source.data.packages.map((pkg) => ({
      id: pkg.id,
      project: source.project,
      pkg,
    })),
  );
  const complete = !issues.length;
  const metric = (value: number) =>
    loading ? "—" : complete ? value : `${value} loaded`;
  const selectClass =
    "mt-1 block min-h-11 max-w-full rounded-lg border border-slate-300 bg-white px-3 text-sm font-normal";
  return (
    <section className="dashboard-content purchase-dashboard">
      <header className="dashboard-page-header">
        <div>
          <h1>Purchase FMS dashboard</h1>
        </div>
        <button
          disabled={loading}
          onClick={() => setRevision((value) => value + 1)}
          className="dashboard-primary-action"
        >
          {loading ? "Refreshing…" : "Refresh purchase dashboard"}
        </button>
      </header>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <ValidatedLabel className="max-w-full text-xs font-semibold text-slate-600">
          Purchase project
          <ValidatedSelect
            name="project_filter"
            aria-label="Purchase project"
            className={selectClass}
            value={projectFilter}
            onChange={(event) => change("purchaseProject", event.target.value)}
          >
            <option value="">All enabled purchase projects</option>
            {sources.map((source) => (
              <option key={source.project.id} value={source.project.id}>
                {source.project.project_number} · {source.project.name}
              </option>
            ))}
          </ValidatedSelect>
        </ValidatedLabel>
        <p className="text-xs text-slate-500" role="status">
          {loading
            ? "Loading authorised purchase records…"
            : updated
              ? `Last refreshed ${updated}`
              : ""}
        </p>
      </div>
      {!!issues.length && (
        <div
          role="alert"
          className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800"
        >
          <p className="font-semibold">
            Purchase data is incomplete. Counts cover loaded records only.
          </p>
          <ul className="mt-2 list-inside list-disc">
            {issues.map((issue) => (
              <li key={issue}>{issue}</li>
            ))}
          </ul>
        </div>
      )}
      <div className="dashboard-metrics grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Metric
          label="Purchase packages"
          value={metric(packages.length)}
          note="Saved purchase workspaces"
        />
        <Metric
          tone="sage"
          label="My pending tasks"
          value={metric(tasks.filter((task) => task.mine).length)}
          note="Assigned preparation or blocker resolution"
        />
        <Metric
          tone="orange"
          label="Overdue tasks"
          value={metric(tasks.filter((task) => task.overdue).length)}
          note="Pending tasks past their recorded due date"
        />
        <Metric
          tone="ice"
          label="Blocked tasks"
          value={metric(
            tasks.filter((task) => task.step.state === "blocked").length,
          )}
          note="Recorded unresolved task blockers"
        />
      </div>
      <section
        className="dashboard-register"
        aria-label="Purchase project progress"
      >
        <div className="purchase-register-heading">
          <h2>Projects · purchase progress</h2>
          <details className="purchase-legend">
            <summary>Purchase FMS legend</summary>
            <div>
              <p>
                100% = full recorded purchase completion across all 20 weighted
                steps. Project progress averages its active BO lists.
              </p>
              <div className="purchase-legend-flags">
                {(
                  [
                    "complete",
                    "on_track",
                    "delayed",
                    "blocked",
                    "due_soon",
                    "attention",
                    "unplanned",
                  ] as ProgressSignal[]
                ).map((signal) => (
                  <ProgressFlag key={signal} signal={signal} />
                ))}
              </div>
              <p>
                Delayed: deadline passed. Due soon: within two days. Review
                dates: date conflict. No saved BO lists and unavailable progress
                are shown separately; browser drafts earn no credit.
              </p>
            </div>
          </details>
        </div>
        <DataGrid<PurchaseSource & { id: string }>
          label="Purchase project progress"
          stateKey="purchaseProjects"
          rows={selected.map((source) => ({
            ...source,
            id: source.project.id,
          }))}
          loading={loading}
          onRowClick={(row) => router.push(purchaseTaskHref(row.project.id))}
          toolbar={
            <>
              <ValidatedLabel className="text-xs font-semibold text-slate-600">
                Project status
                <ValidatedSelect
                  name="purchase_lifecycle"
                  aria-label="Purchase project status"
                  className={selectClass}
                  value={lifecycle}
                  onChange={(e) => change("purchaseLifecycle", e.target.value)}
                >
                  <option value="">All statuses</option>
                  <option value="active">Active / draft</option>
                  <option value="on_hold">On hold</option>
                  <option value="closed">Closed</option>
                </ValidatedSelect>
              </ValidatedLabel>
              <ValidatedLabel className="text-xs font-semibold text-slate-600">
                Purchase completion
                <ValidatedSelect
                  name="purchase_completion"
                  aria-label="Purchase completion"
                  className={selectClass}
                  value={completion}
                  onChange={(e) => change("purchaseCompletion", e.target.value)}
                >
                  <option value="">All progress</option>
                  <option value="complete">Complete · 100%</option>
                  <option value="in_progress">In progress</option>
                  <option value="not_started">Not started · 0%</option>
                  <option value="unavailable">
                    Not recorded / unavailable
                  </option>
                </ValidatedSelect>
              </ValidatedLabel>
            </>
          }
          columns={[
            {
              key: "project",
              label: "Project",
              value: (row) =>
                `${row.project.name} ${row.project.project_number}`,
              render: (row) => (
                <Link
                  className="font-semibold text-blue-800 underline-offset-4 hover:underline"
                  href={projectAction(row).href}
                >
                  {row.project.name}
                  <span className="mt-1 block text-xs font-normal text-slate-500">
                    {row.project.project_number}
                  </span>
                </Link>
              ),
            },
            {
              key: "lifecycle",
              label: "Project status",
              value: (row) => row.project.lifecycle_status === "draft" ? "Created" : row.project.lifecycle_status.replaceAll("_", " "),
              render: (row) => (
                <GridStatus
                  value={row.project.lifecycle_status === "draft" ? "Created" : row.project.lifecycle_status.replaceAll("_", " ")}
                />
              ),
            },
            {
              key: "progress",
              label: "Purchase completion",
              value: (row) => row.progress?.progress_percent ?? -1,
              render: (row) =>
                row.progress?.progress_percent != null ? (
                  <div className="purchase-project-progress">
                    <strong>{row.progress.progress_percent.toFixed(1)}%</strong>
                    <progress
                      max={100}
                      value={row.progress.progress_percent}
                      aria-label={`${row.project.name} purchase completion`}
                    />
                  </div>
                ) : (
                  <span className="text-slate-500">
                    {row.progressError || "No saved BO lists"}
                  </span>
                ),
            },
            {
              key: "bos",
              label: "BO lists",
              value: (row) => row.progress?.packages.length ?? "—",
            },
            {
              key: "signal",
              label: "Purchase FMS status",
              value: purchaseProjectSignal,
              render: (row) => {
                const signal = purchaseProjectSignal(row);
                return signal === "unavailable" || signal === "unrecorded" ? (
                  <span className="text-slate-500">
                    {signal === "unavailable" ? "Unavailable" : "Not recorded"}
                  </span>
                ) : (
                  <ProgressFlag signal={signal as ProgressSignal} />
                );
              },
            },
          ]}
          actions={(row) => (
            <button type="button" className="min-h-11 w-full min-w-0 whitespace-normal break-words rounded-lg px-2 text-left text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={() => router.push(projectAction(row).href)}>{projectAction(row).label}</button>
          )}
        />
      </section>
      <section className="dashboard-register" aria-label="Purchase work queue">
        <h3 className="text-lg font-bold">Purchase work queue</h3>
        <div className="flex flex-wrap items-end gap-3">
          <ValidatedLabel className="text-xs font-semibold text-slate-600">
            Phase
            <ValidatedSelect
              name="phase"
              className={selectClass}
              aria-label="Purchase phase"
              value={phase}
              onChange={(e) => change("phase", e.target.value)}
            >
              <option value="">All five phases</option>
              {purchasePhases.map((value) => (
                <option key={value} value={value}>
                  {
                    {
                      WO2I: "Work order to indent",
                      I2LOI: "Indent to LOI",
                      LOI2PO: "LOI to purchase order",
                      PO2INS: "Purchase order to inspection",
                      INS2D: "Inspection to delivery",
                    }[value]
                  }
                </option>
              ))}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="text-xs font-semibold text-slate-600">
            Follow-up
            <ValidatedSelect
              name="queue"
              className={selectClass}
              aria-label="Purchase follow-up"
              value={queue}
              onChange={(e) => change("queue", e.target.value)}
            >
              <option value="all">All pending tasks</option>
              <option value="overdue">Overdue</option>
              <option value="blocked">Blocked</option>
              <option value="unassigned">Unassigned</option>
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="flex min-h-11 items-center gap-2 text-sm">
            <ValidatedInput
              type="checkbox"
              checked={mine}
              onChange={(e) => change("mine", e.target.checked ? "true" : "")}
            />
            My assigned work
          </ValidatedLabel>
        </div>
        <DataGrid<PurchaseTask>
          label="Purchase tasks"
          rows={filtered}
          loading={loading}
          initialSort="due"
          columns={[
            { key: "project", label: "Project", value: (r) => r.project.name },
            {
              key: "package",
              label: "Package / material",
              value: (r) => `${r.package.code} · ${r.package.description}`,
            },
            {
              key: "step",
              label: "Step",
              value: (r) => `${r.step.number}. ${r.step.title}`,
            },
            { key: "phase", label: "Phase", value: (r) => r.step.phase },
            { key: "doer", label: "Doer", value: (r) => r.doer },
            { key: "resolver", label: "Resolver", value: (r) => r.resolver },
            {
              key: "due",
              label: "Planned date",
              value: (r) => r.step.planned_date ?? "Not set",
            },
            {
              key: "state",
              label: "Preparation / blocker",
              value: (r) => r.step.state.replaceAll("_", " "),
              render: (r) => (
                <div>
                  <GridStatus value={r.step.state.replaceAll("_", " ")} />
                  {r.overdue && (
                    <p className="mt-1 text-xs text-red-700">Overdue</p>
                  )}
                  {r.step.block_reason && (
                    <p className="mt-1 text-xs">{r.step.block_reason}</p>
                  )}
                </div>
              ),
            },
          ]}
          actions={(r) => (
            <GridAction
              label={`Open ${r.package.code} step ${r.step.number}`}
              onClick={() =>
                router.push(
                  purchaseTaskHref(r.project.id, r.package.id, r.step.number),
                )
              }
            />
          )}
        />
        <details className="text-xs text-slate-500">
          <summary className="min-h-11 cursor-pointer py-3 font-semibold">
            Workflow coverage
          </summary>
          <p className="pb-3 leading-5">
            This queue tracks saved preparation and blockers. Commercial
            decisions are recorded inside steps 3–5. Technical approvals, PO,
            inspection, delivery and Accounts execution are not yet implemented.
          </p>
        </details>
      </section>
      <section className="space-y-3">
        <h3 className="text-lg font-bold">Purchase package register</h3>
        <DataGrid
          label="Purchase packages"
          rows={packages}
          loading={loading}
          columns={[
            { key: "code", label: "Package", value: (r) => r.pkg.code },
            { key: "project", label: "Project", value: (r) => r.project.name },
            { key: "item", label: "Material", value: (r) => r.pkg.description },
            {
              key: "pending",
              label: "Pending preparation steps",
              value: (r) => pendingPurchaseSteps(r.pkg).length,
            },
            {
              key: "blocked",
              label: "Blockers",
              value: (r) =>
                r.pkg.steps.filter((step) => step.state === "blocked").length,
            },
          ]}
          actions={(r) => (
            <GridAction
              label={`Open package ${r.pkg.code}`}
              onClick={() =>
                router.push(
                  purchaseTaskHref(
                    r.project.id,
                    r.pkg.id,
                    pendingPurchaseSteps(r.pkg)[0]?.number,
                  ),
                )
              }
            />
          )}
        />
      </section>
    </section>
  );
}
