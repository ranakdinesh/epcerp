"use client";

import { useEffect, useState } from "react";
import {
  CheckCircleIcon,
  ShieldCheckIcon,
  ExclamationTriangleIcon,
  ArrowRightIcon,
  ArrowLeftIcon,
  UserIcon,
  CurrencyRupeeIcon,
  CalendarDaysIcon,
  DocumentTextIcon,
} from "@heroicons/react/24/outline";
import {
  ValidatedForm,
  ValidatedInput,
  ValidatedLabel,
  ValidatedSelect,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";
import { TenderEligibilitySection } from "./tender-eligibility-verdict";

type Candidate = { id: string; display_name: string; email: string };

type BiddingAuthorization = {
  id: string;
  draft_id: string;
  decision: "authorized" | "rejected" | "conditional";
  manager_id: string | null;
  manager_name?: string;
  estimated_value: number | null;
  emd_amount: number | null;
  emd_exemption_claimed: boolean;
  tender_fee: number | null;
  tender_fee_exemption_claimed: boolean;
  due_date: string | null;
  approval_notes: string;
  authorized_by_name?: string;
  authorized_at: string;
  version: number;
};

type FlowStatus = {
  draft_id: string;
  completed_steps: number[];
  next_step: string;
  status_label: string;
  has_documents: boolean;
  tsc_completed: boolean;
  tsc_decision?: "pending" | "accepted" | "rejected" | "overridden";
  bidding_authorized: boolean;
  bidding_decision?: string;
};

const primaryBtn =
  "inline-flex min-h-10 items-center justify-center gap-2 rounded-xl bg-blue-700 px-5 text-xs font-bold text-white shadow-sm transition hover:bg-blue-800 disabled:opacity-50";
const secondaryBtn =
  "inline-flex min-h-10 items-center justify-center gap-1.5 rounded-xl border border-slate-300 bg-white px-3.5 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200";
const inputClass =
  "mt-1 min-h-10 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-xs text-slate-900 transition focus:border-blue-600 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white";

export function TenderBiddingApproval({
  draftId,
  canWrite,
  onNavigateStep,
  onDone,
}: {
  draftId: string;
  canWrite: boolean;
  onNavigateStep: (step: string) => void;
  onDone?: () => void;
}) {
  const [flow, setFlow] = useState<FlowStatus | null>(null);
  const [bidding, setBidding] = useState<BiddingAuthorization | null>(null);
  const [managers, setManagers] = useState<Candidate[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  // Form State
  const [decision, setDecision] = useState<"authorized" | "rejected" | "conditional">("authorized");
  const [managerId, setManagerId] = useState("");
  const [estimatedValue, setEstimatedValue] = useState("");
  const [emdAmount, setEmdAmount] = useState("");
  const [emdExemption, setEmdExemption] = useState(false);
  const [tenderFee, setTenderFee] = useState("");
  const [feeExemption, setFeeExemption] = useState(false);
  const [dueDate, setDueDate] = useState("");
  const [notes, setNotes] = useState("");
  const [tscJobId, setTscJobId] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    const controller = new AbortController();

    async function load() {
      try {
        setLoading(true);
        const [flowRes, biddingRes, managersRes, tscRes] = await Promise.all([
          fetch(`/api/tender-fms/drafts/${encodeURIComponent(draftId)}/flow-status`, {
            cache: "no-store",
            signal: controller.signal,
          }).then((r) => (r.ok ? r.json() : null)),
          fetch(`/api/tender-fms/drafts/${encodeURIComponent(draftId)}/bidding-approval`, {
            cache: "no-store",
            signal: controller.signal,
          }).then((r) => (r.ok ? r.json() : null)),
          fetch(`/api/tender-fms/assignment-options?page=1&page_size=100`, {
            cache: "no-store",
            signal: controller.signal,
          }).then((r) => (r.ok ? r.json() : { items: [] })),
          fetch(`/api/tender-fms/drafts/${encodeURIComponent(draftId)}/tsc`, {
            cache: "no-store",
            signal: controller.signal,
          }).then((r) => (r.ok ? r.json() : { items: [] })),
        ]);

        if (!active) return;
        setFlow(flowRes);
        setBidding(biddingRes);
        setManagers(managersRes?.items ?? []);
        if (tscRes?.items?.length) {
          setTscJobId(tscRes.items[0].id);
        }

        if (biddingRes) {
          setDecision(biddingRes.decision);
          setManagerId(biddingRes.manager_id || "");
          setEstimatedValue(biddingRes.estimated_value ? String(biddingRes.estimated_value) : "");
          setEmdAmount(biddingRes.emd_amount ? String(biddingRes.emd_amount) : "");
          setEmdExemption(Boolean(biddingRes.emd_exemption_claimed));
          setTenderFee(biddingRes.tender_fee ? String(biddingRes.tender_fee) : "");
          setFeeExemption(Boolean(biddingRes.tender_fee_exemption_claimed));
          setDueDate(biddingRes.due_date || "");
          setNotes(biddingRes.approval_notes || "");
        }
      } catch (e) {
        if (!controller.signal.aborted) {
          setError(e instanceof Error ? e.message : "Failed to load bidding approval details");
        }
      } finally {
        if (active) setLoading(false);
      }
    }

    void load();
    return () => {
      active = false;
      controller.abort();
    };
  }, [draftId]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!canWrite || saving) return;

    setSaving(true);
    setError("");
    setNotice("");

    try {
      const payload = {
        decision,
        manager_id: managerId || null,
        estimated_value: estimatedValue ? parseFloat(estimatedValue) : null,
        emd_amount: emdExemption ? 0 : emdAmount ? parseFloat(emdAmount) : null,
        emd_exemption_claimed: emdExemption,
        tender_fee: feeExemption ? 0 : tenderFee ? parseFloat(tenderFee) : null,
        tender_fee_exemption_claimed: feeExemption,
        due_date: dueDate || null,
        approval_notes: notes.trim(),
      };

      const res = await fetch(`/api/tender-fms/drafts/${encodeURIComponent(draftId)}/bidding-approval`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.detail || "Failed to record bidding authorization");
      }

      const saved: BiddingAuthorization = await res.json();
      setBidding(saved);
      setNotice(
        saved.decision === "authorized"
          ? "Formal Bidding Authorization granted. Tender handed off to Bid Preparation & Costing."
          : saved.decision === "conditional"
          ? "Conditional Bidding recorded. Pre-bid queries & clarifications required before final submission."
          : "Tender marked as Do Not Bid (No-Go)."
      );

      // Trigger tender progress event to update left menu checkmarks
      window.dispatchEvent(new Event("tender-progress-saved"));

      // Refresh flow
      const updatedFlow: FlowStatus = await fetch(
        `/api/tender-fms/drafts/${encodeURIComponent(draftId)}/flow-status`,
        { cache: "no-store" }
      ).then((r) => r.json());
      setFlow(updatedFlow);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save authorization");
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="flex min-h-64 items-center justify-center p-8 text-slate-500">
        <span className="text-sm font-medium">Loading Step 3: Bidding Approval…</span>
      </div>
    );
  }

  const isAuthorized = bidding && (bidding.decision === "authorized" || bidding.decision === "conditional");

  return (
    <div className="space-y-6">
      {/* Step Header */}
      <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
                Tender Selection · Step 3 of 3
              </span>
              <span
                className={`inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-xs font-semibold ${
                  isAuthorized
                    ? "bg-emerald-50 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                    : "bg-amber-50 text-amber-800 dark:bg-amber-950 dark:text-amber-300"
                }`}
              >
                {isAuthorized ? "Bidding Authorized" : "Pending Executive Sign-off"}
              </span>
            </div>
            <h2 className="mt-1 text-xl font-bold text-slate-900 dark:text-white">
              Approval for Bidding the Tender (TS-02)
            </h2>
            <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
              Executive authorization, Tender Manager assignment, and allocation of bidding budgets.
            </p>
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              className={secondaryBtn}
              onClick={() => onNavigateStep("TS-TSC")}
              title="Return to Step 2"
            >
              <ArrowLeftIcon className="h-4 w-4" />
              Review Step 2: TSC
            </button>
          </div>
        </div>
      </div>

      {/* Preceding Steps Status Cards */}
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="flex h-6 w-6 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                ✓
              </span>
              <h3 className="text-sm font-bold text-slate-900 dark:text-white">
                Step 1: Document Intake
              </h3>
            </div>
            <span className="text-[11px] font-semibold text-emerald-700 dark:text-emerald-400">
              Verified
            </span>
          </div>
          <p className="mt-2 text-xs text-slate-600 dark:text-slate-400">
            Source documents and tender specifications verified and indexed in cloud document storage.
          </p>
        </div>

        <div className="rounded-2xl border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="flex h-6 w-6 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                ✓
              </span>
              <h3 className="text-sm font-bold text-slate-900 dark:text-white">
                Step 2: TSC & Eligibility Verdict
              </h3>
            </div>
            <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-bold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
              {flow?.tsc_decision ? flow.tsc_decision.toUpperCase() : "VERIFIED"}
            </span>
          </div>
          <p className="mt-2 text-xs text-slate-600 dark:text-slate-400">
            Technical Scrutiny Checklist evaluated against Company Profile with human sign-off recorded.
          </p>
        </div>
      </div>

      {notice && (
        <div className="flex items-center gap-2 rounded-2xl border border-emerald-200 bg-emerald-50/80 p-4 text-xs font-semibold text-emerald-900 dark:border-emerald-900 dark:bg-emerald-950/60 dark:text-emerald-200">
          <CheckCircleIcon className="h-5 w-5 shrink-0 text-emerald-600" />
          <span>{notice}</span>
        </div>
      )}

      {error && (
        <div className="flex items-center gap-2 rounded-2xl border border-red-200 bg-red-50/80 p-4 text-xs font-semibold text-red-900 dark:border-red-900 dark:bg-red-950/60 dark:text-red-200">
          <ExclamationTriangleIcon className="h-5 w-5 shrink-0 text-red-600" />
          <span>{error}</span>
        </div>
      )}

      {/* Authorized Summary Card (If already signed off) */}
      {bidding && (
        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center gap-2 border-b border-slate-100 pb-3 dark:border-slate-800">
            <ShieldCheckIcon className="h-5 w-5 text-emerald-600" />
            <h3 className="text-sm font-bold text-slate-900 dark:text-white">
              Recorded Bidding Authorization
            </h3>
            <span
              className={`ml-auto rounded-full px-2.5 py-0.5 text-xs font-bold uppercase ${
                bidding.decision === "authorized"
                  ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                  : bidding.decision === "conditional"
                  ? "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300"
                  : "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300"
              }`}
            >
              {bidding.decision.replace("_", " ")}
            </span>
          </div>

          <div className="mt-4 grid gap-4 sm:grid-cols-3 text-xs">
            <div>
              <span className="text-slate-500 dark:text-slate-400">Accountable Manager</span>
              <p className="mt-0.5 font-bold text-slate-900 dark:text-white">
                {bidding.manager_name || "Unassigned"}
              </p>
            </div>
            <div>
              <span className="text-slate-500 dark:text-slate-400">Estimated Tender Value</span>
              <p className="mt-0.5 font-bold text-slate-900 dark:text-white">
                {bidding.estimated_value ? `₹ ${bidding.estimated_value.toLocaleString("en-IN")}` : "Not specified"}
              </p>
            </div>
            <div>
              <span className="text-slate-500 dark:text-slate-400">EMD Requirement</span>
              <p className="mt-0.5 font-bold text-slate-900 dark:text-white">
                {bidding.emd_exemption_claimed
                  ? "MSME / Exemption Claimed"
                  : bidding.emd_amount
                  ? `₹ ${bidding.emd_amount.toLocaleString("en-IN")}`
                  : "None"}
              </p>
            </div>
            <div>
              <span className="text-slate-500 dark:text-slate-400">Tender Fee</span>
              <p className="mt-0.5 font-bold text-slate-900 dark:text-white">
                {bidding.tender_fee_exemption_claimed
                  ? "Exemption Claimed"
                  : bidding.tender_fee
                  ? `₹ ${bidding.tender_fee.toLocaleString("en-IN")}`
                  : "None"}
              </p>
            </div>
            <div>
              <span className="text-slate-500 dark:text-slate-400">Submission Due Date</span>
              <p className="mt-0.5 font-bold text-slate-900 dark:text-white">
                {bidding.due_date || "Not specified"}
              </p>
            </div>
            <div>
              <span className="text-slate-500 dark:text-slate-400">Authorized By</span>
              <p className="mt-0.5 font-bold text-slate-900 dark:text-white">
                {bidding.authorized_by_name || "Executive"} on{" "}
                {new Date(bidding.authorized_at).toLocaleDateString("en-IN")}
              </p>
            </div>
          </div>

          {bidding.approval_notes && (
            <div className="mt-4 rounded-xl bg-slate-50 p-3 text-xs dark:bg-slate-800/60">
              <span className="font-semibold text-slate-700 dark:text-slate-300">Approval Directives & Strategy:</span>
              <p className="mt-1 text-slate-600 dark:text-slate-400">{bidding.approval_notes}</p>
            </div>
          )}
        </div>
      )}

      {/* Eligibility Verdict Section (Manager Review) */}
      {tscJobId ? (
        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="mb-4">
            <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
              Technical & Financial Qualification
            </span>
            <h3 className="text-base font-bold text-slate-900 dark:text-white">
              Automated Eligibility Evaluation (TS-02)
            </h3>
            <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
              Rule-based evaluation against company profile credentials and financial qualifications.
            </p>
          </div>
          <TenderEligibilitySection
            base={`drafts/${draftId}/tsc`}
            jobId={tscJobId}
            canWrite={canWrite}
          />
        </div>
      ) : null}

      {/* Authorization Form */}
      {canWrite && (
        <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <h3 className="text-base font-bold text-slate-900 dark:text-white">
            {bidding ? "Update Bidding Decision & Directives" : "Grant Formal Bidding Authorization"}
          </h3>
          <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
            Confirm corporate commitment to participate in this tender opportunity.
          </p>

          <ValidatedForm onSubmit={handleSave} className="mt-5 space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <ValidatedLabel required className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Bid Decision
                  <ValidatedSelect
                    className={inputClass}
                    value={decision}
                    onChange={(e) => setDecision(e.target.value as typeof decision)}
                    disabled={saving}
                  >
                    <option value="authorized">Authorized to Bid (Proceed to Bid Preparation)</option>
                    <option value="conditional">Conditional Bidding (Pre-Bid Clarifications Required)</option>
                    <option value="rejected">Do Not Bid (No-Go Decision)</option>
                  </ValidatedSelect>
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Assign Accountable Tender Manager
                  <ValidatedSelect
                    className={inputClass}
                    value={managerId}
                    onChange={(e) => setManagerId(e.target.value)}
                    disabled={saving}
                  >
                    <option value="">Select a Manager</option>
                    {managers.map((m) => (
                      <option key={m.id} value={m.id}>
                        {m.display_name} ({m.email})
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-3">
              <div>
                <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Estimated Tender Value (₹)
                  <ValidatedInput
                    type="number"
                    step="0.01"
                    className={inputClass}
                    placeholder="e.g. 45000000"
                    value={estimatedValue}
                    onChange={(e) => setEstimatedValue(e.target.value)}
                    disabled={saving}
                  />
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  EMD Amount (₹)
                  <ValidatedInput
                    type="number"
                    step="0.01"
                    className={inputClass}
                    placeholder="e.g. 500000"
                    value={emdAmount}
                    disabled={saving || emdExemption}
                    onChange={(e) => setEmdAmount(e.target.value)}
                  />
                </ValidatedLabel>
                <ValidatedLabel className="mt-1.5 flex cursor-pointer items-center gap-1.5 text-[11px] font-semibold text-slate-600 dark:text-slate-400">
                  <ValidatedInput
                    type="checkbox"
                    checked={emdExemption}
                    disabled={saving}
                    onChange={(e) => setEmdExemption(e.target.checked)}
                    className="h-3.5 w-3.5 accent-blue-600"
                  />
                  MSME / Startup Exemption Claimed
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                  Tender Fee (₹)
                  <ValidatedInput
                    type="number"
                    step="0.01"
                    className={inputClass}
                    placeholder="e.g. 10000"
                    value={tenderFee}
                    disabled={saving || feeExemption}
                    onChange={(e) => setTenderFee(e.target.value)}
                  />
                </ValidatedLabel>
                <ValidatedLabel className="mt-1.5 flex cursor-pointer items-center gap-1.5 text-[11px] font-semibold text-slate-600 dark:text-slate-400">
                  <ValidatedInput
                    type="checkbox"
                    checked={feeExemption}
                    disabled={saving}
                    onChange={(e) => setFeeExemption(e.target.checked)}
                    className="h-3.5 w-3.5 accent-blue-600"
                  />
                  Fee Exemption Claimed
                </ValidatedLabel>
              </div>
            </div>

            <div>
              <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                Bid Submission Portal Due Date
                <ValidatedInput
                  type="date"
                  className={inputClass}
                  value={dueDate}
                  onChange={(e) => setDueDate(e.target.value)}
                  disabled={saving}
                />
              </ValidatedLabel>
            </div>

            <div>
              <ValidatedLabel className="text-xs font-bold text-slate-700 dark:text-slate-300">
                Management Directives & Strategy Notes
                <ValidatedTextarea
                  rows={3}
                  className={inputClass}
                  placeholder="e.g. Approved with minimum target margin of 14%. Focus on consortium partner for high-voltage testing scope. Raise pre-bid query regarding BG format clause 14.2."
                  value={notes}
                  onChange={(e) => setNotes(e.target.value)}
                  disabled={saving}
                />
              </ValidatedLabel>
            </div>

            <div className="flex justify-end pt-3">
              <button type="submit" disabled={saving} className={primaryBtn}>
                {saving ? "Recording..." : bidding ? "Update Bidding Decision" : "Authorize & Release for Bidding"}
                <ArrowRightIcon className="h-4 w-4" />
              </button>
            </div>
          </ValidatedForm>
        </section>
      )}

      {/* Downstream FMS Flow Pipeline Card */}
      <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <h3 className="text-sm font-bold text-slate-900 dark:text-white">
          Downstream Tender Execution Pipeline (Post Step 3)
        </h3>
        <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">
          Once Bidding Authorization is complete, the tender transitions to the subsequent execution phases:
        </p>

        <div className="mt-4 grid gap-3 sm:grid-cols-3 text-xs">
          <div className="rounded-xl border border-slate-100 bg-slate-50 p-3 dark:border-slate-800 dark:bg-slate-850">
            <span className="font-bold text-blue-700 dark:text-blue-400">Stage 2: Lead Generation</span>
            <p className="mt-1 font-semibold text-slate-900 dark:text-white">LG-01 · Share Details & Queries</p>
            <p className="mt-1 text-[11px] text-slate-500">
              Submit pre-bid queries over portal or email and track client addenda.
            </p>
          </div>

          <div className="rounded-xl border border-slate-100 bg-slate-50 p-3 dark:border-slate-800 dark:bg-slate-850">
            <span className="font-bold text-blue-700 dark:text-blue-400">Stage 3: Costing & Vendors</span>
            <p className="mt-1 font-semibold text-slate-900 dark:text-white">CV-01 · Enquiries & BOQ</p>
            <p className="mt-1 text-[11px] text-slate-500">
              Float inquiries to approved vendors, compare quotations, and finalize working BOQ.
            </p>
          </div>

          <div className="rounded-xl border border-slate-100 bg-slate-50 p-3 dark:border-slate-800 dark:bg-slate-850">
            <span className="font-bold text-blue-700 dark:text-blue-400">Stage 4: Technical Bid</span>
            <p className="mt-1 font-semibold text-slate-900 dark:text-white">TB-01 · Bid Compilation</p>
            <p className="mt-1 text-[11px] text-slate-500">
              Compile statutory documents, review bid cover, and submit finalized tender packet.
            </p>
          </div>
        </div>

        {isAuthorized && (
          <div className="mt-4 flex justify-end">
            <button
              type="button"
              onClick={() => onNavigateStep("LG-03")}
              className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-5 text-sm font-bold text-white transition hover:bg-blue-800"
            >
              Continue to Step 4 · Pre-Bid Queries
              <ArrowRightIcon className="h-4 w-4" />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
