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
  ListBulletIcon,
  PlusIcon,
  PencilSquareIcon,
  TrashIcon,
  ArrowRightIcon,
  TagIcon,
  BuildingOfficeIcon,
  ArrowPathIcon,
} from "@heroicons/react/24/outline";

export type BidItem = {
  id: string;
  draft_id: string;
  item_code: string;
  description: string;
  category: string;
  unit: string;
  quantity: number;
  approved_makes: string;
  estimated_rate: number;
  l1_vendor_name: string;
  l1_unit_rate: number;
  l1_total_amount: number;
  sort_order: number;
  created_at: string;
  updated_at: string;
  version: number;
};

const CATEGORIES = [
  "Electrical - HT/LT Switchgear & Panels",
  "Electrical - Cables & Conductors",
  "Electrical - Transformers & Inverters",
  "Mechanical - Structures & Modules",
  "Civil - Foundation & Erection Works",
  "Instrumentation & Automation",
  "Freight & Transportation",
  "General / Miscellaneous",
];

const UNITS = ["NOS", "SET", "MTR", "KM", "LOT", "MT", "SQM", "JOB"];

const primaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white transition hover:bg-blue-800 disabled:opacity-50";
const secondaryBtn =
  "inline-flex min-h-11 items-center justify-center gap-2 rounded-xl border border-slate-300 bg-white px-4 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200";
const inputClass =
  "mt-1 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm text-slate-900 transition focus:border-blue-500 focus:outline-none dark:border-slate-700 dark:bg-slate-800 dark:text-white";

export function TenderBOQItems({
  draftId,
  canWrite = true,
  onNavigateStep,
}: {
  draftId: string;
  canWrite?: boolean;
  onNavigateStep?: (step: string) => void;
}) {
  const [items, setItems] = useState<BidItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showModal, setShowModal] = useState(false);
  const [editingItem, setEditingItem] = useState<BidItem | null>(null);

  // Form states
  const [itemCode, setItemCode] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState(CATEGORIES[0]);
  const [unit, setUnit] = useState("NOS");
  const [quantity, setQuantity] = useState<number | "">("");
  const [approvedMakes, setApprovedMakes] = useState("");
  const [estimatedRate, setEstimatedRate] = useState<number | "">("");
  const [saving, setSaving] = useState(false);

  const loadItems = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError("");
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/bid-items`, { signal });
      if (!res.ok) throw new Error("Failed to load tender BOQ items");
      const data = await res.json();
      setItems(data);
    } catch (e) {
      if (!signal?.aborted) setError(e instanceof Error ? e.message : "Error loading items");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [draftId]);

  useEffect(() => {
    const c = new AbortController();
    const initial = setTimeout(() => void loadItems(c.signal), 0);
    return () => { clearTimeout(initial); c.abort(); };
  }, [loadItems]);

  function openCreate() {
    setEditingItem(null);
    setItemCode("");
    setDescription("");
    setCategory(CATEGORIES[0]);
    setUnit("NOS");
    setQuantity(1);
    setApprovedMakes("");
    setEstimatedRate("");
    setShowModal(true);
  }

  function openEdit(item: BidItem) {
    setEditingItem(item);
    setItemCode(item.item_code);
    setDescription(item.description);
    setCategory(item.category);
    setUnit(item.unit);
    setQuantity(item.quantity);
    setApprovedMakes(item.approved_makes);
    setEstimatedRate(item.estimated_rate || "");
    setShowModal(true);
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!description.trim() || saving) return;
    setSaving(true);
    setError("");
    try {
      const payload = {
        item_code: itemCode,
        description,
        category,
        unit,
        quantity: Number(quantity) || 1,
        approved_makes: approvedMakes,
        estimated_rate: Number(estimatedRate) || 0,
      };

      const url = editingItem
        ? `/api/tender-fms/drafts/${draftId}/bid-items/${editingItem.id}`
        : `/api/tender-fms/drafts/${draftId}/bid-items`;
      const method = editingItem ? "PATCH" : "POST";

      const res = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.detail || "Failed to save item");
      }

      setShowModal(false);
      await loadItems();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to save BOQ item");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(itemId: string) {
    if (!confirm("Are you sure you want to delete this BOQ item?")) return;
    try {
      const res = await fetch(`/api/tender-fms/drafts/${draftId}/bid-items/${itemId}`, {
        method: "DELETE",
      });
      if (!res.ok) throw new Error("Delete failed");
      await loadItems();
      window.dispatchEvent(new Event("tender-progress-saved"));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delete failed");
    }
  }

  const totalEstimatedCost = items.reduce(
    (sum, item) => sum + (Number(item.quantity) * Number(item.estimated_rate || 0)),
    0
  );
  const itemsWithL1 = items.filter((i) => i.l1_unit_rate > 0).length;
  const totalL1Amount = items.reduce((sum, item) => sum + Number(item.l1_total_amount || 0), 0);

  return (
    <div className="space-y-6">
      {/* Header Card */}
      <div className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <span className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-100 text-sm font-bold text-blue-700 dark:bg-blue-950 dark:text-blue-300">
                5
              </span>
              <h2 className="text-xl font-bold text-slate-900 dark:text-white">
                Tender BOQ Items & Approved Make List
              </h2>
            </div>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              List the materials and packages required to be purchased, tag mandated approved brands (Make List), and monitor L-1 market pricing.
            </p>
          </div>

          <div className="flex items-center gap-2">
            {canWrite && (
              <button type="button" onClick={openCreate} className={primaryBtn + " text-xs"}>
                <PlusIcon className="h-4 w-4" />
                Add BOQ Item
              </button>
            )}
          </div>
        </div>

        {/* Stats strip */}
        <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div className="rounded-xl border border-slate-100 bg-slate-50 p-3 dark:border-slate-800 dark:bg-slate-850">
            <span className="text-[11px] font-semibold text-slate-500 uppercase">Total BOQ Items</span>
            <p className="text-lg font-bold text-slate-900 dark:text-white">{items.length}</p>
          </div>
          <div className="rounded-xl border border-blue-100 bg-blue-50/60 p-3 dark:border-blue-900/30 dark:bg-blue-950/20">
            <span className="text-[11px] font-semibold text-blue-700 dark:text-blue-300 uppercase">Estimated Budget</span>
            <p className="text-lg font-bold text-blue-800 dark:text-blue-200">
              ₹{totalEstimatedCost.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
            </p>
          </div>
          <div className="rounded-xl border border-emerald-100 bg-emerald-50/60 p-3 dark:border-emerald-900/30 dark:bg-emerald-950/20">
            <span className="text-[11px] font-semibold text-emerald-700 dark:text-emerald-300 uppercase">L-1 Sourced Items</span>
            <p className="text-lg font-bold text-emerald-800 dark:text-emerald-200">
              {itemsWithL1} / {items.length}
            </p>
          </div>
          <div className="rounded-xl border border-purple-100 bg-purple-50/60 p-3 dark:border-purple-900/30 dark:bg-purple-950/20">
            <span className="text-[11px] font-semibold text-purple-700 dark:text-purple-300 uppercase">L-1 Material Total</span>
            <p className="text-lg font-bold text-purple-800 dark:text-purple-200">
              ₹{totalL1Amount.toLocaleString("en-IN", { maximumFractionDigits: 0 })}
            </p>
          </div>
        </div>
      </div>

      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-xs font-semibold text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-300">
          {error}
        </div>
      )}

      {/* Items Table Card */}
      <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="flex items-center justify-between border-b border-slate-100 pb-3 dark:border-slate-800">
          <h3 className="text-sm font-bold text-slate-900 dark:text-white">
            Purchasable Items & Packages ({items.length})
          </h3>
          <button
            type="button"
            onClick={() => void loadItems()}
            className="text-xs font-semibold text-blue-700 hover:underline dark:text-blue-400"
          >
            <ArrowPathIcon className="inline h-3.5 w-3.5 mr-1" />
            Refresh
          </button>
        </div>

        {loading ? (
          <div className="py-12 text-center text-xs text-slate-500">Loading BOQ items…</div>
        ) : items.length === 0 ? (
          <div className="py-12 text-center">
            <ListBulletIcon className="mx-auto h-10 w-10 text-slate-300 dark:text-slate-600" />
            <p className="mt-2 text-sm font-semibold text-slate-700 dark:text-slate-300">
              No items added to tender BOQ yet
            </p>
            <p className="mt-1 text-xs text-slate-500">
              Add the key items to be procured along with their tender-approved brand makes.
            </p>
            {canWrite && (
              <button type="button" onClick={openCreate} className={primaryBtn + " mt-4 text-xs"}>
                <PlusIcon className="h-4 w-4" />
                Add First Item
              </button>
            )}
          </div>
        ) : (
          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="border-b border-slate-200 bg-slate-50 text-[11px] font-bold uppercase text-slate-600 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300">
                <tr>
                  <th className="py-3 px-3">Item / Scope</th>
                  <th className="py-3 px-3">Category</th>
                  <th className="py-3 px-3">Qty & Unit</th>
                  <th className="py-3 px-3">Approved Makes</th>
                  <th className="py-3 px-3">Est. Rate</th>
                  <th className="py-3 px-3">L-1 Sourced Rate</th>
                  <th className="py-3 px-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                {items.map((item) => (
                  <tr key={item.id} className="hover:bg-slate-50/50 dark:hover:bg-slate-850">
                    <td className="py-3 px-3 max-w-xs">
                      {item.item_code && (
                        <span className="font-mono text-[10px] text-slate-400 block">{item.item_code}</span>
                      )}
                      <p className="font-semibold text-slate-900 dark:text-white line-clamp-2">
                        {item.description}
                      </p>
                    </td>
                    <td className="py-3 px-3">
                      <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-semibold text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                        {item.category.split(" - ")[0]}
                      </span>
                    </td>
                    <td className="py-3 px-3 whitespace-nowrap">
                      <span className="font-bold text-slate-900 dark:text-white">{item.quantity}</span>{" "}
                      <span className="text-slate-500 text-[11px]">{item.unit}</span>
                    </td>
                    <td className="py-3 px-3 max-w-xs">
                      {item.approved_makes ? (
                        <div className="flex items-center gap-1 text-slate-700 dark:text-slate-300">
                          <BuildingOfficeIcon className="h-3.5 w-3.5 shrink-0 text-slate-400" />
                          <span className="line-clamp-2">{item.approved_makes}</span>
                        </div>
                      ) : (
                        <span className="text-slate-400 italic">Open / As per spec</span>
                      )}
                    </td>
                    <td className="py-3 px-3 whitespace-nowrap">
                      {item.estimated_rate ? (
                        <span className="text-slate-600 dark:text-slate-400">
                          ₹{item.estimated_rate.toLocaleString("en-IN")}
                        </span>
                      ) : (
                        <span className="text-slate-400">-</span>
                      )}
                    </td>
                    <td className="py-3 px-3 whitespace-nowrap">
                      {item.l1_unit_rate > 0 ? (
                        <div>
                          <span className="font-bold text-emerald-700 dark:text-emerald-400">
                            ₹{item.l1_unit_rate.toLocaleString("en-IN")}
                          </span>
                          <span className="block text-[10px] text-slate-500">
                            by {item.l1_vendor_name}
                          </span>
                        </div>
                      ) : (
                        <span className="rounded-full bg-amber-50 px-2 py-0.5 text-[10px] font-semibold text-amber-700 dark:bg-amber-950 dark:text-amber-300">
                          Pending quotes
                        </span>
                      )}
                    </td>
                    <td className="py-3 px-3 text-right whitespace-nowrap">
                      <div className="flex items-center justify-end gap-1">
                        <button
                          type="button"
                          onClick={() => openEdit(item)}
                          className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-700 dark:hover:bg-slate-800"
                          title="Edit item"
                        >
                          <PencilSquareIcon className="h-4 w-4" />
                        </button>
                        <button
                          type="button"
                          onClick={() => handleDelete(item.id)}
                          className="rounded-lg p-1.5 text-slate-400 hover:bg-red-50 hover:text-red-700 dark:hover:bg-red-950/40"
                          title="Delete item"
                        >
                          <TrashIcon className="h-4 w-4" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Step continuation action */}
      <div className="flex flex-col gap-3 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between dark:border-slate-800 dark:bg-slate-900">
        <div>
          <h4 className="text-sm font-bold text-slate-900 dark:text-white">
            Ready for Step 6 · Vendor Quotations & L-1 Comparison
          </h4>
          <p className="mt-0.5 text-xs text-slate-500">
            Log vendor quotations received from the approved make list and select lowest compliant (L-1) quotes.
          </p>
        </div>
        <button
          type="button"
          onClick={() => onNavigateStep?.("CV-03")}
          className={primaryBtn + " gap-2 px-5 py-2 text-sm"}
        >
          Continue to Step 6 · Vendor Quotes & L-1
          <ArrowRightIcon className="h-4 w-4" />
        </button>
      </div>

      {/* Modal: Add/Edit Item */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="w-full max-w-xl rounded-2xl bg-white p-6 shadow-xl dark:bg-slate-900">
            <h3 className="text-lg font-bold text-slate-900 dark:text-white">
              {editingItem ? "Edit Tender BOQ Item" : "Add Tender BOQ Item"}
            </h3>
            <p className="mt-1 text-xs text-slate-500">
              Specify the material or package details and mandated approved brand makes.
            </p>

            <ValidatedForm onSubmit={handleSave} className="mt-4 space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Item Code / Reference
                  </ValidatedLabel>
                  <ValidatedInput
                    type="text"
                    placeholder="e.g. BOQ-EL-01"
                    value={itemCode}
                    onChange={(e) => setItemCode(e.target.value)}
                    className={inputClass}
                  />
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Category
                  </ValidatedLabel>
                  <ValidatedSelect
                    value={category}
                    onChange={(e) => setCategory(e.target.value)}
                    className={inputClass}
                  >
                    {CATEGORIES.map((c) => (
                      <option key={c} value={c}>
                        {c}
                      </option>
                    ))}
                  </ValidatedSelect>
                </div>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Item Description & Technical Scope *
                </ValidatedLabel>
                <ValidatedTextarea
                  required
                  rows={3}
                  placeholder="e.g. 11kV 630A 25kA Outdoor Vacuum Circuit Breaker with metering and protection CT/PT..."
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Quantity *
                  </ValidatedLabel>
                  <ValidatedInput
                    type="number"
                    step="0.01"
                    min="0.01"
                    required
                    value={quantity}
                    onChange={(e) => setQuantity(e.target.value === "" ? "" : Number(e.target.value))}
                    className={inputClass}
                  />
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Unit of Measurement *
                  </ValidatedLabel>
                  <ValidatedSelect
                    value={unit}
                    onChange={(e) => setUnit(e.target.value)}
                    className={inputClass}
                  >
                    {UNITS.map((u) => (
                      <option key={u} value={u}>
                        {u}
                      </option>
                    ))}
                  </ValidatedSelect>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                    Est. Budget Rate (₹)
                  </ValidatedLabel>
                  <ValidatedInput
                    type="number"
                    step="0.01"
                    min="0"
                    placeholder="e.g. 250000"
                    value={estimatedRate}
                    onChange={(e) => setEstimatedRate(e.target.value === "" ? "" : Number(e.target.value))}
                    className={inputClass}
                  />
                </div>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold uppercase text-slate-600 dark:text-slate-400">
                  Approved Makes / Brands (Tender Make List)
                </ValidatedLabel>
                <ValidatedInput
                  type="text"
                  placeholder="e.g. ABB / Schneider / Siemens / L&T"
                  value={approvedMakes}
                  onChange={(e) => setApprovedMakes(e.target.value)}
                  className={inputClass}
                />
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className={secondaryBtn + " text-xs"}
                  disabled={saving}
                >
                  Cancel
                </button>
                <button type="submit" disabled={saving} className={primaryBtn + " text-xs"}>
                  {saving ? "Saving…" : "Save Item"}
                </button>
              </div>
            </ValidatedForm>
          </div>
        </div>
      )}
    </div>
  );
}
