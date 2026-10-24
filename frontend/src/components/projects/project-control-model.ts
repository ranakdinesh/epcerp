import type { ProcessBoard } from "./project-process-data";

export type ControlTask = {
  id: string;
  process_id: string | null;
  work_package_id: string;
  code: string;
  name: string;
  owner_user_id: string | null;
  planned_start_date: string | null;
  planned_finish_date: string | null;
  actual_start_date: string | null;
  actual_finish_date: string | null;
  status: string;
  is_milestone: boolean;
  is_active: boolean;
  version: number;
};
export type TaskEdge = {
  predecessor_activity_id: string;
  successor_activity_id: string;
  relationship_type: string;
};
export type ControlRow = {
  id: string;
  parent: string | null;
  kind: "process" | "task";
  name: string;
  owner: string;
  start: string | null;
  finish: string | null;
  status: string;
  milestone: boolean;
  actualStart: string | null;
  actualFinish: string | null;
  confirmed: boolean;
};
export type ControlEdge = { from: string; to: string; relationship: string };
export const DAY = 86400000;
export function day(value: string) {
  return Date.parse(`${value.slice(0, 10)}T00:00:00Z`) / DAY;
}
export function iso(value: number) {
  return new Date(value * DAY).toISOString().slice(0, 10);
}
export function downstream(id: string, edges: ControlEdge[]): Set<string> {
  const graph = new Map<string, string[]>();
  for (const edge of edges)
    graph.set(edge.from, [...(graph.get(edge.from) ?? []), edge.to]);
  const found = new Set<string>();
  const pending = [...(graph.get(id) ?? [])];
  while (pending.length) {
    const next = pending.pop()!;
    if (next === id || found.has(next)) continue;
    found.add(next);
    pending.push(...(graph.get(next) ?? []));
  }
  return found;
}
export function overdue(row: ControlRow, today: string) {
  return !["completed", "bypassed"].includes(row.status) && row.finish
    ? Math.max(0, day(today) - day(row.finish))
    : 0;
}
export function controlModel(
  board: ProcessBoard,
  tasks: ControlTask[],
  edges: TaskEdge[],
  owners: { id: string; label: string }[],
) {
  const names = new Map(
    [...board.owners, ...owners].map((o) => [o.id, o.label]),
  );
  const rows: ControlRow[] = board.processes.map((p) => ({
    id: p.id,
    parent: null,
    kind: "process",
    name: `${String(p.position).padStart(2, "0")} · ${p.name}`,
    owner: names.get(p.owner_user_id ?? "") ?? "Unassigned",
    start: p.planned_start_date,
    finish: p.planned_finish_date,
    status: p.status,
    milestone: false,
    actualStart: p.started_at?.slice(0, 10) ?? null,
    actualFinish: p.completed_at?.slice(0, 10) ?? null,
    confirmed: p.dependencies_confirmed,
  }));
  rows.push(
    ...tasks
      .filter((t) => t.is_active)
      .map((t) => ({
        id: t.id,
        parent: t.process_id,
        kind: "task" as const,
        name: `${t.code} · ${t.name}`,
        owner: names.get(t.owner_user_id ?? "") ?? "Unassigned",
        start: t.planned_start_date,
        finish: t.planned_finish_date,
        status: t.status,
        milestone: t.is_milestone,
        actualStart: t.actual_start_date,
        actualFinish: t.actual_finish_date,
        confirmed: true,
      })),
  );
  const ids = new Set(rows.map((r) => r.id));
  const links: ControlEdge[] = [
    ...board.dependencies.map((e) => ({
      from: e.predecessor_id,
      to: e.successor_id,
      relationship: "process completion gate",
    })),
    ...edges.map((e) => ({
      from: e.predecessor_activity_id,
      to: e.successor_activity_id,
      relationship: e.relationship_type.replaceAll("_", " "),
    })),
  ].filter((e) => ids.has(e.from) && ids.has(e.to));
  return { rows, links };
}

export function pendingDependencies(rows: ControlRow[], edges: ControlEdge[]) {
  const incomplete = new Set(
    rows
      .filter((r) => !["completed", "bypassed"].includes(r.status))
      .map((r) => r.id),
  );
  return edges.filter((e) => incomplete.has(e.from) && incomplete.has(e.to));
}
