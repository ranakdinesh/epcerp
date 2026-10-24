"use client";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedTextarea } from "@/components/ui/validated-controls";

import { useState, type FormEvent } from "react";
import { DataGrid, GridAction } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { button, input, secondary } from "./purchase-data";
import {
  FMS_PHASES,
  FMS_STEPS,
  EXAMPLE_TODAY,
  currentStep,
  packageStatus,
  examplePackages,
  updateExample,
  type ExamplePackage,
  type ExampleAction,
} from "./fms-model";

function Status({ value }: { value: string }) {
  const color =
    value === "Completed" || value === "Done"
      ? "bg-emerald-50 text-emerald-800"
      : value === "Blocked"
        ? "bg-rose-50 text-rose-800"
        : value === "Overdue"
          ? "bg-amber-50 text-amber-900"
          : "bg-slate-100 text-slate-700";
  return (
    <span
      className={`inline-block whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-semibold ${color}`}
    >
      {value}
    </span>
  );
}

export function FmsPreview() {
  const [packages, setPackages] = useState(examplePackages);
  const [phase, setPhase] = useState("");
  const [selected, setSelected] = useState<string | null>(null);
  const [catalogue, setCatalogue] = useState(false);
  const [view, setView] = useState<"queue" | "register">("queue");
  const [notice, setNotice] = useState("");
  const pkg = packages.find((row) => row.id === selected);
  const filtered = packages.filter(
    (row) =>
      !phase || FMS_STEPS[(currentStep(row)?.number ?? 20) - 1].phase === phase,
  );

  return (
    <section
      className="min-w-0 space-y-5"
      aria-label="Purchase FMS example workspace"
    >
      <div className="flex flex-wrap items-start justify-between gap-4 rounded-xl border border-amber-200 bg-amber-50 p-4">
        <div className="max-w-3xl">
          <p className="text-sm font-bold text-amber-950">
            Interactive preview · Fictional packages only
          </p>
          <p className="mt-1 text-sm text-amber-900">
            Try the 20-step workflow before rollout. Changes last only while
            this preview is open; nothing is saved to your project. Example
            clock: 18 Sep 2026.
          </p>
        </div>
        <button
          className={secondary}
          onClick={() => {
            setPackages(examplePackages());
            setPhase("");
            setSelected(null);
            setNotice("Examples reset. No project records changed.");
          }}
        >
          Reset examples
        </button>
      </div>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h3 className="text-xl font-bold text-slate-900">
            Every purchase. One clear next step.
          </h3>
          <p className="mt-1 text-sm text-slate-600">
            Find an item, open its next action, and follow the evidence through
            to Accounts.
          </p>
        </div>
        <button className={secondary} onClick={() => setCatalogue(true)}>
          View all 20 steps
        </button>
      </div>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {[
          ["Example packages", packages.length],
          [
            "Overdue next steps",
            packages.filter((p) => packageStatus(p) === "Overdue").length,
          ],
          [
            "Blocked packages",
            packages.filter((p) => packageStatus(p) === "Blocked").length,
          ],
          [
            "Completed packages",
            packages.filter((p) => packageStatus(p) === "Completed").length,
          ],
        ].map(([label, count]) => (
          <div
            key={label}
            className="rounded-xl border border-slate-200 bg-white px-4 py-3"
          >
            <p className="text-xs font-semibold text-slate-600">{label}</p>
            <p className="mt-1 text-2xl font-bold tabular-nums text-slate-900">
              {count}
            </p>
          </div>
        ))}
      </div>
      <div
        className="grid gap-2 sm:grid-cols-2 xl:grid-cols-5"
        aria-label="Filter by current phase"
      >
        {FMS_PHASES.map((p) => (
          <button
            key={p.code}
            aria-pressed={phase === p.code}
            onClick={() => setPhase(phase === p.code ? "" : p.code)}
            className={`min-h-20 rounded-xl border p-3 text-left focus-visible:outline-2 focus-visible:outline-blue-600 ${phase === p.code ? "border-blue-700 bg-blue-50 text-blue-950" : "border-slate-200 bg-white text-slate-700 hover:bg-slate-50"}`}
          >
            <span className="block text-xs font-semibold">
              {p.code} · Steps {p.range}
            </span>
            <span className="mt-1 block text-sm font-bold">{p.title}</span>
          </button>
        ))}
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex gap-2" aria-label="Preview layout">
          <button
            className={view === "queue" ? button : secondary}
            aria-pressed={view === "queue"}
            onClick={() => setView("queue")}
          >
            Work queue
          </button>
          <button
            className={view === "register" ? button : secondary}
            aria-pressed={view === "register"}
            onClick={() => setView("register")}
          >
            Step register
          </button>
        </div>
        {phase && (
          <button className={secondary} onClick={() => setPhase("")}>
            Clear phase filter
          </button>
        )}
      </div>
      {notice && (
        <p role="status" className="text-sm text-slate-700">
          {notice}
        </p>
      )}
      {view === "queue" ? (
        <DataGrid<ExamplePackage>
          label="FMS packages"
          rows={filtered}
          status={packageStatus}
          initialSort="id"
          columns={[
            {
              key: "id",
              label: "Package / item",
              value: (r) => `${r.id} ${r.item}`,
              render: (r) => (
                <button
                  className="min-h-11 max-w-64 text-left font-semibold text-blue-800 underline decoration-blue-200 underline-offset-4"
                  onClick={() => setSelected(r.id)}
                >
                  {r.item}
                  <span className="mt-1 block text-xs font-normal text-slate-500">
                    {r.id} · {r.quantity}
                  </span>
                </button>
              ),
            },
            { key: "vendor", label: "Supplier", value: (r) => r.vendor },
            {
              key: "step",
              label: "Next action",
              value: (r) => {
                const s = currentStep(r);
                return s
                  ? `${String(s.number).padStart(2, "0")} · ${FMS_STEPS[s.number - 1].title}`
                  : "All 20 steps resolved";
              },
            },
            { key: "owner", label: "Current doer", value: (r) => r.owner },
            {
              key: "due",
              label: "Planned date",
              value: (r) => currentStep(r)?.planned ?? "—",
            },
            {
              key: "status",
              label: "Status",
              value: packageStatus,
              render: (r) => (
                <div>
                  <Status value={packageStatus(r)} />
                  {r.blocked && (
                    <p className="mt-1 max-w-48 text-xs text-rose-800">
                      {r.blocked}
                    </p>
                  )}
                </div>
              ),
            },
            {
              key: "progress",
              label: "Steps resolved",
              value: (r) => r.steps.filter((s) => s.state !== "pending").length,
              render: (r) => (
                <span className="whitespace-nowrap tabular-nums">
                  {r.steps.filter((s) => s.state !== "pending").length} / 20
                </span>
              ),
            },
          ]}
          actions={(r) => (
            <GridAction
              label={`Open ${r.id} workflow`}
              onClick={() => setSelected(r.id)}
            />
          )}
        />
      ) : (
        <StepRegister packages={filtered} open={setSelected} />
      )}
      <p className="text-xs text-slate-600">
        Step counts describe this demonstration, not physical progress. Planned
        dates, owners and evidence are examples. Live procurement timing and
        approval rules still require configuration.
      </p>
      {pkg && (
        <RecordDrawer
          title={`${pkg.id} · ${pkg.item}`}
          onClose={() => setSelected(null)}
        >
          <PackageDetail
            key={pkg.id}
            pkg={pkg}
            update={(next) => {
              setPackages((rows) =>
                rows.map((row) => (row.id === next.id ? next : row)),
              );
              setNotice(`${next.id}: example updated only in this preview.`);
            }}
          />
        </RecordDrawer>
      )}
      {catalogue && (
        <RecordDrawer
          title="Purchase FMS · 20-step reference"
          onClose={() => setCatalogue(false)}
        >
          <p className="rounded-lg bg-amber-50 p-3 text-sm text-amber-900">
            Source: purchase-fms.zip. TAT and weight are source values awaiting
            policy confirmation; proposed teams are not assigned users.
          </p>
          {FMS_PHASES.map((p) => (
            <section key={p.code} className="space-y-2">
              <h3 className="font-bold text-slate-900">
                {p.code} · {p.title}
              </h3>
              {FMS_STEPS.filter((s) => s.phase === p.code).map((s) => (
                <article
                  key={s.id}
                  className="rounded-xl border border-slate-200 bg-white p-4"
                >
                  <p className="font-semibold">
                    {String(s.number).padStart(2, "0")} · {s.title}
                  </p>
                  <p className="mt-2 text-sm text-slate-600">{s.evidence}</p>
                  <p className="mt-2 text-xs text-slate-600">
                    Proposed team: {s.team} · Source TAT: {s.tat} · Source
                    weight: {s.weight}
                  </p>
                </article>
              ))}
            </section>
          ))}
        </RecordDrawer>
      )}
    </section>
  );
}

function StepRegister({
  packages,
  open,
}: {
  packages: ExamplePackage[];
  open: (id: string) => void;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 bg-white">
      <div className="border-b border-slate-200 p-4">
        <h4 className="font-bold">Planned / actual register</h4>
        <p className="mt-1 text-sm text-slate-600">
          Scroll across the 20 steps. Open a package to update its current
          action.
        </p>
      </div>
      <div
        role="region"
        aria-label="Twenty-step purchase register, scroll horizontally"
        tabIndex={0}
        className="overflow-x-auto focus-visible:outline-2 focus-visible:outline-blue-600"
      >
        <table className="vtpl-data-table w-full text-left text-sm">
          <thead className="bg-slate-50">
            <tr>
              <th
                scope="col"
                className="sticky left-0 z-10 min-w-56 bg-slate-50 p-4"
              >
                Example package
              </th>
              {FMS_STEPS.map((s) => (
                <th scope="col" key={s.id} className="min-w-44 p-3 align-top">
                  <span className="block text-xs font-medium text-slate-500">
                    {s.phase} · {String(s.number).padStart(2, "0")}
                  </span>
                  {s.title}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200">
            {packages.map((p) => (
              <tr key={p.id}>
                <th scope="row" className="sticky left-0 z-10 bg-white p-4">
                  <button
                    onClick={() => open(p.id)}
                    className="min-h-11 text-left text-blue-800 underline"
                  >
                    {p.id}
                    <span className="block text-xs font-normal">{p.item}</span>
                  </button>
                </th>
                {p.steps.map((s) => (
                  <td key={s.number} className="p-3 align-top">
                    <Status
                      value={
                        s.state === "done"
                          ? "Done"
                          : s.state === "not_applicable"
                            ? "Not applicable"
                            : currentStep(p)?.number === s.number
                              ? packageStatus(p)
                              : "Upcoming"
                      }
                    />
                    <span className="mt-2 block whitespace-nowrap text-xs">
                      Plan: {s.planned}
                    </span>
                    <span className="mt-1 block whitespace-nowrap text-xs text-slate-600">
                      Actual: {s.actual || "—"}
                    </span>
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {!packages.length && (
          <p className="p-8 text-center text-slate-600">
            No example packages in this phase.
          </p>
        )}
      </div>
    </div>
  );
}

function PackageDetail({
  pkg,
  update,
}: {
  pkg: ExamplePackage;
  update: (p: ExamplePackage) => void;
}) {
  const [number, setNumber] = useState(currentStep(pkg)?.number ?? 20);
  const s = pkg.steps[number - 1];
  const definition = FMS_STEPS[number - 1];
  const current = currentStep(pkg);
  return (
    <>
      <p className="rounded-lg bg-amber-50 p-3 text-sm text-amber-900">
        Preview only · Changes are temporary. No purchase, approval or payment
        is issued.
      </p>
      <div className="flex flex-wrap justify-between gap-3">
        <p className="text-sm text-slate-600">
          {pkg.vendor}
          <br />
          {pkg.quantity} · {pkg.owner}
        </p>
        <Status value={packageStatus(pkg)} />
      </div>
      <div
        className="grid grid-cols-5 gap-2 sm:grid-cols-10"
        aria-label="Select purchase step"
      >
        {pkg.steps.map((step) => (
          <button
            key={step.number}
            aria-label={`Step ${step.number}: ${FMS_STEPS[step.number - 1].title}, ${step.state}`}
            aria-pressed={step.number === number}
            onClick={() => setNumber(step.number)}
            className={`min-h-11 rounded-lg border text-sm font-semibold ${step.number === number ? "border-blue-800 bg-blue-800 text-white" : step.state !== "pending" ? "border-emerald-200 bg-emerald-50 text-emerald-800" : "border-slate-300 bg-white text-slate-700"}`}
          >
            {String(step.number).padStart(2, "0")}
            {step.state !== "pending" && <span aria-hidden="true"> ✓</span>}
          </button>
        ))}
      </div>
      <section className="rounded-xl border border-slate-200 bg-white p-5">
        <p className="text-xs font-bold uppercase tracking-wide text-slate-500">
          {definition.phase} · Step {number} of 20
        </p>
        <h3 className="mt-2 text-xl font-bold">{definition.title}</h3>
        <dl className="mt-4 grid grid-cols-2 gap-4 text-sm">
          <div>
            <dt className="text-slate-500">Planned</dt>
            <dd className="mt-1 font-semibold">{s.planned}</dd>
          </div>
          <div>
            <dt className="text-slate-500">Actual</dt>
            <dd className="mt-1 font-semibold">{s.actual || "Not recorded"}</dd>
          </div>
          <div className="col-span-2">
            <dt className="text-slate-500">Expected evidence</dt>
            <dd className="mt-1">{definition.evidence}</dd>
          </div>
        </dl>
        {s.state !== "pending" ? (
          <div className="mt-4 space-y-2">
            <Status value={s.state === "done" ? "Done" : "Not applicable"} />
            <p className="text-sm">{s.evidence || s.note}</p>
            {s.note && s.evidence && (
              <p className="text-sm text-slate-600">{s.note}</p>
            )}
            {current && (
              <button
                className={secondary}
                onClick={() => setNumber(current.number)}
              >
                Go to current step {current.number}
              </button>
            )}
          </div>
        ) : current?.number !== number ? (
          <p className="mt-4 rounded-lg bg-slate-100 p-3 text-sm">
            Upcoming step. Resolve step {current?.number} first.
          </p>
        ) : (
          <StepActionForm
            key={`${number}-${pkg.blocked}`}
            pkg={pkg}
            number={number}
            apply={(action, evidence, note) => {
              const next = updateExample(pkg, number, action, evidence, note);
              update(next);
              setNumber(currentStep(next)?.number ?? 20);
            }}
          />
        )}
      </section>
      <details className="rounded-xl border border-slate-200 bg-white p-4">
        <summary className="min-h-8 cursor-pointer font-semibold">
          Preview session history ({pkg.history.length})
        </summary>
        <ol className="mt-3 space-y-3 text-sm text-slate-600">
          {pkg.history.map((entry, i) => (
            <li key={i}>{entry}</li>
          ))}
        </ol>
      </details>
    </>
  );
}

function StepActionForm({
  pkg,
  number,
  apply,
}: {
  pkg: ExamplePackage;
  number: number;
  apply: (action: ExampleAction, evidence: string, note: string) => void;
}) {
  const [evidence, setEvidence] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState("");
  function run(action: ExampleAction) {
    try {
      apply(action, evidence, note);
      setError("");
    } catch (e) {
      setError((e as Error).message);
    }
  }
  function submit(e: FormEvent) {
    e.preventDefault();
    run(pkg.blocked ? "resume" : "complete");
  }
  return (
    <ValidatedForm onSubmit={submit} className="mt-5 space-y-4">
      {pkg.blocked ? (
        <p className="rounded-lg bg-rose-50 p-3 text-sm text-rose-800">
          Blocked: {pkg.blocked}
        </p>
      ) : (
        <ValidatedLabel className="block text-sm font-semibold" htmlFor="fms-evidence">
          Example evidence reference
          <ValidatedInput name="evidence"
            id="fms-evidence"
            className={input}
            maxLength={500}
            value={evidence}
            onChange={(e) => setEvidence(e.target.value)}
            placeholder="e.g. Example approved comparison CST-01"
          />
          <span className="mt-1 block text-xs font-normal text-slate-500">
            Text reference only; this preview does not upload or verify
            documents.
          </span>
        </ValidatedLabel>
      )}
      <ValidatedLabel className="block text-sm font-semibold" htmlFor="fms-note">
        {pkg.blocked ? "Resolution reason" : "Remarks / blocker reason"}
        <ValidatedTextarea name="note"
          id="fms-note"
          className={input}
          rows={3}
          maxLength={1000}
          value={note}
          onChange={(e) => setNote(e.target.value)}
          placeholder={
            pkg.blocked
              ? "What resolved the issue?"
              : "Explain an exception or add handoff notes"
          }
        />
      </ValidatedLabel>
      {error && (
        <p role="alert" className="text-sm text-red-700">
          {error}
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        <button type="submit" className={button}>
          {pkg.blocked ? "Simulate resolution" : "Simulate completion"}
        </button>
        {!pkg.blocked && (
          <>
            <button
              type="button"
              className={secondary}
              onClick={() => run("block")}
            >
              Simulate blocker
            </button>
            {number === 11 && (
              <button
                type="button"
                className={secondary}
                onClick={() => run("skip")}
              >
                Mark example not applicable
              </button>
            )}
          </>
        )}
      </div>
      <p className="text-xs text-slate-500">
        Recorded against the example clock, {EXAMPLE_TODAY}.
      </p>
    </ValidatedForm>
  );
}
