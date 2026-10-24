"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";

import { FormEvent, useEffect, useRef, useState } from "react";
import {
  type ProcessBoard,
  type ProjectProcess,
  processButton,
  processSecondary,
  processRequest,
  processStatus,
} from "@/components/projects/project-process-data";

type History = {
  id: string;
  action: string;
  reason: string;
  created_at: string;
  created_by_id: string | null;
  before: Record<string, unknown>;
  after: Record<string, unknown>;
};

const historyLabels: Record<string, string> = {
  status: "Status",
  owner_user_id: "Accountable owner",
  planned_start_date: "Planned start",
  planned_finish_date: "Planned finish",
  notes: "Outcome notes",
  completion_reference: "Evidence reference",
  dependencies_confirmed: "Prerequisites reviewed",
  predecessor_ids: "Prerequisites",
  started_at: "Started",
  completed_at: "Completed",
  version: "Revision",
};
function historyValue(
  field: string,
  value: unknown,
  board: ProcessBoard,
): string {
  if (value === null || value === undefined || value === "")
    return "Not recorded";
  if (field === "predecessor_ids" && Array.isArray(value))
    return (
      value
        .map((id) => {
          const item = board.processes.find((process) => process.id === id);
          return item
            ? `${item.position}. ${item.name}`
            : "Unavailable process";
        })
        .join(", ") || "None"
    );
  if (field === "owner_user_id")
    return (
      board.owners.find((owner) => owner.id === value)?.label ??
      "Previous / unavailable member"
    );
  if (field === "status" && typeof value === "string")
    return processStatus(value);
  if (typeof value === "boolean") return value ? "Yes" : "No";
  return String(value);
}

export function ProjectProcessEditor({
  projectId,
  process,
  board,
  onSaved,
  onOpenWorkspace,
}: {
  projectId: string;
  process: ProjectProcess;
  board: ProcessBoard;
  onSaved: (record: ProjectProcess, predecessorIds: string[]) => void;
  onOpenWorkspace: (workspace: string) => void;
}) {
  const initialPredecessors = board.dependencies
    .filter((edge) => edge.successor_id === process.id)
    .map((edge) => edge.predecessor_id);
  const [predecessors, setPredecessors] = useState(initialPredecessors);
  const [status, setStatus] = useState(process.status);
  const [confirmed, setConfirmed] = useState(process.dependencies_confirmed);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [history, setHistory] = useState<History[] | null>(null);
  const [historyError, setHistoryError] = useState("");
  const [historyReload, setHistoryReload] = useState(0);
  const errorRef = useRef<HTMLParagraphElement>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    headingRef.current?.focus();
  }, []);
  useEffect(() => {
    if (error) errorRef.current?.focus();
  }, [error]);
  useEffect(() => {
    const controller = new AbortController();
    void processRequest<History[]>(
      `/api/projects/${projectId}/processes/${process.id}/history`,
      { signal: controller.signal },
    )
      .then((result) => {
        if (!controller.signal.aborted) {
          setHistory(result);
          setHistoryError("");
        }
      })
      .catch((reason: unknown) => {
        if (!controller.signal.aborted)
          setHistoryError(
            reason instanceof Error
              ? reason.message
              : "Unable to load history.",
          );
      });
    return () => controller.abort();
  }, [projectId, process.id, historyReload]);
  const locked = !!process.started_at;
  async function save(event: FormEvent<HTMLFormElement>) {
    const submittedForm = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : undefined;
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setSaving(true);
    setError("");
    try {
      const result = await processRequest<ProjectProcess>(
        `/api/projects/${projectId}/processes/${process.id}`,
        {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            expected_version: process.version,
            owner_user_id: form.get("owner_user_id") || null,
            planned_start_date: form.get("planned_start_date") || null,
            planned_finish_date: form.get("planned_finish_date") || null,
            status,
            notes: form.get("notes"),
            completion_reference: form.get("completion_reference"),
            predecessor_ids: predecessors,
            dependencies_confirmed: confirmed,
            reason: form.get("reason"),
          }),
        },
      );
      onSaved(result, predecessors);
    } catch (reason) {
      if (displayServerErrors(submittedForm, reason)) return;
      setError(
        reason instanceof Error
          ? reason.message
          : "Unable to save this process.",
      );
    } finally {
      setSaving(false);
    }
  }
  const mapped = predecessors
    .map((id) => board.processes.find((item) => item.id === id))
    .filter((item): item is ProjectProcess => !!item);
  return (
    <div className="space-y-5">
      <div className="rounded-2xl border border-slate-200 bg-white p-5 sm:p-6">
        <p className="text-xs font-bold uppercase tracking-wider text-blue-700">
          Process {process.position} of 19
        </p>
        <h3
          ref={headingRef}
          tabIndex={-1}
          className="mt-2 text-2xl font-extrabold text-slate-900 outline-none"
        >
          {process.name}
        </h3>
        <p className="mt-2 text-sm leading-6 text-slate-600">
          {process.purpose}
        </p>
        <div className="mt-5 grid gap-4 sm:grid-cols-2">
          <div>
            <h4 className="text-sm font-bold text-slate-800">
              Expected outputs
            </h4>
            <ul className="mt-2 list-disc space-y-1 pl-5 text-sm leading-6 text-slate-600">
              {process.deliverables.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </div>
          <div className="rounded-xl bg-slate-50 p-4 text-sm text-slate-600">
            <p>
              Recorded status:{" "}
              <strong className="capitalize text-slate-800">
                {process.status === "completed" ? "✓ " : ""}
                {processStatus(process.status)}
              </strong>
            </p>
            <p className="mt-2">
              Started:{" "}
              {process.started_at
                ? new Date(process.started_at).toLocaleString()
                : "Not started"}
            </p>
            <p className="mt-2">
              Completed:{" "}
              {process.completed_at
                ? new Date(process.completed_at).toLocaleString()
                : "Not completed"}
            </p>
            {process.workspace ? (
              <button
                type="button"
                className={processSecondary + " mt-4"}
                onClick={() => onOpenWorkspace(process.workspace!)}
              >
                {process.workspace === "contracts"
                  ? "Open contract control"
                  : process.workspace === "planning"
                    ? "Open work packages"
                    : process.workspace === "contractor_work"
                      ? "Open contractor work & manpower"
                      : "Open project team"}
              </button>
            ) : null}
          </div>
        </div>
      </div>
      <ValidatedForm
        onSubmit={(event) => void save(event)}
        className="rounded-2xl border border-slate-200 bg-white p-5 sm:p-6"
        aria-label="Process details"
      >
        <h4 className="text-lg font-bold text-slate-900">
          Accountability and progress
        </h4>
        {!board.can_write ? (
          <p className="mt-3 text-sm text-slate-600">
            You have read-only access to this project’s processes.
          </p>
        ) : null}
        {error ? (
          <p
            ref={errorRef}
            tabIndex={-1}
            role="alert"
            className="mt-4 rounded-xl bg-red-50 p-4 text-red-800"
          >
            {error}
          </p>
        ) : null}
        <fieldset
          disabled={saving || !board.can_write}
          className="mt-5 space-y-5"
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <ValidatedLabel className="text-sm font-semibold text-slate-700">
              Accountable owner
              <ValidatedSelect
                name="owner_user_id"
                defaultValue={process.owner_user_id ?? ""}
                required={status === "in_progress" || status === "completed"}
                className="field"
              >
                <option value="">Not assigned</option>
                {process.owner_user_id &&
                !board.owners.some(
                  (owner) => owner.id === process.owner_user_id,
                ) ? (
                  <option value={process.owner_user_id}>
                    Inactive member — select a new owner
                  </option>
                ) : null}
                {board.owners.map((owner) => (
                  <option key={owner.id} value={owner.id}>
                    {owner.label}
                  </option>
                ))}
              </ValidatedSelect>
            </ValidatedLabel>
            <ValidatedLabel className="text-sm font-semibold text-slate-700">
              Process status
              <ValidatedSelect name="status"
                value={status}
                onChange={(event) =>
                  setStatus(event.target.value as ProjectProcess["status"])
                }
                className="field"
              >
                <option value="not_started">
                  Not started / reopen for planning
                </option>
                {process.status === "bypassed" && (
                  <option value="bypassed" disabled>
                    Bypassed · reopen to change
                  </option>
                )}
                <option value="in_progress">In progress</option>
                <option value="blocked">Blocked</option>
                <option value="completed">Completed</option>
              </ValidatedSelect>
            </ValidatedLabel>
            <ValidatedLabel className="text-sm font-semibold text-slate-700">
              Planned start
              <ValidatedInput
                name="planned_start_date"
                type="date"
                defaultValue={process.planned_start_date ?? ""}
                className="field"
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-sm font-semibold text-slate-700">
              Planned finish
              <ValidatedInput
                name="planned_finish_date"
                type="date"
                defaultValue={process.planned_finish_date ?? ""}
                className="field"
              />
            </ValidatedLabel>
          </div>
          {!board.owners.length ? (
            <p className="rounded-xl bg-amber-50 p-3 text-sm text-amber-900">
              Assign an active employee in Project team before starting a
              process. You can still plan dates and dependencies now.
            </p>
          ) : null}
          <div>
            <h4 className="text-base font-bold text-slate-900">
              Project-specific prerequisites
            </h4>
            <p className="mt-1 text-sm leading-6 text-slate-600">
              Select the processes that must complete first. Leave the selection
              empty for a root process, then confirm the map. Unrelated
              processes may run in parallel.
            </p>
            {locked ? (
              <p className="mt-2 text-sm text-amber-900">
                This process has started. Save it as Not started before changing
                its dependency map. Downstream processes must be reopened first
                when applicable.
              </p>
            ) : null}
            <div className="mt-3 grid max-h-80 gap-2 overflow-y-auto rounded-xl border border-slate-200 p-3 sm:grid-cols-2">
              {board.processes
                .filter((item) => item.id !== process.id)
                .map((item) => (
                  <ValidatedLabel
                    key={item.id}
                    className="flex min-h-11 items-start gap-3 rounded-lg p-2 text-sm text-slate-700 hover:bg-slate-50"
                  >
                    <ValidatedInput
                      type="checkbox"
                      disabled={locked}
                      checked={predecessors.includes(item.id)}
                      onChange={(event) => {
                        setPredecessors((current) =>
                          event.target.checked
                            ? [...current, item.id]
                            : current.filter((id) => id !== item.id),
                        );
                        setConfirmed(false);
                      }}
                      className="mt-1 h-4 w-4 shrink-0"
                    />
                    <span>
                      {item.position}. {item.name}
                      <span className="mt-1 block text-xs capitalize text-slate-500">
                        {processStatus(item.status)}
                      </span>
                    </span>
                  </ValidatedLabel>
                ))}
            </div>
            <ValidatedLabel className="mt-3 flex min-h-11 items-center gap-3 text-sm font-semibold text-slate-800">
              <ValidatedInput
                type="checkbox"
                disabled={locked}
                checked={confirmed}
                onChange={(event) => setConfirmed(event.target.checked)}
                className="h-5 w-5 shrink-0"
              />
              I have reviewed this project’s prerequisites for this process.
            </ValidatedLabel>
            {mapped.length ? (
              <p className="mt-2 text-sm text-amber-900">
                {
                  mapped.filter(
                    (item) => !["completed", "bypassed"].includes(item.status),
                  ).length
                }{" "}
                of {mapped.length} selected prerequisites remain unresolved.
              </p>
            ) : null}
          </div>
          <ValidatedLabel className="block text-sm font-semibold text-slate-700">
            Progress / outcome notes
            <ValidatedTextarea
              name="notes"
              required={status === "completed"}
              maxLength={4000}
              rows={3}
              defaultValue={process.notes}
              className="field"
            />
          </ValidatedLabel>
          <ValidatedLabel className="block text-sm font-semibold text-slate-700">
            Completion evidence / approval reference
            <ValidatedInput
              name="completion_reference"
              required={status === "completed"}
              maxLength={1000}
              defaultValue={process.completion_reference}
              className="field"
              placeholder="Document number, approved record, or evidence reference"
            />
            <span className="mt-2 block text-xs font-normal leading-5 text-slate-500">
              Recording process completion does not replace contract, technical,
              safety, or quality approvals. Keep those approvals in their
              governing records.
            </span>
          </ValidatedLabel>
          {board.can_write ? (
            <ValidatedLabel className="block text-sm font-semibold text-slate-700">
              Reason for change
              <ValidatedTextarea
                name="reason"
                required
                maxLength={1000}
                rows={2}
                className="field"
                placeholder="Explain the planning or progress change for the project history"
              />
            </ValidatedLabel>
          ) : null}
        </fieldset>
        {board.can_write ? (
          <div className="mt-5 flex flex-wrap items-center justify-end gap-3">
            {process.status === "completed" ? (
              <p className="text-sm text-amber-900">
                Select a reopened status before saving changes.
              </p>
            ) : null}
            <button className={processButton} disabled={saving}>
              {saving ? "Saving…" : "Save process"}
            </button>
          </div>
        ) : null}
      </ValidatedForm>
      <section className="rounded-2xl border border-slate-200 bg-white p-5 sm:p-6">
        <h4 className="text-lg font-bold text-slate-900">Process history</h4>
        {historyError ? (
          <p role="alert" className="mt-3 text-red-800">
            {historyError}
            <button
              type="button"
              className="ml-2 underline"
              onClick={() => setHistoryReload((value) => value + 1)}
            >
              Retry
            </button>
          </p>
        ) : null}
        {history === null && !historyError ? (
          <p role="status" className="mt-3 text-sm text-slate-600">
            Loading history…
          </p>
        ) : null}
        <ol className="mt-3 divide-y divide-slate-100">
          {history?.map((event) => (
            <li key={event.id} className="py-3 text-sm">
              <p className="font-semibold text-slate-800">{event.reason}</p>
              <p className="mt-1 text-xs text-slate-500">
                {new Date(event.created_at).toLocaleString()} ·{" "}
                {event.created_by_id
                  ? (board.owners.find(
                      (owner) => owner.id === event.created_by_id,
                    )?.label ?? `User ${event.created_by_id}`)
                  : "System initialization"}
              </p>
              <p className="mt-1 text-sm capitalize text-slate-600">
                {typeof event.before.status === "string"
                  ? `${processStatus(event.before.status)} → `
                  : ""}
                {typeof event.after.status === "string"
                  ? processStatus(event.after.status)
                  : event.action}
              </p>
              <details className="mt-2">
                <summary className="cursor-pointer text-xs text-blue-700">
                  Recorded changes
                </summary>
                <dl className="mt-2 space-y-3 rounded-lg bg-slate-50 p-3 text-xs text-slate-600">
                  {Object.keys(event.after)
                    .filter(
                      (field) =>
                        historyLabels[field] &&
                        JSON.stringify(event.before[field]) !==
                          JSON.stringify(event.after[field]),
                    )
                    .map((field) => (
                      <div key={field}>
                        <dt className="font-bold text-slate-800">
                          {historyLabels[field]}
                        </dt>
                        <dd className="mt-1 break-words">
                          {historyValue(field, event.before[field], board)} →{" "}
                          {historyValue(field, event.after[field], board)}
                        </dd>
                      </div>
                    ))}
                </dl>
              </details>
            </li>
          ))}
        </ol>
      </section>
    </div>
  );
}
