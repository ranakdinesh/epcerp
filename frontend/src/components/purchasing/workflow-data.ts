export type WorkflowStep = {
  number: number;
  title: string;
  phase: string;
  evidence_required: string;
  team: string;
  doer_id: string | null;
  resolver_id: string | null;
  planned_date: string | null;
  state: string;
  notes: string;
  block_reason: string;
  blocked_at: string | null;
  blocked_resolver_id: string | null;
  resolution_due: string | null;
  updated_at: string | null;
  updated_by_id: string | null;
};
export type PurchasePackage = {
  id: string;
  project_id: string;
  indent_id: string;
  demand_id: string;
  code: string;
  description: string;
  version: number;
  workflow_version: number;
  process_code: string;
  steps: WorkflowStep[];
};
export type WorkflowList = {
  members: { id: string; label: string; capabilities: string[] }[];
  server_time: string;
  packages: PurchasePackage[];
  candidates: {
    indent_id: string;
    indent_number: string;
    demand_id: string;
    description: string;
    quantity: string;
  }[];
  can_create: boolean;
  actor_id: string;
};
export type Workspace = {
  package: PurchasePackage;
  source: {
    project_name: string;
    site: string;
    indent_number: string;
    description: string;
    quantity: string;
    uom: string;
    required_date: string;
    full_kit: string;
    release_at: string;
    project_available: boolean;
    eligible: boolean;
    blockers: string[];
  };
  members: { id: string; label: string; capabilities: string[] }[];
  actions: Record<string, string[]>;
  can_assign: boolean;
  history: {
    id: string;
    action: string;
    reason: string;
    created_at: string;
    created_by_id: string | null;
  }[];
  server_time: string;
};
export function workflowStatus(pkg: PurchasePackage) {
  if (pkg.steps.some((s) => s.state === "blocked")) return "Blocked";
  if (
    pkg.steps.some(
      (s) => s.number > 2 && (!s.doer_id || !s.resolver_id || !s.planned_date),
    )
  )
    return "Needs assignments";
  return "Preparation";
}
