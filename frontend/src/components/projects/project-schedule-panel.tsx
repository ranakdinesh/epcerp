"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";

import { useRouter, useSearchParams } from "next/navigation";
import { ProjectTaskEditor } from "./project-task-editor";
import { FormEvent, useEffect, useRef, useState } from "react";

type Activity = {
  id: string;
  process_id: string | null;
  version: number;
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
};
type Dependency = {
  id: string;
  predecessor_activity_id: string;
  successor_activity_id: string;
  relationship_type: string;
};
type Option = { id: string; label: string };
type Options = {
  processes: Option[];
  owners: Option[];
  work_packages: Option[];
  can_write: boolean;
};
const button =
  "min-h-11 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white hover:bg-blue-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600 disabled:opacity-50";
const secondary =
  "min-h-11 rounded-xl border border-slate-300 px-4 text-sm font-bold text-slate-700 hover:bg-slate-50";
const relationships = [
  "finish_to_start",
  "start_to_start",
  "finish_to_finish",
  "start_to_finish",
];
const label = (value: string) => value.replaceAll("_", " ");
async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, { cache: "no-store", ...init });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    const detail = payload.detail;
    throw new Error(
      typeof detail === "string"
        ? detail
        : Array.isArray(detail)
          ? detail
              .map(
                (item: { loc?: string[]; msg?: string }) =>
                  `${item.loc?.slice(1).join(" ")}: ${item.msg}`,
              )
              .join(". ")
          : "Unable to complete this request. Please try again.",
    );
  }
  return payload as T;
}

export function ProjectSchedulePanel({ projectId }: { projectId: string }) {
  // Keying the content prevents a previous project's records or form from surviving navigation.
  return <ScheduleContent key={projectId} projectId={projectId} />;
}

function ScheduleContent({ projectId }: { projectId: string }) {
  const router = useRouter();
  const params = useSearchParams();
  const selectedActivity = params.get("activity");
  function selectActivity(id: string) {
    const next = new URLSearchParams(params);
    if (id) next.set("activity", id);
    else next.delete("activity");
    router.push(`/dashboard?${next}`, { scroll: false });
  }
  const base = `/api/projects/${projectId}/schedule`;
  const [data, setData] = useState<{
    activities: Activity[];
    dependencies: Dependency[];
    options: Options;
  } | null>(null);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [mode, setMode] = useState<"activity" | "dependency" | null>(null);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState("");
  const [notice, setNotice] = useState("");
  const errorRef = useRef<HTMLParagraphElement>(null);
  const formRef = useRef<HTMLFormElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    void Promise.all([
      request<Activity[]>(`${base}/activities`, { signal: controller.signal }),
      request<Dependency[]>(`${base}/dependencies`, {
        signal: controller.signal,
      }),
      request<Options>(`${base}/editor-options`, { signal: controller.signal }),
    ])
      .then(([activities, dependencies, options]) => {
        if (!controller.signal.aborted) {
          setData({ activities, dependencies, options });
          setError("");
        }
      })
      .catch((reason: unknown) => {
        if (!controller.signal.aborted)
          setError(
            reason instanceof Error
              ? reason.message
              : "Unable to load schedule.",
          );
      });
    return () => controller.abort();
  }, [base, reload]);

  useEffect(() => {
    if (mode)
      formRef.current?.querySelector<HTMLElement>("input, select")?.focus();
  }, [mode]);
  useEffect(() => {
    if (formError) errorRef.current?.focus();
  }, [formError]);

  function open(next: "activity" | "dependency") {
    setFormError("");
    setMode(next);
  }
  async function save(event: FormEvent<HTMLFormElement>) {
    const submittedForm = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : undefined;
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const payload =
      mode === "activity"
        ? {
            process_id: form.get("process_id") || null,
            work_package_id: form.get("work_package_id"),
            code: form.get("code"),
            name: form.get("name"),
            owner_user_id: form.get("owner_user_id"),
            planned_start_date: form.get("planned_start_date") || null,
            planned_finish_date: form.get("planned_finish_date") || null,
            is_milestone: form.get("is_milestone") === "on",
            status: "not_started",
          }
        : {
            predecessor_activity_id: form.get("predecessor_activity_id"),
            successor_activity_id: form.get("successor_activity_id"),
            relationship_type: form.get("relationship_type"),
          };
    setSaving(true);
    setFormError("");
    try {
      const result = await request<Activity | Dependency>(
        `${base}/${mode === "activity" ? "activities" : "dependencies"}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(payload),
        },
      );
      setData((current) =>
        current
          ? mode === "activity"
            ? {
                ...current,
                activities: [...current.activities, result as Activity],
              }
            : {
                ...current,
                dependencies: [...current.dependencies, result as Dependency],
              }
          : current,
      );
      setNotice(
        mode === "activity" ? "Activity created." : "Prerequisite added.",
      );
      setMode(null);
      setReload((value) => value + 1);
    } catch (reason) {
      if (displayServerErrors(submittedForm, reason)) return;
      setFormError(
        reason instanceof Error ? reason.message : "Unable to save schedule.",
      );
    } finally {
      setSaving(false);
    }
  }

  const activities = data?.activities ?? [];
  const active = activities.filter((item) => item.is_active);
  const activityById = new Map(activities.map((item) => [item.id, item]));
  const owners = new Map(
    data?.options.owners.map((item) => [item.id, item.label]) ?? [],
  );
  const packages = new Map(
    data?.options.work_packages.map((item) => [item.id, item.label]) ?? [],
  );
  const prerequisites = new Map<string, Dependency[]>();
  for (const edge of data?.dependencies ?? []) {
    const existing = prerequisites.get(edge.successor_activity_id) ?? [];
    existing.push(edge);
    prerequisites.set(edge.successor_activity_id, existing);
  }

  return (
    <section className="space-y-5">
      {data &&
      selectedActivity &&
      active.find((a) => a.id === selectedActivity) ? (
        <ProjectTaskEditor
          key={selectedActivity}
          projectId={projectId}
          task={active.find((a) => a.id === selectedActivity)!}
          options={data.options}
          onClose={() => selectActivity("")}
          onSaved={() => {
            setReload((v) => v + 1);
            setNotice("Task updated.");
          }}
        />
      ) : null}
      {data &&
      selectedActivity &&
      !active.some((a) => a.id === selectedActivity) ? (
        <p role="alert">The selected task is not available in this project.</p>
      ) : null}
      <div className="flex flex-col gap-4 rounded-2xl border border-slate-200 bg-white p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-xs font-bold uppercase tracking-[0.14em] text-blue-700">
            Project management
          </p>
          <h3 className="mt-1 text-xl font-extrabold text-slate-900">
            Execution schedule
          </h3>
          <p className="mt-1 text-sm text-slate-600">
            Draft activities, milestones and predecessor relationships. No
            approved baseline yet.
          </p>
        </div>
        {data?.options.can_write ? (
          <div className="flex shrink-0 flex-wrap gap-2">
            <button
              type="button"
              className={secondary}
              disabled={saving || active.length < 2}
              onClick={() => open("dependency")}
            >
              Add prerequisite
            </button>
            <button
              type="button"
              className={button}
              disabled={saving}
              onClick={() => open("activity")}
            >
              Add activity
            </button>
          </div>
        ) : null}
      </div>
      {error ? (
        <div role="alert" className="rounded-xl bg-red-50 p-4 text-red-800">
          {error}{" "}
          <button
            type="button"
            className="min-h-11 px-3 underline"
            onClick={() => setReload((value) => value + 1)}
          >
            Retry
          </button>
        </div>
      ) : null}
      <p role="status" className="text-sm text-green-800">
        {notice}
      </p>
      {!data && !error ? (
        <p role="status" className="p-5 text-slate-600">
          Loading schedule…
        </p>
      ) : null}
      {mode && data ? (
        <ValidatedForm
          key={mode}
          ref={formRef}
          onSubmit={(event) => void save(event)}
          className="rounded-2xl border border-blue-200 bg-white p-5"
          aria-labelledby="schedule-form-title"
        >
          <h4
            id="schedule-form-title"
            className="text-lg font-bold text-slate-900"
          >
            {mode === "activity"
              ? "New activity or milestone"
              : "New prerequisite"}
          </h4>
          {formError ? (
            <p
              ref={errorRef}
              tabIndex={-1}
              role="alert"
              className="mt-3 rounded-xl bg-red-50 p-3 text-red-800"
            >
              {formError}
            </p>
          ) : null}
          <fieldset
            disabled={saving}
            className="mt-4 grid gap-4 sm:grid-cols-2"
          >
            {mode === "activity" ? (
              <>
                <ValidatedLabel className="block text-sm font-bold text-slate-700">
                  Project process
                  <ValidatedSelect
                    name="process_id"
                    className="mt-1 min-h-11 w-full rounded-xl border border-slate-300 bg-white px-3"
                  >
                    <option value="">Unassigned</option>
                    {data.options.processes.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.label}
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Work package
                  <ValidatedSelect
                    required
                    name="work_package_id"
                    defaultValue=""
                    className="field"
                  >
                    <option value="" disabled>
                      Select a work package
                    </option>
                    {data.options.work_packages.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.label}
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Accountable owner
                  <ValidatedSelect
                    required
                    name="owner_user_id"
                    defaultValue=""
                    className="field"
                  >
                    <option value="" disabled>
                      Select an active project member
                    </option>
                    {data.options.owners.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.label}
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Activity code
                  <ValidatedInput
                    required
                    maxLength={64}
                    name="code"
                    className="field"
                    placeholder="ACT-001"
                  />
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Activity name
                  <ValidatedInput
                    required
                    maxLength={255}
                    name="name"
                    className="field"
                  />
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Planned start
                  <ValidatedInput
                    type="date"
                    name="planned_start_date"
                    className="field"
                  />
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Planned finish
                  <ValidatedInput
                    type="date"
                    name="planned_finish_date"
                    className="field"
                  />
                </ValidatedLabel>
                <ValidatedLabel className="flex min-h-11 items-center gap-2 text-sm font-semibold text-slate-700">
                  <ValidatedInput
                    type="checkbox"
                    name="is_milestone"
                    className="h-5 w-5"
                  />
                  Milestone (start and finish must match)
                </ValidatedLabel>
                {!data.options.work_packages.length ||
                !data.options.owners.length ? (
                  <p className="text-sm text-amber-800">
                    An active work package and assigned project member are
                    required. Set these up in Planning and Team before adding
                    activities.
                  </p>
                ) : null}
              </>
            ) : (
              <>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Predecessor
                  <ValidatedSelect
                    required
                    name="predecessor_activity_id"
                    defaultValue=""
                    className="field"
                  >
                    <option value="" disabled>
                      Select predecessor
                    </option>
                    {active.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.code} — {item.name}
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Successor
                  <ValidatedSelect
                    required
                    name="successor_activity_id"
                    defaultValue=""
                    className="field"
                  >
                    <option value="" disabled>
                      Select successor
                    </option>
                    {active.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.code} — {item.name}
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
                <ValidatedLabel className="text-sm font-semibold text-slate-700">
                  Relationship
                  <ValidatedSelect name="relationship_type" className="field">
                    {relationships.map((item) => (
                      <option key={item} value={item}>
                        {label(item)}
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
                <p className="text-sm text-slate-600">
                  A predecessor’s start or finish is required before the
                  successor’s specified start or finish. Self-links and cycles
                  are rejected.
                </p>
              </>
            )}
          </fieldset>
          <div className="mt-5 flex flex-wrap justify-end gap-2">
            <button
              type="button"
              disabled={saving}
              className={secondary}
              onClick={() => setMode(null)}
            >
              Cancel
            </button>
            <button
              disabled={
                saving ||
                (mode === "activity" &&
                  (!data.options.owners.length ||
                    !data.options.work_packages.length))
              }
              className={button}
            >
              {saving
                ? "Saving…"
                : mode === "activity"
                  ? "Create activity"
                  : "Save prerequisite"}
            </button>
          </div>
        </ValidatedForm>
      ) : null}
      {data ? (
        <div className="overflow-x-auto rounded-2xl border border-slate-200 bg-white">
          <table className="vtpl-data-table w-full min-w-[850px] text-left">
            <caption className="sr-only">
              Project activities and prerequisite readiness
            </caption>
            <thead className="bg-slate-50 text-xs font-bold uppercase tracking-wide text-slate-500">
              <tr>
                {[
                  "Activity / package",
                  "Owner",
                  "Planned dates",
                  "Status",
                  "Prerequisites",
                ].map((title) => (
                  <th key={title} scope="col" className="px-5 py-3">
                    {title}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {activities.map((item) => (
                <tr key={item.id}>
                  <td className="px-5 py-4">
                    <p className="font-mono text-xs font-bold text-blue-700">
                      {item.code} ·{" "}
                      {item.is_milestone ? "Milestone" : "Activity"}
                    </p>
                    <button
                      type="button"
                      className="mt-1 min-h-11 text-left font-bold text-blue-700 underline"
                      onClick={() => selectActivity(item.id)}
                    >
                      {item.name}
                    </button>
                    <p className="text-xs text-slate-500">
                      {data?.options.processes.find(
                        (p) => p.id === item.process_id,
                      )?.label ?? "No process assigned"}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                      {packages.get(item.work_package_id) ??
                        "Inactive work package"}
                    </p>
                  </td>
                  <td className="px-5 py-4 text-sm text-slate-700">
                    {item.owner_user_id
                      ? (owners.get(item.owner_user_id) ??
                        "Inactive or unavailable member")
                      : "Unassigned"}
                  </td>
                  <td className="px-5 py-4 text-sm text-slate-600">
                    {item.planned_start_date ?? "Not planned"}
                    <br />
                    to {item.planned_finish_date ?? "Not planned"}
                  </td>
                  <td className="px-5 py-4 text-sm capitalize text-slate-700">
                    {label(item.status)}
                    {!item.is_active ? " (inactive)" : ""}
                  </td>
                  <td className="px-5 py-4 text-sm text-slate-700">
                    {prerequisites.get(item.id)?.map((edge) => {
                      const predecessor = activityById.get(
                        edge.predecessor_activity_id,
                      );
                      const recorded = edge.relationship_type.startsWith(
                        "finish",
                      )
                        ? predecessor?.actual_finish_date
                        : predecessor?.actual_start_date;
                      return (
                        <p key={edge.id} className="mb-2">
                          <span className="font-semibold">
                            {predecessor?.code ?? "Unavailable activity"}
                          </span>{" "}
                          · {label(edge.relationship_type)}
                          <br />
                          <span
                            className={
                              recorded ? "text-green-800" : "text-amber-800"
                            }
                          >
                            {recorded
                              ? "Predecessor event recorded"
                              : "Predecessor event pending"}
                          </span>
                        </p>
                      );
                    }) ?? "None"}
                  </td>
                </tr>
              ))}
              {!activities.length ? (
                <tr>
                  <td
                    colSpan={5}
                    className="px-5 py-12 text-center text-sm text-slate-600"
                  >
                    No schedule activities yet.
                    {data.options.can_write
                      ? " Add the first activity to start the project network."
                      : " Your project planner has not added activities."}
                  </td>
                </tr>
              ) : null}
            </tbody>
          </table>
        </div>
      ) : null}
    </section>
  );
}
