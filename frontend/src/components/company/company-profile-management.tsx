"use client";

import { useEffect, useState, useCallback, useId } from "react";
import { actionButtonClass } from "@/components/ui/action-button";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import {
  ValidatedForm,
  ValidatedInput,
  ValidatedLabel,
  ValidatedSelect,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";
import {
  BuildingOffice2Icon,
  CurrencyRupeeIcon,
  ShieldCheckIcon,
  DocumentCheckIcon,
  BriefcaseIcon,
  PlusIcon,
  PencilSquareIcon,
  TrashIcon,
} from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import type {
  FullCompanySetup,
  CompanyProfile,
  CompanyFinancialYear,
  CompanyLicence,
  CompanyWorkExperience,
  CompanyCertificate,
} from "@/server/company/types";

const primary = actionButtonClass("primary");
const secondary = actionButtonClass("secondary");
const danger = actionButtonClass("danger");
const fieldClass = "mt-1 min-h-11 w-full rounded-xl border border-slate-300 bg-white p-3 text-sm text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100";

export function CompanyProfileManagement() {
  const user = useAuthenticatedUser();
  const canWrite =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("settings.company.write") ||
    user.permissions.includes("admin.organisation.write");

  const [data, setData] = useState<FullCompanySetup | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [tab, setTab] = useState<"general" | "statutory" | "financials" | "licences" | "experience">("general");

  // Drawer states
  const [editingYear, setEditingYear] = useState<CompanyFinancialYear | "new" | null>(null);
  const [editingLicence, setEditingLicence] = useState<CompanyLicence | "new" | null>(null);
  const [editingExperience, setEditingExperience] = useState<CompanyWorkExperience | "new" | null>(null);
  const [editingCert, setEditingCert] = useState<CompanyCertificate | "new" | null>(null);

  const refresh = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    try {
      const res = await fetch("/api/settings/company-profile", { cache: "no-store", signal });
      if (!res.ok) throw new Error("Unable to load company profile");
      const body = (await res.json()) as FullCompanySetup;
      setData(body);
      setError("");
    } catch (e) {
      if (!signal?.aborted) setError(e instanceof Error ? e.message : "Failed to load company profile");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, []);

  useEffect(() => {
    const c = new AbortController();
    const initial = setTimeout(() => void refresh(c.signal), 0);
    return () => { clearTimeout(initial); c.abort(); };
  }, [refresh]);

  async function saveProfile(updates: Partial<CompanyProfile>) {
    if (!data) return;
    setError("");
    setNotice("");
    try {
      const res = await fetch("/api/settings/company-profile", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...updates, expected_version: data.profile.version }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Unable to save profile");
      }
      setNotice("Company profile saved successfully.");
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Save failed");
    }
  }

  if (loading && !data) return <p className="p-6 text-sm text-slate-500">Loading company profile and eligibility setup…</p>;

  return (
    <div className="mx-auto w-full max-w-[1500px] space-y-6 p-4 sm:p-6">
      <header className="flex flex-col gap-4 border-b border-slate-200 pb-5 dark:border-slate-700 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-400">
            Organisation · Tender Qualification Setup
          </span>
          <h1 className="mt-1 text-2xl font-bold text-slate-900 dark:text-white">
            Organisation & Tender Eligibility Setup
          </h1>
          <p className="mt-1 text-sm text-slate-600 dark:text-slate-300">
            Maintain verified legal registrations, financial capacity, technical licences, and past project credentials required for the company to qualify and pass tenders.
          </p>
        </div>
        {data && (
          <div className="text-right text-xs text-slate-500">
            <span>Version {data.profile.version}</span>
            <span className="block">Updated: {new Date(data.profile.updated_at).toLocaleString()}</span>
          </div>
        )}
      </header>

      {notice && <p role="status" className="rounded-xl bg-blue-50 p-4 text-sm font-semibold text-blue-900 dark:bg-blue-950/40 dark:text-blue-200">{notice}</p>}
      {error && <p role="alert" className="rounded-xl bg-red-50 p-4 text-sm font-semibold text-red-900 dark:bg-red-950/40 dark:text-red-200">{error}</p>}

      {/* Tabs */}
      <nav aria-label="Company Profile Tabs" className="flex flex-wrap gap-2 border-b border-slate-200 pb-2 dark:border-slate-700">
        <button
          type="button"
          onClick={() => setTab("general")}
          className={`flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold ${
            tab === "general" ? "bg-blue-700 text-white shadow-sm" : "border border-slate-300 text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          }`}
        >
          <BuildingOffice2Icon className="h-4 w-4" /> Legal & Identification
        </button>
        <button
          type="button"
          onClick={() => setTab("statutory")}
          className={`flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold ${
            tab === "statutory" ? "bg-blue-700 text-white shadow-sm" : "border border-slate-300 text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          }`}
        >
          <ShieldCheckIcon className="h-4 w-4" /> MSME & Declarations
        </button>
        <button
          type="button"
          onClick={() => setTab("financials")}
          className={`flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold ${
            tab === "financials" ? "bg-blue-700 text-white shadow-sm" : "border border-slate-300 text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          }`}
        >
          <CurrencyRupeeIcon className="h-4 w-4" /> Financials & Solvency ({data?.financials.length ?? 0})
        </button>
        <button
          type="button"
          onClick={() => setTab("licences")}
          className={`flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold ${
            tab === "licences" ? "bg-blue-700 text-white shadow-sm" : "border border-slate-300 text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          }`}
        >
          <DocumentCheckIcon className="h-4 w-4" /> Electrical Licences ({data?.licences.length ?? 0})
        </button>
        <button
          type="button"
          onClick={() => setTab("experience")}
          className={`flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold ${
            tab === "experience" ? "bg-blue-700 text-white shadow-sm" : "border border-slate-300 text-slate-700 hover:bg-slate-100 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800"
          }`}
        >
          <BriefcaseIcon className="h-4 w-4" /> Past EPC Projects ({data?.work_experience.length ?? 0})
        </button>
      </nav>

      {/* Tab 1: General & Legal */}
      {tab === "general" && data && (
        <LegalTab profile={data.profile} canWrite={canWrite} onSave={saveProfile} />
      )}

      {/* Tab 2: MSME & Statutory */}
      {tab === "statutory" && data && (
        <StatutoryTab profile={data.profile} canWrite={canWrite} onSave={saveProfile} />
      )}

      {/* Tab 3: Financial Strength */}
      {tab === "financials" && data && (
        <FinancialsTab
          financials={data.financials}
          certificates={data.certificates}
          canWrite={canWrite}
          onAddYear={() => setEditingYear("new")}
          onEditYear={(y) => setEditingYear(y)}
          onAddCert={() => setEditingCert("new")}
          onEditCert={(c) => setEditingCert(c)}
          onRefresh={refresh}
        />
      )}

      {/* Tab 4: Licences */}
      {tab === "licences" && data && (
        <LicencesTab
          licences={data.licences}
          canWrite={canWrite}
          onAdd={() => setEditingLicence("new")}
          onEdit={(l) => setEditingLicence(l)}
          onRefresh={refresh}
        />
      )}

      {/* Tab 5: Work Experience */}
      {tab === "experience" && data && (
        <ExperienceTab
          experience={data.work_experience}
          canWrite={canWrite}
          onAdd={() => setEditingExperience("new")}
          onEdit={(w) => setEditingExperience(w)}
          onRefresh={refresh}
        />
      )}

      {/* Modals / Drawers for sub-records */}
      {editingYear && (
        <RecordDrawer title={editingYear === "new" ? "Add Financial Year" : `Edit FY ${editingYear.financial_year}`} onClose={() => setEditingYear(null)}>
          <FinancialYearForm
            item={editingYear === "new" ? undefined : editingYear}
            onSaved={() => { setEditingYear(null); void refresh(); }}
          />
        </RecordDrawer>
      )}

      {editingLicence && (
        <RecordDrawer title={editingLicence === "new" ? "Add Electrical Licence" : `Edit Licence ${editingLicence.licence_number}`} onClose={() => setEditingLicence(null)}>
          <LicenceForm
            item={editingLicence === "new" ? undefined : editingLicence}
            onSaved={() => { setEditingLicence(null); void refresh(); }}
          />
        </RecordDrawer>
      )}

      {editingExperience && (
        <RecordDrawer title={editingExperience === "new" ? "Add Completed Project" : `Edit Project: ${editingExperience.project_title}`} onClose={() => setEditingExperience(null)}>
          <ExperienceForm
            item={editingExperience === "new" ? undefined : editingExperience}
            onSaved={() => { setEditingExperience(null); void refresh(); }}
          />
        </RecordDrawer>
      )}

      {editingCert && (
        <RecordDrawer title={editingCert === "new" ? "Add Bank / Solvency Certificate" : `Edit Certificate`} onClose={() => setEditingCert(null)}>
          <CertificateForm
            item={editingCert === "new" ? undefined : editingCert}
            onSaved={() => { setEditingCert(null); void refresh(); }}
          />
        </RecordDrawer>
      )}
    </div>
  );
}

// ---------------- TAB 1: LEGAL & IDENTIFICATION ----------------
function LegalTab({ profile, canWrite, onSave }: { profile: CompanyProfile; canWrite: boolean; onSave: (u: Partial<CompanyProfile>) => Promise<void> }) {
  const [legalName, setLegalName] = useState(profile.legal_name || "");
  const [tradeName, setTradeName] = useState(profile.trade_name || "");
  const [cin, setCin] = useState(profile.cin || "");
  const [pan, setPan] = useState(profile.pan || "");
  const [gstin, setGstin] = useState(profile.gstin || "");
  const [incDate, setIncDate] = useState(profile.incorporation_date || "");
  const [address, setAddress] = useState(profile.registered_address || "");
  const [state, setState] = useState(profile.registered_state || "");
  const [city, setCity] = useState(profile.registered_city || "");
  const [pin, setPin] = useState(profile.registered_pincode || "");
  const [busy, setBusy] = useState(false);

  return (
    <ValidatedForm
      className="space-y-6 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        await onSave({
          legal_name: legalName,
          trade_name: tradeName,
          cin,
          pan,
          gstin,
          incorporation_date: incDate || null,
          registered_address: address,
          registered_state: state,
          registered_city: city,
          registered_pincode: pin,
        });
        setBusy(false);
      }}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <ValidatedLabel className="block text-sm font-semibold">
          Legal Company Name (as per Certificate of Incorporation)
          <ValidatedInput required value={legalName} onChange={(e) => setLegalName(e.target.value)} disabled={!canWrite || busy} className={fieldClass} />
        </ValidatedLabel>
        <ValidatedLabel className="block text-sm font-semibold">
          Trade / Brand Name
          <ValidatedInput value={tradeName} onChange={(e) => setTradeName(e.target.value)} disabled={!canWrite || busy} className={fieldClass} />
        </ValidatedLabel>
        <ValidatedLabel className="block text-sm font-semibold">
          Corporate Identity Number (CIN)
          <ValidatedInput value={cin} onChange={(e) => setCin(e.target.value)} disabled={!canWrite || busy} className={fieldClass} placeholder="e.g. U40106MH2015PTC123456" />
        </ValidatedLabel>
        <ValidatedLabel className="block text-sm font-semibold">
          Company PAN
          <ValidatedInput value={pan} onChange={(e) => setPan(e.target.value.toUpperCase())} disabled={!canWrite || busy} className={fieldClass} maxLength={10} placeholder="ABCDE1234F" />
        </ValidatedLabel>
        <ValidatedLabel className="block text-sm font-semibold">
          Primary GSTIN
          <ValidatedInput value={gstin} onChange={(e) => setGstin(e.target.value.toUpperCase())} disabled={!canWrite || busy} className={fieldClass} maxLength={15} placeholder="27ABCDE1234F1Z5" />
        </ValidatedLabel>
        <ValidatedLabel className="block text-sm font-semibold">
          Incorporation Date
          <ValidatedInput type="date" value={incDate} onChange={(e) => setIncDate(e.target.value)} disabled={!canWrite || busy} className={fieldClass} />
        </ValidatedLabel>
      </div>

      <div className="border-t border-slate-200 pt-4 dark:border-slate-800">
        <h3 className="mb-3 text-base font-bold">Registered Office Address</h3>
        <div className="grid gap-4 sm:grid-cols-3">
          <div className="sm:col-span-3">
            <ValidatedLabel className="block text-sm font-semibold">
              Full Address
              <ValidatedTextarea value={address} onChange={(e) => setAddress(e.target.value)} disabled={!canWrite || busy} className={fieldClass} rows={2} />
            </ValidatedLabel>
          </div>
          <ValidatedLabel className="block text-sm font-semibold">
            State
            <ValidatedInput value={state} onChange={(e) => setState(e.target.value)} disabled={!canWrite || busy} className={fieldClass} placeholder="Maharashtra" />
          </ValidatedLabel>
          <ValidatedLabel className="block text-sm font-semibold">
            City
            <ValidatedInput value={city} onChange={(e) => setCity(e.target.value)} disabled={!canWrite || busy} className={fieldClass} placeholder="Pune" />
          </ValidatedLabel>
          <ValidatedLabel className="block text-sm font-semibold">
            PIN Code
            <ValidatedInput value={pin} onChange={(e) => setPin(e.target.value)} disabled={!canWrite || busy} className={fieldClass} maxLength={6} placeholder="411001" />
          </ValidatedLabel>
        </div>
      </div>

      {canWrite && (
        <div className="flex justify-end pt-2">
          <button type="submit" disabled={busy} className={primary}>
            {busy ? "Saving…" : "Save Legal Details"}
          </button>
        </div>
      )}
    </ValidatedForm>
  );
}

// ---------------- TAB 2: MSME & STATUTORY ----------------
function StatutoryTab({ profile, canWrite, onSave }: { profile: CompanyProfile; canWrite: boolean; onSave: (u: Partial<CompanyProfile>) => Promise<void> }) {
  const [udyam, setUdyam] = useState(profile.msme_udyam_number || "");
  const [category, setCategory] = useState(profile.msme_category || "");
  const [activity, setActivity] = useState(profile.msme_activity || "");
  const [startup, setStartup] = useState(profile.startup_recognition_number || "");
  const [epfo, setEpfo] = useState(profile.epfo_registration || "");
  const [esic, setEsic] = useState(profile.esic_registration || "");
  const [iso9001, setIso9001] = useState(profile.iso_9001);
  const [iso14001, setIso14001] = useState(profile.iso_14001);
  const [iso45001, setIso45001] = useState(profile.iso_45001);
  const [cleanBlacklist, setCleanBlacklist] = useState(profile.non_blacklisted_declaration);
  const [cleanInsolvency, setCleanInsolvency] = useState(profile.clean_insolvency_declaration);
  const [busy, setBusy] = useState(false);

  return (
    <ValidatedForm
      className="space-y-6 rounded-2xl border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900"
      onSubmit={async (e) => {
        e.preventDefault();
        setBusy(true);
        await onSave({
          msme_udyam_number: udyam,
          msme_category: category as CompanyProfile["msme_category"],
          msme_activity: activity as CompanyProfile["msme_activity"],
          startup_recognition_number: startup,
          epfo_registration: epfo,
          esic_registration: esic,
          iso_9001: iso9001,
          iso_14001: iso14001,
          iso_45001: iso45001,
          non_blacklisted_declaration: cleanBlacklist,
          clean_insolvency_declaration: cleanInsolvency,
        });
        setBusy(false);
      }}
    >
      <div>
        <h3 className="text-base font-bold">MSME / Udyam & Startup Recognition (EMD Exemption)</h3>
        <p className="mt-1 text-xs text-slate-500">
          Essential for tenders granting 100% EMD and Tender Fee exemptions.
        </p>
        <div className="mt-4 grid gap-4 sm:grid-cols-3">
          <ValidatedLabel className="block text-sm font-semibold">
            Udyam Registration No.
            <ValidatedInput value={udyam} onChange={(e) => setUdyam(e.target.value.toUpperCase())} disabled={!canWrite || busy} className={fieldClass} placeholder="UDYAM-MH-12-0000000" />
          </ValidatedLabel>
          <ValidatedLabel className="block text-sm font-semibold">
            Enterprise Category
            <ValidatedSelect value={category} onChange={(e) => setCategory(e.target.value)} disabled={!canWrite || busy} className={fieldClass}>
              <option value="">Not Applicable / None</option>
              <option value="micro">Micro Enterprise</option>
              <option value="small">Small Enterprise</option>
              <option value="medium">Medium Enterprise</option>
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="block text-sm font-semibold">
            Primary Activity
            <ValidatedSelect value={activity} onChange={(e) => setActivity(e.target.value)} disabled={!canWrite || busy} className={fieldClass}>
              <option value="">None</option>
              <option value="services">Services (EPC, Installation)</option>
              <option value="manufacturing">Manufacturing</option>
              <option value="trading">Trading</option>
            </ValidatedSelect>
          </ValidatedLabel>
          <ValidatedLabel className="block text-sm font-semibold sm:col-span-3">
            Startup India (DPIIT) Recognition Number
            <ValidatedInput value={startup} onChange={(e) => setStartup(e.target.value)} disabled={!canWrite || busy} className={fieldClass} placeholder="DIPP12345" />
          </ValidatedLabel>
        </div>
      </div>

      <div className="border-t border-slate-200 pt-4 dark:border-slate-800">
        <h3 className="text-base font-bold">Labor & Statutory Registrations</h3>
        <div className="mt-4 grid gap-4 sm:grid-cols-2">
          <ValidatedLabel className="block text-sm font-semibold">
            EPFO (Provident Fund) Code No.
            <ValidatedInput value={epfo} onChange={(e) => setEpfo(e.target.value)} disabled={!canWrite || busy} className={fieldClass} placeholder="MH/PUN/0012345/000" />
          </ValidatedLabel>
          <ValidatedLabel className="block text-sm font-semibold">
            ESIC Registration No.
            <ValidatedInput value={esic} onChange={(e) => setEsic(e.target.value)} disabled={!canWrite || busy} className={fieldClass} placeholder="31001234560001001" />
          </ValidatedLabel>
        </div>
      </div>

      <div className="border-t border-slate-200 pt-4 dark:border-slate-800">
        <h3 className="text-base font-bold">ISO Certifications & Quality</h3>
        <div className="mt-3 flex flex-wrap gap-6">
          <ValidatedLabel className="flex items-center gap-2 text-sm font-medium">
            <ValidatedInput type="checkbox" checked={iso9001} onChange={(e) => setIso9001(e.target.checked)} disabled={!canWrite || busy} className="h-4 w-4 rounded" />
            ISO 9001:2015 (Quality Management System)
          </ValidatedLabel>
          <ValidatedLabel className="flex items-center gap-2 text-sm font-medium">
            <ValidatedInput type="checkbox" checked={iso14001} onChange={(e) => setIso14001(e.target.checked)} disabled={!canWrite || busy} className="h-4 w-4 rounded" />
            ISO 14001:2015 (Environmental Management)
          </ValidatedLabel>
          <ValidatedLabel className="flex items-center gap-2 text-sm font-medium">
            <ValidatedInput type="checkbox" checked={iso45001} onChange={(e) => setIso45001(e.target.checked)} disabled={!canWrite || busy} className="h-4 w-4 rounded" />
            ISO 45001:2018 (Occupational Health & Safety)
          </ValidatedLabel>
        </div>
      </div>

      <div className="border-t border-slate-200 pt-4 dark:border-slate-800">
        <h3 className="text-base font-bold">Integrity & Insolvency Declarations</h3>
        <div className="mt-3 space-y-3">
          <ValidatedLabel className="flex items-center gap-2 text-sm font-medium">
            <ValidatedInput type="checkbox" checked={cleanBlacklist} onChange={(e) => setCleanBlacklist(e.target.checked)} disabled={!canWrite || busy} className="h-4 w-4 rounded" />
            <strong>Clean Track Record:</strong> Company has not been debarred / blacklisted by any Government Department, PSU, or autonomous body.
          </ValidatedLabel>
          <ValidatedLabel className="flex items-center gap-2 text-sm font-medium">
            <ValidatedInput type="checkbox" checked={cleanInsolvency} onChange={(e) => setCleanInsolvency(e.target.checked)} disabled={!canWrite || busy} className="h-4 w-4 rounded" />
            <strong>Clean Insolvency:</strong> Company is not under Corporate Insolvency Resolution Process (CIRP) or NCLT liquidation.
          </ValidatedLabel>
        </div>
      </div>

      {canWrite && (
        <div className="flex justify-end pt-2">
          <button type="submit" disabled={busy} className={primary}>
            {busy ? "Saving…" : "Save Statutory & MSME"}
          </button>
        </div>
      )}
    </ValidatedForm>
  );
}

// ---------------- TAB 3: FINANCIALS & CERTIFICATES ----------------
function FinancialsTab({
  financials,
  certificates,
  canWrite,
  onAddYear,
  onEditYear,
  onAddCert,
  onEditCert,
  onRefresh,
}: {
  financials: CompanyFinancialYear[];
  certificates: CompanyCertificate[];
  canWrite: boolean;
  onAddYear: () => void;
  onEditYear: (y: CompanyFinancialYear) => void;
  onAddCert: () => void;
  onEditCert: (c: CompanyCertificate) => void;
  onRefresh: () => Promise<void>;
}) {
  const avgTurnover = financials.length > 0
    ? financials.slice(0, 3).reduce((sum, f) => sum + f.turnover_excluding_gst, 0) / Math.min(financials.length, 3)
    : 0;

  async function deleteYear(id: string) {
    if (!confirm("Delete this financial year record?")) return;
    await fetch(`/api/settings/company-profile/financials/${id}`, { method: "DELETE" });
    await onRefresh();
  }

  async function deleteCert(id: string) {
    if (!confirm("Deactivate this certificate?")) return;
    await fetch(`/api/settings/company-profile/certificates/${id}`, { method: "DELETE" });
    await onRefresh();
  }

  return (
    <div className="space-y-8">
      {/* Financial Summary KPI Box */}
      <div className="grid gap-4 sm:grid-cols-3">
        <div className="rounded-2xl border border-blue-200 bg-blue-50 p-5 dark:border-blue-900/50 dark:bg-blue-950/30">
          <span className="text-xs font-bold uppercase tracking-wider text-blue-700 dark:text-blue-300">
            3-Year Average Turnover (AATO)
          </span>
          <p className="mt-1 text-2xl font-black text-blue-900 dark:text-white">
            ₹{(avgTurnover / 10000000).toFixed(2)} Cr
          </p>
          <span className="text-xs text-slate-600 dark:text-slate-400">
            Arithmetic mean across last 3 audited years
          </span>
        </div>
        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-bold uppercase tracking-wider text-slate-500">
            Latest Audited Net Worth
          </span>
          <p className="mt-1 text-2xl font-black text-slate-900 dark:text-white">
            ₹{((financials[0]?.net_worth ?? 0) / 10000000).toFixed(2)} Cr
          </p>
          <span className="text-xs text-slate-500">FY {financials[0]?.financial_year ?? "N/A"}</span>
        </div>
        <div className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-bold uppercase tracking-wider text-slate-500">
            Solvency Certificate Limit
          </span>
          <p className="mt-1 text-2xl font-black text-slate-900 dark:text-white">
            ₹{((certificates.find((c) => c.kind === "solvency")?.amount ?? 0) / 10000000).toFixed(2)} Cr
          </p>
          <span className="text-xs text-slate-500">
            {certificates.find((c) => c.kind === "solvency")?.issuer_name ?? "No active certificate"}
          </span>
        </div>
      </div>

      {/* Year-by-Year Audited Accounts */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-lg font-bold">Annual Audited Financial Statements</h3>
            <p className="text-xs text-slate-500">Excluding GST. Used to establish turnover, net worth, and profitability.</p>
          </div>
          {canWrite && (
            <button type="button" onClick={onAddYear} className={primary}>
              <PlusIcon className="h-4 w-4" /> Add Financial Year
            </button>
          )}
        </div>

        <DataGrid
          label="Financial Years"
          rows={financials}
          columns={[
            { key: "year", label: "Financial Year", value: (y) => y.financial_year },
            {
              key: "turnover",
              label: "Turnover (Excl. GST)",
              value: (y) => y.turnover_excluding_gst,
              render: (y) => <strong>₹{(y.turnover_excluding_gst / 10000000).toFixed(2)} Cr</strong>,
            },
            {
              key: "net_worth",
              label: "Net Worth",
              value: (y) => y.net_worth,
              render: (y) => <span>₹{(y.net_worth / 10000000).toFixed(2)} Cr</span>,
            },
            {
              key: "pat",
              label: "Profit After Tax (PAT)",
              value: (y) => y.profit_after_tax,
              render: (y) => (
                <span className={y.profit_after_tax >= 0 ? "text-emerald-700 font-semibold" : "text-red-600 font-semibold"}>
                  ₹{(y.profit_after_tax / 10000000).toFixed(2)} Cr
                </span>
              ),
            },
            {
              key: "audited",
              label: "Audit Status",
              value: (y) => (y.is_audited ? "Audited CA Statement" : "Provisional"),
            },
          ]}
          actions={(y) => (
            <div className="flex justify-end gap-1">
              {canWrite && (
                <>
                  <button className={secondary} onClick={() => onEditYear(y)} title="Edit">
                    <PencilSquareIcon className="h-4 w-4" />
                  </button>
                  <button className={danger} onClick={() => void deleteYear(y.id)} title="Delete">
                    <TrashIcon className="h-4 w-4" />
                  </button>
                </>
              )}
            </div>
          )}
        />
      </section>

      {/* Bank Certificates (Solvency / Working Capital) */}
      <section className="space-y-4 border-t border-slate-200 pt-6 dark:border-slate-800">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-lg font-bold">Bank Solvency & Credit Facility Certificates</h3>
            <p className="text-xs text-slate-500">Dated solvency certificates and bank facility sanction letters.</p>
          </div>
          {canWrite && (
            <button type="button" onClick={onAddCert} className={primary}>
              <PlusIcon className="h-4 w-4" /> Add Certificate
            </button>
          )}
        </div>

        <DataGrid
          label="Bank Certificates"
          rows={certificates}
          columns={[
            {
              key: "kind",
              label: "Certificate Kind",
              value: (c) => c.kind.replaceAll("_", " ").toUpperCase(),
            },
            { key: "issuer", label: "Issuing Bank / Authority", value: (c) => c.issuer_name },
            {
              key: "amount",
              label: "Amount (INR)",
              value: (c) => c.amount,
              render: (c) => <strong>₹{(c.amount / 10000000).toFixed(2)} Cr</strong>,
            },
            { key: "valid_until", label: "Valid Until", value: (c) => c.valid_until || "Ongoing" },
          ]}
          actions={(c) => (
            <div className="flex justify-end gap-1">
              {canWrite && (
                <>
                  <button className={secondary} onClick={() => onEditCert(c)} title="Edit">
                    <PencilSquareIcon className="h-4 w-4" />
                  </button>
                  <button className={danger} onClick={() => void deleteCert(c.id)} title="Deactivate">
                    <TrashIcon className="h-4 w-4" />
                  </button>
                </>
              )}
            </div>
          )}
        />
      </section>
    </div>
  );
}

// ---------------- TAB 4: ELECTRICAL LICENCES ----------------
function LicencesTab({
  licences,
  canWrite,
  onAdd,
  onEdit,
  onRefresh,
}: {
  licences: CompanyLicence[];
  canWrite: boolean;
  onAdd: () => void;
  onEdit: (l: CompanyLicence) => void;
  onRefresh: () => Promise<void>;
}) {
  async function deleteLicence(id: string) {
    if (!confirm("Deactivate this licence?")) return;
    await fetch(`/api/settings/company-profile/licences/${id}`, { method: "DELETE" });
    await onRefresh();
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-bold">Electrical Contractor Licences by State</h3>
          <p className="text-xs text-slate-500">
            State-wise statutory licences authorizing installation, testing, and commissioning up to specified voltage ratings.
          </p>
        </div>
        {canWrite && (
          <button type="button" onClick={onAdd} className={primary}>
            <PlusIcon className="h-4 w-4" /> Add Licence
          </button>
        )}
      </div>

      <DataGrid
        label="Licences"
        rows={licences}
        columns={[
          { key: "state", label: "State / Jurisdiction", value: (l) => l.state },
          { key: "number", label: "Licence Number", value: (l) => l.licence_number },
          { key: "class", label: "Grade / Class", value: (l) => l.class_grade },
          {
            key: "voltage",
            label: "Max Authorized Voltage",
            value: (l) => l.maximum_voltage_kv,
            render: (l) => <span className="font-bold text-blue-700 dark:text-blue-400">{l.maximum_voltage_kv} kV</span>,
          },
          {
            key: "valid",
            label: "Valid Until",
            value: (l) => l.valid_until,
            render: (l) => {
              const isExpired = new Date(l.valid_until) < new Date();
              return (
                <span className={isExpired ? "font-bold text-red-600" : "text-slate-800 dark:text-slate-200"}>
                  {l.valid_until} {isExpired ? "(EXPIRED)" : ""}
                </span>
              );
            },
          },
        ]}
        actions={(l) => (
          <div className="flex justify-end gap-1">
            {canWrite && (
              <>
                <button className={secondary} onClick={() => onEdit(l)} title="Edit">
                  <PencilSquareIcon className="h-4 w-4" />
                </button>
                <button className={danger} onClick={() => void deleteLicence(l.id)} title="Delete">
                  <TrashIcon className="h-4 w-4" />
                </button>
              </>
            )}
          </div>
        )}
      />
    </div>
  );
}

// ---------------- TAB 5: PAST WORK EXPERIENCE ----------------
function ExperienceTab({
  experience,
  canWrite,
  onAdd,
  onEdit,
  onRefresh,
}: {
  experience: CompanyWorkExperience[];
  canWrite: boolean;
  onAdd: () => void;
  onEdit: (w: CompanyWorkExperience) => void;
  onRefresh: () => Promise<void>;
}) {
  async function deleteExp(id: string) {
    if (!confirm("Deactivate this project record?")) return;
    await fetch(`/api/settings/company-profile/experience/${id}`, { method: "DELETE" });
    await onRefresh();
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-lg font-bold">Executed Projects Repository (PQC Track Record)</h3>
          <p className="text-xs text-slate-500">
            Completed EPC contracts used to satisfy the 1-work (80%), 2-work (50%), or 3-work (40%) criteria.
          </p>
        </div>
        {canWrite && (
          <button type="button" onClick={onAdd} className={primary}>
            <PlusIcon className="h-4 w-4" /> Add Project Experience
          </button>
        )}
      </div>

      <DataGrid
        label="Executed Projects"
        rows={experience}
        columns={[
          {
            key: "title",
            label: "Project Scope & Title",
            value: (w) => w.project_title,
            render: (w) => (
              <div>
                <strong className="block text-slate-900 dark:text-white">{w.project_title}</strong>
                <span className="text-xs text-slate-500">{w.scope_description.slice(0, 80)}…</span>
              </div>
            ),
          },
          {
            key: "client",
            label: "Client / Employer",
            value: (w) => w.client_name,
            render: (w) => (
              <div>
                <span>{w.client_name}</span>
                <span className="block text-xs uppercase font-semibold text-slate-500">{w.client_type.replace("_", " ")}</span>
              </div>
            ),
          },
          {
            key: "value",
            label: "Executed Value (Excl. GST)",
            value: (w) => w.executed_value_excluding_gst,
            render: (w) => (
              <strong className="text-base text-emerald-700 dark:text-emerald-400">
                ₹{(w.executed_value_excluding_gst / 10000000).toFixed(2)} Cr
              </strong>
            ),
          },
          {
            key: "technical",
            label: "Technical Specs",
            value: (w) => `${w.voltage_kv ? `${w.voltage_kv} kV` : ""} ${w.capacity_description}`,
          },
          { key: "state", label: "State", value: (w) => w.state },
          { key: "completed", label: "Completion Date", value: (w) => w.completed_on },
        ]}
        actions={(w) => (
          <div className="flex justify-end gap-1">
            {canWrite && (
              <>
                <button className={secondary} onClick={() => onEdit(w)} title="Edit">
                  <PencilSquareIcon className="h-4 w-4" />
                </button>
                <button className={danger} onClick={() => void deleteExp(w.id)} title="Delete">
                  <TrashIcon className="h-4 w-4" />
                </button>
              </>
            )}
          </div>
        )}
      />
    </div>
  );
}

// ---------------- SUB-RECORD MODAL FORMS ----------------
function FinancialYearForm({ item, onSaved }: { item?: CompanyFinancialYear; onSaved: () => void }) {
  const [year, setYear] = useState(item?.financial_year || "2024-2025");
  const [turnover, setTurnover] = useState(item ? String(item.turnover_excluding_gst) : "150000000");
  const [netWorth, setNetWorth] = useState(item ? String(item.net_worth) : "40000000");
  const [pat, setPat] = useState(item ? String(item.profit_after_tax) : "8000000");
  const [audited, setAudited] = useState(item?.is_audited ?? true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const id = useId();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await fetch("/api/settings/company-profile/financials", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          id: item?.id,
          financial_year: year,
          turnover_excluding_gst: parseFloat(turnover) || 0,
          net_worth: parseFloat(netWorth) || 0,
          profit_after_tax: parseFloat(pat) || 0,
          is_audited: audited,
          expected_version: item?.version,
        }),
      });
      if (!res.ok) {
        const b = await res.json().catch(() => ({}));
        throw new Error(b.detail || "Unable to save financial year");
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error saving");
    } finally {
      setBusy(false);
    }
  }

  return (
    <ValidatedForm className="space-y-4" onSubmit={submit}>
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <ValidatedLabel className="block text-sm font-semibold">
        Financial Year
        <ValidatedInput value={year} onChange={(e) => setYear(e.target.value)} required placeholder="2024-2025" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Turnover Excluding GST (in INR)
        <ValidatedInput type="number" step="0.01" value={turnover} onChange={(e) => setTurnover(e.target.value)} required className={fieldClass} />
        <span className="mt-1 block text-xs text-slate-500">
          ≈ ₹{((parseFloat(turnover) || 0) / 10000000).toFixed(2)} Crores
        </span>
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Audited Net Worth (in INR)
        <ValidatedInput type="number" step="0.01" value={netWorth} onChange={(e) => setNetWorth(e.target.value)} required className={fieldClass} />
        <span className="mt-1 block text-xs text-slate-500">
          ≈ ₹{((parseFloat(netWorth) || 0) / 10000000).toFixed(2)} Crores
        </span>
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Profit After Tax / PAT (in INR)
        <ValidatedInput type="number" step="0.01" value={pat} onChange={(e) => setPat(e.target.value)} required className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel htmlFor={`${id}-audited-checkbox`} className="flex items-center gap-2 text-sm font-medium">
        <ValidatedInput id={`${id}-audited-checkbox`} type="checkbox" checked={audited} onChange={(e) => setAudited(e.target.checked)} className="h-4 w-4 rounded" />
        Audited Financial Statement (CA Certified)
      </ValidatedLabel>
      <button type="submit" disabled={busy} className={primary}>
        {busy ? "Saving…" : "Save Financial Year"}
      </button>
    </ValidatedForm>
  );
}

function LicenceForm({ item, onSaved }: { item?: CompanyLicence; onSaved: () => void }) {
  const [state, setState] = useState(item?.state || "Maharashtra");
  const [number, setNumber] = useState(item?.licence_number || "");
  const [classGrade, setClassGrade] = useState(item?.class_grade || "Class A");
  const [voltage, setVoltage] = useState(item ? String(item.maximum_voltage_kv) : "33");
  const [issuedOn, setIssuedOn] = useState(item?.issued_on || "");
  const [validUntil, setValidUntil] = useState(item?.valid_until || "2027-03-31");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await fetch("/api/settings/company-profile/licences", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          id: item?.id,
          state,
          licence_number: number,
          class_grade: classGrade,
          maximum_voltage_kv: parseFloat(voltage) || 33,
          issued_on: issuedOn || null,
          valid_until: validUntil,
          expected_version: item?.version,
        }),
      });
      if (!res.ok) {
        const b = await res.json().catch(() => ({}));
        throw new Error(b.detail || "Unable to save licence");
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error saving");
    } finally {
      setBusy(false);
    }
  }

  return (
    <ValidatedForm className="space-y-4" onSubmit={submit}>
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <ValidatedLabel className="block text-sm font-semibold">
        State / Licensing Authority
        <ValidatedInput value={state} onChange={(e) => setState(e.target.value)} required placeholder="Maharashtra" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Licence Number
        <ValidatedInput value={number} onChange={(e) => setNumber(e.target.value)} required placeholder="ELC-12345/MH" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Class / Grade
        <ValidatedInput value={classGrade} onChange={(e) => setClassGrade(e.target.value)} required placeholder="Class A / Super A" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Maximum Authorized Voltage (kV)
        <ValidatedInput type="number" step="0.1" value={voltage} onChange={(e) => setVoltage(e.target.value)} required placeholder="33" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Issue Date
        <ValidatedInput type="date" value={issuedOn} onChange={(e) => setIssuedOn(e.target.value)} className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Valid Until Date
        <ValidatedInput type="date" value={validUntil} onChange={(e) => setValidUntil(e.target.value)} required className={fieldClass} />
      </ValidatedLabel>
      <button type="submit" disabled={busy} className={primary}>
        {busy ? "Saving…" : "Save Licence"}
      </button>
    </ValidatedForm>
  );
}

function ExperienceForm({ item, onSaved }: { item?: CompanyWorkExperience; onSaved: () => void }) {
  const [title, setTitle] = useState(item?.project_title || "");
  const [client, setClient] = useState(item?.client_name || "");
  const [clientType, setClientType] = useState(item?.client_type || "psu");
  const [value, setValue] = useState(item ? String(item.executed_value_excluding_gst) : "50000000");
  const [state, setState] = useState(item?.state || "Maharashtra");
  const [voltage, setVoltage] = useState(item?.voltage_kv ? String(item.voltage_kv) : "");
  const [capacity, setCapacity] = useState(item?.capacity_description || "");
  const [completedOn, setCompletedOn] = useState(item?.completed_on || "2024-06-30");
  const [certRef, setCertRef] = useState(item?.completion_certificate_ref || "");
  const [scope, setScope] = useState(item?.scope_description || "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await fetch("/api/settings/company-profile/experience", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          id: item?.id,
          project_title: title,
          client_name: client,
          client_type: clientType,
          executed_value_excluding_gst: parseFloat(value) || 0,
          state,
          voltage_kv: voltage ? parseFloat(voltage) : null,
          capacity_description: capacity,
          completed_on: completedOn,
          completion_certificate_ref: certRef,
          scope_description: scope,
          expected_version: item?.version,
        }),
      });
      if (!res.ok) {
        const b = await res.json().catch(() => ({}));
        throw new Error(b.detail || "Unable to save experience");
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error saving");
    } finally {
      setBusy(false);
    }
  }

  return (
    <ValidatedForm className="space-y-4" onSubmit={submit}>
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <ValidatedLabel className="block text-sm font-semibold">
        Project Title
        <ValidatedInput value={title} onChange={(e) => setTitle(e.target.value)} required placeholder="Supply & Installation of 33/11kV Substation" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Client / Authority Name
        <ValidatedInput value={client} onChange={(e) => setClient(e.target.value)} required placeholder="MSEDCL / PGCIL / NTPC" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Client Classification
        <ValidatedSelect value={clientType} onChange={(e) => setClientType(e.target.value as CompanyWorkExperience["client_type"])} className={fieldClass}>
          <option value="psu">PSU (Public Sector Undertaking)</option>
          <option value="central_govt">Central Government</option>
          <option value="state_govt">State Government</option>
          <option value="public_ltd">Public Limited Company</option>
          <option value="private">Private Entity</option>
        </ValidatedSelect>
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Executed Contract Value (Excl. GST in INR)
        <ValidatedInput type="number" step="0.01" value={value} onChange={(e) => setValue(e.target.value)} required className={fieldClass} />
        <span className="mt-1 block text-xs text-slate-500">
          ≈ ₹{((parseFloat(value) || 0) / 10000000).toFixed(2)} Crores
        </span>
      </ValidatedLabel>
      <div className="grid gap-4 sm:grid-cols-2">
        <ValidatedLabel className="block text-sm font-semibold">
          State / Location
          <ValidatedInput value={state} onChange={(e) => setState(e.target.value)} placeholder="Maharashtra" className={fieldClass} />
        </ValidatedLabel>
        <ValidatedLabel className="block text-sm font-semibold">
          Voltage Class (kV)
          <ValidatedInput type="number" step="0.1" value={voltage} onChange={(e) => setVoltage(e.target.value)} placeholder="33" className={fieldClass} />
        </ValidatedLabel>
      </div>
      <ValidatedLabel className="block text-sm font-semibold">
        Capacity / Quantities
        <ValidatedInput value={capacity} onChange={(e) => setCapacity(e.target.value)} placeholder="2x10 MVA Transformers, 15 km Line" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Completion Date
        <ValidatedInput type="date" value={completedOn} onChange={(e) => setCompletedOn(e.target.value)} required className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Client Completion Certificate Reference No.
        <ValidatedInput value={certRef} onChange={(e) => setCertRef(e.target.value)} placeholder="CC/2024/0087" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Scope Summary
        <ValidatedTextarea value={scope} onChange={(e) => setScope(e.target.value)} placeholder="Complete turnkey execution including civil foundations, switchgear..." className={fieldClass} rows={2} />
      </ValidatedLabel>
      <button type="submit" disabled={busy} className={primary}>
        {busy ? "Saving…" : "Save Project"}
      </button>
    </ValidatedForm>
  );
}

function CertificateForm({ item, onSaved }: { item?: CompanyCertificate; onSaved: () => void }) {
  const [kind, setKind] = useState(item?.kind || "solvency");
  const [issuer, setIssuer] = useState(item?.issuer_name || "State Bank of India");
  const [amount, setAmount] = useState(item ? String(item.amount) : "30000000");
  const [issuedOn, setIssuedOn] = useState(item?.issued_on || "");
  const [validUntil, setValidUntil] = useState(item?.valid_until || "");
  const [certNum, setCertNum] = useState(item?.certificate_number || "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await fetch("/api/settings/company-profile/certificates", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          id: item?.id,
          kind,
          issuer_name: issuer,
          certificate_number: certNum,
          amount: parseFloat(amount) || 0,
          issued_on: issuedOn || null,
          valid_until: validUntil || null,
          expected_version: item?.version,
        }),
      });
      if (!res.ok) {
        const b = await res.json().catch(() => ({}));
        throw new Error(b.detail || "Unable to save certificate");
      }
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Error saving");
    } finally {
      setBusy(false);
    }
  }

  return (
    <ValidatedForm className="space-y-4" onSubmit={submit}>
      {error && <p role="alert" className="text-sm text-red-600">{error}</p>}
      <ValidatedLabel className="block text-sm font-semibold">
        Certificate Type
        <ValidatedSelect value={kind} onChange={(e) => setKind(e.target.value as CompanyCertificate["kind"])} className={fieldClass}>
          <option value="solvency">Bank Solvency Certificate</option>
          <option value="working_capital">Working Capital Sanction Letter</option>
          <option value="bank_guarantee_limit">Bank Guarantee (BG) Facility Limit</option>
        </ValidatedSelect>
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Issuing Bank / Branch
        <ValidatedInput value={issuer} onChange={(e) => setIssuer(e.target.value)} required placeholder="HDFC Bank / SBI" className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Certificate Amount (INR)
        <ValidatedInput type="number" step="0.01" value={amount} onChange={(e) => setAmount(e.target.value)} required className={fieldClass} />
        <span className="mt-1 block text-xs text-slate-500">
          ≈ ₹{((parseFloat(amount) || 0) / 10000000).toFixed(2)} Crores
        </span>
      </ValidatedLabel>
      <ValidatedLabel className="block text-sm font-semibold">
        Certificate / Ref Number
        <ValidatedInput value={certNum} onChange={(e) => setCertNum(e.target.value)} placeholder="SOLV/2026/0991" className={fieldClass} />
      </ValidatedLabel>
      <div className="grid gap-4 sm:grid-cols-2">
        <ValidatedLabel className="block text-sm font-semibold">
          Issue Date
          <ValidatedInput type="date" value={issuedOn} onChange={(e) => setIssuedOn(e.target.value)} className={fieldClass} />
        </ValidatedLabel>
        <ValidatedLabel className="block text-sm font-semibold">
          Valid Until Date
          <ValidatedInput type="date" value={validUntil} onChange={(e) => setValidUntil(e.target.value)} className={fieldClass} />
        </ValidatedLabel>
      </div>
      <button type="submit" disabled={busy} className={primary}>
        {busy ? "Saving…" : "Save Certificate"}
      </button>
    </ValidatedForm>
  );
}
