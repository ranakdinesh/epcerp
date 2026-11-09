"use client";

import { useEffect, useState, useCallback } from "react";
import {
  ValidatedForm,
  ValidatedLabel,
  ValidatedInput,
  ValidatedSelect,
} from "@/components/ui/validated-controls";
import {
  CurrencyRupeeIcon,
  PlusIcon,
  CheckBadgeIcon,
  CheckCircleIcon,
  XCircleIcon,
  ArrowRightIcon,
  TrashIcon,
  BuildingOfficeIcon,
  ArrowPathIcon,
} from "@heroicons/react/24/outline";
import type { BidItem } from "./tender-boq-items";

export type VendorQuotation = {
  id: string;
  draft_id: string;
  bid_item_id: string;
  vendor_id: string | null;
  vendor_name: string;
  make_offered: string;
  unit_rate: number;
  freight_and_insurance: number;
  gst_percent: number;
  delivery_weeks: number;
  is_technically_compliant: boolean;
  is_l1: boolean;
  remarks: string;
  quote_ref: string;
  created_at: string;
  updated_at: string;
  version: number;
};

const primaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white transition hover:bg-blue-800 disabled:opacity-50";
const secondaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200";
const inputClass =
  "mt-1 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm text-slate-900 transition focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white";

export function TenderVendorComparison({
  draftId,
  canWrite = true,
  onNavigateStep,
}: {
  draftId: string;
  canWrite?: boolean;
  onNavigateStep?: (step: string) => void;
}) {
  const [items, setItems] = useState<BidItem[]>([]);
  const [selectedItemId, setSelectedItemId] = useState<string>("");
  const [quotes, setQuotes] = useState<VendorQuotation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showAddQuote, setShowAddQuote] = useState(false);

  // Quote form state
  const [vendorName, setVendorName] = useState("");
  const [makeOffered, setMakeOffered] = useState("");
  const [unitRate, setUnitRate] = useState<number | "">("");
  const [freight, setFreight] = useState<number | "">("");
  const [gstPercent, setGstPercent] = useState<number>(18);
  const [deliveryWeeks, setDeliveryWeeks] = useState<number>(4);
  const [isCompliant, setIsCompliant] = useState(true);
  const [remarks, setRemarks] = useState("");
  const [quoteRef, setQuoteRef] = useState("");
  const [saving, setSaving] = useState(false);

  const loadData = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError("");
    try {
      // 1. Load items
      const itemsRes = await fetch(`/api/tender-fms/drafts/${draftId}/bid-items`, { signal });
      if (!itemsRes.ok) throw new Error("Failed to load BOQ items");
      const itemsData: BidItem[] = await itemsRes.json();
      setItems(itemsData);

      const targetItemId = selectedItemId || itemsData[0]?.id || "";
      if (!selectedItemId && itemsData[0]?.id) {
        setSelectedItemId(itemsData[0].id);
      }

      // 2. Load quotations
      const quotesRes = await fetch(
        `/api/tender-fms/drafts/${draftId}/vendor-quotations${targetItemId ? `?item_id=${targetItemId}` : ""}`,
        { signal }
      );
      if (!quotesRes.ok) throw new Error("Failed to load quotations");
      const quotesData = await quotesRes.json();
      setQuotes(quotesData);
    } catch (e) {
      if (!signal?.aborted) setError(e instanceof Error ? e.message : "Error loading sourcing data");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [draftId, selectedItemId]);

  useEffect(() => {
    const c = new AbortController();
    const initial = setTimeout(() => void loadData(c.signal), 0);
    return () => { clearTimeout(initial); c.abort(); };
  }, [loadData]);

  async function handleAddQuote(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedItemId || !vendorName.trim() || saving) return;
    setSaving(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/vendor-quotations`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          bid_item_id: selectedItemId,
          vendor_name: vendorName.trim(),
          make_offered: makeOffered.trim(),
          unit_rate: Number(unitRate) || 0,
          freight_and_insurance: Number(freight) || 0,
          gst_percent: gstPercent,
          delivery_weeks: deliveryWeeks,
          is_technically_compliant: isCompliant,
          remarks: remarks.trim(),
          quote_ref: quoteRef.trim(),
        }),
      });
      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.detail || "Failed to save quotation");
      }
      setShowAddQuote(false);
      setVendorName("");
      setMakeOffered("");
      setUnitRate("");
      setFreight("");
      setRemarks("");
      setQuoteRef("");
      await loadData();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to add quotation");
    } finally {
      setSaving(false);
    }
  }

  async function handleSelectL1(quoteId: string) {
    if (!selectedItemId) return;
    setSaving(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/vendor-quotations/select-l1`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          bid_item_id: selectedItemId,
          quote_id: quoteId,
        }),
      });
      if (!res.ok) throw new Error("Failed to lock L-1 quote");
      await loadData();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to select L-1");
    } finally {
      setSaving(false);
    }
  }

  async function handleDeleteQuote(quoteId: string) {
    if (!confirm("Are you sure you want to remove this quotation?")) return;
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/vendor-quotations/${quoteId}`, {
        method: "DELETE",
      });
      if (!res.ok) throw new Error("Failed to delete quotation");
      await loadData();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delete failed");
    }
  }

  const selectedItem = items.find((i) => i.id === selectedItemId);
  const itemsWithL1Count = items.filter((i) => i.l1_unit_rate > 0).length;

  return (
    <div className="space-y-6">
      {/* Header Card */}
      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-100 text-sm font-bold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                6
              </span>
              <h2 className="text-xl font-bold text-slate-900 dark:text-white">
                Vendor Quotations & L-1 Price Comparison (PCT)
              </h2>
            </div>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              Collect supplier quotes against approved make lists, verify technical compliance, and establish the Lowest Compliant (L-1) rate per package.
            </p>
          </div>

          <div className="flex items-center gap-2">
            {canWrite && selectedItemId && (
              <button
                type="button"
                onClick={() => setShowAddQuote(true)}
                className={primaryBtn + " text-xs"}
              >
                <PlusIcon className="h-4 w-4" />
                Add Vendor Quote
              </button>
            )}
          </div>
        </div>

        {/* Progress Strip */}
        <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div className="rounded-xl border border-slate-100 bg-slate-50 p-3 dark:border-slate-800 dark:bg-slate-850">
            <span className="text-[11px] font-semibold text-slate-500 uppercase">Total Items</span>
            <p className="text-lg font-bold text-slate-900 dark:text-white">{items.length}</p>
          </div>
          <div className="rounded-xl border border-emerald-100 bg-emerald-50/60 p-3 dark:border-emerald-900/30 dark:bg-emerald-950/20">
            <span className="text-[11px] font-semibold text-emerald-700 dark:text-emerald-300 uppercase">L-1 Finalized</span>
            <p className="text-lg font-bold text-emerald-800 dark:text-emerald-200">
              {itemsWithL1Count} / {items.length}
            </p>
          </div>
          <div className="rounded-xl border border-blue-100 bg-blue-50/60 p-3 dark:border-blue-900/30 dark:bg-blue-950/20">
            <span className="text-[11px] font-semibold text-blue-700 dark:text-blue-300 uppercase">Received Quotes</span>
            <p className="text-lg font-bold text-blue-800 dark:text-blue-200">{quotes.length}</p>
          </div>
          <div className="rounded-xl border border-purple-100 bg-purple-50/60 p-3 dark:border-purple-900/30 dark:bg-purple-950/20">
            <span className="text-[11px] font-semibold text-purple-700 dark:text-purple-300 uppercase">Sourcing Progress</span>
            <p className="text-lg font-bold text-purple-800 dark:text-purple-200">
              {items.length ? `${Math.round((itemsWithL1Count / items.length) * 100)}%` : "0%"}
            </p>
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-xs font-semibold text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-300">
          {error}
        </div>
      )}

      {/* Main Two-Column Sourcing Layout */}
      <div className="grid gap-6 lg:grid-cols-3">
        {/* Left Column: Item Selector */}
        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <h3 className="text-sm font-bold text-slate-900 dark:text-white border-b border-slate-100 pb-3 dark:border-slate-800">
            Select BOQ Item ({items.length})
          </h3>

          <div className="mt-3 space-y-1.5 max-h-[500px] overflow-y-auto">
            {items.map((it) => {
              const isSelected = it.id === selectedItemId;
              const hasL1 = it.l1_unit_rate > 0;
              return (
                <button
                  key={it.id}
                  type="button"
                  onClick={() => setSelectedItemId(it.id)}
                  className={`w-full text-left rounded-xl p-3 text-xs transition border ${
                    isSelected
                      ? "border-blue-600 bg-blue-50/70 text-blue-950 dark:border-blue-500 dark:bg-blue-950/40 dark:text-blue-200 shadow-xs"
                      : "border-slate-100 bg-slate-50/50 hover:bg-slate-100/70 dark:border-slate-800 dark:bg-slate-850 dark:hover:bg-slate-800"
                  }`}
                >
                  <div className="flex items-center justify-between gap-1">
                    <span className="font-mono text-[10px] text-slate-400">
                      {it.item_code || "ITEM"}
                    </span>
                    {hasL1 ? (
                      <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-bold text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300">
                        L-1 Locked
                      </span>
                    ) : (
                      <span className="rounded-full bg-slate-200 px-2 py-0.5 text-[10px] font-semibold text-slate-600 dark:bg-slate-700 dark:text-slate-300">
                        Pending
                      </span>
                    )}
                  </div>
                  <p className="mt-1 font-semibold text-slate-900 dark:text-white line-clamp-2">
                    {it.description}
                  </p>
                  <div className="mt-1 flex items-center justify-between text-[11px] text-slate-500">
                    <span>
                      Qty: {it.quantity} {it.unit}
                    </span>
                    {hasL1 && (
                      <span className="font-bold text-emerald-700 dark:text-emerald-400">
                        ₹{it.l1_unit_rate.toLocaleString("en-IN")}
                      </span>
                    )}
                  </div>
                </button>
              );
            })}
          </div>
        </div>

        {/* Right Column: Comparative Statement for Selected Item */}
        <div className="lg:col-span-2 space-y-4">
          {selectedItem ? (
            <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
              <div className="border-b border-slate-100 pb-4 dark:border-slate-800">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <span className="text-[10px] font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
                      Price Comparative Table (PCT)
                    </span>
                    <h3 className="text-base font-bold text-slate-900 dark:text-white">
                      {selectedItem.description}
                    </h3>
                  </div>
                  {canWrite && (
                    <button
                      type="button"
                      onClick={() => setShowAddQuote(true)}
                      className={primaryBtn + " text-xs"}
                    >
                      <PlusIcon className="h-4 w-4" />
                      Add Quote
                    </button>
                  )}
                </div>

                <div className="mt-2 flex flex-wrap items-center gap-4 text-xs text-slate-600 dark:text-slate-400">
                  <span>
                    Scope Qty: <strong className="text-slate-900 dark:text-white">{selectedItem.quantity} {selectedItem.unit}</strong>
                  </span>
                  <span>
                    Approved Makes: <strong className="text-slate-900 dark:text-white">{selectedItem.approved_makes || "Standard / As per spec"}</strong>
                  </span>
                  {selectedItem.l1_unit_rate > 0 && (
                    <span className="rounded-full bg-emerald-50 px-2.5 py-0.5 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300 font-semibold">
                      Current L-1: ₹{selectedItem.l1_unit_rate.toLocaleString("en-IN")} ({selectedItem.l1_vendor_name})
                    </span>
                  )}
                </div>
              </div>

              {/* Quotations List */}
              {quotes.length === 0 ? (
                <div className="py-12 text-center">
                  <CurrencyRupeeIcon className="mx-auto h-10 w-10 text-slate-300 dark:text-slate-600" />
                  <p className="mt-2 text-sm font-semibold text-slate-700 dark:text-slate-300">
                    No vendor quotations received yet
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    Float RFQs to suppliers on the approved make list and log their rates here.
                  </p>
                  {canWrite && (
                    <button
                      type="button"
                      onClick={() => setShowAddQuote(true)}
                      className={primaryBtn + " mt-4 text-xs"}
                    >
                      <PlusIcon className="h-4 w-4" />
                      Add First Quote
                    </button>
                  )}
                </div>
              ) : (
                <div className="mt-4 overflow-x-auto">
                  <table className="w-full text-left text-xs">
                    <thead className="border-b border-slate-200 bg-slate-50 text-[11px] font-bold uppercase text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300">
                      <tr>
                        <th className="py-3 px-3">Vendor / Supplier</th>
                        <th className="py-3 px-3">Make Offered</th>
                        <th className="py-3 px-3">Basic Unit Rate</th>
                        <th className="py-3 px-3">F&I / GST</th>
                        <th className="py-3 px-3">Total Package</th>
                        <th className="py-3 px-3">Compliance</th>
                        <th className="py-3 px-3 text-right">Action / L-1</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                      {quotes.map((q, idx) => {
                        const totalPkg =
                          (q.unit_rate + q.freight_and_insurance) *
                          Number(selectedItem.quantity);
                        return (
                          <tr
                            key={q.id}
                            className={`${
                              q.is_l1
                                ? "bg-emerald-50/50 dark:bg-emerald-950/20"
                                : "hover:bg-slate-50/50 dark:hover:bg-slate-850"
                            }`}
                          >
                            <td className="py-3 px-3">
                              <div className="flex items-center gap-1.5 font-bold text-slate-900 dark:text-white">
                                <span>{q.vendor_name}</span>
                                {q.is_l1 && (
                                  <CheckBadgeIcon className="h-4 w-4 text-emerald-600" title="Selected L-1" />
                                )}
                              </div>
                              {q.quote_ref && (
                                <span className="text-[10px] text-slate-400 font-mono">Ref: {q.quote_ref}</span>
                              )}
                            </td>
                            <td className="py-3 px-3">
                              <span className="text-slate-700 dark:text-slate-300">
                                {q.make_offered || "-"}
                              </span>
                            </td>
                            <td className="py-3 px-3 whitespace-nowrap">
                              <span className="font-bold text-slate-900 dark:text-white">
                                ₹{q.unit_rate.toLocaleString("en-IN")}
                              </span>
                            </td>
                            <td className="py-3 px-3 text-slate-500 whitespace-nowrap">
                              <span>F&I: ₹{q.freight_and_insurance}</span>
                              <span className="block text-[10px]">GST: {q.gst_percent}%</span>
                            </td>
                            <td className="py-3 px-3 whitespace-nowrap">
                              <span className="font-bold text-slate-900 dark:text-white">
                                ₹{totalPkg.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
                              </span>
                            </td>
                            <td className="py-3 px-3">
                              {q.is_technically_compliant ? (
                                <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300">
                                  Compliant
                                </span>
                              ) : (
                                <span className="rounded-full bg-red-50 px-2 py-0.5 text-[10px] font-semibold text-red-700 dark:bg-red-950 dark:text-red-300">
                                  Deviated
                                </span>
                              )}
                            </td>
                            <td className="py-3 px-3 text-right whitespace-nowrap">
                              <div className="flex items-center justify-end gap-2">
                                {q.is_l1 ? (
                                  <span className="rounded-lg bg-emerald-600 px-3 py-1 text-[11px] font-bold text-white shadow-xs">
                                    L-1 Selected
                                  </span>
                                ) : (
                                  canWrite && (
                                    <button
                                      type="button"
                                      disabled={!q.is_technically_compliant || saving}
                                      onClick={() => handleSelectL1(q.id)}
                                      className="rounded-lg border border-emerald-600 px-2.5 py-1 text-xs font-semibold text-emerald-700 hover:bg-emerald-50 dark:border-emerald-500 dark:text-emerald-300 dark:hover:bg-emerald-950/40"
                                      title="Select this rate as lowest compliant"
                                    >
                                      Select L-1
                                    </button>
                                  )
                                )}
                                {canWrite && (
                                  <button
                                    type="button"
                                    onClick={() => handleDeleteQuote(q.id)}
                                    className="p-1 text-slate-400 hover:text-red-600"
                                    title="Delete quote"
                                  >
                                    <TrashIcon className="h-4 w-4" />
                                  </button>
                                )}
                              </div>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          ) : (
            <div className="py-12 text-center text-xs text-slate-500">
              Please add or select a BOQ item on the left to review quotations.
            </div>
          )}

          {/* Step continuation action */}
          <div className="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between dark:border-slate-800 dark:bg-slate-900">
            <div>
              <h4 className="text-sm font-bold text-slate-900 dark:text-white">
                Ready for Step 7 · Project Working Costing & Bid Budget
              </h4>
              <p className="mt-0.5 text-xs text-slate-500">
                Aggregate all L-1 material costs, factor in freight, civil works, overheads, and target profit margin.
              </p>
            </div>
            <button
              type="button"
              onClick={() => onNavigateStep?.("CV-07")}
              className={primaryBtn + " gap-2 px-5 py-2 text-sm"}
            >
              Continue to Step 7 · Working Budget
              <ArrowRightIcon className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Modal: Add Quote */}
      {showAddQuote && selectedItem && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-xl rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900">
            <h3 className="text-lg font-bold text-slate-900 dark:text-white">
              Add Vendor Quotation
            </h3>
            <p className="mt-1 text-xs text-slate-500">
              Item: <strong>{selectedItem.description}</strong> (Qty: {selectedItem.quantity} {selectedItem.unit})
            </p>

            <ValidatedForm onSubmit={handleAddQuote} className="mt-4 space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Vendor / Supplier Name *
                  </ValidatedLabel>
                  <ValidatedInput
                    type="text"
                    required
                    placeholder="e.g. Schneider Electric India Pvt Ltd"
                    value={vendorName}
                    onChange={(e) => setVendorName(e.target.value)}
                    className={inputClass}
                  />
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Brand Make Offered
                  </ValidatedLabel>
                  <ValidatedInput
                    type="text"
                    placeholder="e.g. Schneider / Areva"
                    value={makeOffered}
                    onChange={(e) => setMakeOffered(e.target.value)}
                    className={inputClass}
                  />
                </div>
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Basic Unit Rate (₹) *
                  </ValidatedLabel>
                  <ValidatedInput
                    type="number"
                    step="0.01"
                    min="0"
                    required
                    placeholder="e.g. 210000"
                    value={unitRate}
                    onChange={(e) => setUnitRate(e.target.value === "" ? "" : Number(e.target.value))}
                    className={inputClass}
                  />
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    F&I per Unit (₹)
                  </ValidatedLabel>
                  <ValidatedInput
                    type="number"
                    step="0.01"
                    min="0"
                    placeholder="e.g. 5000"
                    value={freight}
                    onChange={(e) => setFreight(e.target.value === "" ? "" : Number(e.target.value))}
                    className={inputClass}
                  />
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    GST Rate (%)
                  </ValidatedLabel>
                  <ValidatedSelect
                    value={gstPercent}
                    onChange={(e) => setGstPercent(Number(e.target.value))}
                    className={inputClass}
                  >
                    <option value={0}>0%</option>
                    <option value={5}>5%</option>
                    <option value={12}>12%</option>
                    <option value={18}>18%</option>
                    <option value={28}>28%</option>
                  </ValidatedSelect>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Delivery Lead Time (Weeks)
                  </ValidatedLabel>
                  <ValidatedInput
                    type="number"
                    min="1"
                    value={deliveryWeeks}
                    onChange={(e) => setDeliveryWeeks(Number(e.target.value))}
                    className={inputClass}
                  />
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Supplier Quote Reference No.
                  </ValidatedLabel>
                  <ValidatedInput
                    type="text"
                    placeholder="e.g. SE/QT/2026/8912"
                    value={quoteRef}
                    onChange={(e) => setQuoteRef(e.target.value)}
                    className={inputClass}
                  />
                </div>
              </div>

              <div>
                <ValidatedLabel className="flex items-center gap-2 text-xs font-semibold text-slate-700 dark:text-slate-300">
                  <ValidatedInput
                    type="checkbox"
                    checked={isCompliant}
                    onChange={(e) => setIsCompliant(e.target.checked)}
                    className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                  />
                  <span>Technically compliant with tender specifications & QAP</span>
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Commercial Terms & Scope Remarks
                </ValidatedLabel>
                <ValidatedInput
                  type="text"
                  placeholder="e.g. Ex-works rate; testing charges included; 10% advance"
                  value={remarks}
                  onChange={(e) => setRemarks(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddQuote(false)}
                  className={secondaryBtn + " text-xs"}
                  disabled={saving}
                >
                  Cancel
                </button>
                <button type="submit" disabled={saving} className={primaryBtn + " text-xs"}>
                  {saving ? "Saving…" : "Save Quotation"}
                </button>
              </div>
            </ValidatedForm>
          </div>
        </div>
      )}
    </div>
  );
}
