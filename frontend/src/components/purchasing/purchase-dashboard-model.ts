import type {
  PurchasePackage,
  WorkflowList,
  WorkflowStep,
} from "./workflow-data";
export type PurchaseProject = {
  id: string;
  name: string;
  project_number: string;
  is_active: boolean;
  purchase_enabled: boolean;
  lifecycle_status: string;
};
import type { ProgressReport } from "./purchase-progress";
export type PurchaseSource = {
  project: PurchaseProject;
  data: WorkflowList;
  progress?: ProgressReport | null;
  progressError?: string;
};
export function matchesPurchaseProject(
  source: PurchaseSource,
  lifecycle: string,
  completion: string,
) {
  const state = source.project.lifecycle_status;
  if (
    lifecycle &&
    (lifecycle === "active"
      ? !["draft", "active"].includes(state)
      : state !== lifecycle)
  )
    return false;
  const percent = source.progress?.progress_percent;
  if (completion === "complete") return percent != null && percent >= 100;
  if (completion === "in_progress")
    return percent != null && percent > 0 && percent < 100;
  if (completion === "not_started") return percent === 0;
  if (completion === "unavailable") return percent == null;
  return true;
}
export function purchaseProjectSignal(source: PurchaseSource) {
  if (!source.progress) return "unavailable";
  const signals = source.progress.packages.map((bo) => bo.signal);
  if (!signals.length) return "unrecorded";
  if (
    source.progress.progress_percent != null &&
    source.progress.progress_percent >= 100
  )
    return "complete";
  return (
    [
      "blocked",
      "delayed",
      "attention",
      "due_soon",
      "unplanned",
      "on_track",
    ].find((signal) => signals.includes(signal as (typeof signals)[number])) ??
    "on_track"
  );
}
export type PurchaseTask = {
  id: string;
  project: PurchaseProject;
  package: PurchasePackage;
  step: WorkflowStep;
  doer: string;
  resolver: string;
  mine: boolean;
  overdue: boolean;
};
export const purchasePhases = [
  "WO2I",
  "I2LOI",
  "LOI2PO",
  "PO2INS",
  "INS2D",
] as const;
export function pendingPurchaseSteps(pkg: PurchasePackage) {
  return pkg.steps.filter(
    (step) =>
      !["source_linked", "completed", "not_applicable", "skipped"].includes(
        step.state,
      ),
  );
}
export function purchaseTasks(
  sources: PurchaseSource[],
  actor: string,
): PurchaseTask[] {
  return sources.flatMap(({ project, data }) =>
    data.packages.flatMap((pkg) =>
      pendingPurchaseSteps(pkg).map((step) => {
        const owner = (id: string | null) =>
          !id
            ? "Unassigned"
            : (data.members.find((member) => member.id === id)?.label ??
              "Inactive or unavailable member");
        return {
          id: `${pkg.id}:${step.number}`,
          project,
          package: pkg,
          step,
          doer: owner(step.doer_id),
          resolver: owner(step.blocked_resolver_id ?? step.resolver_id),
          mine:
            step.doer_id === actor ||
            (step.state === "blocked" && step.blocked_resolver_id === actor),
          overdue:
            !!step.planned_date &&
            step.planned_date < data.server_time.slice(0, 10),
        };
      }),
    ),
  );
}
export function purchaseTaskHref(project: string, pkg?: string, step?: number) {
  const query = new URLSearchParams({
    screen: "projects",
    project,
    tab: "purchasing",
    purchaseView: pkg ? "workflow" : "bo",
  });
  if (pkg) query.set("purchase", pkg);
  if (step) query.set("purchaseStep", String(step));
  return `/dashboard?${query}`;
}

/** Open saved unfinished work, preserving the BO that produced the status. */
export function purchaseProjectAction(
  source: PurchaseSource,
  canOpenStep: (number: number) => boolean = () => true,
) {
  const fallback = { label: "Open purchase workspace", href: purchaseTaskHref(source.project.id) };
  if (!source.progress) return fallback;
  const bos = source.progress.packages;
  if (!bos.length) {
    return canOpenStep(1)
      ? { label: "Step 1 · Prepare BO item list", href: `${fallback.href}&purchaseStep=1` }
      : fallback;
  }
  const candidates = bos.flatMap(bo => bo.steps
    .filter(step => !["completed", "not_applicable"].includes(step.state) && canOpenStep(step.number))
    .map(step => ({ bo, step })));
  candidates.sort((a, b) => a.step.number - b.step.number || a.bo.bo_id.localeCompare(b.bo.bo_id));
  const next = candidates[0];
  const destination = (bo: string, step: number) => {
    const query = new URLSearchParams({ screen: "projects", project: source.project.id,
      tab: "purchasing", purchaseView: "bo", bo, purchaseStep: String(step) });
    return `/dashboard?${query}`;
  };
  if (next) return { label: `Step ${next.step.number} · ${next.step.title}`, href: destination(next.bo.bo_id, next.step.number) };
  const complete = bos.every(bo => bo.steps.length > 0 && bo.steps.every(step => ["completed", "not_applicable"].includes(step.state)));
  if (complete) {
    const bo = [...bos].sort((a, b) => a.bo_id.localeCompare(b.bo_id))[0];
    const last = [...bo.steps].filter(step => canOpenStep(step.number)).sort((a, b) => b.number - a.number)[0];
    if (last) return { label: "Purchase complete · View records", href: destination(bo.bo_id, last.number) };
  }
  return fallback;
}
