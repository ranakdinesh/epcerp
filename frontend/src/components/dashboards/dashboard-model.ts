export type DashboardProject = {
  id: string;
  is_active: boolean;
  project_number: string;
  name: string;
  lifecycle_status: string;
  planned_start_date: string | null;
  actual_start_date?: string | null;
  planned_completion_date: string | null;
  contract_value: string | null;
  currency: string;
  operations_manager_name: string;
  loi_number: string;
  po_number: string;
};
export function portfolioMetrics(projects: DashboardProject[], today: string) {
  const open = projects.filter((p) => p.lifecycle_status !== "closed");
  const overdue = open.filter(
    (p) => p.planned_completion_date && p.planned_completion_date < today,
  );
  const currencies: Record<string, number> = {};
  for (const p of projects)
    if (
      p.contract_value !== null &&
      p.contract_value !== "" &&
      Number.isFinite(Number(p.contract_value))
    )
      currencies[p.currency] =
        (currencies[p.currency] ?? 0) + Number(p.contract_value);
  return {
    overdue,
    undated: open.filter((p) => !p.planned_completion_date).length,
    active: projects.filter((p) => p.lifecycle_status === "active").length,
    missingReference: projects.filter((p) => !p.loi_number && !p.po_number)
      .length,
    currencies,
  };
}
export function completionBuckets(projects: DashboardProject[], today: string) {
  const origin = Date.parse(today + "T00:00:00Z");
  const labels = [
    "Past target",
    "Next 30 days",
    "31–90 days",
    "Later",
    "No target",
  ];
  const counts = [0, 0, 0, 0, 0];
  for (const p of projects.filter((p) => p.lifecycle_status !== "closed")) {
    const days = p.planned_completion_date
      ? (Date.parse(p.planned_completion_date + "T00:00:00Z") - origin) /
        86400000
      : null;
    counts[
      days === null ? 4 : days < 0 ? 0 : days <= 30 ? 1 : days <= 90 ? 2 : 3
    ]++;
  }
  return labels.map((label, i) => ({ label, value: counts[i] }));
}

export function matchesProjectStatus(project: DashboardProject, status: string) {
  const archived = !project.is_active || project.lifecycle_status === "closed";
  return status === "all" || (status === "archived" ? archived : !archived);
}

export function assignmentHref(projectId: string, warning: {kind: string; key: string}) {
  const query = new URLSearchParams({screen: "projects", project: projectId, tab: warning.kind});
  if (warning.kind === "processes") query.set("process", warning.key);
  if (warning.kind === "control") query.set("task", warning.key);
  if (warning.kind === "purchasing") {
    const [purchase, step] = warning.key.split(":");
    query.set("purchaseView", "workflow");
    query.set("purchase", purchase);
    query.set("purchaseStep", step);
  }
  return `/dashboard?${query}`;
}
