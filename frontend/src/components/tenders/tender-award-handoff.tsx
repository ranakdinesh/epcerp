"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import {
  ValidatedForm,
  ValidatedLabel,
  ValidatedInput,
} from "@/components/ui/validated-controls";
import {
  TrophyIcon,
  CheckCircleIcon,
  RocketLaunchIcon,
  ArrowRightIcon,
  DocumentCheckIcon,
  BuildingOfficeIcon,
  ClockIcon,
  CurrencyRupeeIcon,
} from "@heroicons/react/24/outline";
import type { BidCosting } from "./tender-bid-costing";

const primaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white transition hover:bg-blue-800 disabled:opacity-50";
const successBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-emerald-600 px-5 text-sm font-bold text-white transition hover:bg-emerald-700 disabled:opacity-50 shadow-md";
const secondaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200";
const inputClass =
  "mt-1 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm text-slate-900 transition focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white";

export function TenderAwardHandoff({
  draftId,
  canWrite = true,
}: {
  draftId: string;
  canWrite?: boolean;
}) {
  const [costing, setCosting] = useState<BidCosting | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  // Submission Form
  const [submissionDate, setSubmissionDate] = useState<string>(
    new Date().toISOString().split("T")[0]
  );
  const [portalRef, setPortalRef] = useState("");

  // Award Handoff Form
  const [projectTitle, setProjectTitle] = useState("");
  const [contractRef, setContractRef] = useState("");
  const [clientName, setClientName] = useState("");
  const [createdProjectId, setCreatedProjectId] = useState<string | null>(null);
  const [projectNumber, setProjectNumber] = useState<string | null>(null);

  const loadCosting = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/bid-costing`, { signal });
      if (!res.ok) throw new Error("Failed to load bid costing data");
      const data: BidCosting = await res.json();
      setCosting(data);
      if (data.submission_date) setSubmissionDate(data.submission_date.slice(0, 10));
      if (data.portal_reference) setPortalRef(data.portal_reference);
      if (data.project_id) setCreatedProjectId(data.project_id);
      if (data.contract_reference) setContractRef(data.contract_reference);
    } catch (e) {
      if (!signal?.aborted) setError(e instanceof Error ? e.message : "Error loading data");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [draftId]);

  useEffect(() => {
    const c = new AbortController();
    const initial = setTimeout(() => void loadCosting(c.signal), 0);
    return () => { clearTimeout(initial); c.abort(); };
  }, [loadCosting]);

  async function handleRecordSubmission(e: React.FormEvent) {
    e.preventDefault();
    if (!portalRef.trim() || saving) return;
    setSaving(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/bid-costing`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          status: "submitted",
          submission_date: submissionDate,
          portal_reference: portalRef,
        }),
      });
      if (!res.ok) throw new Error("Failed to update submission");
      await loadCosting();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Submission record failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleAwardAndCreateProject(e: React.FormEvent) {
    e.preventDefault();
    if (!contractRef.trim() || !projectTitle.trim() || !clientName.trim() || saving) return;
    setSaving(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/award-project`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          project_title: projectTitle,
          contract_reference: contractRef,
          client_name: clientName,
        }),
      });

      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.detail || "Award handoff failed");
      }

      const out = await res.json();
      setCreatedProjectId(out.project_id);
      if (out.project_number) setProjectNumber(out.project_number);
      await loadCosting();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Award handoff failed");
    } finally {
      setSaving(false);
    }
  }

  const isSubmitted = costing?.status === "submitted" || costing?.status === "awarded";
  const isAwarded = costing?.status === "awarded" || Boolean(createdProjectId);

  return (
    <div className="space-y-6">
      {/* Header Card */}
      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-100 text-sm font-bold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                8
              </span>
              <h2 className="text-xl font-bold text-slate-900 dark:text-white">
                Tender Submission & Execution Project Handoff
              </h2>
            </div>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              Record e-procurement portal submission, track award decision, and merge the tender with Projects and Purchase FMS upon contract award.
            </p>
          </div>

          <div>
            <span
              className={`rounded-full px-3 py-1 text-xs font-bold uppercase ${
                isAwarded
                  ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                  : isSubmitted
                  ? "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300"
                  : "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300"
              }`}
            >
              Stage: {isAwarded ? "Contract Awarded" : isSubmitted ? "Bid Submitted" : "Ready for Submission"}
            </span>
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-xs font-semibold text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-300">
          {error}
        </div>
      )}

      {/* SUCCESS BANNER: When Project has been Created / Awarded */}
      {isAwarded && (
        <div className="rounded-2xl border-2 border-emerald-300 bg-emerald-50/80 p-6 dark:border-emerald-800 dark:bg-emerald-950/40">
          <div className="flex flex-col sm:flex-row sm:items-center gap-4">
            <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-emerald-600 text-white">
              <TrophyIcon className="h-7 w-7" />
            </div>
            <div className="flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="text-lg font-bold text-emerald-900 dark:text-emerald-200">
                  Tender Successfully Awarded & Converted to Execution Project!
                </h3>
                {projectNumber && (
                  <span className="rounded-md bg-emerald-700 px-2 py-0.5 text-xs font-black uppercase tracking-wide text-white">
                    {projectNumber}
                  </span>
                )}
              </div>
              <p className="text-xs text-emerald-800 dark:text-emerald-300 mt-1">
                Tender FMS has successfully merged with the Projects and Purchase FMS modules.
                The bid value is recorded on the project. Procurement can match the transferred draft BOQ lines to catalogue items and set required dates.
              </p>
              <div className="mt-4 flex flex-wrap gap-3">
                <Link
                  href={`/dashboard?screen=projects${createdProjectId ? `&project=${encodeURIComponent(createdProjectId)}` : ""}`}
                  className="inline-flex items-center gap-1.5 rounded-xl bg-emerald-700 px-4 py-2.5 text-xs font-bold text-white hover:bg-emerald-800 shadow-xs"
                >
                  <RocketLaunchIcon className="h-4 w-4" />
                  Open in Projects Workspace
                </Link>
                <Link
                  href={`/dashboard?screen=purchase-fms${createdProjectId ? `&project=${encodeURIComponent(createdProjectId)}&step=PUR-01` : ""}`}
                  className="inline-flex items-center gap-1.5 rounded-xl border border-emerald-600 bg-white px-4 py-2.5 text-xs font-bold text-emerald-800 hover:bg-emerald-50 shadow-xs dark:bg-slate-900 dark:text-emerald-200"
                >
                  <BuildingOfficeIcon className="h-4 w-4" />
                  Initiate Procurement in Purchase FMS (PUR-01) →
                </Link>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Step 8A: Submission Record Card */}
      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex items-center gap-2 border-b border-slate-100 pb-3 dark:border-slate-800">
          <DocumentCheckIcon className="h-5 w-5 text-blue-600 dark:text-blue-400" />
          <h3 className="text-base font-bold text-slate-900 dark:text-white">
            Section A · Online Tender Portal Submission
          </h3>
        </div>

        <ValidatedForm onSubmit={handleRecordSubmission} className="mt-4 space-y-4 text-xs">
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                Submission Date *
              </ValidatedLabel>
              <ValidatedInput
                type="date"
                required
                value={submissionDate}
                onChange={(e) => setSubmissionDate(e.target.value)}
                className={inputClass}
                disabled={isAwarded}
              />
            </div>
            <div>
              <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                Portal Bid Reference / Acknowledgement Number *
              </ValidatedLabel>
              <ValidatedInput
                type="text"
                required
                placeholder="e.g. GEM/2026/B/982142 or CPP-TND-7612"
                value={portalRef}
                onChange={(e) => setPortalRef(e.target.value)}
                className={inputClass}
                disabled={isAwarded}
              />
            </div>
          </div>

          <div className="flex items-center justify-between pt-2">
            <div className="text-slate-500">
              Total Approved Bid Value:{" "}
              <strong className="text-slate-900 dark:text-white">
                ₹{costing?.total_bid_value ? costing.total_bid_value.toLocaleString("en-IN") : "0"}
              </strong>
            </div>
            {canWrite && !isAwarded && (
              <button type="submit" disabled={saving} className={primaryBtn + " text-xs"}>
                {isSubmitted ? "Update Submission Info" : "Mark as Submitted on Portal"}
              </button>
            )}
          </div>
        </ValidatedForm>
      </div>

      {/* Step 8B: Award Hand-Off to Projects & Purchase FMS */}
      {!isAwarded && (
        <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center gap-2 border-b border-slate-100 pb-3 dark:border-slate-800">
            <TrophyIcon className="h-5 w-5 text-emerald-600 dark:text-emerald-400" />
            <h3 className="text-base font-bold text-slate-900 dark:text-white">
              Section B · Post-Award Handoff (Merge with Projects & Purchase FMS)
            </h3>
          </div>

          <p className="mt-2 text-xs text-slate-500">
            When the client issues the official Letter of Award (LOA) or Contract Order, use this action to generate the execution project, record its contract value, and transfer the costed items to a draft BOQ for procurement review.
          </p>

          <ValidatedForm onSubmit={handleAwardAndCreateProject} className="mt-4 space-y-4 text-xs">
            <div className="grid gap-4 sm:grid-cols-3">
              <div>
                <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                  Execution Project Title *
                </ValidatedLabel>
                <ValidatedInput
                  type="text"
                  required
                  placeholder="e.g. 11kV Substation EPC Project - Package A"
                  value={projectTitle}
                  onChange={(e) => setProjectTitle(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div>
                <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                  LOA / Work Order / Contract Ref *
                </ValidatedLabel>
                <ValidatedInput
                  type="text"
                  required
                  placeholder="e.g. LOA-DISCOM-2026-081"
                  value={contractRef}
                  onChange={(e) => setContractRef(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div>
                <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                  Client / Authority Name
                </ValidatedLabel>
                <ValidatedInput
                  type="text"
                  required
                  placeholder="Exact name from the client register"
                  value={clientName}
                  onChange={(e) => setClientName(e.target.value)}
                  className={inputClass}
                />
              </div>
            </div>

            <div className="rounded-xl border border-blue-100 bg-blue-50/60 p-4 text-blue-950 dark:border-blue-900/40 dark:bg-blue-950/20 dark:text-blue-200">
              <span className="font-bold text-xs uppercase block">Automatic System Actions on Award:</span>
              <ul className="mt-1 space-y-1 text-xs">
                <li>• Creates a persistent execution project record in <strong className="font-semibold">Projects FMS</strong>.</li>
                <li>• Records the project contract value from the submitted costing sheet (₹{costing?.total_bid_value.toLocaleString("en-IN") || "0"}).</li>
                <li>• Transfers BOQ items as a draft to <strong className="font-semibold">Purchase FMS</strong> for indenting and purchase order execution.</li>
              </ul>
            </div>

            {canWrite && (
              <div className="flex justify-end pt-2">
                <button
                  type="submit"
                  disabled={saving || !isSubmitted}
                  className={successBtn + " text-xs"}
                  title={!isSubmitted ? "Mark the bid as submitted first" : "Create execution project"}
                >
                  <CheckCircleIcon className="h-5 w-5" />
                  Award Tender & Create Execution Project
                </button>
              </div>
            )}
          </ValidatedForm>
        </div>
      )}
    </div>
  );
}
