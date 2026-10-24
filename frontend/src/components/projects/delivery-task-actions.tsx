"use client";
import { ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useState } from "react";
import {
  DeliveryBoard,
  DeliveryNode,
  TaskIssue,
  hours,
  deliveryField as field,
} from "./delivery-plan-data";
import {
  processRequest,
  processButton,
  processSecondary,
} from "./project-process-data";
import { useProjectUser } from "@/components/projects/project-permissions";

export function DeliveryTaskActions({
  projectId,
  board,
  node,
  issues,
  onSaved,
}: {
  projectId: string;
  board: DeliveryBoard;
  node: DeliveryNode;
  issues: TaskIssue[];
  onSaved: () => void;
}) {
  const user = useProjectUser();
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState("");
  const [evidence, setEvidence] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [doer, setDoer] = useState(node.doer_id ?? "");
  const [resolver, setResolver] = useState(node.resolver_id ?? "");
  const [allowance, setAllowance] = useState(
    node.resolution_minutes === null
      ? ""
      : String(node.resolution_minutes / 60),
  );
  const own = board.can_manage || node.effective_doer_id === user.id;
  const canEscalate =
    own &&
    (user.permissions.includes("system:admin") ||
      user.permissions.includes("projects.escalations.write"));
  const open = issues.some((i) => i.node_id === node.id && !i.resolved_at);
  const member = (id: string | null) =>
    board.members.find((m) => m.id === id)?.label ?? "Unassigned / unavailable";
  async function action(path: string, payload: object, method = "POST") {
    setError("");
    if (!reason.trim()) {
      setReasonError("Enter a reason or describe the blocker.");
      return;
    }
    setSaving(true);
    try {
      await processRequest(`/api/projects/${projectId}/${path}`, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      setReason("");
      setEvidence("");
      onSaved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  }
  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <div>
          <p className="text-xs font-bold text-slate-500">DOER</p>
          <p className="mt-1 text-sm font-bold">
            {member(node.effective_doer_id)}
          </p>
          <p className="text-xs text-slate-500">
            {node.doer_id ? "Assigned here" : "Inherited from parent"}
          </p>
        </div>
        <div>
          <p className="text-xs font-bold text-slate-500">PROBLEM RESOLVER</p>
          <p className="mt-1 text-sm font-bold">
            {member(node.effective_resolver_id)}
          </p>
          <p className="text-xs text-slate-500">
            {node.resolver_id ? "Assigned here" : "Inherited from parent"}
          </p>
        </div>
        <div>
          <p className="text-xs font-bold text-slate-500">RESOLUTION WINDOW</p>
          <p className="mt-1 text-sm font-bold">
            {hours(node.effective_resolution_minutes ?? 0)} elapsed
          </p>
          <p className="text-xs text-slate-500">
            24/7 clock · snapshot at escalation
          </p>
        </div>
      </div>
      <p className="text-sm text-slate-600">
        Actual start:{" "}
        {node.actual_start
          ? new Date(node.actual_start).toLocaleString()
          : "Not recorded"}{" "}
        · Finish:{" "}
        {node.actual_finish
          ? new Date(node.actual_finish).toLocaleString()
          : "Not recorded"}
      </p>
      {node.evidence ? (
        <p className="break-words rounded-xl bg-emerald-50 p-3 text-sm text-emerald-900">
          Outcome reference: {node.evidence}
        </p>
      ) : null}
      {error ? (
        <p role="alert" className="rounded-xl bg-red-50 p-3 text-red-800">
          {error}
        </p>
      ) : null}
      {(own &&
        node.leaf &&
        board.plan?.state === "active" &&
        node.status !== "completed") ||
      board.can_manage ? (
        <ValidatedLabel className="block text-sm font-bold">
          Action reason / blocker description
          <ValidatedTextarea name="reason"
            required error={reasonError}
            value={reason}
            onChange={(e) => { setReason(e.target.value); setReasonError(""); }}
            maxLength={2000}
            rows={3}
            className={field}
          />
        </ValidatedLabel>
      ) : null}
      {own &&
      node.leaf &&
      board.plan?.state === "active" &&
      node.status !== "completed" ? (
        <div className="space-y-3">
          <ValidatedLabel className="block text-sm font-bold">
            Completion outcome / evidence reference
            <ValidatedInput name="evidence"
              value={evidence}
              onChange={(e) => setEvidence(e.target.value)}
              maxLength={2000}
              className={field}
            />
          </ValidatedLabel>
          <div className="flex flex-wrap gap-2">
            {board.can_execute ? (
              <>
                <button
                  disabled={saving || open}
                  className={processSecondary}
                  onClick={() =>
                    void action(`delivery-plan/nodes/${node.id}/execution`, {
                      expected_version: board.plan!.version,
                      status: "in_progress",
                      evidence: "",
                      reason,
                    })
                  }
                >
                  Start task
                </button>
                <button
                  disabled={saving || open || !evidence.trim()}
                  className={processButton}
                  onClick={() =>
                    void action(`delivery-plan/nodes/${node.id}/execution`, {
                      expected_version: board.plan!.version,
                      status: "completed",
                      evidence,
                      reason,
                    })
                  }
                >
                  Complete with evidence
                </button>
              </>
            ) : null}
            {canEscalate ? (
              <button
                disabled={saving || open}
                className={processSecondary}
                onClick={() =>
                  void action("task-escalations", { node_id: node.id, reason })
                }
              >
                Escalate to problem resolver
              </button>
            ) : null}
          </div>
          {open ? (
            <p className="text-sm font-semibold text-red-700">
              Resolve the open escalation before execution can continue.
            </p>
          ) : null}
          {node.unmet_predecessors.length ? (
            <p className="text-sm text-amber-800">
              Prerequisites still required: {node.unmet_predecessors.join(", ")}
              . Starting or completing is blocked until their outcomes are
              recorded.
            </p>
          ) : null}
        </div>
      ) : null}
      {board.can_manage ? (
        <details className="rounded-xl border border-slate-200 p-4">
          <summary className="min-h-9 cursor-pointer font-bold">
            Change assignments / inheritance
          </summary>
          <div className="mt-3 grid gap-3 sm:grid-cols-3">
            <ValidatedLabel className="text-sm font-bold">
              Doer
              <ValidatedSelect name="doer"
                value={doer}
                onChange={(e) => setDoer(e.target.value)}
                className={field}
              >
                <option value="">
                  {node.parent_code ? "Inherit" : "Unassigned"}
                </option>
                {board.members.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.label}
                  </option>
                ))}
              </ValidatedSelect>
            </ValidatedLabel>
            <ValidatedLabel className="text-sm font-bold">
              Problem resolver
              <ValidatedSelect name="resolver"
                value={resolver}
                onChange={(e) => setResolver(e.target.value)}
                className={field}
              >
                <option value="">
                  {node.parent_code ? "Inherit" : "Unassigned"}
                </option>
                {board.members.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.label}
                  </option>
                ))}
              </ValidatedSelect>
            </ValidatedLabel>
            <ValidatedLabel className="text-sm font-bold">
              Allowance (hours)
              <ValidatedInput name="allowance"
                type="number"
                min="0.0167"
                max="720"
                step="any"
                value={allowance}
                placeholder="Inherit"
                onChange={(e) => setAllowance(e.target.value)}
                className={field}
              />
            </ValidatedLabel>
          </div>
          <button
            disabled={saving}
            className={processSecondary + " mt-3"}
            onClick={() =>
              void action(
                `delivery-plan/nodes/${node.id}/assignments`,
                {
                  expected_version: board.plan!.version,
                  doer_id: doer || null,
                  resolver_id: resolver || null,
                  resolution_minutes: allowance
                    ? Math.round(Number(allowance) * 60)
                    : null,
                  reason,
                },
                "PATCH",
              )
            }
          >
            Save assignments
          </button>
          <p className="mt-2 text-xs text-slate-500">
            Child overrides remain. Open escalations retain the resolver and
            deadline captured when raised.
          </p>
        </details>
      ) : null}
      {!node.leaf ? (
        <p className="text-sm text-slate-500">
          This is a summary. Its completion is calculated from weighted subtask
          outcomes.
        </p>
      ) : null}
    </div>
  );
}

export function EscalationQueue({
  projectId,
  board,
  issues,
  onSaved,
  onSelect,
}: {
  projectId: string;
  board: DeliveryBoard;
  issues: TaskIssue[];
  onSaved: () => void;
  onSelect: (code: string) => void;
}) {
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [answerErrors, setAnswerErrors] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [saving, setSaving] = useState("");
  async function resolve(issue: TaskIssue) {
    if (!answers[issue.id]?.trim()) {
      setAnswerErrors(current => ({...current, [issue.id]: "Enter the resolution outcome first."}));
      return;
    }
    setSaving(issue.id);
    setError("");
    try {
      await processRequest(
        `/api/projects/${projectId}/task-escalations/${issue.id}/resolve`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            expected_version: issue.version,
            resolution: answers[issue.id],
          }),
        },
      );
      onSaved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving("");
    }
  }
  const sorted = [...issues].sort(
    (a, b) =>
      Number(Boolean(a.resolved_at)) - Number(Boolean(b.resolved_at)) ||
      b.overdue_minutes - a.overdue_minutes,
  );
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-5">
      <h4 className="text-lg font-bold">Problem resolver queue</h4>
      <p className="mt-1 text-xs text-slate-500">
        Total blocked time and time beyond the resolver deadline are tracked
        separately. Clock: elapsed UTC time, 24/7. Refreshed every minute while
        this view is open.
      </p>
      {error ? (
        <p role="alert" className="mt-3 text-red-700">
          {error}
        </p>
      ) : null}
      {!issues.length ? (
        <p className="mt-4 rounded-xl bg-slate-50 p-4 text-sm text-slate-600">
          No task escalations recorded.
        </p>
      ) : null}
      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        {sorted.map((issue) => {
          const node = board.nodes.find((n) => n.id === issue.node_id);
          return (
            <article
              key={issue.id}
              className={`rounded-xl border p-4 ${issue.resolved_at ? "border-slate-200" : issue.overdue_minutes > 0 ? "border-red-300 bg-red-50" : "border-amber-200 bg-amber-50"}`}
            >
              <button
                onClick={() => node && onSelect(node.code)}
                className="min-h-11 text-left text-sm font-bold text-blue-800 underline"
              >
                {node?.code} · {node?.name ?? "Task"}
              </button>
              <p className="mt-1 text-sm">{issue.reason}</p>
              <p className="mt-2 text-xs text-slate-600">
                Resolver:{" "}
                {board.members.find((m) => m.id === issue.resolver_id)?.label ??
                  "Former project member"}{" "}
                · Due {new Date(issue.due_at).toLocaleString()}
              </p>
              <p className="mt-2 text-sm font-bold">
                {issue.resolved_at ? "Resolved" : "Open"} · Blocked{" "}
                {hours(issue.blocked_minutes)} · Beyond deadline{" "}
                {hours(issue.overdue_minutes)}
              </p>
              {!issue.resolved_at && issue.overdue_minutes > 0 ? (
                <p className="mt-1 text-xs font-bold text-red-800">
                  Project manager attention: resolution deadline missed.
                </p>
              ) : null}
              {issue.resolved_at ? (
                <p className="mt-2 text-sm">Resolution: {issue.resolution}</p>
              ) : null}
              {issue.can_resolve ? (
                <div className="mt-3">
                  <ValidatedLabel className="text-sm font-bold">
                    Resolution outcome
                    <ValidatedTextarea
                      required error={answerErrors[issue.id]}
                      value={answers[issue.id] ?? ""}
                      onChange={(e) =>
                        { setAnswers({ ...answers, [issue.id]: e.target.value }); setAnswerErrors(current => ({...current, [issue.id]: ""})); }
                      }
                      maxLength={2000}
                      className={field}
                    />
                  </ValidatedLabel>
                  <button
                    disabled={saving === issue.id}
                    onClick={() => void resolve(issue)}
                    className={processButton + " mt-2"}
                  >
                    Resolve escalation
                  </button>
                </div>
              ) : null}
            </article>
          );
        })}
      </div>
    </section>
  );
}
