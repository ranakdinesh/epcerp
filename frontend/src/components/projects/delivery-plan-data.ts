export type TemplateNode = {
  code: string;
  parent_code: string | null;
  name: string;
  process_code: string;
  weight_bp: number;
  start_bp: number;
  finish_bp: number;
  resolution_minutes: number | null;
  predecessors: string[];
};
export type TaskTemplate = {
  id: string;
  family_id: string;
  revision: number;
  name: string;
  description: string;
  is_active: boolean;
  nodes: TemplateNode[];
};
export type DeliveryNode = TemplateNode & {
  id: string;
  start_date: string | null;
  finish_date: string | null;
  doer_id: string | null;
  resolver_id: string | null;
  effective_doer_id: string | null;
  effective_resolver_id: string | null;
  effective_resolution_minutes: number | null;
  status: string;
  evidence: string;
  progress: number;
  leaf: boolean;
  unmet_predecessors: string[];
  actual_start: string | null;
  actual_finish: string | null;
};
export type DeliveryPlan = {
  id: string;
  template_id: string;
  template_name: string;
  template_revision: number;
  manager_id: string | null;
  start_date: string | null;
  finish_date: string | null;
  state: string;
  version: number;
};
export type DeliveryBoard = {
  plan: DeliveryPlan | null;
  nodes: DeliveryNode[];
  members: { id: string; label: string }[];
  progress: number;
  can_manage: boolean;
  can_execute: boolean;
  can_generate: boolean;
  as_of: string;
};
export type TaskIssue = {
  id: string;
  node_id: string;
  reporter_id: string;
  resolver_id: string;
  reason: string;
  opened_at: string;
  due_at: string;
  resolved_at: string | null;
  resolution: string;
  version: number;
  resolution_minutes: number;
  blocked_minutes: number;
  overdue_minutes: number;
  remaining_minutes: number;
  can_resolve: boolean;
};
const DAY = 86400000;
export function planDates(
  nodes: TemplateNode[],
  start: string,
  finish: string,
): Map<string, [string, string]> {
  const result = new Map<string, [string, string]>();
  const byCode = new Map(nodes.map((n) => [n.code, n]));
  const parse = (s: string) => Date.parse(s + "T00:00:00Z");
  const iso = (n: number) => new Date(n).toISOString().slice(0, 10);
  if (
    !Number.isFinite(parse(start)) ||
    !Number.isFinite(parse(finish)) ||
    finish < start
  )
    throw Error("Enter valid project start and finish dates first.");
  function compute(code: string): [string, string] {
    const stored = result.get(code);
    if (stored) return stored;
    const n = byCode.get(code)!;
    const [a, b] = n.parent_code ? compute(n.parent_code) : [start, finish];
    const span = (parse(b) - parse(a)) / DAY;
    const dates: [string, string] = [
      iso(parse(a) + Math.floor((span * n.start_bp) / 10000) * DAY),
      iso(parse(a) + Math.floor((span * n.finish_bp) / 10000) * DAY),
    ];
    result.set(code, dates);
    return dates;
  }
  for (const n of nodes) compute(n.code);
  return result;
}
export function outline<T extends { code: string; parent_code: string | null }>(
  nodes: T[],
): T[] {
  const rows: T[] = [];
  function add(parent: string | null) {
    for (const n of nodes.filter((n) => n.parent_code === parent)) {
      rows.push(n);
      add(n.code);
    }
  }
  add(null);
  return rows;
}
export function depth(
  code: string,
  nodes: { code: string; parent_code: string | null }[],
) {
  let value = 0;
  let n = nodes.find((n) => n.code === code);
  while (n?.parent_code) {
    value++;
    n = nodes.find((p) => p.code === n!.parent_code);
  }
  return value;
}
export function siblingTotal(
  node: { parent_code: string | null },
  nodes: { parent_code: string | null; weight_bp: number }[],
) {
  return (
    nodes
      .filter((n) => n.parent_code === node.parent_code)
      .reduce((sum, n) => sum + n.weight_bp, 0) / 100
  );
}
export function hours(minutes: number) {
  return `${(minutes / 60).toFixed(1)} h`;
}
export const deliveryField =
  "mt-1 min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-normal";
export function deliveryEdges(nodes: DeliveryNode[]) {
  const children = new Map<string, string[]>();
  for (const n of nodes)
    if (n.parent_code)
      children.set(n.parent_code, [
        ...(children.get(n.parent_code) ?? []),
        n.code,
      ]);
  const leafCache = new Map<string, string[]>();
  function leaves(code: string): string[] {
    if (!leafCache.has(code))
      leafCache.set(
        code,
        children.has(code) ? children.get(code)!.flatMap(leaves) : [code],
      );
    return leafCache.get(code)!;
  }
  const edges = new Map<
    string,
    { from: string; to: string; relationship: string }
  >();
  for (const n of nodes)
    for (const p of n.predecessors)
      for (const from of leaves(p))
        for (const to of leaves(n.code))
          edges.set(`${from}:${to}`, {
            from,
            to,
            relationship: "finish to start",
          });
  return { edges: [...edges.values()], leaves };
}

export function blockedMinutes(
  issues: TaskIssue[],
  nodeIds: Set<string>,
  asOf: string,
): number {
  const intervals = issues
    .filter((i) => nodeIds.has(i.node_id))
    .map((i) => [Date.parse(i.opened_at), Date.parse(i.resolved_at ?? asOf)])
    .filter(([a, b]) => Number.isFinite(a) && Number.isFinite(b) && b >= a)
    .sort((a, b) => a[0] - b[0]);
  let total = 0,
    start = 0,
    end = 0;
  for (const [a, b] of intervals) {
    if (!end) {
      start = a;
      end = b;
    } else if (a <= end) {
      end = Math.max(end, b);
    } else {
      total += end - start;
      start = a;
      end = b;
    }
  }
  return (total + (end - start)) / 60000;
}
