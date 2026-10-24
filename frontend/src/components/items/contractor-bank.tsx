"use client";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { RequiredSelection, ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";

import { useEffect, useState, type FormEvent } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { input, button, secondary } from "./item-data";
import { RecordDrawer } from "@/components/ui/record-drawer";

export type PaymentTerms = {
  advance_pct: number;
  ra_bill_pct?: number;
  progress_pct?: number;
  retention_pct: number;
  credit_days: number;
  notes?: string;
};

export type Contractor = {
  id: string;
  code: string;
  name: string;
  contact_person: string;
  phone: string;
  email: string;
  trade_capabilities: string[];
  pan: string;
  gstin: string;
  esi_code: string;
  epf_code: string;
  labor_license_no: string;
  payment_terms: PaymentTerms;
  rating: number;
  data: Record<string, unknown>;
  version: number;
  is_active: boolean;
  created_at: string;
  created_by_id?: string | null;
  updated_at: string;
  updated_by_id?: string | null;
};

export const COMMON_TRADES = [
  { key: "ht_lt_electrical", label: "HT / LT Electrical" },
  { key: "cable_laying", label: "Cable Laying" },
  { key: "transformer_erection", label: "Transformer Erection" },
  { key: "switchgear_installation", label: "Switchgear Installation" },
  { key: "transmission_line", label: "Transmission Line" },
  { key: "civil_foundations", label: "Civil Foundations" },
  { key: "cable_trench_excavation", label: "Cable Trench Excavation" },
  { key: "solar_mms_assembly", label: "Solar MMS Assembly" },
  { key: "solar_module_mounting", label: "Solar Module Mounting" },
  { key: "inverter_commissioning", label: "Inverter Commissioning" },
  { key: "earthing_lightning_protection", label: "Earthing & Lightning Protection" },
  { key: "street_lighting", label: "Street Lighting" },
  { key: "testing_commissioning", label: "Testing & Commissioning" },
  { key: "general_labor", label: "General Labor" },
];

export function tradeLabel(tradeKey: string): string {
  const found = COMMON_TRADES.find((t) => t.key === tradeKey.toLowerCase());
  if (found) return found.label;
  return tradeKey.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

export function ContractorBank() {
  const user = useAuthenticatedUser();
  const canWrite =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.contractors.write");

  const [contractors, setContractors] = useState<Contractor[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "inactive">("active");
  const [selectedTrade, setSelectedTrade] = useState<string>("all");

  // Drawers
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingContractor, setEditingContractor] = useState<Contractor | null>(null);
  const [historyContractor, setHistoryContractor] = useState<Contractor | null>(null);

  // Form state
  const [formCode, setFormCode] = useState("");
  const [formName, setFormName] = useState("");
  const [formContactPerson, setFormContactPerson] = useState("");
  const [formPhone, setFormPhone] = useState("");
  const [formEmail, setFormEmail] = useState("");
  const [formLocation, setFormLocation] = useState("");
  const [formTrades, setFormTrades] = useState<string[]>([]);
  const [newTradeInput, setNewTradeInput] = useState("");
  const [formPan, setFormPan] = useState("");
  const [formGstin, setFormGstin] = useState("");
  const [formEsi, setFormEsi] = useState("");
  const [formEpf, setFormEpf] = useState("");
  const [formLaborLicense, setFormLaborLicense] = useState("");
  const [formAdvancePct, setFormAdvancePct] = useState(10);
  const [formProgressPct, setFormProgressPct] = useState(80);
  const [formRetentionPct, setFormRetentionPct] = useState(10);
  const [formCreditDays, setFormCreditDays] = useState(15);
  const [formPaymentNotes, setFormPaymentNotes] = useState("");
  const [formRating, setFormRating] = useState(5);
  const [formReason, setFormReason] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [reload, setReload] = useState(0);

  useEffect(() => {
    let live = true;
    fetch("/api/settings/contractors", { cache: "no-store" })
      .then(async (res) => {
        if (!res.ok) throw new Error("Failed to load contractors");
        return res.json();
      })
      .then((data) => {
        if (live) {
          setContractors(data);
          setError(null);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (live) {
          setError(err instanceof Error ? err.message : "Error loading contractors");
          setLoading(false);
        }
      });
    return () => {
      live = false;
    };
  }, [reload]);

  const openCreateDrawer = () => {
    if (!canWrite) return;
    setEditingContractor(null);
    setFormCode(`CONT-${String(contractors.length + 1).padStart(3, "0")}`);
    setFormName("");
    setFormContactPerson("");
    setFormPhone("");
    setFormEmail("");
    setFormTrades([]);
    setFormLocation("");
    setNewTradeInput("");
    setFormPan("");
    setFormGstin("");
    setFormEsi("");
    setFormEpf("");
    setFormLaborLicense("");
    setFormAdvancePct(10);
    setFormProgressPct(80);
    setFormRetentionPct(10);
    setFormCreditDays(15);
    setFormPaymentNotes("Milestone measurement certified by VTPL PE before each release");
    setFormRating(5);
    setFormReason("");
    setFormError(null);
    setDrawerOpen(true);
  };

  const openEditDrawer = (contractor: Contractor) => {
    if (!canWrite) return;
    setEditingContractor(contractor);
    setFormCode(contractor.code);
    setFormName(contractor.name);
    setFormContactPerson(contractor.contact_person);
    setFormPhone(contractor.phone);
    setFormEmail(contractor.email);
    setFormTrades(contractor.trade_capabilities || []);
    setFormLocation(String(contractor.data?.location ?? ""));
    setNewTradeInput("");
    setFormPan(contractor.pan || "");
    setFormGstin(contractor.gstin || "");
    setFormEsi(contractor.esi_code || "");
    setFormEpf(contractor.epf_code || "");
    setFormLaborLicense(contractor.labor_license_no || "");
    setFormAdvancePct(contractor.payment_terms?.advance_pct ?? 10);
    setFormProgressPct(
      contractor.payment_terms?.ra_bill_pct ?? contractor.payment_terms?.progress_pct ?? 80
    );
    setFormRetentionPct(contractor.payment_terms?.retention_pct ?? 10);
    setFormCreditDays(contractor.payment_terms?.credit_days ?? 15);
    setFormPaymentNotes(
      (contractor.data?.notes as string | undefined) ??
        contractor.payment_terms?.notes ??
        ""
    );
    setFormRating(contractor.rating || 5);
    setFormReason("");
    setFormError(null);
    setDrawerOpen(true);
  };

  const toggleTrade = (trade: string) => {
    setFormTrades((curr) =>
      curr.includes(trade) ? curr.filter((t) => t !== trade) : [...curr, trade]
    );
  };

  const addCustomTrade = () => {
    const trimmed = newTradeInput.trim().toLowerCase().replace(/[^a-z0-9]+/g, "_");
    if (trimmed && !formTrades.includes(trimmed)) {
      setFormTrades([...formTrades, trimmed]);
      setNewTradeInput("");
    }
  };

  const [tradeError, setTradeError] = useState("");
  const [termsError, setTermsError] = useState("");
  const handleSave = async (e: FormEvent) => {
    const submittedForm = e.currentTarget instanceof HTMLFormElement ? e.currentTarget : undefined;
    e.preventDefault();
    setFormError(null);

    // Validate percentage sum
    const totalPct = Number(formAdvancePct) + Number(formProgressPct) + Number(formRetentionPct);
    if (Math.abs(totalPct - 100) > 0.01) {
      setTermsError(`Payment milestone percentages must equal 100%. Current sum: ${totalPct}%`);
      return;
    }

    if (editingContractor && !formReason.trim()) {
      setFormError("A reason for modifying this contractor is required for audit traceability.");
      return;
    }

    if (formTrades.length === 0) {
      setTradeError("At least one trade capability must be selected.");
      return;
    }

    setSubmitting(true);
    try {
      let res: Response;
      if (editingContractor) {
        const updatePayload = {
          name: formName.trim(),
          contact_person: formContactPerson.trim(),
          phone: formPhone.trim(),
          email: formEmail.trim(),
          trade_capabilities: formTrades,
          pan: formPan.trim().toUpperCase(),
          gstin: formGstin.trim().toUpperCase(),
          esi_code: formEsi.trim(),
          epf_code: formEpf.trim(),
          labor_license_no: formLaborLicense.trim(),
          payment_terms: {
            advance_pct: Number(formAdvancePct),
            ra_bill_pct: Number(formProgressPct),
            retention_pct: Number(formRetentionPct),
            credit_days: Number(formCreditDays),
          },
          data: { ...editingContractor.data, location: formLocation.trim(), notes: formPaymentNotes.trim() },
          version: editingContractor.version,
          reason: formReason.trim(),
        };
        res = await fetch(`/api/settings/contractors/${editingContractor.id}`, {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(updatePayload),
        });
      } else {
        const createPayload = {
          code: formCode.trim().toUpperCase(),
          name: formName.trim(),
          contact_person: formContactPerson.trim(),
          phone: formPhone.trim(),
          email: formEmail.trim(),
          trade_capabilities: formTrades,
          pan: formPan.trim().toUpperCase(),
          gstin: formGstin.trim().toUpperCase(),
          esi_code: formEsi.trim(),
          epf_code: formEpf.trim(),
          labor_license_no: formLaborLicense.trim(),
          payment_terms: {
            advance_pct: Number(formAdvancePct),
            ra_bill_pct: Number(formProgressPct),
            retention_pct: Number(formRetentionPct),
            credit_days: Number(formCreditDays),
          },
          data: { location: formLocation.trim(), notes: formPaymentNotes.trim() },
          reason: formReason.trim() || "Initial contractor onboarding",
        };
        res = await fetch("/api/settings/contractors", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(createPayload),
        });
      }

      if (!res.ok) {
        const errorData = await res.json().catch(() => ({}));
        if (displayServerErrors(submittedForm, errorData)) return;
        throw new Error(typeof errorData.detail === "string" ? errorData.detail : "Failed to save contractor.");
      }

      setDrawerOpen(false);
      setReload((r) => r + 1);
    } catch (err: unknown) {
      setFormError(err instanceof Error ? err.message : "Error saving contractor");
    } finally {
      setSubmitting(false);
    }
  };

  const handleToggleActive = async (contractor: Contractor) => {
    const nextState = !contractor.is_active;
    const actionLabel = nextState ? "reactivate" : "deactivate";
    const reason = window.prompt(
      `Please provide an audit reason to ${actionLabel} contractor "${contractor.name}":`
    );
    if (!reason || !reason.trim()) return;

    try {
      const res = await fetch(`/api/settings/contractors/${contractor.id}/active`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          is_active: nextState,
          version: contractor.version,
          reason: reason.trim(),
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        alert(err.detail || `Failed to ${actionLabel} contractor.`);
        return;
      }
      setReload((r) => r + 1);
    } catch {
      alert(`Network error while trying to ${actionLabel} contractor.`);
    }
  };

  const filtered = contractors.filter((c) => {
    if (statusFilter === "active" && !c.is_active) return false;
    if (statusFilter === "inactive" && c.is_active) return false;
    if (selectedTrade !== "all" && !c.trade_capabilities?.includes(selectedTrade)) return false;
    if (search.trim()) {
      const q = search.toLowerCase();
      return (
        c.code.toLowerCase().includes(q) ||
        c.name.toLowerCase().includes(q) ||
        c.contact_person.toLowerCase().includes(q) ||
        c.phone.toLowerCase().includes(q) ||
        c.trade_capabilities?.some((t) => t.toLowerCase().includes(q)) ||
        c.pan.toLowerCase().includes(q) ||
        c.gstin.toLowerCase().includes(q)
      );
    }
    return true;
  });

  return (
    <div className="space-y-6">
      {/* Top Banner */}
      <div className="rounded-2xl border border-slate-200 bg-white p-5 sm:p-6 shadow-sm">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <span className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-50 text-indigo-700 font-bold text-sm">
                👷
              </span>
              <h2 className="text-xl font-extrabold text-slate-900">
                Contractor Master & Labor Bank
              </h2>
            </div>
            
          </div>
          {canWrite && (
            <button
              onClick={openCreateDrawer}
              className={`${button} shrink-0 bg-blue-700 hover:bg-blue-800 text-white font-bold py-2.5 px-4 rounded-xl shadow`}
            >
              + Register Contractor
            </button>
          )}
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-white p-4 rounded-xl border border-slate-200">
        <div className="flex flex-wrap items-center gap-3">
          <ValidatedInput name="search"
            type="search"
            placeholder="Search by code, contractor name, trade, PAN..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-72 rounded-lg border border-slate-300 px-3 py-1.5 text-sm outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500"
          />

          <ValidatedSelect name="selected_trade"
            value={selectedTrade}
            onChange={(e) => setSelectedTrade(e.target.value)}
            className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm bg-white outline-none focus:border-blue-500"
          >
            <option value="all">All Trade Capabilities</option>
            {COMMON_TRADES.map(({ key, label }) => (
              <option key={key} value={key}>
                {label}
              </option>
            ))}
          </ValidatedSelect>

          <div className="flex items-center rounded-lg border border-slate-200 bg-slate-50 p-1 text-xs font-semibold">
            {(["active", "inactive", "all"] as const).map((mode) => (
              <button
                key={mode}
                onClick={() => setStatusFilter(mode)}
                className={`rounded px-2.5 py-1 capitalize transition ${
                  statusFilter === mode ? "bg-white text-blue-700 shadow-sm" : "text-slate-600"
                }`}
              >
                {mode}
              </button>
            ))}
          </div>
        </div>

        <div className="text-xs text-slate-500 font-medium">
          Showing {filtered.length} of {contractors.length} contractors
        </div>
      </div>

      {/* Grid or Loader */}
      {loading ? (
        <div className="p-12 text-center text-slate-500">Loading contractor master...</div>
      ) : error ? (
        <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm font-semibold text-red-700">
          {error}
        </div>
      ) : filtered.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-12 text-center">
          <p className="text-sm font-semibold text-slate-700">No contractors found.</p>
          <p className="mt-1 text-xs text-slate-500">
            {search ? "Try adjusting your search criteria." : "Register your first contractor above."}
          </p>
        </div>
      ) : (
        <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
          <table className="vtpl-data-table w-full text-left text-sm text-slate-700">
            <thead className="border-b border-slate-200 bg-slate-50 text-xs font-bold uppercase tracking-wider text-slate-500">
              <tr>
                <th className="px-4 py-3">Contractor / Code</th>
                <th className="px-4 py-3">Trade Capabilities</th>
                <th className="px-4 py-3">Contact Person & Phone</th>
                <th className="px-4 py-3">Statutory (ESI/PF/Labor)</th>
                <th className="px-4 py-3">Payment Terms</th>
                <th className="px-4 py-3">Rating</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {filtered.map((item) => (
                <tr key={item.id} className="hover:bg-slate-50/70 transition-colors">
                  <td className="px-4 py-3">
                    <div className="font-bold text-slate-900">{item.name}</div>
                    <div className="font-mono text-xs text-blue-700 font-medium">{item.code}</div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1 max-w-xs">
                      {item.trade_capabilities && item.trade_capabilities.length > 0 ? (
                        item.trade_capabilities.map((trade) => (
                          <span
                            key={trade}
                            className="inline-flex items-center rounded-md bg-indigo-50 px-2 py-0.5 text-[11px] font-semibold text-indigo-700 border border-indigo-100"
                          >
                            {tradeLabel(trade)}
                          </span>
                        ))
                      ) : (
                        <span className="text-xs text-slate-400 italic">No trades specified</span>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="font-medium text-slate-800">{item.contact_person || "—"}</div>
                    <div className="text-xs text-slate-500">{item.phone}</div>
                    {item.email && <div className="text-xs text-slate-400">{item.email}</div>}
                  </td>
                  <td className="px-4 py-3 text-xs">
                    <div>
                       <span className="text-slate-500 font-medium">ESI:</span>{" "}
                      <span className="font-mono">{item.esi_code || "—"}</span>
                    </div>
                    <div>
                      <span className="text-slate-500 font-medium">EPF:</span>{" "}
                      <span className="font-mono">{item.epf_code || "—"}</span>
                    </div>
                    <div>
                      <span className="text-slate-500 font-medium">Lic:</span>{" "}
                      <span className="font-mono">{item.labor_license_no || "—"}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-xs">
                    <div className="font-semibold text-slate-800">
                      {item.payment_terms?.advance_pct ?? 10}% Adv /{" "}
                      {item.payment_terms?.ra_bill_pct ?? item.payment_terms?.progress_pct ?? 80}% Prog /{" "}
                      {item.payment_terms?.retention_pct ?? 10}% Ret
                    </div>
                    <div className="text-[11px] text-slate-500">
                      Credit: {item.payment_terms?.credit_days ?? 15} days
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-1 text-amber-500 font-bold text-xs">
                      <span>★</span>
                      <span className="text-slate-700">{item.rating?.toFixed(1) || "5.0"}</span>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold ${
                        item.is_active
                          ? "bg-emerald-50 text-emerald-700 border border-emerald-200"
                          : "bg-slate-100 text-slate-600 border border-slate-200"
                      }`}
                    >
                      {item.is_active ? "Active" : "Inactive"}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-2">
                      {canWrite ? <button
                        onClick={() => openEditDrawer(item)}
                        className="rounded-lg border border-slate-200 px-2.5 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                      >
                        Edit
                      </button> : null}
                      <button
                        onClick={() => setHistoryContractor(item)}
                        className="rounded-lg border border-slate-200 px-2.5 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                      >
                        History
                      </button>
                      {canWrite && (
                        <button
                          onClick={() => handleToggleActive(item)}
                          className={`rounded-lg border px-2.5 py-1 text-xs font-semibold ${
                            item.is_active
                              ? "border-red-200 text-red-700 hover:bg-red-50"
                              : "border-emerald-200 text-emerald-700 hover:bg-emerald-50"
                          }`}
                        >
                          {item.is_active ? "Deactivate" : "Activate"}
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Drawer: Add / Edit Contractor */}
      {drawerOpen && (
        <RecordDrawer
          onClose={() => setDrawerOpen(false)}
          title={editingContractor ? `Edit Contractor: ${editingContractor.name}` : "Register New Contractor"}
        >
        <ValidatedForm onSubmit={handleSave} className="space-y-5 p-1">
          {formError && (
            <div className="rounded-xl border border-red-200 bg-red-50 p-3 text-xs font-bold text-red-700">
              {formError}
            </div>
          )}

          {/* Section 1: Basic Information */}
          <div className="space-y-3">
            <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500 border-b pb-1">
              Contractor Information
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Contractor Code *

                <ValidatedInput name="code"
                  required
                  value={formCode}
                  onChange={(e) => setFormCode(e.target.value.toUpperCase())}
                  placeholder="CONT-001"
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Location
                <ValidatedInput name="location"
                  value={formLocation}
                  onChange={(e) => setFormLocation(e.target.value)}
                  placeholder="City / operating area"
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Contractor / Firm Name *

                <ValidatedInput name="name"
                  required
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  placeholder="e.g. Shyam Electrical Works"
                  className={input}
                /></ValidatedLabel>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Contact Person *

                <ValidatedInput name="contact_person"
                  required
                  value={formContactPerson}
                  onChange={(e) => setFormContactPerson(e.target.value)}
                  placeholder="Proprietor / Supervisor"
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Mobile Phone *

                <ValidatedInput name="phone"
                  required
                  value={formPhone}
                  onChange={(e) => setFormPhone(e.target.value)}
                  placeholder="+91 98765 43210"
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Email Address

                <ValidatedInput name="email"
                  type="email"
                  value={formEmail}
                  onChange={(e) => setFormEmail(e.target.value)}
                  placeholder="contractor@domain.com"
                  className={input}
                /></ValidatedLabel>
              </div>
            </div>
          </div>

          {/* Section 2: Trade Capabilities */}
          <div className="space-y-2">
            <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500 border-b pb-1">
              Trade Specializations & Capabilities<span className="text-red-600" aria-hidden="true"> *</span>
            </h4>
            <p className="text-xs text-slate-500">
              Select all scopes of work this contractor is equipped and licensed to execute:
            </p>
            <RequiredSelection as="div" selectionCount={formTrades.length} onInvalid={() => setTradeError("At least one trade capability must be selected.")} className="flex flex-wrap gap-2 pt-1">
              {COMMON_TRADES.map(({ key, label }) => {
                const selected = formTrades.includes(key);
                return (
                  <button
                    type="button"
                    key={key}
                    onClick={() => toggleTrade(key)}
                    className={`rounded-lg px-3 py-1.5 text-xs font-semibold border transition ${
                      selected
                        ? "bg-indigo-600 text-white border-indigo-600 shadow-sm"
                        : "bg-slate-50 text-slate-700 border-slate-200 hover:bg-slate-100"
                    }`}
                  >
                    {selected ? "✓ " : "+ "}
                    {label}
                  </button>
                );
              })}
            </RequiredSelection>
            {tradeError && !formTrades.length ? <p role="alert" className="text-sm text-red-600">{tradeError}</p> : null}
            {/* Custom Trade Input */}
            <div className="flex items-center gap-2 pt-2">
              <ValidatedInput name="new_trade_input"
                value={newTradeInput}
                onChange={(e) => setNewTradeInput(e.target.value)}
                placeholder="Other trade (e.g. Scaffolding, HT Splicing)..."
                className="rounded-lg border border-slate-300 px-3 py-1 text-xs outline-none focus:border-blue-500"
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    addCustomTrade();
                  }
                }}
              />
              <button
                type="button"
                onClick={addCustomTrade}
                className="rounded-lg bg-slate-100 px-3 py-1 text-xs font-bold text-slate-700 hover:bg-slate-200"
              >
                Add Trade
              </button>
            </div>
          </div>

          {/* Section 3: Statutory Compliance */}
          <div className="space-y-3">
            <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500 border-b pb-1">
              Statutory Compliance & Legal Registrations
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">PAN Number
                <ValidatedInput name="pan"
                  value={formPan}
                  onChange={(e) => setFormPan(e.target.value.toUpperCase())}
                  placeholder="AAAAA1234A"
                  maxLength={10}
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">GSTIN
                <ValidatedInput name="gstin"
                  value={formGstin}
                  onChange={(e) => setFormGstin(e.target.value.toUpperCase())}
                  placeholder="24AAAAA1234A1Z5"
                  maxLength={15}
                  className={input}
                /></ValidatedLabel>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">ESI Code
                <ValidatedInput name="esi"
                  value={formEsi}
                  onChange={(e) => setFormEsi(e.target.value)}
                  placeholder="e.g. 31000123450000999"
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">EPF Code
                <ValidatedInput name="epf"
                  value={formEpf}
                  onChange={(e) => setFormEpf(e.target.value)}
                  placeholder="e.g. GJ/AHD/0012345"
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Labour License No.

                <ValidatedInput name="labor_license"
                  value={formLaborLicense}
                  onChange={(e) => setFormLaborLicense(e.target.value)}
                  placeholder="e.g. ALC/2026/045"
                  className={input}
                /></ValidatedLabel>
              </div>
            </div>
          </div>

          {/* Section 4: Milestone Payment Terms */}
          <div className="space-y-3">
            <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500 border-b pb-1">
              Standard Milestone Payment Terms (Must sum to 100%)
            </h4>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Advance (%)

                <ValidatedInput name="advance_pct" error={Math.abs(Number(formAdvancePct) + Number(formProgressPct) + Number(formRetentionPct) - 100) > 0.01 ? termsError : ""}
                  type="number"
                  min="0"
                  max="100"
                  step="0.5"
                  value={formAdvancePct}
                  onChange={(e) => setFormAdvancePct(Number(e.target.value))}
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Progress RA Bill (%)

                <ValidatedInput name="progress_pct" error={Math.abs(Number(formAdvancePct) + Number(formProgressPct) + Number(formRetentionPct) - 100) > 0.01 ? termsError : ""}
                  type="number"
                  min="0"
                  max="100"
                  step="0.5"
                  value={formProgressPct}
                  onChange={(e) => setFormProgressPct(Number(e.target.value))}
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Retention / Handover (%)

                <ValidatedInput name="retention_pct" error={Math.abs(Number(formAdvancePct) + Number(formProgressPct) + Number(formRetentionPct) - 100) > 0.01 ? termsError : ""}
                  type="number"
                  min="0"
                  max="100"
                  step="0.5"
                  value={formRetentionPct}
                  onChange={(e) => setFormRetentionPct(Number(e.target.value))}
                  className={input}
                /></ValidatedLabel>
              </div>
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Credit Days

                <ValidatedInput name="credit_days"
                  type="number"
                  min="0"
                  max="180"
                  value={formCreditDays}
                  onChange={(e) => setFormCreditDays(Number(e.target.value))}
                  className={input}
                /></ValidatedLabel>
              </div>
            </div>

            <div className="flex items-center justify-between text-xs font-semibold px-2 py-1.5 rounded-lg bg-slate-50 border border-slate-200">
              <span className="text-slate-600">Total Milestone Percentage:</span>
              <span
                className={
                  Math.abs(Number(formAdvancePct) + Number(formProgressPct) + Number(formRetentionPct) - 100) < 0.01
                    ? "text-emerald-700 font-bold"
                    : "text-red-700 font-bold"
                }
              >
                {Number(formAdvancePct) + Number(formProgressPct) + Number(formRetentionPct)}% / 100%
              </span>
            </div>

            <div>
              <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                Payment Terms Notes

              <ValidatedTextarea name="payment_notes"
                rows={2}
                value={formPaymentNotes}
                onChange={(e) => setFormPaymentNotes(e.target.value)}
                placeholder="e.g. Paid weekly against joint measurement sheet verified by Site In-charge."
                className={input}
              /></ValidatedLabel>
            </div>
          </div>

          {/* Section 5: Rating & Reason */}
          <div className="space-y-3">
            <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500 border-b pb-1">
              Performance Rating & Audit Trail
            </h4>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Performance Rating (1.0 to 5.0)

                <ValidatedInput name="rating"
                  type="number"
                  min="1"
                  max="5"
                  step="0.1"
                  value={formRating}
                  onChange={(e) => setFormRating(Number(e.target.value))}
                  className={input}
                /></ValidatedLabel>
              </div>
            </div>

            {editingContractor && (
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Audit Reason for Modification *

                <ValidatedTextarea name="reason"
                  required
                  rows={2}
                  value={formReason}
                  onChange={(e) => setFormReason(e.target.value)}
                  placeholder="Explain why this contractor profile or terms are being updated..."
                  className={input}
                /></ValidatedLabel>
              </div>
            )}
          </div>

          <div className="flex items-center justify-end gap-3 pt-4 border-t">
            <button
              type="button"
              onClick={() => setDrawerOpen(false)}
              className={secondary}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submitting}
              className={`${button} bg-blue-700 text-white font-bold py-2 px-5 rounded-xl`}
            >
              {submitting ? "Saving..." : editingContractor ? "Update Contractor" : "Register Contractor"}
            </button>
          </div>
        </ValidatedForm>
      </RecordDrawer>
      )}

      {/* Drawer: Audit History */}
      {historyContractor && (
        <RecordDrawer
          onClose={() => setHistoryContractor(null)}
          title={`Audit Log: ${historyContractor.name}`}
        >
          <ContractorAuditLog contractorId={historyContractor.id} />
        </RecordDrawer>
      )}
    </div>
  );
}

function ContractorAuditLog({ contractorId }: { contractorId: string }) {
  const [events, setEvents] = useState<
    Array<{
      id: string;
      action: string;
      reason: string;
      created_at: string;
      created_by_id: string | null;
      before: Record<string, unknown>;
      after: Record<string, unknown>;
    }>
  >([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetch(`/api/settings/contractors/${contractorId}/history`)
      .then((r) => r.json())
      .then((data) => setEvents(Array.isArray(data) ? data : []))
      .catch(() => setEvents([]))
      .finally(() => setLoading(false));
  }, [contractorId]);

  if (loading) {
    return <div className="p-6 text-sm text-slate-500">Loading audit trail...</div>;
  }

  if (events.length === 0) {
    return (
      <div className="p-6 text-sm text-slate-500 italic">
        No recorded modification events for this contractor.
      </div>
    );
  }

  return (
    <div className="space-y-4 p-2">
      {events.map((ev) => (
        <div key={ev.id} className="rounded-xl border border-slate-200 bg-white p-4 text-xs space-y-1.5 shadow-sm">
          <div className="flex items-center justify-between">
            <span className="font-bold text-slate-900 uppercase tracking-wide bg-slate-100 px-2 py-0.5 rounded">
              {ev.action}
            </span>
            <span className="text-slate-400 font-mono">
              {new Date(ev.created_at).toLocaleString()}
            </span>
          </div>
          {ev.reason && (
            <p className="text-slate-700 italic">
              <strong>Reason:</strong> {ev.reason}
            </p>
          )}
        </div>
      ))}
    </div>
  );
}
