"use client";
import { ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";
import { useProjectUser } from "@/components/projects/project-permissions";
import { PurchaseSourcing } from "./purchase-sourcing";
import { useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { request, input, button, secondary } from "./purchase-data";
import { type Workspace, type WorkflowStep } from "./workflow-data";

export function PurchaseTaskWorkspace({
  project,
  identity,
  back,
}: {
  project: string;
  identity: string;
  back: () => void;
}) {
  const user = useProjectUser();
  const [sourcingDirty, setSourcingDirty] = useState(false);
  const params = useSearchParams(),
    router = useRouter();
  const requested = Number(params.get("purchaseStep") || 3),
    number = requested >= 1 && requested <= 20 ? requested : 3;
  const [workspace, setWorkspace] = useState<Workspace | null>(null),
    [error, setError] = useState(""),
    [reload, setReload] = useState(0),
    [taskDirty, setDirty] = useState(false);
  const dirty = taskDirty || sourcingDirty;
  const [context, setContext] = useState<"requirement" | "history">(
    "requirement",
  );
  const base = `/api/projects/${project}/purchase-workflow/${identity}`;
  useEffect(() => {
    let live = true;
    request<Workspace>(base)
      .then((w) => {
        if (live) {
          setWorkspace(w);
          setError("");
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [base, reload]);
  useEffect(() => {
    if (!dirty) return;
    function warn(e: BeforeUnloadEvent) {
      e.preventDefault();
      e.returnValue = "";
    }
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  // Guard links and parent workspace controls as well as the local step selector.
  useEffect(() => {
    if (!dirty) return;
    const node = document.querySelector("[data-purchase-editor]");
    function guard(e: MouseEvent) {
      const target = e.target as Element;
      if (node?.contains(target)) return;
      if (
        target.closest("a,button") &&
        !window.confirm("Discard unsaved purchase changes?")
      ) {
        e.preventDefault();
        e.stopPropagation();
      }
    }
    document.addEventListener("click", guard, true);
    return () => document.removeEventListener("click", guard, true);
  }, [dirty]);
  function leave(action: () => void) {
    if (dirty && !window.confirm("Discard unsaved purchase changes?")) return;
    setDirty(false);
    setSourcingDirty(false);
    action();
  }
  function select(n: number) {
    leave(() => {
      const q = new URLSearchParams(params.toString());
      q.set("purchaseStep", String(n));
      router.replace(`/dashboard?${q}`, { scroll: false });
    });
  }
  if (!workspace)
    return (
      <div>
        {error ? (
          <p role="alert">{error}</p>
        ) : (
          <p role="status">Loading purchase task workspace…</p>
        )}
        <button className={secondary} onClick={() => setReload((n) => n + 1)}>
          Retry
        </button>
        <button className={secondary} onClick={back}>
          Back to packages
        </button>
      </div>
    );
  const { package: pkg, source, members } = workspace,
    step = pkg.steps[number - 1];
  const name = (id: string | null) =>
    members.find((m) => m.id === id)?.label ??
    (id ? "Former project member" : "Unassigned");
  return (
    <section data-purchase-editor className="min-w-0 space-y-4">
      <header className="sticky top-0 z-10 rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold text-slate-500">
              {source.project_name} · {source.site || "Site not recorded"} ·{" "}
              {pkg.code}
            </p>
            <h3 className="mt-1 text-xl font-bold">{pkg.description}</h3>
            <p className="mt-1 text-sm text-slate-600">
              {source.quantity} {source.uom} · Required at site{" "}
              {source.required_date} · Workflow v{pkg.workflow_version}
            </p>
          </div>
          <button className={secondary} onClick={() => leave(back)}>
            Back to packages
          </button>
        </div>
        <p className="mt-3 text-sm">
          <strong>
            Step {number} · {step.team || "Team not assigned"}
          </strong>{" "}
          · Doer: {name(step.doer_id)} · Due:{" "}
          {step.planned_date || "Not planned"}
        </p>
        <p className="mt-2 text-xs text-slate-600">
          Saved version {pkg.version} ·{" "}
          {dirty ? "Unsaved changes" : "All changes saved"} · Sourcing decisions
          are tracked in steps 3–5; purchase orders remain a separate workflow.
        </p>
      </header>
      {(!source.project_available ||
        !source.eligible ||
        source.blockers.length > 0) && (
        <div
          role="alert"
          className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900"
        >
          <strong>Source controls block changes.</strong>
          <p>
            {!source.project_available
              ? "Project unavailable, closed, on hold or purchasing disabled. "
              : ""}
            {source.blockers.join("; ") ||
              (!source.eligible ? "Client make-policy review required." : "")}
          </p>
        </div>
      )}
      <div className="grid min-w-0 gap-4 xl:grid-cols-[240px_minmax(0,1fr)_300px]">
        <nav
          aria-label="Purchase workflow steps"
          className="max-h-72 overflow-y-auto rounded-xl border border-slate-200 bg-white p-2 xl:max-h-[70vh]"
        >
          {pkg.steps.map((s, i) => (
            <div key={s.number}>
              {(i === 0 || s.phase !== pkg.steps[i - 1].phase) && (
                <p className="px-2 pb-1 pt-4 text-xs font-bold text-slate-500">
                  {s.phase}
                </p>
              )}
              <button
                aria-current={number === s.number ? "step" : undefined}
                onClick={() => select(s.number)}
                className={`min-h-14 w-full rounded-lg px-3 py-2 text-left text-sm ${number === s.number ? "bg-blue-800 text-white" : "text-slate-700 hover:bg-slate-100"}`}
              >
                <span className="font-semibold">
                  {String(s.number).padStart(2, "0")} · {s.title}
                </span>
                <span className="mt-1 block text-xs">
                  {s.state.replaceAll("_", " ")} · {s.planned_date || "Undated"}
                </span>
              </button>
            </div>
          ))}
        </nav>
        <div className="min-w-0 rounded-xl border border-slate-200 bg-white p-5">
          <h4 className="text-xl font-bold">{step.title}</h4>
          <p className="mt-2 text-sm text-slate-600">
            Required outcome / evidence: {step.evidence_required}
          </p>
          {number >= 3 &&
            number <= 5 &&
            (user.permissions.includes("system:admin") ||
              user.permissions.includes("projects.purchase_sourcing.read")) && (
              <PurchaseSourcing
                key={`${identity}-${number}`}
                base={base}
                step={number}
                revision={pkg.version}
                dirty={setSourcingDirty}
              />
            )}
          {number <= 2 ? (
            <div className="mt-4 rounded-lg bg-emerald-50 p-4 text-sm text-emerald-900">
              Source linked to {source.indent_number}, released{" "}
              {new Date(source.release_at).toLocaleString()}. Manage requirement
              and indent decisions through the existing controls. No new
              approval was performed here.
            </div>
          ) : (
            <details
              open={number > 5 || !step.doer_id}
              className="mt-5 border-t pt-3"
            >
              <summary className="min-h-11 cursor-pointer font-semibold">
                Responsibility, preparation & blockers
              </summary>
              <TaskEditor
                key={`${identity}-${number}-${pkg.version}`}
                workspace={workspace}
                step={step}
                base={base}
                dirty={setDirty}
                updated={(w) => {
                  setDirty(false);
                  setWorkspace(w);
                }}
              />
            </details>
          )}
        </div>
        <aside className="min-w-0 rounded-xl border border-slate-200 bg-white p-4">
          <div className="mb-4 flex flex-wrap gap-2">
            <button
              aria-pressed={context === "requirement"}
              className={context === "requirement" ? button : secondary}
              onClick={() => setContext("requirement")}
            >
              Requirement
            </button>
            <button
              aria-pressed={context === "history"}
              className={context === "history" ? button : secondary}
              onClick={() => setContext("history")}
            >
              Activity
            </button>
          </div>
          {context === "requirement" ? (
            <dl className="space-y-4 text-sm">
              <div>
                <dt className="font-bold">
                  Full-kit specification / reference
                </dt>
                <dd className="mt-1 whitespace-pre-wrap break-words text-slate-600">
                  {source.full_kit}
                </dd>
              </div>
              <div>
                <dt className="font-bold">Released indent</dt>
                <dd>{source.indent_number}</dd>
              </div>
              <div>
                <dt className="font-bold">Source quantity</dt>
                <dd>
                  {source.quantity} {source.uom}
                </dd>
              </div>
              <div>
                <dt className="font-bold">Client make controls</dt>
                <dd>{source.eligible ? "Eligible" : "Review required"}</dd>
              </div>
              <div>
                <dt className="font-bold">Problem resolver</dt>
                <dd>{name(step.blocked_resolver_id || step.resolver_id)}</dd>
              </div>
              <div>
                <dt className="font-bold">Last step update</dt>
                <dd>
                  {step.updated_at
                    ? new Date(step.updated_at).toLocaleString()
                    : "Not recorded"}{" "}
                  · {name(step.updated_by_id)}
                </dd>
              </div>
              <div className="rounded-lg bg-slate-50 p-3 text-slate-600">
                RFQs, offers and LOI decisions are available in steps 3–5 with
                sourcing access. Technical approvals, purchase orders and
                payment remain separate.
              </div>
            </dl>
          ) : (
            <>
              <p className="mb-3 text-xs text-slate-500">
                Most recent 100 package events
              </p>
              <ol className="max-h-[60vh] space-y-4 overflow-y-auto">
                {workspace.history.map((e) => (
                  <li
                    key={e.id}
                    className="border-l-2 border-slate-200 pl-3 text-sm"
                  >
                    <p className="font-semibold">
                      {e.action} · {name(e.created_by_id)}
                    </p>
                    <p className="whitespace-pre-wrap break-words text-slate-600">
                      {e.reason}
                    </p>
                    <time className="text-xs text-slate-500">
                      {new Date(e.created_at).toLocaleString()}
                    </time>
                  </li>
                ))}
              </ol>
            </>
          )}
        </aside>
      </div>
    </section>
  );
}

function TaskEditor({
  workspace,
  step,
  base,
  dirty,
  updated,
}: {
  workspace: Workspace;
  step: WorkflowStep;
  base: string;
  dirty: (v: boolean) => void;
  updated: (w: Workspace) => void;
}) {
  const [notes, setNotes] = useState(step.notes),
    [reason, setReason] = useState(""),
    [deadline, setDeadline] = useState("");
  const [team, setTeam] = useState(step.team),
    [doer, setDoer] = useState(step.doer_id || ""),
    [resolver, setResolver] = useState(step.resolver_id || ""),
    [due, setDue] = useState(step.planned_date || "");
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const actions = workspace.actions[String(step.number)] || [];
  const assignmentChanged =
    team !== step.team ||
    doer !== (step.doer_id || "") ||
    resolver !== (step.resolver_id || "") ||
    due !== (step.planned_date || "");
  const notesChanged = notes !== step.notes;
  function change(set: (s: string) => void, value: string) {
    set(value);
    dirty(true);
  }
  async function send(path: string, payload: object) {
    setBusy(true);
    setError("");
    try {
      updated(
        await request<Workspace>(base + path, {
          version: workspace.package.version,
          reason,
          ...payload,
        }),
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="mt-5 space-y-5">
      {step.state === "blocked" && (
        <div className="rounded-lg bg-rose-50 p-3 text-sm text-rose-900">
          <strong>Blocked: {step.block_reason}</strong>
          <p>
            Resolver deadline: {step.resolution_due} · Opened{" "}
            {step.blocked_at ? new Date(step.blocked_at).toLocaleString() : "—"}
          </p>
        </div>
      )}
      {workspace.can_assign && (
        <details
          className="rounded-lg border border-slate-200 p-3"
          open={!step.doer_id}
        >
          <summary className="min-h-8 cursor-pointer font-semibold">
            Responsibility & planned date
          </summary>
          <fieldset
            disabled={busy || step.state === "blocked"}
            className="mt-3 grid gap-3 sm:grid-cols-2"
          >
            <ValidatedLabel className="text-sm">
              Responsible team
              <ValidatedInput name="team"
                className={input}
                maxLength={120}
                value={team}
                onChange={(e) => change(setTeam, e.target.value)}
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-sm">
              Planned date
              <ValidatedInput name="due"
                type="date"
                className={input}
                value={due}
                onChange={(e) => change(setDue, e.target.value)}
              />
            </ValidatedLabel>
            <ValidatedLabel className="text-sm">
              Task doer
              <ValidatedSelect name="doer"
                className={input}
                value={doer}
                onChange={(e) => change(setDoer, e.target.value)}
              >
                <option value="">Select doer</option>
                {workspace.members
                  .filter((m) =>
                    m.capabilities.includes("purchase_workflow.execute"),
                  )
                  .map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.label}
                    </option>
                  ))}
              </ValidatedSelect>
            </ValidatedLabel>
            <ValidatedLabel className="text-sm">
              Problem resolver
              <ValidatedSelect name="resolver"
                className={input}
                value={resolver}
                onChange={(e) => change(setResolver, e.target.value)}
              >
                <option value="">Select resolver</option>
                {workspace.members
                  .filter((m) =>
                    m.capabilities.includes("purchase_workflow.resolve"),
                  )
                  .map((m) => (
                    <option key={m.id} value={m.id}>
                      {m.label}
                    </option>
                  ))}
              </ValidatedSelect>
            </ValidatedLabel>
          </fieldset>
          <p className="mt-2 text-xs text-slate-600">
            Members need both global permission and project capability. Doer and
            resolver must differ. Add people in Project team.
          </p>
        </details>
      )}
      <ValidatedLabel className="block text-sm font-semibold">
        Preparation notes
        <ValidatedTextarea name="notes"
          readOnly={!actions.includes("save") || busy}
          className={input}
          rows={7}
          maxLength={4000}
          value={notes}
          onChange={(e) => change(setNotes, e.target.value)}
        />
      </ValidatedLabel>
      <p className="rounded-lg bg-slate-50 p-3 text-sm text-slate-600">
        Preparation may be recorded in advance. It does not unlock execution,
        approve a document, issue an RFQ or complete this step.
      </p>
      {(actions.length > 0 || workspace.can_assign) && (
        <>
          <ValidatedLabel className="block text-sm font-semibold">
            Reason / handoff note
            <ValidatedTextarea name="reason"
              disabled={busy}
              className={input}
              rows={2}
              maxLength={2000}
              value={reason}
              onChange={(e) => change(setReason, e.target.value)}
            />
          </ValidatedLabel>
          {actions.includes("block") && (
            <ValidatedLabel className="block text-sm">
              Resolver deadline (required to report a blocker)
              <ValidatedInput name="deadline"
                disabled={busy}
                type="date"
                className={input}
                value={deadline}
                onChange={(e) => change(setDeadline, e.target.value)}
              />
            </ValidatedLabel>
          )}
        </>
      )}
      {error && (
        <p
          role="alert"
          className="rounded-lg bg-red-50 p-3 text-sm text-red-800"
        >
          {error} Your unsaved input is retained.
        </p>
      )}
      {assignmentChanged && notesChanged && (
        <p className="text-sm text-amber-800">
          Save assignment and notes separately. Restore one set of changes
          before saving the other.
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        {workspace.can_assign && (
          <button
            disabled={
              busy ||
              !reason.trim() ||
              !team.trim() ||
              !doer ||
              !resolver ||
              !due ||
              step.state === "blocked" ||
              notesChanged
            }
            className={secondary}
            onClick={() =>
              send("/assignments", {
                rows: [
                  {
                    number: step.number,
                    team,
                    doer_id: doer,
                    resolver_id: resolver,
                    planned_date: due,
                  },
                ],
              })
            }
          >
            Save assignment
          </button>
        )}
        {actions
          .filter((a) => a !== "start")
          .map((action) => (
            <button
              key={action}
              disabled={
                busy ||
                !reason.trim() ||
                assignmentChanged ||
                (action === "block" && !deadline)
              }
              className={action === "save" ? button : secondary}
              onClick={() =>
                send(`/steps/${step.number}/commands`, {
                  action,
                  notes,
                  resolution_due: deadline || null,
                })
              }
            >
              {busy
                ? "Saving…"
                : action === "save"
                  ? "Save preparation"
                  : action === "block"
                    ? "Report blocker"
                    : "Resolve blocker"}
            </button>
          ))}
      </div>
      {!actions.length && (
        <p className="text-sm text-slate-600">
          Read-only task: assignment, permissions or source controls currently
          prevent preparation changes.
        </p>
      )}
    </div>
  );
}
