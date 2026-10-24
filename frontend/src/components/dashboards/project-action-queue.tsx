"use client";
import { ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { useState } from "react";
import Link from "next/link";
import type {
  DeliveryBoard,
  TaskIssue,
} from "@/components/projects/delivery-plan-data";
import { DataGrid } from "@/components/ui/data-grid";
import { projectActions, type DashboardAction } from "./action-model";
export function ProjectActionQueue({
  board,
  issues,
  userId,
  projectId,
  loading,
}: {
  board: DeliveryBoard | null;
  issues: TaskIssue[] | null;
  userId: string;
  projectId: string;
  loading: boolean;
}) {
  const [mine, setMine] = useState(false);
  const actions = projectActions(
    board,
    issues,
    userId,
    new Date().toISOString().slice(0, 10),
  );
  const control = `/dashboard?screen=projects&project=${projectId}&tab=control`;
  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="text-xs font-bold uppercase tracking-widest text-[#b73d13]">
            Act next
          </p>
          <h3 className="mt-1 text-lg font-bold">
            Ownership & resolution queue
          </h3>
          <p className="mt-1 text-xs text-slate-500">
            Recorded escalations, past task targets and missing assignments.
            These are signals, not forecast project delay.
          </p>
        </div>
        <ValidatedLabel className="flex min-h-11 items-center gap-2 text-sm">
          <ValidatedInput
            type="checkbox"
            checked={mine}
            onChange={(e) => setMine(e.target.checked)}
          />
          My responsibilities only
        </ValidatedLabel>
      </div>
      {!loading && !board?.plan ? (
        <div className="rounded-xl border border-slate-200 bg-white p-4 text-sm">
          <p>No readable delivery plan is available for the action queue.</p>
          <Link
            className="mt-2 inline-block py-2 font-semibold text-[#b73d13] underline"
            href={control}
          >
            Open project planning & assignments
          </Link>
        </div>
      ) : (
        <>
          <p className="text-xs text-slate-500">
            {issues === null
              ? "Escalations unavailable; task-date and assignment checks only."
              : "Includes accessible task escalation records."}
          </p>
          <DataGrid<DashboardAction>
            label="Project action queue"
            rows={mine ? actions.filter((a) => a.mine) : actions}
            loading={loading}
            columns={[
              {
                key: "priority",
                label: "Attention",
                value: (r) => r.priority,
                render: (r) => (
                  <span
                    className={`rounded-md px-2 py-1 text-xs font-semibold ${r.priority.includes("overdue") ? "bg-red-100 text-red-800" : "bg-orange-50 text-orange-900"}`}
                  >
                    {r.priority}
                  </span>
                ),
              },
              {
                key: "task",
                label: "Task / issue",
                value: (r) => `${r.task} ${r.reason}`,
                render: (r) => (
                  <div className="max-w-sm">
                    <p className="font-semibold">{r.task}</p>
                    <p className="mt-1 text-xs text-slate-500">{r.reason}</p>
                  </div>
                ),
              },
              { key: "doer", label: "Doer", value: (r) => r.doer },
              {
                key: "resolver",
                label: "Problem resolver",
                value: (r) => r.resolver,
              },
              {
                key: "due",
                label: "Due",
                value: (r) =>
                  r.due
                    ? new Date(
                        r.due.length === 10 ? r.due + "T00:00:00" : r.due,
                      ).toLocaleString(undefined, {
                        dateStyle: "medium",
                        ...(r.due.length > 10
                          ? { timeStyle: "short" as const }
                          : {}),
                      })
                    : "Not set",
              },
            ]}
            actions={(r) => (
              <Link
                className="inline-flex min-h-11 items-center px-3 text-sm font-bold text-[#b73d13] underline"
                href={`${control}&task=${encodeURIComponent(r.code)}`}
              >
                Review
              </Link>
            )}
          />
        </>
      )}
    </section>
  );
}
