"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";

import { FormEvent, useState } from "react";

export type O2teStep = {
  id: string;
  step_code: string;
  title: string;
  sequence: number;
  status: "not_started" | "in_progress" | "completed" | "not_applicable";
  owner_label: string;
  due_date: string | null;
  notes: string;
  completed_at: string | null;
  is_required: boolean;
};

type O2teChecklistProps = {
  projectId: string;
  steps: O2teStep[];
  onUpdated: () => Promise<void>;
};

const STATUS_LABELS = {
  not_started: "Not started",
  in_progress: "In progress",
  completed: "Completed",
  not_applicable: "Not applicable",
} as const;

function message(payload: unknown, fallback: string): string {
  if (typeof payload === "object" && payload !== null && "detail" in payload) {
    const { detail } = payload;
    if (typeof detail === "string") return detail;
  }
  return fallback;
}

function statusClass(status: O2teStep["status"]): string {
  return {
    completed: "bg-emerald-50 text-emerald-700",
    in_progress: "bg-blue-50 text-blue-700",
    not_applicable: "bg-slate-100 text-slate-600",
    not_started: "bg-amber-50 text-amber-800",
  }[status];
}

export function O2teChecklist({ projectId, steps, onUpdated }: O2teChecklistProps) {
  const [editing, setEditing] = useState<O2teStep | null>(null);
  const [status, setStatus] = useState<O2teStep["status"]>("not_started");
  const [ownerLabel, setOwnerLabel] = useState("");
  const [dueDate, setDueDate] = useState("");
  const [notes, setNotes] = useState("");
  const [error, setError] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const required = steps.filter((step) => step.is_required);
  const completed = required.filter((step) => step.status === "completed").length;

  function beginEdit(step: O2teStep) {
    setEditing(step);
    setStatus(step.status);
    setOwnerLabel(step.owner_label);
    setDueDate(step.due_date ?? "");
    setNotes(step.notes);
    setError("");
  }

  async function saveStep(event: FormEvent<HTMLFormElement>) {
    const submittedForm = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : undefined;
    event.preventDefault();
    if (!editing) return;
    setIsSaving(true);
    setError("");
    try {
      const response = await fetch(`/api/projects/${projectId}/o2te/${editing.id}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          status,
          owner_label: ownerLabel,
          due_date: dueDate || null,
          notes,
        }),
      });
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(message(payload, "Unable to update this O2TE obligation."));
      await onUpdated();
      setEditing(null);
    } catch (saveError) {
      if (displayServerErrors(submittedForm, saveError)) return;
      setError(saveError instanceof Error ? saveError.message : "Unable to update this O2TE obligation.");
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white">
      <div className="flex flex-col gap-3 border-b border-slate-100 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
        <div><p className="text-xs font-bold uppercase tracking-[0.13em] text-blue-700">Contract activation</p><h4 className="mt-1 font-extrabold text-slate-900">O2TE contractual readiness</h4></div>
        <span className={`w-fit rounded-full px-3 py-1.5 text-xs font-extrabold ${completed === required.length ? "bg-emerald-50 text-emerald-700" : "bg-amber-50 text-amber-800"}`}>{completed} of {required.length} required steps complete</span>
      </div>
      <div className="divide-y divide-slate-100">
        {steps.map((step) => <div className="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between" key={step.id}>
          <div className="min-w-0"><div className="flex items-center gap-2"><span className="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-slate-100 text-xs font-extrabold text-slate-600">{step.sequence}</span><p className="text-sm font-bold text-slate-800">{step.title}</p></div><p className="mt-1 pl-8 text-xs text-slate-500">{step.owner_label || "Owner not assigned"}{step.due_date ? ` · Due ${new Intl.DateTimeFormat("en-IN", { day: "2-digit", month: "short", year: "numeric" }).format(new Date(`${step.due_date}T00:00:00`))}` : " · No due date"}</p></div>
          <div className="flex items-center gap-2 pl-8 sm:pl-0"><span className={`rounded-full px-2.5 py-1 text-xs font-bold ${statusClass(step.status)}`}>{STATUS_LABELS[step.status]}</span><button className="min-h-10 rounded-lg border border-slate-200 px-3 text-xs font-bold text-slate-700 transition hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500" onClick={() => beginEdit(step)} type="button">Update</button></div>
        </div>)}
      </div>
      {editing ? <div className="fixed inset-0 z-[60] flex items-end bg-slate-950/50 backdrop-blur-sm sm:items-center sm:justify-center sm:p-5"><ValidatedForm aria-labelledby="o2te-step-title" aria-modal="true" className="vtpl-modal w-full rounded-t-3xl bg-white p-5 shadow-2xl sm:max-w-lg sm:rounded-3xl sm:p-6" onSubmit={(event) => void saveStep(event)} role="dialog"><div className="flex items-start justify-between gap-4"><div><p className="text-xs font-bold uppercase tracking-[0.13em] text-blue-700">O2TE obligation {editing.sequence}</p><h5 className="mt-1 text-lg font-extrabold text-slate-900" id="o2te-step-title">{editing.title}</h5></div><button aria-label="Close O2TE obligation" className="min-h-11 rounded-xl px-3 text-sm font-bold text-slate-500 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500" onClick={() => setEditing(null)} type="button">Close</button></div>{error ? <p className="mt-4 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm font-semibold text-red-800" role="alert">{error}</p> : null}<div className="mt-5 grid gap-4"><ValidatedLabel className="text-sm font-bold text-slate-700">Status<ValidatedSelect name="status" className="field" onChange={(event) => setStatus(event.target.value as O2teStep["status"])} value={status}>{Object.entries(STATUS_LABELS).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</ValidatedSelect></ValidatedLabel><ValidatedLabel className="text-sm font-bold text-slate-700">Accountable owner<ValidatedInput name="owner_label" className="field" maxLength={128} onChange={(event) => setOwnerLabel(event.target.value)} placeholder="For example, Contracts Executive" value={ownerLabel} /></ValidatedLabel><ValidatedLabel className="text-sm font-bold text-slate-700">Due date<ValidatedInput name="due_date" className="field" onChange={(event) => setDueDate(event.target.value)} type="date" value={dueDate} /></ValidatedLabel><ValidatedLabel className="text-sm font-bold text-slate-700">Notes <span className="font-medium text-slate-400">required if not applicable</span><ValidatedTextarea name="notes" className="field" maxLength={2000} onChange={(event) => setNotes(event.target.value)} rows={4} value={notes} /></ValidatedLabel></div><div className="mt-6 flex justify-end gap-2"><button className="min-h-11 rounded-xl border border-slate-200 px-4 text-sm font-bold text-slate-700 focus:outline-none focus:ring-2 focus:ring-blue-500" onClick={() => setEditing(null)} type="button">Cancel</button><button className="min-h-11 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white disabled:opacity-60 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2" disabled={isSaving} type="submit">{isSaving ? "Saving..." : "Save obligation"}</button></div></ValidatedForm></div> : null}
    </section>
  );
}
