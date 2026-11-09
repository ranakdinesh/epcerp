"use client";

import { useEffect, useState, useCallback } from "react";
import {
  CheckCircleIcon,
  XCircleIcon,
  ExclamationTriangleIcon,
  ArrowPathIcon,
  ShieldCheckIcon,
  ArrowRightIcon,
} from "@heroicons/react/24/outline";
import { actionButtonClass } from "@/components/ui/action-button";
import {
  ValidatedForm,
  ValidatedLabel,
  ValidatedSelect,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";
import type { EligibilityVerdict } from "@/server/tenders/tsc/eligibility";

const primary = actionButtonClass("primary");
const secondary = actionButtonClass("secondary");

export function TenderEligibilitySection({
  base,
  jobId,
  canWrite,
  onProceedToStep3,
}: {
  base: string;
  jobId: string;
  canWrite: boolean;
  onProceedToStep3?: () => void;
}) {
  const [verdict, setVerdict] = useState<EligibilityVerdict | null>(null);
  const [loading, setLoading] = useState(false);
  const [evaluating, setEvaluating] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  // Verification form
  const [decision, setDecision] = useState<"accepted" | "rejected" | "overridden">("accepted");
  const [remarks, setRemarks] = useState("");
  const [verifying, setVerifying] = useState(false);

  const fetchVerdict = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    try {
      const res = await fetch(`/api/tender-fms/${base}/${jobId}/eligibility`, {
        cache: "no-store",
        signal,
      });
      if (res.ok) {
        const data = await res.json();
        setVerdict(data);
      }
    } catch {
      // not evaluated yet
    } finally {
      setLoading(false);
    }
  }, [base, jobId]);

  useEffect(() => {
    const c = new AbortController();
    const initial = setTimeout(() => void fetchVerdict(c.signal), 0);
    return () => { clearTimeout(initial); c.abort(); };
  }, [fetchVerdict]);

  async function runEvaluation() {
    setEvaluating(true);
    setError("");
    setNotice("");
    try {
      const res = await fetch(`/api/tender-fms/${base}/${jobId}/eligibility`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Eligibility evaluation failed");
      }
      const data = await res.json();
      setVerdict(data);
      setNotice("AI eligibility evaluation completed.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Evaluation failed");
    } finally {
      setEvaluating(false);
    }
  }

  async function submitVerification(e: React.FormEvent) {
    e.preventDefault();
    setVerifying(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/${base}/${jobId}/eligibility/verify`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ decision, remarks }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Unable to save verification");
      }
      const data = await res.json();
      setVerdict(data);
      setNotice("Human verification decision recorded successfully.");
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Verification error");
    } finally {
      setVerifying(false);
    }
  }

  return (
    <section className="space-y-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between border-b border-slate-100 pb-4 dark:border-slate-800">
        <div>
          <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
            Tender PQC & Company Profile Matching
          </span>
          <h3 className="text-lg font-bold text-slate-900 dark:text-white">
            Company Tender Eligibility Verdict
          </h3>
          <p className="text-xs text-slate-500">
            Automated evaluation comparing company turnover, net worth, licences, and completed works with this tender’s checklist answers.
          </p>
        </div>
        <div>
          <button
            type="button"
            className={primary}
            disabled={evaluating || loading}
            onClick={() => void runEvaluation()}
          >
            {evaluating ? (
              <>Evaluating…</>
            ) : verdict ? (
              <>
                <ArrowPathIcon className="h-4 w-4" /> Re-evaluate Eligibility
              </>
            ) : (
              <>
                <ShieldCheckIcon className="h-4 w-4" /> Evaluate Eligibility
              </>
            )}
          </button>
        </div>
      </div>

      {notice && (
        <p role="status" className="rounded-xl bg-blue-50 p-3 text-sm font-semibold text-blue-900 dark:bg-blue-950/40 dark:text-blue-200">
          {notice}
        </p>
      )}
      {error && (
        <p role="alert" className="rounded-xl bg-red-50 p-3 text-sm font-semibold text-red-900 dark:bg-red-950/40 dark:text-red-200">
          {error}
        </p>
      )}

      {!verdict && !evaluating && (
        <div className="rounded-xl border border-dashed border-slate-300 p-8 text-center text-slate-500 dark:border-slate-700">
          <ShieldCheckIcon className="mx-auto h-10 w-10 text-slate-400" />
          <h4 className="mt-2 text-sm font-bold text-slate-700 dark:text-slate-300">
            Eligibility Not Evaluated Yet
          </h4>
          <p className="mx-auto mt-1 max-w-md text-xs text-slate-500">
            Once you have reviewed the checklist questions, click <strong>&quot;Evaluate Eligibility&quot;</strong> to cross-check company setup credentials and generate an AI PQC verdict.
          </p>
        </div>
      )}

      {verdict && (
        <div className="space-y-6">
          {/* Verdict Banner */}
          <div
            className={`flex flex-col gap-4 rounded-xl border p-5 sm:flex-row sm:items-center sm:justify-between ${
              verdict.verdict === "eligible"
                ? "border-emerald-200 bg-emerald-50 text-emerald-950 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-200"
                : verdict.verdict === "conditionally_eligible"
                ? "border-amber-200 bg-amber-50 text-amber-950 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200"
                : "border-red-200 bg-red-50 text-red-950 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-200"
            }`}
          >
            <div className="flex items-start gap-3">
              {verdict.verdict === "eligible" ? (
                <CheckCircleIcon className="h-8 w-8 text-emerald-600 dark:text-emerald-400 shrink-0" />
              ) : verdict.verdict === "conditionally_eligible" ? (
                <ExclamationTriangleIcon className="h-8 w-8 text-amber-600 dark:text-amber-400 shrink-0" />
              ) : (
                <XCircleIcon className="h-8 w-8 text-red-600 dark:text-red-400 shrink-0" />
              )}
              <div>
                <span className="text-xs font-bold uppercase tracking-widest">
                  AI Verdict Result
                </span>
                <h4 className="text-xl font-black uppercase">
                  {verdict.verdict.replace("_", " ")}
                </h4>
                <p className="mt-1 text-sm font-medium leading-relaxed">
                  {verdict.overall_summary}
                </p>
                <div className="mt-2 flex flex-wrap gap-4 text-xs opacity-75">
                  <span>Evaluator: {verdict.model_used}</span>
                  <span>Evaluated: {new Date(verdict.evaluated_at).toLocaleString()}</span>
                </div>
              </div>
            </div>
          </div>

          {/* Criteria Matcher Matrix */}
          <div className="overflow-x-auto rounded-xl border border-slate-200 dark:border-slate-800">
            <table className="w-full text-left text-sm">
              <thead className="bg-slate-50 text-xs uppercase font-bold text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                <tr>
                  <th className="p-3">Category</th>
                  <th className="p-3">Criterion</th>
                  <th className="p-3">Tender Requirement</th>
                  <th className="p-3">Company Evidence Value</th>
                  <th className="p-3">Status</th>
                  <th className="p-3">Notes & Analysis</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                {verdict.criteria_matrix.map((c, i) => (
                  <tr key={i} className="hover:bg-slate-50/50 dark:hover:bg-slate-800/50">
                    <td className="p-3 font-semibold text-xs text-slate-500 whitespace-nowrap">{c.category}</td>
                    <td className="p-3 font-bold text-slate-900 dark:text-white">{c.criterion}</td>
                    <td className="p-3 text-xs text-slate-700 dark:text-slate-300 max-w-xs">{c.tender_requirement}</td>
                    <td className="p-3 text-xs font-semibold text-slate-900 dark:text-slate-100 max-w-xs">{c.company_value}</td>
                    <td className="p-3 whitespace-nowrap">
                      <span
                        className={`inline-flex rounded-full px-2.5 py-0.5 text-xs font-bold ${
                          c.status === "met"
                            ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                            : c.status === "conditional"
                            ? "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300"
                            : c.status === "needs_review"
                            ? "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300"
                            : "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300"
                        }`}
                      >
                        {c.status.replace("_", " ").toUpperCase()}
                      </span>
                    </td>
                    <td className="p-3 text-xs text-slate-600 dark:text-slate-400">{c.notes}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Gaps and Risks */}
          {verdict.gaps_and_risks.length > 0 && (
            <div className="rounded-xl border border-amber-200 bg-amber-50/50 p-4 dark:border-amber-900/40 dark:bg-amber-950/20">
              <h4 className="text-xs font-bold uppercase tracking-wider text-amber-800 dark:text-amber-300">
                Identified Disqualification Gaps &amp; Pre-Bid Warnings ({verdict.gaps_and_risks.length})
              </h4>
              <ul className="mt-2 list-inside list-disc space-y-1 text-xs text-amber-950 dark:text-amber-200">
                {verdict.gaps_and_risks.map((gap, idx) => (
                  <li key={idx}>{gap}</li>
                ))}
              </ul>
            </div>
          )}

          {/* Human Verification & Decision Sign-Off */}
          <div className="rounded-xl border border-slate-200 bg-slate-50 p-5 dark:border-slate-800 dark:bg-slate-800/50">
            <h4 className="text-sm font-bold uppercase tracking-wider text-slate-800 dark:text-slate-200">
              Human Review &amp; Formal Bidding Sign-Off
            </h4>
            <p className="mt-1 text-xs text-slate-500">
              An authorized Tender Manager or Director must review the AI verdict, address any caveats, and record the final decision.
            </p>

            {verdict.human_verified ? (
              <div className="mt-4 space-y-4">
                <div className="rounded-xl border border-emerald-200 bg-white p-4 shadow-sm dark:border-slate-700 dark:bg-slate-900">
                  <div className="flex items-center gap-2">
                    <CheckCircleIcon className="h-5 w-5 text-emerald-600" />
                    <span className="text-sm font-bold text-slate-900 dark:text-white">
                      Verified Decision: {verdict.human_decision.toUpperCase()}
                    </span>
                  </div>
                  <p className="mt-2 text-sm text-slate-700 dark:text-slate-300">
                    <strong>Remarks:</strong> {verdict.human_remarks || "No remarks provided"}
                  </p>
                  <span className="mt-2 block text-xs text-slate-500">
                    Verified by {verdict.verified_by_name || "Manager"} on {verdict.verified_at ? new Date(verdict.verified_at).toLocaleString() : "date"}
                  </span>
                </div>

                <div className="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-blue-200 bg-blue-50/80 p-4 dark:border-blue-900/60 dark:bg-blue-950/40">
                  <div className="flex items-center gap-3">
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-blue-700 text-xs font-bold text-white shadow-sm">
                      3
                    </span>
                    <div>
                      <p className="text-xs font-bold text-blue-950 dark:text-blue-100">
                        Tender Selection Pipeline · Next Step
                      </p>
                      <p className="text-[11px] text-blue-700 dark:text-blue-300">
                        Formal verification recorded ({verdict.human_decision.toUpperCase()}). Proceed to Step 3 for Executive Bidding Approval &amp; Manager Assignment.
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    className="inline-flex min-h-10 items-center justify-center gap-2 rounded-xl bg-blue-700 px-5 text-xs font-bold text-white shadow-sm transition hover:bg-blue-800"
                    onClick={() => {
                      if (onProceedToStep3) {
                        onProceedToStep3();
                      } else {
                        window.dispatchEvent(new CustomEvent("tender-navigate-step", { detail: "TS-02" }));
                      }
                    }}
                  >
                    Proceed to Step 3 · Approval for Bidding
                    <ArrowRightIcon className="h-4 w-4" />
                  </button>
                </div>
              </div>
            ) : canWrite ? (
              <ValidatedForm className="mt-4 space-y-4" onSubmit={submitVerification}>
                <div className="grid gap-4 sm:grid-cols-2">
                  <ValidatedLabel className="block text-sm font-semibold">
                    Decision Action
                    <ValidatedSelect
                      value={decision}
                      onChange={(e) => setDecision(e.target.value as typeof decision)}
                      className="mt-1 min-h-11 w-full rounded-xl border border-slate-300 bg-white p-2 text-sm dark:border-slate-700 dark:bg-slate-900"
                    >
                      <option value="accepted">Accept AI Verdict &amp; Clear for Bidding</option>
                      <option value="rejected">Reject Tender (Do Not Bid)</option>
                      <option value="overridden">Override AI Verdict with Caveats</option>
                    </ValidatedSelect>
                  </ValidatedLabel>
                  <ValidatedLabel className="block text-sm font-semibold sm:col-span-2">
                    Reviewer Sign-off Remarks
                    <ValidatedTextarea
                      required
                      minLength={3}
                      value={remarks}
                      onChange={(e) => setRemarks(e.target.value)}
                      placeholder="e.g. Reviewed 3-year turnover and PQC criteria. Eligible to bid with MSME EMD waiver. Proceeding to Costing."
                      className="mt-1 min-h-11 w-full rounded-xl border border-slate-300 bg-white p-2 text-sm dark:border-slate-700 dark:bg-slate-900"
                      rows={2}
                    />
                  </ValidatedLabel>
                </div>
                <div className="flex justify-end">
                  <button type="submit" disabled={verifying} className={primary}>
                    {verifying ? "Recording Sign-off…" : "Submit Human Verification Sign-off"}
                  </button>
                </div>
              </ValidatedForm>
            ) : (
              <p className="mt-3 text-xs text-slate-500">
                You have read-only access. Tender Manager or Director approval required to sign off.
              </p>
            )}
          </div>
        </div>
      )}
    </section>
  );
}
