"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { FormEvent, useEffect, useState } from "react";
import type { ControlTask } from "./project-control-model";
import {
  processRequest,
  processSecondary,
  processButton,
  processStatus,
} from "./project-process-data";

type Option = { id: string; label: string };
type Event = {
  id: string;
  reason: string;
  created_at: string;
  created_by_id: string | null;
  before: Record<string, unknown>;
  after: Record<string, unknown>;
};
export function ProjectTaskEditor({
  projectId,
  task,
  options,
  onSaved,
  onClose,
}: {
  projectId: string;
  task: ControlTask;
  options: { owners: Option[]; processes: Option[]; can_write: boolean };
  onSaved: () => void;
  onClose: () => void;
}) {
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [history, setHistory] = useState<Event[] | null>(null);
  const [historyError, setHistoryError] = useState("");
  const base = `/api/projects/${projectId}/schedule/activities/${task.id}`;
  useEffect(() => {
    const c = new AbortController();
    void processRequest<Event[]>(`${base}/history`, { signal: c.signal })
      .then(setHistory)
      .catch((e: Error) => {
        if (!c.signal.aborted) setHistoryError(e.message);
      });
    return () => c.abort();
  }, [base, task.version]);
  async function save(e: FormEvent<HTMLFormElement>) {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    setSaving(true);
    setError("");
    try {
      await processRequest(base, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          expected_version: task.version,
          process_id: f.get("process_id") || null,
          owner_user_id: f.get("owner_user_id"),
          status: f.get("status"),
          planned_start_date: f.get("planned_start_date") || null,
          planned_finish_date: f.get("planned_finish_date") || null,
          actual_start_date: f.get("actual_start_date") || null,
          actual_finish_date: f.get("actual_finish_date") || null,
          reason: f.get("reason"),
        }),
      });
      onSaved();
    } catch (e) {
      if (displayServerErrors(submittedForm, e)) return;
      setError(e instanceof Error ? e.message : "Unable to update task");
    } finally {
      setSaving(false);
    }
  }
  const input =
    "mt-1 min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 font-normal";
  function value(field: string, v: unknown) {
    if (!v) return "Not recorded";
    if (field === "owner_user_id")
      return (
        options.owners.find((o) => o.id === v)?.label ?? "Former project member"
      );
    if (field === "process_id")
      return (
        options.processes.find((o) => o.id === v)?.label ??
        "Unavailable process"
      );
    return processStatus(String(v));
  }
  return (
    <section
      className="rounded-2xl border border-blue-200 bg-white p-5"
      aria-labelledby="task-editor-title"
    >
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-bold uppercase text-blue-700">
            Schedule task · revision {task.version}
          </p>
          <h4 id="task-editor-title" className="mt-1 text-xl font-bold">
            {task.code} · {task.name}
          </h4>
        </div>
        <button className={processSecondary} onClick={onClose}>
          Close task
        </button>
      </div>
      <p className="mt-3 text-sm text-slate-600">
        Record actual work and its process link. These updates do not approve or
        complete the parent process.
      </p>
      {error ? (
        <p role="alert" className="mt-4 rounded-lg bg-red-50 p-3 text-red-800">
          {error}
        </p>
      ) : null}
      <ValidatedForm key={task.version} onSubmit={save} className="mt-5">
        <fieldset
          disabled={saving || !options.can_write}
          className="grid gap-4 sm:grid-cols-2"
        >
          <ValidatedLabel className="text-sm font-semibold">
            Project process
            <ValidatedSelect
              name="process_id"
              defaultValue={task.process_id ?? ""}
              className={input}
            >
              <option value="">Unassigned</option>
              {options.processes.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.label}
                </option>
              ))}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="text-sm font-semibold">
            Task owner
            <ValidatedSelect
              name="owner_user_id"
              defaultValue={task.owner_user_id ?? ""}
              required
              className={input}
            >
              <option value="">Select active member</option>
              {options.owners.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.label}
                </option>
              ))}
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="text-sm font-semibold">
            Task status
            <ValidatedSelect name="status" defaultValue={task.status} className={input}>
              {["not_started", "in_progress", "blocked", "completed"].map(
                (s) => (
                  <option key={s} value={s}>
                    {processStatus(s)}
                  </option>
                ),
              )}
            </ValidatedSelect>
          </ValidatedLabel>
          {(
            [
              "planned_start_date",
              "planned_finish_date",
              "actual_start_date",
              "actual_finish_date",
            ] as const
          ).map((field) => (
            <ValidatedLabel key={field} className="text-sm font-semibold">
              {processStatus(field)}
              <ValidatedInput
                name={field}
                type="date"
                defaultValue={task[field] ?? ""}
                className={input}
              />
            </ValidatedLabel>
          ))}
          <ValidatedLabel className="text-sm font-semibold sm:col-span-2">
            Update reason / blocker detail
            <ValidatedTextarea
              name="reason"
              maxLength={2000}
              required
              rows={3}
              className={input + " py-2"}
              placeholder="What changed, why, and what is needed next?"
            />
          </ValidatedLabel>
        </fieldset>
        {options.can_write ? (
          <button disabled={saving} className={processButton + " mt-4"}>
            {saving ? "Saving…" : "Save task update"}
          </button>
        ) : (
          <p className="mt-3 text-sm text-slate-600">
            Read-only schedule access.
          </p>
        )}
      </ValidatedForm>
      <div className="mt-6 border-t border-slate-200 pt-4">
        <h5 className="font-bold">Task update history</h5>
        {historyError ? (
          <p role="alert" className="mt-2 text-sm text-red-700">
            {historyError}
          </p>
        ) : null}
        {history === null && !historyError ? (
          <p className="mt-2 text-sm">Loading history…</p>
        ) : null}
        {history?.length === 0 ? (
          <p className="mt-2 text-sm text-slate-500">
            No task updates recorded. Creation is attributed separately.
          </p>
        ) : null}
        {history?.map((event) => (
          <details
            key={event.id}
            className="mt-3 rounded-lg border border-slate-200 p-3"
          >
            <summary className="cursor-pointer text-sm font-semibold">
              {new Date(event.created_at).toLocaleString()} · {event.reason}
            </summary>
            <p className="mt-2 text-xs text-slate-500">
              Updated by{" "}
              {options.owners.find((o) => o.id === event.created_by_id)
                ?.label ??
                event.created_by_id ??
                "System"}
            </p>
            <dl className="mt-2 space-y-2">
              {Object.keys(event.after)
                .filter(
                  (k) => event.before[k] !== event.after[k] && k !== "version",
                )
                .map((k) => (
                  <div key={k} className="text-sm">
                    <dt className="font-semibold">{processStatus(k)}</dt>
                    <dd className="text-slate-600">
                      {value(k, event.before[k])} → {value(k, event.after[k])}
                    </dd>
                  </div>
                ))}
            </dl>
          </details>
        ))}
      </div>
    </section>
  );
}
