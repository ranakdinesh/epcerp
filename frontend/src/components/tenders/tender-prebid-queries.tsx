"use client";

import { useEffect, useState, useCallback } from "react";
import {
  ValidatedForm,
  ValidatedLabel,
  ValidatedInput,
  ValidatedSelect,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";
import {
  QuestionMarkCircleIcon,
  PlusIcon,
  CheckCircleIcon,
  XCircleIcon,
  ArrowRightIcon,
  DocumentArrowDownIcon,
  ChatBubbleLeftRightIcon,
  ArrowPathIcon,
  TrashIcon,
} from "@heroicons/react/24/outline";

export type PrebidQuery = {
  id: string;
  draft_id: string;
  category: "technical" | "commercial" | "make_list" | "boq" | "legal" | "other";
  clause_reference: string;
  tender_specification: string;
  query_description: string;
  justification: string;
  status: "draft" | "submitted" | "answered" | "accepted" | "rejected";
  client_response: string;
  corrigendum_reference: string;
  submitted_at: string | null;
  responded_at: string | null;
  created_at: string;
  updated_at: string;
  version: number;
};

const CATEGORIES: { id: PrebidQuery["category"]; label: string }[] = [
  { id: "technical", label: "Technical Specification" },
  { id: "commercial", label: "Commercial / Payment Terms" },
  { id: "make_list", label: "Approved Make List / Brands" },
  { id: "boq", label: "BOQ / Quantity Scope" },
  { id: "legal", label: "Legal / GCC / SCC Clauses" },
  { id: "other", label: "General Clarification" },
];

const primaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white transition hover:bg-blue-800 disabled:opacity-50";
const secondaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-750";
const inputClass =
  "mt-1 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm text-slate-900 transition focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white";

export function TenderPrebidQueries({
  draftId,
  canWrite = true,
  onNavigateStep,
}: {
  draftId: string;
  canWrite?: boolean;
  onNavigateStep?: (step: string) => void;
}) {
  const [queries, setQueries] = useState<PrebidQuery[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showAddModal, setShowAddModal] = useState(false);
  const [editingQuery, setEditingQuery] = useState<PrebidQuery | null>(null);

  // Form state
  const [category, setCategory] = useState<PrebidQuery["category"]>("technical");
  const [clauseRef, setClauseRef] = useState("");
  const [tenderSpec, setTenderSpec] = useState("");
  const [queryDesc, setQueryDesc] = useState("");
  const [justification, setJustification] = useState("");
  const [saving, setSaving] = useState(false);

  // Response edit state
  const [activeResponseQuery, setActiveResponseQuery] = useState<PrebidQuery | null>(null);
  const [clientResponse, setClientResponse] = useState("");
  const [corrigendumRef, setCorrigendumRef] = useState("");
  const [queryStatus, setQueryStatus] = useState<PrebidQuery["status"]>("answered");

  const loadQueries = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/prebid-queries`, { signal });
      if (!res.ok) throw new Error("Failed to load pre-bid queries");
      const data = await res.json();
      setQueries(data);
    } catch (e) {
      if (!signal?.aborted) setError(e instanceof Error ? e.message : "Error loading queries");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [draftId]);

  useEffect(() => {
    const c = new AbortController();
    const initial = setTimeout(() => void loadQueries(c.signal), 0);
    return () => { clearTimeout(initial); c.abort(); };
  }, [loadQueries]);

  async function handleCreateQuery(e: React.FormEvent) {
    e.preventDefault();
    if (!tenderSpec.trim() || !queryDesc.trim() || saving) return;
    setSaving(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/prebid-queries`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          category,
          clause_reference: clauseRef,
          tender_specification: tenderSpec,
          query_description: queryDesc,
          justification,
        }),
      });
      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.detail || "Failed to save pre-bid query");
      }
      setShowAddModal(false);
      setClauseRef("");
      setTenderSpec("");
      setQueryDesc("");
      setJustification("");
      await loadQueries();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save query");
    } finally {
      setSaving(false);
    }
  }

  async function handleSaveResponse(e: React.FormEvent) {
    e.preventDefault();
    if (!activeResponseQuery || saving) return;
    setSaving(true);
    setError("");
    try {
      const res = await fetch(
        `/api/tender-fms/drafts/${draftId}/prebid-queries/${activeResponseQuery.id}`,
        {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: queryStatus,
            client_response: clientResponse,
            corrigendum_reference: corrigendumRef,
          }),
        }
      );
      if (!res.ok) throw new Error("Failed to update query response");
      setActiveResponseQuery(null);
      await loadQueries();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save response");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(queryId: string) {
    if (!confirm("Are you sure you want to remove this pre-bid query?")) return;
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/prebid-queries/${queryId}`, {
        method: "DELETE",
      });
      if (!res.ok) throw new Error("Failed to delete query");
      await loadQueries();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delete failed");
    }
  }

  function exportToCSV() {
    const headers = [
      "S.No",
      "Category",
      "Clause Reference",
      "Tender Specification",
      "Query / Clarification Sought",
      "Justification",
      "Status",
      "Client Response",
      "Corrigendum Reference",
    ];
    const rows = queries.map((q, idx) => [
      idx + 1,
      q.category.toUpperCase(),
      `"${q.clause_reference.replaceAll('"', '""')}"`,
      `"${q.tender_specification.replaceAll('"', '""')}"`,
      `"${q.query_description.replaceAll('"', '""')}"`,
      `"${q.justification.replaceAll('"', '""')}"`,
      q.status.toUpperCase(),
      `"${q.client_response.replaceAll('"', '""')}"`,
      `"${q.corrigendum_reference.replaceAll('"', '""')}"`,
    ]);
    const csvContent =
      "data:text/csv;charset=utf-8," +
      [headers.join(","), ...rows.map((e) => e.join(","))].join("\n");
    const encodedUri = encodeURI(csvContent);
    const link = document.createElement("a");
    link.setAttribute("href", encodedUri);
    link.setAttribute("download", `Prebid_Queries_${draftId.slice(0, 8)}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }

  const answeredCount = queries.filter((q) =>
    ["answered", "accepted", "rejected"].includes(q.status)
  ).length;

  return (
    <div className="space-y-6">
      {/* Header card */}
      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-100 text-sm font-bold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                4
              </span>
              <h2 className="text-xl font-bold text-slate-900 dark:text-white">
                Pre-Bid Queries & Corrigendum Tracker
              </h2>
            </div>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              Formulate formal technical, commercial, and BOQ clarifications before the pre-bid deadline, and track client Corrigendum / Addendum replies.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {queries.length > 0 && (
              <button
                type="button"
                onClick={exportToCSV}
                className={secondaryBtn + " text-xs"}
                title="Download CSV for portal upload"
              >
                <DocumentArrowDownIcon className="h-4 w-4" />
                Export CSV
              </button>
            )}
            {canWrite && (
              <button
                type="button"
                onClick={() => setShowAddModal(true)}
                className={primaryBtn + " text-xs"}
              >
                <PlusIcon className="h-4 w-4" />
                Add Pre-Bid Query
              </button>
            )}
          </div>
        </div>

        {/* Stats strip */}
        <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div className="rounded-xl border border-slate-100 bg-slate-50 p-3 dark:border-slate-800 dark:bg-slate-850">
            <span className="text-[11px] font-semibold text-slate-500 uppercase">Total Queries</span>
            <p className="text-lg font-bold text-slate-900 dark:text-white">{queries.length}</p>
          </div>
          <div className="rounded-xl border border-amber-100 bg-amber-50/60 p-3 dark:border-amber-900/30 dark:bg-amber-950/20">
            <span className="text-[11px] font-semibold text-amber-700 dark:text-amber-300 uppercase">Pending / Submitted</span>
            <p className="text-lg font-bold text-amber-800 dark:text-amber-200">
              {queries.filter((q) => q.status === "draft" || q.status === "submitted").length}
            </p>
          </div>
          <div className="rounded-xl border border-emerald-100 bg-emerald-50/60 p-3 dark:border-emerald-900/30 dark:bg-emerald-950/20">
            <span className="text-[11px] font-semibold text-emerald-700 dark:text-emerald-300 uppercase">Answered / Accepted</span>
            <p className="text-lg font-bold text-emerald-800 dark:text-emerald-200">
              {queries.filter((q) => q.status === "answered" || q.status === "accepted").length}
            </p>
          </div>
          <div className="rounded-xl border border-purple-100 bg-purple-50/60 p-3 dark:border-purple-900/30 dark:bg-purple-950/20">
            <span className="text-[11px] font-semibold text-purple-700 dark:text-purple-300 uppercase">Clarification Rate</span>
            <p className="text-lg font-bold text-purple-800 dark:text-purple-200">
              {queries.length ? `${Math.round((answeredCount / queries.length) * 100)}%` : "0%"}
            </p>
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-xs font-semibold text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-300">
          {error}
        </div>
      )}

      {/* Query List */}
      <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex items-center justify-between border-b border-slate-100 pb-3 dark:border-slate-800">
          <h3 className="text-sm font-bold text-slate-900 dark:text-white">
            Pre-Bid Query Sheet ({queries.length})
          </h3>
          <button
            type="button"
            onClick={() => void loadQueries()}
            className="text-xs font-semibold text-blue-700 hover:underline dark:text-blue-400"
          >
            <ArrowPathIcon className="inline h-3.5 w-3.5 mr-1" />
            Refresh
          </button>
        </div>

        {loading ? (
          <div className="py-12 text-center text-xs text-slate-500">Loading queries…</div>
        ) : queries.length === 0 ? (
          <div className="py-12 text-center">
            <QuestionMarkCircleIcon className="mx-auto h-10 w-10 text-slate-300 dark:text-slate-600" />
            <p className="mt-2 text-sm font-semibold text-slate-700 dark:text-slate-300">
              No pre-bid queries raised yet
            </p>
            <p className="mt-1 text-xs text-slate-500">
              If the tender document has restrictive clauses, ambiguous specifications, or missing BOQ items, raise queries to obtain client addenda.
            </p>
            {canWrite && (
              <button
                type="button"
                onClick={() => setShowAddModal(true)}
                className={primaryBtn + " mt-4 text-xs"}
              >
                <PlusIcon className="h-4 w-4" />
                Add First Query
              </button>
            )}
          </div>
        ) : (
          <div className="mt-4 divide-y divide-slate-100 dark:divide-slate-800">
            {queries.map((q, idx) => (
              <div key={q.id} className="py-4 first:pt-0 last:pb-0">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-xs font-bold text-slate-400">#{idx + 1}</span>
                      <span className="rounded-full bg-slate-100 px-2.5 py-0.5 text-[11px] font-semibold text-slate-700 dark:bg-slate-800 dark:text-slate-300 uppercase">
                        {q.category}
                      </span>
                      {q.clause_reference && (
                        <span className="text-xs font-semibold text-slate-600 dark:text-slate-400">
                          Clause: <span className="text-slate-900 dark:text-white">{q.clause_reference}</span>
                        </span>
                      )}
                      <span
                        className={`rounded-full px-2 py-0.5 text-[10px] font-bold uppercase ${
                          q.status === "accepted"
                            ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                            : q.status === "answered"
                            ? "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300"
                            : q.status === "rejected"
                            ? "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300"
                            : q.status === "submitted"
                            ? "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300"
                            : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400"
                        }`}
                      >
                        {q.status}
                      </span>
                    </div>

                    <div className="grid gap-2 text-xs sm:grid-cols-2 pt-1">
                      <div className="rounded-lg bg-slate-50 p-2.5 dark:bg-slate-850">
                        <span className="font-semibold text-slate-500 uppercase text-[10px]">Tender Clause / Specification</span>
                        <p className="mt-1 text-slate-800 dark:text-slate-200">{q.tender_specification}</p>
                      </div>
                      <div className="rounded-lg bg-blue-50/60 p-2.5 dark:bg-blue-950/20">
                        <span className="font-semibold text-blue-700 dark:text-blue-300 uppercase text-[10px]">Query / Clarification Sought</span>
                        <p className="mt-1 text-slate-900 dark:text-slate-100">{q.query_description}</p>
                        {q.justification && (
                          <p className="mt-1 text-[11px] text-slate-500 italic">Reason: {q.justification}</p>
                        )}
                      </div>
                    </div>

                    {q.client_response && (
                      <div className="mt-2 rounded-lg border border-emerald-200 bg-emerald-50/60 p-2.5 text-xs text-emerald-950 dark:border-emerald-900/40 dark:bg-emerald-950/20 dark:text-emerald-200">
                        <span className="font-bold uppercase text-[10px] text-emerald-800 dark:text-emerald-300">
                          Client Response / Corrigendum Decision
                          {q.corrigendum_reference ? ` (${q.corrigendum_reference})` : ""}:
                        </span>
                        <p className="mt-0.5">{q.client_response}</p>
                      </div>
                    )}
                  </div>

                  {canWrite && (
                    <div className="flex items-center gap-1.5 self-end sm:self-start">
                      <button
                        type="button"
                        onClick={() => {
                          setActiveResponseQuery(q);
                          setClientResponse(q.client_response || "");
                          setCorrigendumRef(q.corrigendum_reference || "");
                          setQueryStatus(q.status === "draft" ? "answered" : q.status);
                        }}
                        className="rounded-lg border border-slate-200 bg-white px-2.5 py-1 text-xs font-semibold text-blue-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-blue-400"
                        title="Log or edit client reply"
                      >
                        <ChatBubbleLeftRightIcon className="inline h-3.5 w-3.5 mr-1" />
                        Log Reply
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDelete(q.id)}
                        className="rounded-lg p-1.5 text-slate-400 hover:bg-red-50 hover:text-red-700 dark:hover:bg-red-950/40"
                        title="Delete query"
                      >
                        <TrashIcon className="h-4 w-4" />
                      </button>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Step continuation action */}
      <div className="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between dark:border-slate-800 dark:bg-slate-900">
        <div>
          <h4 className="text-sm font-bold text-slate-900 dark:text-white">
            Ready for Step 5 · Tender BOQ Items & Make List
          </h4>
          <p className="mt-0.5 text-xs text-slate-500">
            Define the list of items to purchase, set approved brand makes, and begin floating RFQs to vendors.
          </p>
        </div>
        <button
          type="button"
          onClick={() => onNavigateStep?.("CV-01")}
          className={primaryBtn + " gap-2 px-5 py-2 text-sm"}
        >
          Continue to Step 5 · BOQ & Make List
          <ArrowRightIcon className="h-4 w-4" />
        </button>
      </div>

      {/* Modal: Add Query */}
      {showAddModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-xl rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900">
            <h3 className="text-lg font-bold text-slate-900 dark:text-white">Add Pre-Bid Query</h3>
            <p className="mt-1 text-xs text-slate-500">
              Record a query to clarify with the tender authority during the pre-bid meeting.
            </p>

            <ValidatedForm onSubmit={handleCreateQuery} className="mt-4 space-y-4">
              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Category
                </ValidatedLabel>
                <ValidatedSelect
                  value={category}
                  onChange={(e) => setCategory(e.target.value as PrebidQuery["category"])}
                  className={inputClass}
                >
                  {CATEGORIES.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.label}
                    </option>
                  ))}
                </ValidatedSelect>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Clause Reference / Page No
                </ValidatedLabel>
                <ValidatedInput
                  type="text"
                  placeholder="e.g. Vol II, Section 4, Clause 12.3 / Page 45"
                  value={clauseRef}
                  onChange={(e) => setClauseRef(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Existing Tender Specification / Text *
                </ValidatedLabel>
                <ValidatedTextarea
                  required
                  rows={3}
                  placeholder="Paste the relevant clause or requirement as printed in the tender..."
                  value={tenderSpec}
                  onChange={(e) => setTenderSpec(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Clarification / Amendment Sought *
                </ValidatedLabel>
                <ValidatedTextarea
                  required
                  rows={3}
                  placeholder="State the requested change, relaxation, or clarification..."
                  value={queryDesc}
                  onChange={(e) => setQueryDesc(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Justification / Operational Reason
                </ValidatedLabel>
                <ValidatedInput
                  type="text"
                  placeholder="e.g. Enhances market competition / Standard industry practice as per IS/IEC"
                  value={justification}
                  onChange={(e) => setJustification(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className={secondaryBtn + " text-xs"}
                  disabled={saving}
                >
                  Cancel
                </button>
                <button type="submit" disabled={saving} className={primaryBtn + " text-xs"}>
                  {saving ? "Saving…" : "Save Query"}
                </button>
              </div>
            </ValidatedForm>
          </div>
        </div>
      )}

      {/* Modal: Log Client Reply */}
      {activeResponseQuery && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900">
            <h3 className="text-lg font-bold text-slate-900 dark:text-white">
              Log Pre-Bid Reply / Corrigendum
            </h3>
            <p className="mt-1 text-xs text-slate-500">
              Query #{activeResponseQuery.clause_reference || activeResponseQuery.id.slice(0, 8)}
            </p>

            <ValidatedForm onSubmit={handleSaveResponse} className="mt-4 space-y-4">
              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Client Verdict / Status
                </ValidatedLabel>
                <ValidatedSelect
                  value={queryStatus}
                  onChange={(e) => setQueryStatus(e.target.value as PrebidQuery["status"])}
                  className={inputClass}
                >
                  <option value="submitted">Submitted (Awaiting Reply)</option>
                  <option value="answered">Answered / Clarified</option>
                  <option value="accepted">Accepted / Clause Amended</option>
                  <option value="rejected">Rejected / As per Tender Terms</option>
                </ValidatedSelect>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Corrigendum / Addendum Reference
                </ValidatedLabel>
                <ValidatedInput
                  type="text"
                  placeholder="e.g. Corrigendum-1 dtd 10-Nov-2026"
                  value={corrigendumRef}
                  onChange={(e) => setCorrigendumRef(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Official Client Response Text *
                </ValidatedLabel>
                <ValidatedTextarea
                  required
                  rows={4}
                  placeholder="Paste the official response from the pre-bid minutes or corrigendum..."
                  value={clientResponse}
                  onChange={(e) => setClientResponse(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setActiveResponseQuery(null)}
                  className={secondaryBtn + " text-xs"}
                  disabled={saving}
                >
                  Cancel
                </button>
                <button type="submit" disabled={saving} className={primaryBtn + " text-xs"}>
                  {saving ? "Saving…" : "Save Response"}
                </button>
              </div>
            </ValidatedForm>
          </div>
        </div>
      )}
    </div>
  );
}
