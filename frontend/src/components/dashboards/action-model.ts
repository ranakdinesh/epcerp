import type {
  DeliveryBoard,
  TaskIssue,
} from "@/components/projects/delivery-plan-data";
export type DashboardAction = {
  id: string;
  task: string;
  reason: string;
  doer: string;
  resolver: string;
  due: string;
  priority: string;
  code: string;
  mine: boolean;
};
export function projectActions(
  board: DeliveryBoard | null,
  issues: TaskIssue[] | null,
  userId: string,
  today: string,
): DashboardAction[] {
  if (!board?.plan) return [];
  const name = (id: string | null) =>
    board.members.find((m) => m.id === id)?.label ?? "Unassigned";
  const open = issues?.filter((i) => !i.resolved_at) ?? [];
  const actions: DashboardAction[] = open.map((i) => {
    const n = board.nodes.find((n) => n.id === i.node_id);
    return {
      id: i.id,
      task: n?.name ?? "Task escalation",
      reason: i.reason,
      doer: name(n?.effective_doer_id ?? null),
      resolver: name(i.resolver_id),
      due: i.due_at,
      priority:
        i.overdue_minutes > 0 ? "Resolution overdue" : "Open escalation",
      code: n?.code ?? "",
      mine: n?.effective_doer_id === userId || i.resolver_id === userId,
    };
  });
  for (const n of board.nodes.filter((n) => n.leaf && n.progress < 100)) {
    if (open.some((i) => i.node_id === n.id)) continue;
    const overdue = !!n.finish_date && n.finish_date < today;
    const unassigned = !n.effective_doer_id || !n.effective_resolver_id;
    if (!overdue && !unassigned) continue;
    actions.push({
      id: n.id,
      task: n.name,
      reason: overdue
        ? "Planned finish has passed; review progress and recovery action."
        : "Assign an execution owner and problem resolver.",
      doer: name(n.effective_doer_id),
      resolver: name(n.effective_resolver_id),
      due: n.finish_date ?? "",
      priority: overdue ? "Task past target" : "Assignment needed",
      code: n.code,
      mine:
        n.effective_doer_id === userId || n.effective_resolver_id === userId,
    });
  }
  const rank = [
    "Resolution overdue",
    "Open escalation",
    "Task past target",
    "Assignment needed",
  ];
  return actions.sort(
    (a, b) =>
      rank.indexOf(a.priority) - rank.indexOf(b.priority) ||
      a.due.localeCompare(b.due),
  );
}
