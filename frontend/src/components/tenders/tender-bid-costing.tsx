"use client";

import { useEffect, useState, useCallback } from "react";
import {
  ValidatedLabel,
  ValidatedInput,
} from "@/components/ui/validated-controls";
import {
  CalculatorIcon,
  CheckBadgeIcon,
  ArrowRightIcon,
  CurrencyRupeeIcon,
  ShieldCheckIcon,
  ChartBarIcon,
  ArrowPathIcon,
} from "@heroicons/react/24/outline";

export type BidCosting = {
  id: string;
  draft_id: string;
  material_cost: number;
  freight_cost: number;
  civil_and_erection_cost: number;
  contingency_percent: number;
  contingency_amount: number;
  overhead_percent: number;
  overhead_amount: number;
  margin_percent: number;
  margin_amount: number;
  total_bid_value: number;
  status: "draft" | "approved" | "submitted" | "awarded" | "lost";
  submission_date: string | null;
  portal_reference: string;
  award_date: string | null;
  contract_reference: string;
  project_id: string | null;
  approved_by_id: string | null;
  approved_at: string | null;
  created_at: string;
  updated_at: string;
  version: number;
};

const primaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white transition hover:bg-blue-800 disabled:opacity-50";
const successBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-emerald-600 px-4 text-sm font-bold text-white transition hover:bg-emerald-700 disabled:opacity-50";
const secondaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200";
const inputClass =
  "mt-1 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm text-slate-900 transition focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white";

export function TenderBidCosting({
  draftId,
  canWrite = true,
  onNavigateStep,
}: {
  draftId: string;
  canWrite?: boolean;
  onNavigateStep?: (step: string) => void;
}) {
  const [costing, setCosting] = useState<BidCosting | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  // Form states
  const [materialCost, setMaterialCost] = useState<number>(0);
  const [freightCost, setFreightCost] = useState<number>(0);
  const [civilCost, setCivilCost] = useState<number>(0);
  const [contingencyPct, setContingencyPct] = useState<number>(3);
  const [overheadPct, setOverheadPct] = useState<number>(5);
  const [marginPct, setMarginPct] = useState<number>(10);

  const loadCosting = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/bid-costing`, { signal });
      if (!res.ok) throw new Error("Failed to load bid costing data");
      const data: BidCosting = await res.json();
      setCosting(data);
      setMaterialCost(data.material_cost || 0);
      setFreightCost(data.freight_cost || 0);
      setCivilCost(data.civil_and_erection_cost || 0);
      setContingencyPct(data.contingency_percent || 3);
      setOverheadPct(data.overhead_percent || 5);
      setMarginPct(data.margin_percent || 10);
    } catch (e) {
      if (!signal?.aborted) setError(e instanceof Error ? e.message : "Error loading costing sheet");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [draftId]);

  useEffect(() => {
    const c = new AbortController();
    const initial = setTimeout(() => void loadCosting(c.signal), 0);
    return () => { clearTimeout(initial); c.abort(); };
  }, [loadCosting]);

  // Derived calculations
  const directMaterial = Number(materialCost) || 0;
  const directFreight = Number(freightCost) || 0;
  const directCivil = Number(civilCost) || 0;
  const baseDirectTotal = directMaterial + directFreight + directCivil;

  const contingencyAmount = baseDirectTotal * (contingencyPct / 100);
  const overheadAmount = baseDirectTotal * (overheadPct / 100);
  const subtotalNetCost = baseDirectTotal + contingencyAmount + overheadAmount;

  const marginAmount = subtotalNetCost * (marginPct / 100);
  const finalBidPrice = subtotalNetCost + marginAmount;

  async function handleSave(statusToSet: "draft" | "approved" = "draft") {
    setSaving(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/bid-costing`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          material_cost: directMaterial,
          freight_cost: directFreight,
          civil_and_erection_cost: directCivil,
          contingency_percent: contingencyPct,
          overhead_percent: overheadPct,
          margin_percent: marginPct,
          status: statusToSet,
        }),
      });

      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.detail || "Failed to save costing");
      }

      await loadCosting();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  const isApproved = costing?.status === "approved" || costing?.status === "submitted" || costing?.status === "awarded";

  return (
    <div className="space-y-6">
      {/* Header Card */}
      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-100 text-sm font-bold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                7
              </span>
              <h2 className="text-xl font-bold text-slate-900 dark:text-white">
                Project Working Costing & Bid Budget
              </h2>
            </div>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              Consolidate L-1 purchase rates with site execution, freight, overheads, contingency, and target profitability to arrive at the competitive bid price.
            </p>
          </div>

          <div className="flex items-center gap-2">
            {costing?.status && (
              <span
                className={`rounded-full px-3 py-1 text-xs font-bold uppercase ${
                  isApproved
                    ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300"
                    : "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300"
                }`}
              >
                Status: {costing.status}
              </span>
            )}
          </div>
        </div>

        {/* Top Summary Banner */}
        <div className="mt-5 rounded-xl border border-blue-100 bg-gradient-to-r from-blue-50/70 to-indigo-50/70 p-4 dark:border-blue-900/40 dark:from-blue-950/30 dark:to-indigo-950/30">
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
            <div>
              <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
                Final Estimated Tender Bid Price
              </span>
              <p className="text-3xl font-extrabold text-slate-900 dark:text-white">
                ₹{finalBidPrice.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </p>
              <p className="text-xs text-slate-500">
                Net Margin: ₹{marginAmount.toLocaleString("en-IN", { maximumFractionDigits: 0 })} ({marginPct}%)
              </p>
            </div>
            {canWrite && !isApproved && (
              <button
                type="button"
                disabled={saving}
                onClick={() => void handleSave("approved")}
                className={successBtn + " text-xs"}
              >
                <CheckBadgeIcon className="h-5 w-5" />
                Approve Bid Budget
              </button>
            )}
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-xs font-semibold text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-300">
          {error}
        </div>
      )}

      {/* Main Form & Calculation Grid */}
      <div className="grid gap-6 lg:grid-cols-2">
        {/* Left Column: Direct Costs & Margin Inputs */}
        <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 space-y-5">
          <h3 className="text-base font-bold text-slate-900 dark:text-white border-b border-slate-100 pb-3 dark:border-slate-800">
            Cost Components & Markups
          </h3>

          <div className="space-y-4 text-xs">
            {/* Material Cost */}
            <div>
              <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                Direct Materials Cost (L-1 Sourced) (₹)
              </ValidatedLabel>
              <p className="text-[11px] text-slate-400">
                Auto-aggregated from Step 6 L-1 vendor rates. You may also adjust for manual contingencies.
              </p>
              <ValidatedInput
                type="number"
                step="0.01"
                min="0"
                value={materialCost}
                onChange={(e) => setMaterialCost(Number(e.target.value))}
                className={inputClass}
              />
            </div>

            {/* Freight Cost */}
            <div>
              <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                Transit Logistics & Heavy Freight (₹)
              </ValidatedLabel>
              <ValidatedInput
                type="number"
                step="0.01"
                min="0"
                placeholder="e.g. 150000"
                value={freightCost}
                onChange={(e) => setFreightCost(Number(e.target.value))}
                className={inputClass}
              />
            </div>

            {/* Civil & Erection Works */}
            <div>
              <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                Civil Works, Installation, Testing & Commissioning (₹)
              </ValidatedLabel>
              <ValidatedInput
                type="number"
                step="0.01"
                min="0"
                placeholder="e.g. 450000"
                value={civilCost}
                onChange={(e) => setCivilCost(Number(e.target.value))}
                className={inputClass}
              />
            </div>

            {/* Contingency % */}
            <div className="grid grid-cols-2 gap-3 pt-2">
              <div>
                <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                  Unforeseen Contingency (%)
                </ValidatedLabel>
                <ValidatedInput
                  type="number"
                  step="0.1"
                  min="0"
                  max="50"
                  value={contingencyPct}
                  onChange={(e) => setContingencyPct(Number(e.target.value))}
                  className={inputClass}
                />
              </div>
              <div>
                <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                  Project Overheads (%)
                </ValidatedLabel>
                <ValidatedInput
                  type="number"
                  step="0.1"
                  min="0"
                  max="50"
                  value={overheadPct}
                  onChange={(e) => setOverheadPct(Number(e.target.value))}
                  className={inputClass}
                />
              </div>
            </div>

            {/* Target Margin % */}
            <div>
              <ValidatedLabel className="block font-bold uppercase text-slate-600 dark:text-slate-400">
                Target Bid Profit Margin (%)
              </ValidatedLabel>
              <ValidatedInput
                type="number"
                step="0.1"
                min="0"
                max="100"
                value={marginPct}
                onChange={(e) => setMarginPct(Number(e.target.value))}
                className={inputClass}
              />
            </div>

            {canWrite && (
              <div className="pt-2 flex justify-end">
                <button
                  type="button"
                  disabled={saving}
                  onClick={() => void handleSave("draft")}
                  className={secondaryBtn + " text-xs"}
                >
                  Save Draft Changes
                </button>
              </div>
            )}
          </div>
        </div>

        {/* Right Column: Detailed Breakdown Table */}
        <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 space-y-4">
          <h3 className="text-base font-bold text-slate-900 dark:text-white border-b border-slate-100 pb-3 dark:border-slate-800">
            Costing Sheet Breakdown
          </h3>

          <div className="divide-y divide-slate-100 text-xs dark:divide-slate-800">
            <div className="flex justify-between py-2.5">
              <span className="text-slate-600 dark:text-slate-400">1. Direct Materials (L-1)</span>
              <strong className="text-slate-900 dark:text-white">
                ₹{directMaterial.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>

            <div className="flex justify-between py-2.5">
              <span className="text-slate-600 dark:text-slate-400">2. Freight & Transit Logistics</span>
              <strong className="text-slate-900 dark:text-white">
                ₹{directFreight.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>

            <div className="flex justify-between py-2.5">
              <span className="text-slate-600 dark:text-slate-400">3. Civil & Erection Works</span>
              <strong className="text-slate-900 dark:text-white">
                ₹{directCivil.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>

            <div className="flex justify-between py-2.5 bg-slate-50 px-2 rounded-lg dark:bg-slate-850">
              <span className="font-bold text-slate-700 dark:text-slate-300">Total Direct Cost (1+2+3)</span>
              <strong className="font-bold text-slate-900 dark:text-white">
                ₹{baseDirectTotal.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>

            <div className="flex justify-between py-2.5">
              <span className="text-slate-600 dark:text-slate-400">
                4. Contingency ({contingencyPct}%)
              </span>
              <strong className="text-slate-900 dark:text-white">
                ₹{contingencyAmount.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>

            <div className="flex justify-between py-2.5">
              <span className="text-slate-600 dark:text-slate-400">
                5. Project Overheads ({overheadPct}%)
              </span>
              <strong className="text-slate-900 dark:text-white">
                ₹{overheadAmount.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>

            <div className="flex justify-between py-2.5 bg-slate-50 px-2 rounded-lg dark:bg-slate-850">
              <span className="font-bold text-slate-700 dark:text-slate-300">Subtotal Net Cost</span>
              <strong className="font-bold text-slate-900 dark:text-white">
                ₹{subtotalNetCost.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>

            <div className="flex justify-between py-2.5 text-emerald-700 dark:text-emerald-400">
              <span className="font-semibold">6. Target Profit Margin ({marginPct}%)</span>
              <strong className="font-bold">
                + ₹{marginAmount.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>

            <div className="flex justify-between py-3.5 bg-blue-50 px-3 rounded-xl dark:bg-blue-950/40 text-blue-950 dark:text-blue-100">
              <span className="text-sm font-extrabold uppercase">Final Tender Bid Price</span>
              <strong className="text-base font-extrabold">
                ₹{finalBidPrice.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
              </strong>
            </div>
          </div>
        </div>
      </div>

      {/* Step continuation action */}
      <div className="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between dark:border-slate-800 dark:bg-slate-900">
        <div>
          <h4 className="text-sm font-bold text-slate-900 dark:text-white">
            Ready for Step 8 · Tender Submission & Project Handoff
          </h4>
          <p className="mt-0.5 text-xs text-slate-500">
            Record online portal submission and execute one-click project creation upon tender award.
          </p>
        </div>
        <button
          type="button"
          onClick={() => onNavigateStep?.("CV-08")}
          className={primaryBtn + " gap-2 px-5 py-2 text-sm"}
        >
          Continue to Step 8 · Submission & Award
          <ArrowRightIcon className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
