"use client";
import { IndentDispatchNotifications } from "./indent-dispatch-notifications";
import {
  ValidatedForm,
  ValidatedInput,
  ValidatedTextarea,
  ValidatedLabel,
  ValidatedSelect,
} from "@/components/ui/validated-controls";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import {
  Bars3Icon,
  CalendarDaysIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  ClockIcon,
  XMarkIcon,
} from "@heroicons/react/24/outline";
import {
  CompletionTick,
  PurchaseProgressPanel,
  usePurchaseProgress,
} from "./purchase-progress";
import { createRecordId } from "@/lib/record-id";
import {
  Fragment,
  useEffect,
  useRef,
  useState,
  type SyntheticEvent,
} from "react";
import { useProjectUser } from "@/components/projects/project-permissions";
import { purchaseStepAllowed } from "@/components/projects/workspace-permissions";
import { BOLineGrid } from "./bo-line-grid";
import { PurchaseIndentWorkspace } from "./purchase-indent-workspace";
import { QuotationApprovalGate } from "./quotation-approval-controls";
import { GroupQuotationWorkspace } from "./group-quotation-workspace";
import { PurchaseOrderWorkspace } from "./purchase-order-workspace";
import { PurchaseInspectionWorkspace } from "./purchase-inspection-workspace";
import { PurchaseDeliveryWorkspace } from "./purchase-delivery-workspace";

export type Item = {
  id: string;
  sku: string;
  description: string;
  uom: string;
};
export type Line = {
  id: string;
  item_id: string | null;
  quantity: string | null;
  required_date: string | null;
  reference: string;
  notes: string;
  item_snapshot: Partial<Item>;
  demand_id: string | null;
};
type BO = {
  name?: string;
  id: string;
  project_id: string;
  preparer_id: string;
  version: number;
  state: string;
  lines: Line[];
  completed_at: string | null;
  completed_by_id: string | null;
};
type Listing = { lists: BO[]; can_write: boolean; can_complete: boolean };
const newLine = (): Line => ({
  id: createRecordId(),
  item_id: null,
  quantity: null,
  required_date: null,
  reference: "",
  notes: "",
  item_snapshot: {},
  demand_id: null,
});
const steps = [
  "BO item list",
  "Purchase indent",
  "Receive vendor quotations",
  "Price comparison",
  "Vendor onboarding & LOI",
  "Technical documents",
  "Submit to project team",
  "Client technical approval",
  "Purchase order release",
  "Vendor PO acceptance",
  "Advance payment request",
  "Material inspection",
  "Inspection report / release",
  "Quality certificates",
  "Client dispatch clearance",
  "Material dispatch",
  "Receipt at site",
  "Warranty / guarantee",
  "Project engineer confirmation",
  "Accounts acceptance",
];
async function read<T>(url: string, body?: unknown): Promise<T> {
  const response = await fetch(url, {
    cache: "no-store",
    ...(body
      ? {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        }
      : {}),
  });
  const data = await response.json();
  if (!response.ok)
    throw Error(
      typeof data.detail === "string"
        ? data.detail
        : "Check the item values and try again. Your draft is retained.",
    );
  return data as T;
}
export function PurchaseBOWorkspace({
  project,
  projectName,
  scopeSummary,
  openPackages,
}: {
  project: string;
  projectName?: string;
  scopeSummary?: string;
  openPackages: () => void;
}) {
  const user = useProjectUser();
  const params = useSearchParams();
  const router = useRouter();
  const requestedBO = params.get("bo");
  const requestedStep = Number(params.get("purchaseStep") || Array.from({length:20},(_,i)=>i+1).find(number=>purchaseStepAllowed(user.permissions,number)) || 1);
  const [railOpen, setRailOpen] = useState(true);
  const [activity, setActivity] = useState<
    { action: string; reason: string; created_at: string }[]
  >([]);
  const [activityError, setActivityError] = useState("");
  const contextDialog = useRef<HTMLDialogElement>(null);
  const switchDialog = useRef<HTMLDialogElement>(null);
  const scheduleDialog = useRef<HTMLDialogElement>(null);
  const completionPanel = useRef<HTMLDivElement>(null);
  const completionNote = useRef<HTMLTextAreaElement>(null);
  const completionTrigger = useRef<HTMLButtonElement>(null);
  const [completionOpen, setCompletionOpen] = useState(false);
  const [scheduleViewed, setScheduleViewed] = useState(false);
  const step =
    Number.isInteger(requestedStep) && requestedStep >= 1 && requestedStep <= 20
      ? requestedStep
      : 1;
  function stepHref(number: number) {
    const query = new URLSearchParams(params.toString());
    query.set("purchaseStep", String(number));
    return `/dashboard?${query}`;
  }
  function setStep(number: number) {
    if (!purchaseStepAllowed(user.permissions,number)) return;
    completionPanel.current?.hidePopover();
    window.history.pushState(null, "", stepHref(number));
  }
  function guardNavigation(event: SyntheticEvent) {
    if (
      dirty &&
      !window.confirm("Leave this page and discard unsaved BO changes?")
    )
      event.preventDefault();
  }
  function showCompletion() {
    completionPanel.current?.showPopover();
    completionNote.current?.focus();
  }
  function closeCompletion() {
    completionPanel.current?.hidePopover();
    completionTrigger.current?.focus();
  }
  async function logout() {
    if (dirty && !window.confirm("Sign out and discard unsaved BO changes?"))
      return;
    const response = await fetch("/api/auth/logout", { method: "POST" });
    if (!response.ok) {
      setError("Could not sign out. Try again.");
      return;
    }
    router.replace("/login");
    router.refresh();
  }

  const [listing, setListing] = useState<Listing | null>(null),
    [items, setItems] = useState<Item[]>([]),
    [bo, setBO] = useState<BO | null>(null),
    [dirty, setDirty] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [reason, setReason] = useState(""),
    [revision, setRevision] = useState(0);
  const progress = usePurchaseProgress(
    project,
    `${step}:${bo?.id}:${bo?.version}`,
  );
  const boProgress = progress.data?.packages.find(
    (row) => row.bo_id === bo?.id,
  );
  const base = `/api/projects/${project}/purchase-bo`;
  const fresh = (): BO => ({
    id: createRecordId(),
    project_id: project,
    preparer_id: user.id,
    version: 0,
    state: "draft",
    lines: [newLine()],
    completed_at: null,
    completed_by_id: null,
  });
  useEffect(() => {
    let live = true;
    Promise.all([
      read<Listing>(base),
      read<Item[]>(`/api/projects/${project}/items/catalogue`),
    ])
      .then(([data, catalogue]) => {
        if (live) {
          setRailOpen(!window.matchMedia("(max-width: 599px)").matches);
          setListing(data);
          setItems(catalogue);
          setBO(
            data.lists.find((row) => row.id === requestedBO) ??
              data.lists[0] ?? {
                id: createRecordId(),
                project_id: project,
                preparer_id: user.id,
                version: 0,
                state: "draft",
                lines: [newLine()],
                completed_at: null,
                completed_by_id: null,
              },
          );
          setDirty(false);
          setError("");
        }
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [base, project, user.id, revision, requestedBO]);
  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => {
      e.preventDefault();
      e.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    const guard = (e: MouseEvent) => {
      const target = e.target as Element;
      if (target.closest("[data-bo-editor]")) return;
      if (
        target.closest("a,button") &&
        !window.confirm("Leave this page and discard unsaved BO changes?")
      ) {
        e.preventDefault();
        e.stopImmediatePropagation();
      }
    };
    document.addEventListener("click", guard, true);
    return () => {
      window.removeEventListener("beforeunload", warn);
      document.removeEventListener("click", guard, true);
    };
  }, [dirty]);
  const owner =
    !!bo &&
    (bo.preparer_id === user.id || user.permissions.includes("system:admin"));
  const editable = !!listing?.can_write && owner && bo?.state === "draft";
  async function submit(complete: boolean, reopen = false) {
    if (!bo) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await read<BO>(
        `${base}/${bo.id}${reopen ? "/reopen" : complete ? "/complete" : ""}`,
        complete || reopen
          ? { version: bo.version, reason: reason.trim() }
          : {
              version: bo.version,
              name: bo.name?.trim() || `BO ${bo.id.slice(0,8)}`,
              reason: reason.trim() || "BO list draft saved",
              lines: bo.lines
                .filter(
                  (line) =>
                    line.item_id ||
                    line.quantity ||
                    line.required_date ||
                    line.reference.trim() ||
                    line.notes.trim(),
                )
                .map(
                  ({
                    id,
                    item_id,
                    quantity,
                    required_date,
                    reference,
                    notes,
                  }) => ({
                    id,
                    item_id,
                    quantity: quantity || null,
                    required_date: required_date || null,
                    reference,
                    notes,
                  }),
                ),
            },
      );
      completionPanel.current?.hidePopover();
      setBO(result);
      setListing((current) =>
        current
          ? {
              ...current,
              lists: [
                result,
                ...current.lists.filter((row) => row.id !== result.id),
              ],
            }
          : current,
      );
      setDirty(false);
      setReason("");
      setNotice(
        reopen
          ? "BO list reopened."
          : complete
            ? "BO list completed."
            : "Draft saved.",
      );
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  const missing =
    bo?.lines.flatMap((line, index) => {
      const item =
        items.find((item) => item.id === line.item_id) ?? line.item_snapshot;
      const quantity = Number(line.quantity);
      return [
        ...(!line.item_id
          ? [`Item ${index + 1}: select an Item Bank record`]
          : []),
        ...(!Number.isFinite(quantity) || quantity <= 0
          ? [`Item ${index + 1}: enter a positive quantity`]
          : []),
        ...(["nos", "each", "pcs", "pieces", "number", "numbers"].includes(
          (item.uom ?? "").toLowerCase(),
        ) && !Number.isInteger(quantity)
          ? [`Item ${index + 1}: quantity must be a whole number`]
          : []),
        ...(!line.required_date
          ? [`Item ${index + 1}: set the required-at-site date`]
          : []),
      ];
    }) ?? [];
  const phases = [
    { code: "WO2I", name: "Work order to indent", start: 1, end: 2 },
    { code: "I2LOI", name: "Indent to LOI", start: 3, end: 5 },
    { code: "LOI2PO", name: "LOI to purchase order", start: 6, end: 9 },
    {
      code: "PO2INS",
      name: "Purchase order to inspection",
      start: 10,
      end: 14,
    },
    { code: "INS2D", name: "Inspection to delivery", start: 15, end: 20 },
  ];
  const noSavedLists = !!listing && !listing.can_write && !listing.lists.length;
  const status = noSavedLists
    ? "No saved BO lists"
    : busy
      ? "Saving…"
      : dirty
        ? "Unsaved changes"
        : bo?.version
          ? `Saved · revision ${bo.version}`
          : "Not yet saved";
  const title =
    scopeSummary?.match(/Purchase starting point:\s*([^.]*)/)?.[1] ||
    "Purchase requirement";
  async function showActivity() {
    contextDialog.current?.showModal();
    if (bo?.version) {
      setActivityError("");
      try {
        setActivity(await read(`${base}/${bo.id}/history`));
      } catch (e) {
        setActivityError((e as Error).message);
      }
    }
  }
  function switchBO(id: string) {
    if (
      dirty &&
      !window.confirm("Discard unsaved changes and open another BO list?")
    )
      return;
    const row = listing?.lists.find((row) => row.id === id);
    if (row) {
      setBO(row);
      setDirty(false);
      setStep(1);
      setError("");
      setNotice("");
      setReason("");
      switchDialog.current?.close();
    }
  }
  return (
    <section data-bo-editor className="bo-workspace">
      <div className="bo-appbar">
        <Link href="/dashboard" className="bo-brand" onClick={guardNavigation}>
          VTPL <span>CORE</span>
        </Link>
        <nav aria-label="Breadcrumb" className="bo-breadcrumb">
          <Link
            href="/dashboard?screen=purchase-dashboard"
            onClick={guardNavigation}
          >
            Purchase FMS
          </Link>
          <ChevronRightIcon aria-hidden="true" />
          <Link
            href={`/dashboard?screen=projects&project=${project}&tab=overview`}
            onClick={guardNavigation}
          >
            {projectName || "Project"}
          </Link>
        </nav>
        <IndentDispatchNotifications />
        <details className="bo-account">
          <summary aria-label={`Account: ${user.username}`}>
            <span className="bo-avatar" aria-hidden="true">
              {user.username.slice(0, 1).toUpperCase()}
            </span>
            <span className="bo-account-name">{user.username}</span>
            <ChevronDownIcon aria-hidden="true" />
          </summary>
          <div className="bo-account-menu">
            <strong>{user.username}</strong>
            <button type="button" onClick={() => void logout()}>
              Sign out
            </button>
          </div>
        </details>
      </div>
      <div className={`bo-layout ${railOpen ? "" : "rail-hidden"}`}>
        {railOpen && (
          <nav
            id="bo-steps"
            className="bo-rail"
            aria-label="Purchase workflow steps"
          >
            <div className="bo-rail-title">
              <span>Purchase workflow</span>
              <button
                aria-label="Close workflow"
                onClick={() => setRailOpen(false)}
              >
                <XMarkIcon aria-hidden="true" />
              </button>
            </div>
            {phases.map((group) => (
              <div key={group.code}>
                <p className="bo-phase-title">{group.name}</p>
                {steps.slice(group.start - 1, group.end).map((label, i) => {
                  const number = group.start + i;
                  if (!purchaseStepAllowed(user.permissions,number)) return null;
                  const saved = boProgress?.steps.find(
                    (row) => row.number === number,
                  );
                  return (
                    <a
                      href={stepHref(number)}
                      key={number}
                      className="bo-step"
                      data-state={saved?.state || "unknown"}
                      title={saved?.details || saved?.state.replaceAll("_", " ")}
                      aria-current={step === number ? "step" : undefined}
                      onClick={(event) => {
                        event.preventDefault();
                        setStep(number);
                        if (window.matchMedia("(max-width: 599px)").matches)
                          setRailOpen(false);
                      }}
                    >
                      <span className="bo-step-no">
                        {saved?.state === "completed" ? (
                          <CompletionTick />
                        ) : (
                          number
                        )}
                      </span>
                      <span>
                        {label}
                        {saved && <small className="bo-step-state">{saved.state.replaceAll("_", " ")}</small>}
                        {saved &&
                        [
                          "delayed",
                          "blocked",
                          "due_soon",
                          "attention",
                        ].includes(saved.signal) ? (
                          <small className={`bo-step-signal ${saved.signal}`}>
                            {saved.signal === "delayed"
                              ? `${saved.days_overdue}d overdue`
                              : saved.signal === "blocked"
                                ? "Blocked"
                                : saved.signal === "due_soon"
                                  ? "Due soon"
                                  : "Review dates"}
                          </small>
                        ) : null}
                      </span>
                    </a>
                  );
                })}
              </div>
            ))}
          </nav>
        )}
        <div className="bo-work">
          <header className="bo-record">
            <div className="bo-record-title">
              <button
                type="button"
                className="bo-icon-button"
                aria-label={railOpen ? "Hide workflow" : "Show workflow"}
                aria-expanded={railOpen}
                aria-controls="bo-steps"
                onClick={() => setRailOpen(!railOpen)}
              >
                <Bars3Icon aria-hidden="true" />
              </button>
              <h1>{steps[step - 1]}</h1>
              {step === 1 && bo ? (
                <span
                  className={`bo-pill ${bo.state === "completed" ? "complete" : "amber"}`}
                >
                  {bo.state === "completed" ? "Completed" : "Draft"}
                </span>
              ) : null}
              {listing && (listing.lists.length > 1 || listing.can_write) ? (
                <button
                  type="button"
                  className="bo-list-selector"
                  aria-label="Choose BO list"
                  aria-haspopup="dialog"
                  onClick={() => switchDialog.current?.showModal()}
                >
                  {bo?.name || title}
                  <ChevronDownIcon aria-hidden="true" />
                </button>
              ) : (
                <span className="bo-requirement">{title}</span>
              )}
            </div>
            <div className="bo-record-actions">
              {boProgress && <span className="bo-pill complete" aria-label="Saved BO progress" title="Weighted progress from completed steps">{boProgress.progress_percent.toFixed(1)}% · {boProgress.steps.filter(s=>s.state==="completed"||s.state==="not_applicable").length}/20 steps complete</span>}
              <button
                type="button"
                className="bo-quiet-button"
                onClick={() => {
                  setScheduleViewed(true);
                  scheduleDialog.current?.showModal();
                }}
              >
                <CalendarDaysIcon aria-hidden="true" />
                <span>Schedule</span>
              </button>
              <button
                type="button"
                className="bo-quiet-button"
                onClick={() => void showActivity()}
              >
                <ClockIcon aria-hidden="true" />
                <span>History</span>
              </button>
            </div>
          </header>
          {boProgress?.steps.find(saved=>saved.number===step)?.details && <p aria-label="Current purchase step progress" className="px-5 py-3 text-sm text-slate-700">{boProgress.steps.find(saved=>saved.number===step)?.details}</p>}
          {progress.error && <p role="alert" className="p-3 text-red-800">{progress.error}</p>}
          {!purchaseStepAllowed(user.permissions,step) ? <p role="status" className="p-6">You do not have permission to open this purchase section. Choose an authorised step from the navigation.</p> : step === 2 ? (
            <PurchaseIndentWorkspace
              project={project}
              projectName={projectName}
              lines={bo?.lines || []}
              key={`${bo?.id}-${bo?.version}`}
              boId={bo?.id}
              boVersion={bo?.version}
              onBackToBO={() => setStep(1)}
              onProceedToSourcing={() => setStep(3)}
            />
          ) : step >= 3 && step <= 5 ? (
            <GroupQuotationWorkspace
              project={project}
              boId={bo?.id}
              initialStep={step}
              onNavigateStep={(s) => setStep(s)}
            />
          ) : step >= 6 && step <= 9 ? (
            <QuotationApprovalGate key={`${project}:${bo?.id}:${step}`} project={project} boId={bo?.id} step={step}><PurchaseOrderWorkspace
              project={project}
              projectName={projectName}
              lines={bo?.lines || []}
              boId={bo?.id}
              initialStep={step}
              onNavigateStep={(s) => setStep(s)}
            /></QuotationApprovalGate>
          ) : step >= 10 && step <= 14 ? (
            <QuotationApprovalGate key={`${project}:${bo?.id}:${step}`} project={project} boId={bo?.id} step={step}><PurchaseInspectionWorkspace
              project={project}
              projectName={projectName}
              lines={
                bo?.lines.map((l) => ({
                  id: l.id,
                  item_id: l.item_id || "",
                  item_code:
                    l.item_snapshot.sku || l.reference || "POLE-OCT-09M",
                  item_name:
                    l.item_snapshot.description ||
                    l.notes ||
                    "Hot Dip Octagonal GI Pole 9M",
                  quantity: Number(l.quantity) || 1,
                  unit_of_measure: l.item_snapshot.uom || "Unit",
                })) || []
              }
              boId={bo?.id}
              initialStep={step}
              onNavigateStep={(s) => setStep(s)}
            /></QuotationApprovalGate>
          ) : step >= 15 && step <= 20 ? (
            <QuotationApprovalGate key={`${project}:${bo?.id}:${step}`} project={project} boId={bo?.id} step={step}><PurchaseDeliveryWorkspace
              project={project}
              projectName={projectName}
              lines={
                bo?.lines.map((l) => ({
                  id: l.id,
                  item_id: l.item_id || "",
                  item_code:
                    l.item_snapshot.sku || l.reference || "POLE-OCT-09M",
                  item_name:
                    l.item_snapshot.description ||
                    l.notes ||
                    "Hot Dip Octagonal GI Pole 9M",
                  quantity: Number(l.quantity) || 1,
                  unit_of_measure: l.item_snapshot.uom || "Unit",
                })) || []
              }
              boId={bo?.id}
              initialStep={step}
              onNavigateStep={(s) => setStep(s)}
            /></QuotationApprovalGate>
          ) : (
            <>
              <div className="bo-scroll">
                {error && (
                  <div role="alert" className="bo-message error">
                    {error}
                    {dirty ? " Your current input is retained." : ""}{" "}
                    <button
                      onClick={() => {
                        if (
                          !dirty ||
                          window.confirm(
                            "Reload saved records and discard your unsaved draft?",
                          )
                        )
                          setRevision((n) => n + 1);
                      }}
                    >
                      Reload saved records
                    </button>
                  </div>
                )}
                {notice && (
                  <p role="status" className="bo-message complete">
                    {notice}
                  </p>
                )}
                {!bo && !error && <p>Loading saved BO lists…</p>}
                {noSavedLists && (
                  <section className="bo-card">
                    <h3>No BO lists saved yet</h3>
                    <p>
                      The purchase preparer needs to save a BO list for this
                      project. Saved lists will appear here for your review.
                    </p>
                  </section>
                )}
                {bo && !noSavedLists && (
                  <>
                    {step === 1 ? (
                      <section className="bo-card">
                        {!owner && (
                          <p>
                            Only the list’s preparer can edit or complete it.
                          </p>
                        )}
                        <BOLineGrid
                          key={`${bo.id}-${bo.version}`}
                          lines={bo.lines}
                          items={items}
                          editable={editable}
                          busy={busy}
                          reserved={
                            new Set(
                              (listing?.lists ?? [])
                                .filter((row) => row.id !== bo.id)
                                .flatMap((row) =>
                                  row.lines.flatMap((line) =>
                                    line.item_id ? [line.item_id] : [],
                                  ),
                                ),
                            )
                          }
                          onChange={(lines) => {
                            setBO({ ...bo, lines });
                            setDirty(true);
                            setNotice("");
                          }}
                        />
                      </section>
                    ) : (
                      <section className="bo-card">
                        <h3>{steps[step - 1]}</h3>
                        <p>
                          {step <= 5
                            ? "RFQ, comparison and LOI are available in a purchase package after independent indent review and release."
                            : "This specialist step is not yet connected for execution in this workspace."}
                        </p>
                        <button
                          onClick={() => {
                            if (
                              !dirty ||
                              window.confirm(
                                "Leave and discard unsaved BO changes?",
                              )
                            )
                              openPackages();
                          }}
                        >
                          Open purchase packages
                        </button>
                      </section>
                    )}
                  </>
                )}
              </div>
              <footer className="bo-actionbar">
                <div>
                  <span role="status">{status}</span>
                  {step === 1 && missing.length > 0 ? (
                    <details className="bo-missing">
                      <summary>{missing.length} fields to complete</summary>
                      <ul>
                        {missing.map((message) => (
                          <li key={message}>{message}</li>
                        ))}
                      </ul>
                    </details>
                  ) : null}
                </div>
                <div>
                  {step === 1 && editable && (
                    <button
                      className="primary"
                      disabled={busy}
                      onClick={() => void submit(false)}
                    >
                      Save draft
                    </button>
                  )}
                  {step === 1 &&
                    bo?.state === "draft" &&
                    owner &&
                    listing?.can_complete && (
                      <button
                        ref={completionTrigger}
                        aria-haspopup="dialog"
                        aria-expanded={completionOpen}
                        aria-controls="bo-completion-panel"
                        disabled={
                          busy ||
                          dirty ||
                          !bo.version ||
                          !bo.lines.length ||
                          !!missing.length
                        }
                        title={
                          dirty
                            ? "Save your draft first"
                            : missing.length
                              ? missing.join("; ")
                              : undefined
                        }
                        onClick={showCompletion}
                      >
                        Complete list <ChevronDownIcon aria-hidden="true" />
                      </button>
                    )}
                  {step === 1 &&
                    bo?.state === "completed" &&
                    owner &&
                    listing?.can_write &&
                    listing?.can_complete && (
                      <button
                        ref={completionTrigger}
                        aria-haspopup="dialog"
                        aria-expanded={completionOpen}
                        aria-controls="bo-completion-panel"
                        disabled={busy}
                        onClick={showCompletion}
                      >
                        Reopen BO list
                      </button>
                    )}
                  {step === 1 && bo?.state === "completed" && purchaseStepAllowed(user.permissions,2) && (
                    <button className="primary" onClick={() => setStep(2)}>
                      Continue to indent
                    </button>
                  )}
                </div>
              </footer>
            </>
          )}
        </div>
      </div>
      {step === 1 && owner && listing?.can_complete && bo ? (
        <div
          ref={completionPanel}
          id="bo-completion-panel"
          popover="auto"
          role="dialog"
          aria-labelledby="bo-completion-title"
          className="bo-completion-panel"
          onToggle={(event) => setCompletionOpen(event.newState === "open")}
        >
          <div className="bo-dialog-heading">
            <h2 id="bo-completion-title">
              {bo.state === "completed" ? "Reopen BO list" : "Complete BO list"}
            </h2>
            <button
              type="button"
              aria-label="Close completion"
              disabled={busy}
              onClick={closeCompletion}
            >
              <XMarkIcon aria-hidden="true" />
            </button>
          </div>
          <ValidatedForm
            onSubmit={(event) => {
              event.preventDefault();
              void submit(bo.state !== "completed", bo.state === "completed");
            }}
          >
            <ValidatedLabel>
              {bo.state === "completed" ? "Change reason" : "Completion note"}
              <ValidatedTextarea
                ref={completionNote}
                name="reason"
                required
                aria-label={
                  bo.state === "completed" ? "Change reason" : "Completion note"
                }
                maxLength={2000}
                rows={3}
                disabled={busy}
                value={reason}
                onChange={(event) => setReason(event.target.value)}
              />
            </ValidatedLabel>
            {error ? (
              <p role="alert" className="bo-message error">
                {error}
              </p>
            ) : null}
            <div className="bo-completion-actions">
              <button
                type="submit"
                className="primary"
                disabled={
                  busy ||
                  !reason.trim() ||
                  (bo.state !== "completed" &&
                    (dirty ||
                      !bo.version ||
                      !bo.lines.length ||
                      !!missing.length))
                }
              >
                {busy
                  ? "Saving…"
                  : bo.state === "completed"
                    ? "Reopen"
                    : "Complete"}
                <CheckIcon aria-hidden="true" />
              </button>
            </div>
          </ValidatedForm>
        </div>
      ) : null}
      <dialog
        ref={scheduleDialog}
        className="bo-dialog bo-schedule"
        aria-labelledby="bo-schedule-title"
      >
        <div className="bo-dialog-heading">
          <h2 id="bo-schedule-title">Purchase schedule</h2>
          <button
            aria-label="Close schedule"
            onClick={() => scheduleDialog.current?.close()}
          >
            <XMarkIcon aria-hidden="true" />
          </button>
        </div>
        {scheduleViewed ? <PurchaseProgressPanel project={project} /> : null}
      </dialog>
      <dialog
        ref={switchDialog}
        className="vtpl-modal bo-dialog"
        aria-labelledby="bo-switch-title"
      >
        <div className="bo-dialog-heading">
          <h2 id="bo-switch-title">BO lists</h2>
          <button
            aria-label="Close BO selector"
            onClick={() => switchDialog.current?.close()}
          >
            ×
          </button>
        </div>
        {bo && editable ? <ValidatedLabel>BO name<ValidatedInput name="bo_name" aria-label="BO name" required maxLength={128} value={bo.name || ""} placeholder="e.g. TEST-JATIN BO-A" onChange={event => { setBO({...bo,name:event.target.value}); setDirty(true); }} /></ValidatedLabel> : null}
        {bo && editable ? <button type="button" disabled={busy || !bo.name?.trim()} onClick={async () => { await submit(false); }} className="primary">Save BO name</button> : null}
        <ValidatedLabel>
          BO list
          <ValidatedSelect
            aria-label="BO list"
            value={bo?.id ?? ""}
            disabled={busy}
            onChange={(e) => switchBO(e.target.value)}
          >
            {bo && !listing?.lists.some((row) => row.id === bo.id) && (
              <option value={bo.id}>New unsaved BO list</option>
            )}
            {listing?.lists.map((row) => (
              <option key={row.id} value={row.id}>
                {row.name || `BO ${row.id.slice(0, 8)}`} · {row.state} · revision {row.version}
              </option>
            ))}
          </ValidatedSelect>
        </ValidatedLabel>
        {listing?.can_write && (
          <button
            disabled={busy}
            onClick={() => {
              if (
                dirty &&
                !window.confirm(
                  "Discard unsaved changes and start a new BO list?",
                )
              )
                return;
              setBO(fresh());
              setDirty(false);
              setStep(1);
              setError("");
              setNotice("");
              setReason("");
              switchDialog.current?.close();
            }}
          >
            New BO list
          </button>
        )}
      </dialog>
      <dialog
        ref={contextDialog}
        className="bo-dialog bo-context"
        aria-labelledby="bo-context-title"
      >
        <div className="bo-dialog-heading">
          <h2 id="bo-context-title">Activity history</h2>
          <button
            aria-label="Close context"
            onClick={() => contextDialog.current?.close()}
          >
            ×
          </button>
        </div>
        <div className="bo-context-body">
          {activityError && <p role="alert">{activityError}</p>}
          {!bo?.version ? (
            <p>Save the BO list to start its activity history.</p>
          ) : (
            activity.map((event, i) => (
              <article key={i}>
                <strong>
                  {event.action === "complete_bo_list"
                    ? "BO list completed"
                    : event.action === "reopen_bo_list"
                      ? "BO list reopened"
                      : "Draft saved"}
                </strong>
                <p>{event.reason}</p>
                <small>{new Date(event.created_at).toLocaleString()}</small>
              </article>
            ))
          )}
        </div>
      </dialog>
    </section>
  );
}
