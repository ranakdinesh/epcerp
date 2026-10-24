export { request, input, button, secondary, panel } from "../items/item-data";
export type RecordRow<T> = {
  id: string;
  data: T;
  version: number;
  is_active: boolean;
  created_at: string;
};
export type RuleData = {
  project_id: string | null;
  scope_code: string;
  mode: string;
  allowed: string[];
  excluded: string[];
  effective_from: string;
  effective_until: string | null;
  evidence: string;
};
export type HoldData = {
  scope: string;
  category_code: string;
  demand_ids: string[];
  reason: string;
  evidence: string;
  started_at: string;
  expected_release_at: string | null;
  released_at: string | null;
  doer_id: string;
  resolver_id: string;
};
export type Line = {
  demand_id: string;
  quantity: string;
  budget: string;
  full_kit: string;
};
export type IndentData = {
  number: string;
  state: string;
  currency: string;
  doer_id: string;
  resolver_id: string;
  documents: string;
  lines: Line[];
  released: (Line & { at: string })[];
};
export type DemandSource = {
  id: string;
  active: boolean;
  quantity: string;
  required_date: string;
  released_quantity?: string;
  remaining_quantity?: string;
  item: { description: string; sku: string; uom: string; manufacturer: string };
  policy: { status: string; reasons: string[] };
  holds: { holds: { id: string; reason: string }[]; hold_minutes: number };
};
export type Overview = {
  client_id: string;
  categories: { code: string; name: string }[];
  lines: DemandSource[];
  members: { id: string; label: string }[];
  permissions: Record<string, boolean>;
};
