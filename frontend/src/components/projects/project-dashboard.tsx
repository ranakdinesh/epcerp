"use client";
import {
  ValidatedInput,
  ValidatedLabel,
  ValidatedSelect,
} from "@/components/ui/validated-controls";
import { projectDateVariance } from "./project-date-model";
import { useProjectToday } from "./project-date-status";
import { dashboardLanding } from "@/components/dashboards/landing-model";
import { RecordedProgress } from "@/components/dashboards/recorded-progress";
import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import {
  Bars,
  CompositionChart,
  ChartPanel,
  Metric,
} from "@/components/dashboards/dashboard-charts";
import {
  matchesProjectStatus,
  completionBuckets,
  portfolioMetrics,
  type DashboardProject,
} from "@/components/dashboards/dashboard-model";

export function ProjectDashboard({
  openProjects,
  openProject,
}: {
  openProjects: () => void;
  openProject: (id: string) => void;
}) {
  const user = useAuthenticatedUser(),
    params = useSearchParams();
  const admin = user.permissions.includes("system:admin");
  const allowed = admin || user.permissions.includes("projects.master.read");
  const today = useProjectToday();
  const manager =
    dashboardLanding(user.permissions) === "manager" ||
    (admin && params.get("view") === "manager");
  const status = params.get("status") ?? "active";
  const lifecycle = params.get("lifecycle") ?? "";
  const search = params.get("q") ?? "";
  const managerName = params.get("manager") ?? "";
  const [purchaseHolds, setPurchaseHolds] = useState<
    Record<string, number | null>
  >({});
  const [projects, setProjects] = useState<DashboardProject[]>([]),
    [loading, setLoading] = useState(true),
    [error, setError] = useState(""),
    [refresh, setRefresh] = useState(0),
    [updated, setUpdated] = useState("");
  function change(key: string, value: string) {
    const next = new URLSearchParams(params);
    next.set(key, value);
    window.history.replaceState(null, "", `/dashboard?${next}`);
  }
  useEffect(() => {
    if (!allowed) return;
    const controller = new AbortController();
    async function load() {
      setLoading(true);
      setError("");
      try {
        const read = async (page: number) => {
          const r = await fetch(
            `/api/projects?page=${page}&page_size=100&sort=planned_completion_date_asc`,
            { cache: "no-store", signal: controller.signal },
          );
          if (!r.ok) throw Error("Project records could not be refreshed.");
          return r.json() as Promise<{
            items: DashboardProject[];
            total_pages: number;
          }>;
        };
        const first = await read(1);
        const all = [...first.items];
        for (let page = 2; page <= Math.min(first.total_pages, 50); page += 4) {
          const results = await Promise.all(
            Array.from(
              { length: Math.min(4, first.total_pages - page + 1, 51 - page) },
              (_, i) => read(page + i),
            ),
          );
          results.forEach((r) => all.push(...r.items));
        }
        const visible: DashboardProject[] = [];
        if (admin) visible.push(...all);
        else {
          for (let i = 0; i < all.length; i += 4) {
            const results = await Promise.all(
              all.slice(i, i + 4).map(async (p) => {
                const r = await fetch(`/api/projects/${p.id}/workspace`, {
                  cache: "no-store",
                  signal: controller.signal,
                });
                if (r.status === 403 || r.status === 404) return null;
                if (!r.ok)
                  throw Error(
                    "Workspace access could not be verified. Please refresh.",
                  );
                return p;
              }),
            );
            visible.push(
              ...results.filter((p): p is DashboardProject => p !== null),
            );
          }
        }
        const holds: Record<string, number | null> = {};
        for (let i = 0; i < visible.length; i += 4) {
          await Promise.all(
            visible.slice(i, i + 4).map(async (p) => {
              try {
                const response = await fetch(
                  `/api/projects/${p.id}/purchase-status`,
                  { cache: "no-store", signal: controller.signal },
                );
                if (!response.ok) {
                  holds[p.id] = null;
                  return;
                }
                const status = await response.json();
                holds[p.id] = status.active_holds.length;
              } catch {
                holds[p.id] = null;
              }
            }),
          );
        }
        if (!controller.signal.aborted) {
          setPurchaseHolds(holds);
          setProjects(visible);
          setUpdated(new Date().toLocaleTimeString());
          if (first.total_pages > 50)
            setError(
              "Partial portfolio: showing the first 5,000 records. Use the project register for the full list.",
            );
        }
      } catch (e) {
        if (!controller.signal.aborted) setError((e as Error).message);
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [allowed, admin, refresh]);
  if (!allowed)
    return (
      <section className="rounded-2xl border border-slate-200 bg-white p-6">
        <h2 className="text-2xl font-bold">Delivery overview</h2>
        <p className="mt-3 text-slate-600">
          Project read permission is required to display project dashboards.
        </p>
      </section>
    );
  const visible = projects.filter(
    (p) =>
      matchesProjectStatus(p, status) &&
      (!lifecycle || p.lifecycle_status === lifecycle) &&
      (!manager || !managerName || p.operations_manager_name === managerName) &&
      `${p.name} ${p.project_number} ${p.operations_manager_name}`
        .toLowerCase()
        .includes(search.toLowerCase()),
  );
  const metrics = portfolioMetrics(visible, today);
  const lateProjects = visible.map(project => ({project, status:projectDateVariance(project, today)})).filter(row => row.status.late);
  const pending = loading && !updated;
  const value = (n: number) =>
    pending ? "—" : error && !updated ? "Unavailable" : n;
  return (
    <section className="dashboard-content">
      <header className="dashboard-page-header">
        <div>
          <h1>
            {manager ? "Operations manager dashboard" : "Portfolio dashboard"}
          </h1>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setRefresh((n) => n + 1)}
            disabled={loading}
            className="min-h-11 rounded-xl border border-slate-300 bg-white px-4 text-sm font-semibold"
          >
            {loading ? "Refreshing…" : "Refresh"}
          </button>
          <button onClick={openProjects} className="dashboard-primary-action">
            Project register
          </button>
        </div>
      </header>
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-slate-100 p-3 text-sm">
        {admin ? (
          <div className="flex gap-2" aria-label="Dashboard perspective">
            {["portfolio", "manager"].map((v) => (
              <button
                key={v}
                aria-pressed={(manager ? "manager" : "portfolio") === v}
                onClick={() => change("view", v)}
                className={`min-h-11 rounded-lg px-4 font-semibold ${(manager ? "manager" : "portfolio") === v ? "bg-white text-blue-700 shadow-sm" : "text-slate-600"}`}
              >
                {v === "portfolio" ? "Portfolio overview" : "Manager view"}
              </button>
            ))}
          </div>
        ) : (
          <span>Authorised project workspaces</span>
        )}

        <span className="text-xs text-slate-500">
          {updated ? `Last refreshed ${updated}` : "Waiting for records"}
        </span>
      </div>
      {error && (
        <p
          role="alert"
          className="rounded-xl bg-red-50 p-4 text-sm text-red-800"
        >
          {error}
          {updated ? " Previously loaded records remain visible." : ""}
        </p>
      )}
      <div className="flex flex-wrap gap-3">
        <ValidatedLabel className="text-xs font-semibold text-slate-500">
          Project status
          <ValidatedSelect
            aria-label="Project status"
            value={
              ["active", "archived", "all"].includes(status) ? status : "active"
            }
            onChange={(e) => change("status", e.target.value)}
            className="mt-1 block min-h-11 rounded-lg border border-slate-300 bg-white px-3 text-sm"
          >
            <option value="active">Active</option>
            <option value="archived">Archived</option>
            <option value="all">All</option>
          </ValidatedSelect>
        </ValidatedLabel>
        <ValidatedLabel className="min-w-0 flex-1 text-xs font-semibold text-slate-500">
          Find project or manager
          <ValidatedInput
            name="search"
            className="mt-1 min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900"
            value={search}
            onChange={(e) => change("q", e.target.value)}
            placeholder="Project name, number or manager"
          />
        </ValidatedLabel>
        <ValidatedLabel className="text-xs font-semibold text-slate-500">
          Lifecycle
          <ValidatedSelect
            name="lifecycle"
            className="mt-1 block min-h-11 rounded-lg border border-slate-300 bg-white px-3 text-sm text-slate-900"
            value={lifecycle}
            onChange={(e) => change("lifecycle", e.target.value)}
          >
            <option value="">All stages</option>
            {["draft", "active", "on_hold", "closed"].map((s) => (
              <option key={s} value={s}>
                {s.replaceAll("_", " ")}
              </option>
            ))}
          </ValidatedSelect>
        </ValidatedLabel>
      </div>
      {manager && (
        <ValidatedLabel className="flex flex-wrap items-center gap-3 text-sm font-semibold text-slate-600">
          Manager portfolio
          <ValidatedSelect
            name="manager_name"
            aria-label="Manager portfolio"
            value={managerName}
            onChange={(e) => change("manager", e.target.value)}
            className="min-h-11 rounded-lg border border-slate-300 bg-white px-3"
          >
            <option value="">All accessible managers</option>
            {[
              ...new Set(
                projects.map((p) => p.operations_manager_name).filter(Boolean),
              ),
            ]
              .sort()
              .map((name) => (
                <option key={name}>{name}</option>
              ))}
          </ValidatedSelect>
        </ValidatedLabel>
      )}
      <section aria-label="Projects running late" className="rounded-xl border border-amber-300 bg-amber-50 p-4 text-amber-950">
        <div className="flex flex-wrap items-center justify-between gap-2"><h2 className="font-bold">Projects running late · {pending ? "…" : lateProjects.length}</h2><span className="text-xs">As of {today || "…"} · Asia/Kolkata · current filters</span></div>
        {pending ? <p className="mt-2 text-sm">Loading authorised project dates…</p> : !updated && error ? <p className="mt-2 text-sm">Schedule warnings are unavailable until project records load.</p> : lateProjects.length ? <ul className="mt-3 max-h-64 space-y-2 overflow-y-auto">{lateProjects.map(({project,status}) => <li key={project.id}><button type="button" onClick={() => openProject(project.id)} className="min-h-11 w-full rounded-lg border border-amber-200 bg-white px-3 py-2 text-left hover:bg-amber-100 focus:outline-none focus:ring-2 focus:ring-amber-700"><span className="font-semibold">{project.project_number} · {project.name}</span><span className="mt-1 block text-sm">{status.reasons.join(" · ")}</span></button></li>)}</ul> : <p className="mt-2 text-sm">No date-based delay found in the current projects. Missing dates cannot establish schedule status.</p>}
        <p className="mt-2 text-xs">Late start / start overdue = days after planned start. Completion overdue = days after the fixed target. Closed and deactivated projects are excluded.</p>
      </section>
      <div className="dashboard-metrics grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Metric
          label="Projects in view"
          value={value(visible.length)}
          note="Recorded project register"
        />
        <Metric
          tone="sage"
          label="Active projects"
          value={value(metrics.active)}
          note="Lifecycle marked active"
        />
        <Metric
          tone="orange"
          label="Past target date"
          value={value(metrics.overdue.length)}
          note="Open projects · review target dates"
        />
        <Metric
          tone="ice"
          label={manager ? "Targets not set" : "LOI / PO reference missing"}
          value={value(manager ? metrics.undated : metrics.missingReference)}
          note={
            manager
              ? "Open projects without a finish date"
              : "Neither reference has been recorded"
          }
        />
      </div>
      {pending || (!updated && error) ? (
        <p role="status" className="p-5 text-slate-500">
          {error
            ? "Recorded charts are unavailable until refresh succeeds."
            : "Loading recorded project charts…"}
        </p>
      ) : (
        <>
          <div className="grid gap-5 lg:grid-cols-2">
            <ChartPanel
              title="Portfolio by lifecycle"
              subtitle={`${visible.length} projects in the current filter`}
            >
              <CompositionChart
                label="Project lifecycle distribution"
                data={["draft", "active", "on_hold", "closed"].map((label) => ({
                  label: label.replaceAll("_", " "),
                  value: visible.filter((p) => p.lifecycle_status === label)
                    .length,
                }))}
              />
            </ChartPanel>
            <ChartPanel
              title="Completion horizon"
              subtitle={`Open projects by planned completion · as of ${today}`}
            >
              <Bars data={completionBuckets(visible, today)} />
            </ChartPanel>
          </div>
          {!manager && (
            <ChartPanel
              title="Recorded contract value"
              subtitle="Currencies are shown separately; this is contract value, not revenue or cash collected."
            >
              <div className="flex flex-wrap gap-8">
                {Object.entries(metrics.currencies).map(
                  ([currency, amount]) => (
                    <div key={currency}>
                      <p className="text-xs font-bold text-slate-500">
                        {currency}
                      </p>
                      <p className="mt-1 text-2xl font-bold text-slate-900">
                        {amount.toLocaleString("en-IN", {
                          maximumFractionDigits: 2,
                        })}
                      </p>
                    </div>
                  ),
                )}
                {!Object.keys(metrics.currencies).length && (
                  <p className="text-sm text-slate-500">
                    No contract values recorded for this selection.
                  </p>
                )}
              </div>
            </ChartPanel>
          )}
        </>
      )}
      <RecordedProgress
        projects={projects}
        visible={visible}
        revision={refresh}
      />
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="font-bold text-slate-900">
            {manager ? "Delivery priorities" : "Project portfolio"}
          </h3>
          <span className="text-xs text-slate-500">
            Select a project for its dashboard
          </span>
        </div>
        <DataGrid<DashboardProject>
          label="Dashboard projects"
          rows={visible}
          loading={loading}
          searchable={false}
          initialSort="finish"
          columns={[
            {
              key: "name",
              label: "Project",
              value: (r) => r.name,
              render: (r) => (
                <div>
                  <p className="font-semibold">{r.name}</p>
                  <p className="mt-1 text-xs text-slate-500">
                    {r.project_number}
                  </p>
                </div>
              ),
            },
            {
              key: "manager",
              label: "Manager",
              value: (r) => r.operations_manager_name || "Unassigned",
            },
            {
              key: "status",
              label: "Lifecycle",
              value: (r) => r.lifecycle_status,
              render: (r) => (
                <GridStatus value={r.lifecycle_status.replaceAll("_", " ")} />
              ),
            },
            {
              key: "finish",
              label: "Target completion",
              value: (r) => r.planned_completion_date || "Not set",
            },
            {
              key: "purchase_holds",
              label: "Purchase holds",
              value: (p) =>
                purchaseHolds[p.id] == null
                  ? "Unavailable"
                  : purchaseHolds[p.id]
                    ? `${purchaseHolds[p.id]} active`
                    : "Clear",
            },
            {
              key: "attention",
              label: "Attention",
              value: (r) =>
                r.lifecycle_status === "closed"
                  ? "Closed"
                  : !r.planned_completion_date
                    ? "Set target date"
                    : r.planned_completion_date < today
                      ? "Review overdue target"
                      : "Within target date",
            },
          ]}
          actions={(r) => (
            <GridAction
              label={`Open dashboard for ${r.name}`}
              onClick={() => openProject(r.id)}
            />
          )}
        />
      </div>
    </section>
  );
}
