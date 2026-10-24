"use client";
import {
  ValidatedInput,
  ValidatedLabel,
} from "@/components/ui/validated-controls";

import { useEffect, useState } from "react";
import { useAuthenticatedUser } from "@/components/auth-user-provider";

export type Supplier = {
  id: string;
  code: string;
  name: string;
  data: {
    contact_person_name?: string;
    email?: string;
    phone?: string;
    location?: string;
    gst_number?: string;
    pan_number?: string;
    onboarding_verified?: boolean;
    payment_terms?: {
      advance_pct?: number;
      qc_pct?: number;
      delivery_pct?: number;
      retention_pct?: number;
      credit_days?: number;
      description?: string;
    };
  };
};

export type BOLine = {
  id: string;
  item_id: string | null;
  quantity: string | null;
  required_date: string | null;
  reference: string;
  notes: string;
  item_snapshot: {
    id?: string;
    sku?: string;
    description?: string;
    uom?: string;
    manufacturer?: string;
  };
};

export type TechnicalDoc = {
  id: string;
  title: string;
  category: string;
  standard: string;
  required: boolean;
  status: "pending" | "received" | "approved" | "revision_required";
  received_date?: string;
  revision: string;
  file_name?: string;
  notes?: string;
};

export type Phase3State = {
  // Step 6: Technical Documents
  docs: TechnicalDoc[];
  // Step 7: Project & Client Transmittal
  transmittal_no: string;
  submission_date: string;
  target_approval_date: string;
  reviewing_authority: string;
  transmittal_file_name?: string;
  transmittal_status: "draft" | "submitted" | "under_review";
  // Step 8: Technical Approval
  approval_status:
    | "approved"
    | "approved_with_comments"
    | "revision_required"
    | "pending";
  approval_ref_no: string;
  approval_date: string;
  approver_name: string;
  approver_designation: string;
  qap_approved: boolean;
  approval_file_name?: string;
  approval_remarks: string;
  // Step 9: Purchase Order
  po_number: string;
  po_date: string;
  loi_ref_no: string;
  management_approved: boolean;
  management_approver: string;
  management_approval_date: string;
  delivery_timeline_days: number;
  payment_terms_override?: string;
};

const defaultTechnicalDocs: TechnicalDoc[] = [
  {
    id: "ga-drawing",
    title: "General Arrangement (GA) Drawing",
    category: "Engineering Drawings",
    standard: "As per tender technical data sheet & IS standards",
    required: true,
    status: "received",
    received_date: "2026-09-23",
    revision: "Rev 0",
    file_name: "GA_Drawing_Octagonal_Pole_Rev0.pdf",
    notes: "Dimensional verification matching airport mast foundations.",
  },
  {
    id: "tds",
    title: "Technical Data Sheet (TDS)",
    category: "Specifications",
    standard: "Tender technical parameter schedule",
    required: true,
    status: "received",
    received_date: "2026-09-23",
    revision: "Rev 0",
    file_name: "TDS_Guaranteed_Technical_Particulars.pdf",
    notes: "Contains all electrical and mechanical guarantees.",
  },
  {
    id: "structural-calc",
    title: "Structural & Foundation Design Calculation",
    category: "Design Calculation",
    standard: "IS 875 (Part 3) Wind Load Calculations",
    required: true,
    status: "received",
    received_date: "2026-09-24",
    revision: "Rev 0",
    file_name: "Structural_Design_Calculations_AAI.pdf",
    notes: "Certified for basic wind speed 47 m/s (Zone IV).",
  },
  {
    id: "type-test",
    title: "Type Test Certificates",
    category: "Quality / Lab Reports",
    standard: "CPRI / ERDA / NABL accredited laboratory test",
    required: true,
    status: "received",
    received_date: "2026-09-22",
    revision: "Valid",
    file_name: "CPRI_Type_Test_Certificate.pdf",
    notes: "Valid within 5 years as required by client tender conditions.",
  },
  {
    id: "qap",
    title: "Quality Assurance Plan (QAP)",
    category: "Inspection Hold Points",
    standard: "Factory stage inspection and customer witness points",
    required: true,
    status: "received",
    received_date: "2026-09-24",
    revision: "Rev 0",
    file_name: "Manufacturing_QAP_VishalTech.pdf",
    notes: "Identifies Customer Witness (W) and Review (R) checkpoints.",
  },
  {
    id: "mtc-raw",
    title: "Raw Material Test Certificates (MTC)",
    category: "Material Traceability",
    standard: "IS 2062 Grade E250 / E350 steel certificates",
    required: false,
    status: "pending",
    revision: "Rev 0",
    notes: "To be produced alongside batch dispatch clearance.",
  },
];

export function PurchaseOrderWorkspace({
  project,
  projectName,
  lines,
  boId,
  initialStep = 6,
  onNavigateStep,
}: {
  project: string;
  projectName?: string;
  lines: BOLine[];
  boId?: string;
  initialStep?: number;
  onNavigateStep: (stepNumber: number) => void;
}) {
  const user = useAuthenticatedUser();
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [loading, setLoading] = useState(true);
  const currentStep = initialStep;

  function switchStep(s: number) {
    onNavigateStep(s);
  }

  // Storage key for Phase 3 (Steps 6-9)
  const storageKey = `vtpl_loi2po_${project}_${boId || "default"}`;
  // Read awarded supplier from Sourcing (Phase 2)
  const sourcingStorageKey = `vtpl_sourcing_${project}_${boId || "default"}`;

  const [state, setState] = useState<Phase3State>(() => ({
    docs: defaultTechnicalDocs,
    transmittal_no: `TR/VTP/${new Date().getFullYear()}/AAI/001`,
    submission_date: new Date().toISOString().split("T")[0],
    target_approval_date: new Date(Date.now() + 7 * 86400000)
      .toISOString()
      .split("T")[0],
    reviewing_authority:
      "Airport Authority of India (AAI) / Project Consultant",
    transmittal_status: "submitted",
    transmittal_file_name: "Transmittal_Technical_Submission_Package.pdf",
    approval_status: "approved",
    approval_ref_no: "AAI/ASR/ELECT/2026/APPR-088",
    approval_date: new Date().toISOString().split("T")[0],
    approver_name: "Er. R. K. Sharma",
    approver_designation: "Executive Director (Engineering), AAI",
    qap_approved: true,
    approval_file_name: "AAI_Formal_Technical_Approval_Signed.pdf",
    approval_remarks:
      "Approved as per Category A. All guaranteed parameters comply with airport lighting tender.",
    po_number: `PO/VTP/${new Date().getFullYear()}-${(new Date().getFullYear() + 1).toString().slice(2)}/0045`,
    po_date: new Date()
      .toLocaleDateString("en-GB", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      })
      .replace(/\//g, "."),
    loi_ref_no: `LOI/VTP/${new Date().getFullYear()}/001`,
    management_approved: true,
    management_approver: "Dinesh Kumar (Director)",
    management_approval_date: new Date().toISOString().split("T")[0],
    delivery_timeline_days: 21,
  }));

  const [awardedSupplierId, setAwardedSupplierId] = useState<string>("");
  const [negotiatedRate, setNegotiatedRate] = useState<number>(1757.5);

  // Load suppliers and restore saved state
  useEffect(() => {
    let live = true;
    async function loadData() {
      try {
        const res = await fetch("/api/settings/suppliers");
        if (res.ok) {
          const data: Supplier[] = await res.json();
          if (live) {
            setSuppliers(data);

            // Check if sourcing stage saved an awarded supplier
            const sourcingSaved = localStorage.getItem(sourcingStorageKey);
            if (sourcingSaved) {
              try {
                const parsedSourcing = JSON.parse(sourcingSaved);
                if (parsedSourcing.selectedSupplierId) {
                  setAwardedSupplierId(parsedSourcing.selectedSupplierId);
                }
                if (parsedSourcing.negotiatedRate) {
                  setNegotiatedRate(Number(parsedSourcing.negotiatedRate));
                }
              } catch {
                /* ignore */
              }
            } else if (data.length > 0) {
              setAwardedSupplierId(data[0].id);
            }

            // Restore Phase 3 state
            const saved = localStorage.getItem(storageKey);
            if (saved) {
              try {
                const parsed = JSON.parse(saved);
                setState((prev) => ({ ...prev, ...parsed }));
              } catch {
                /* ignore */
              }
            }
          }
        }
      } catch (err) {
        console.error("Failed to load suppliers for Phase 3:", err);
      } finally {
        if (live) setLoading(false);
      }
    }
    void loadData();
    return () => {
      live = false;
    };
  }, [storageKey, sourcingStorageKey]);

  function persist(updates: Partial<Phase3State>) {
    setState((prev) => {
      const next = { ...prev, ...updates };
      localStorage.setItem(storageKey, JSON.stringify(next));
      return next;
    });
  }

  // Find awarded supplier object
  const currentSupplier =
    suppliers.find((s) => s.id === awardedSupplierId) || suppliers[0];

  // Primary item details
  const validLines = lines.filter(
    (l) => l.item_id || l.item_snapshot?.sku || l.item_snapshot?.description,
  );
  const primaryItem = validLines[0] || null;
  const targetQuantity = primaryItem
    ? parseFloat(primaryItem.quantity || "1") || 1
    : 100;
  const targetUom = primaryItem?.item_snapshot?.uom || "Unit";
  const itemDescription =
    primaryItem?.item_snapshot?.description ||
    primaryItem?.reference ||
    "Supply of High Mast / Octagonal Lighting Poles with complete accessories as per approved tender specifications.";
  const itemSku = primaryItem?.item_snapshot?.sku || "SKU-POLE-OCT-001";

  // Financial calculations for PO
  const unitBasic = negotiatedRate > 0 ? negotiatedRate : 1757.5;
  const taxPct = 18;
  const taxAmountPerUnit = unitBasic * (taxPct / 100);
  const unitLanded = unitBasic + taxAmountPerUnit;
  const totalMaterialCost = unitLanded * targetQuantity;
  const freightTotal = 5000;
  const grandTotal = totalMaterialCost + freightTotal;

  function numberToWords(num: number): string {
    const rounded = Math.round(num);
    return `Rupees ${rounded.toLocaleString("en-IN")} Only`;
  }

  // Format dynamic payment terms
  function getPaymentTermsString(sup?: Supplier): string {
    if (state.payment_terms_override?.trim()) {
      return state.payment_terms_override.trim();
    }
    const terms = sup?.data?.payment_terms;
    if (terms?.description?.trim()) {
      return terms.description.trim();
    }
    const parts: string[] = [];
    if (terms?.advance_pct) parts.push(`${terms.advance_pct}% advance payment`);
    if (terms?.qc_pct)
      parts.push(
        `${terms.qc_pct}% after stage inspection & dispatch clearance`,
      );
    if (terms?.delivery_pct)
      parts.push(
        `${terms.delivery_pct}% after site delivery and physical verification`,
      );
    if (terms?.retention_pct)
      parts.push(
        `${terms.retention_pct}% retention against warranty performance`,
      );
    return parts.length > 0
      ? parts.join(", ") + "."
      : "20% advance, 80% on dispatch.";
  }

  function handlePrintPDF() {
    window.print();
  }

  return (
    <div className="bo-stage flex flex-col flex-1 min-w-0 bg-slate-100 text-slate-900 overflow-hidden">
      {/* Print stylesheet */}
      <style>{`
        @media print {
          body * {
            visibility: hidden !important;
          }
          #purchase-order-document,
          #purchase-order-document * {
            visibility: visible !important;
          }
          #purchase-order-document {
            position: absolute !important;
            left: 0 !important;
            top: 0 !important;
            width: 100% !important;
            margin: 0 !important;
            padding: 10mm !important;
            background: #ffffff !important;
            color: #000000 !important;
            box-shadow: none !important;
            border: none !important;
            font-size: 9.5pt !important;
          }
          .no-print {
            display: none !important;
          }
          @page {
            size: A4 portrait;
            margin: 8mm;
          }
        }
      `}</style>

      {/* Top Workspace Toolbar - Compact & Clean */}
      {currentStep === 9 ? (
        <header className="no-print bo-stage-toolbar">
          {currentStep === 9 && (
            <button
              type="button"
              onClick={handlePrintPDF}
              className="flex items-center gap-1.5 px-4 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs rounded-lg shadow-sm transition-all"
            >
              Print / download PO
            </button>
          )}
        </header>
      ) : null}

      {/* Main Workspace Body */}
      <main className="flex-1 min-w-0 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center p-16 text-slate-500 font-semibold text-sm">
            Loading technical document repository &amp; purchase order
            configuration...
          </div>
        ) : currentStep === 6 ? (
          /* ========================================================================= */
          /* STEP 6: TECHNICAL DOCUMENTS RECEIPT                                       */
          /* ========================================================================= */
          <div className="p-6 max-w-[1400px] mx-auto space-y-4">
            {/* Header Strip */}
            <div className="flex flex-wrap items-center justify-between gap-4 bg-white p-4 rounded-xl border border-slate-200 shadow-sm">
              <div>
                <h3 className="text-base font-bold text-slate-900 mt-0.5">
                  Technical Engineering Documents &amp; Drawings Receipt
                </h3>
                <p className="text-xs text-slate-500">
                  Track vendor GA drawings, Technical Data Sheets (TDS),
                  Structural calculations, and QAP against the issued LOI.
                </p>
              </div>

              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => {
                    const updated = state.docs.map((d) => ({
                      ...d,
                      status: "received" as const,
                      received_date:
                        d.received_date ||
                        new Date().toISOString().split("T")[0],
                    }));
                    persist({ docs: updated });
                  }}
                  className="px-3.5 py-1.5 text-xs font-bold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg border border-slate-300"
                >
                  ✓ Mark All Received
                </button>
                <button
                  type="button"
                  onClick={() => switchStep(7)}
                  className="px-5 py-1.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-lg shadow-sm flex items-center gap-1.5"
                >
                  Proceed to Step 7: Submit to Client →
                </button>
              </div>
            </div>

            {/* Documents Table */}
            <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-hidden">
              <table className="vtpl-data-table w-full text-left border-collapse text-xs">
                <thead>
                  <tr className="bg-slate-50 border-b border-slate-200 text-slate-700 font-bold">
                    <th className="py-3 px-4 w-12 text-center">#</th>
                    <th className="py-3 px-4">Document Title &amp; Category</th>
                    <th className="py-3 px-4">Standard / Spec Requirement</th>
                    <th className="py-3 px-3 text-center">Revision</th>
                    <th className="py-3 px-3 text-center">Receipt Date</th>
                    <th className="py-3 px-3 text-center">Status</th>
                    <th className="py-3 px-4">Attached Document</th>
                    <th className="py-3 px-3 text-center">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {state.docs.map((doc, idx) => (
                    <tr
                      key={doc.id}
                      className="hover:bg-slate-50/70 transition-colors"
                    >
                      <td className="py-3 px-4 text-center font-bold text-slate-400">
                        {idx + 1}
                      </td>
                      <td className="py-3 px-4">
                        <div className="font-bold text-slate-900">
                          {doc.title}
                        </div>
                        <div className="text-[11px] text-slate-500">
                          {doc.category}
                        </div>
                      </td>
                      <td className="py-3 px-4 text-slate-600">
                        {doc.standard}
                        {doc.required && (
                          <span className="ml-2 text-[10px] font-bold text-red-600 bg-red-50 px-1.5 py-0.5 rounded border border-red-200">
                            Mandatory
                          </span>
                        )}
                      </td>
                      <td className="py-3 px-3 text-center">
                        <ValidatedInput
                          type="text"
                          value={doc.revision}
                          onChange={(e) => {
                            const updated = state.docs.map((d) =>
                              d.id === doc.id
                                ? { ...d, revision: e.target.value }
                                : d,
                            );
                            persist({ docs: updated });
                          }}
                          className="w-16 text-center font-mono text-xs border border-slate-200 rounded px-1.5 py-1"
                        />
                      </td>
                      <td className="py-3 px-3 text-center text-slate-600 font-mono">
                        {doc.received_date || "Pending"}
                      </td>
                      <td className="py-3 px-3 text-center">
                        <span
                          className={`inline-block px-2.5 py-1 rounded-full font-bold text-[11px] border ${
                            doc.status === "received" ||
                            doc.status === "approved"
                              ? "bg-emerald-50 text-emerald-700 border-emerald-200"
                              : "bg-amber-50 text-amber-700 border-amber-200"
                          }`}
                        >
                          {doc.status === "received"
                            ? "✓ Received"
                            : doc.status === "approved"
                              ? "✓ Approved"
                              : "⏳ Pending"}
                        </span>
                      </td>
                      <td className="py-3 px-4">
                        {doc.file_name ? (
                          <div className="flex items-center gap-2 text-blue-600 font-medium">
                            <span className="text-base">📄</span>
                            <span
                              className="truncate max-w-[200px]"
                              title={doc.file_name}
                            >
                              {doc.file_name}
                            </span>
                          </div>
                        ) : (
                          <ValidatedLabel className="cursor-pointer text-[11px] text-slate-500 hover:text-blue-600 flex items-center gap-1 font-medium">
                            <span>📎 Attach File</span>
                            <ValidatedInput
                              type="file"
                              className="hidden"
                              onChange={(e) => {
                                const f = e.target.files?.[0];
                                if (f) {
                                  const updated = state.docs.map((d) =>
                                    d.id === doc.id
                                      ? {
                                          ...d,
                                          file_name: f.name,
                                          status: "received" as const,
                                          received_date: new Date()
                                            .toISOString()
                                            .split("T")[0],
                                        }
                                      : d,
                                  );
                                  persist({ docs: updated });
                                }
                              }}
                            />
                          </ValidatedLabel>
                        )}
                      </td>
                      <td className="py-3 px-3 text-center">
                        <button
                          type="button"
                          onClick={() => {
                            const newStatus: "pending" | "received" =
                              doc.status === "received"
                                ? "pending"
                                : "received";
                            const updated: TechnicalDoc[] = state.docs.map(
                              (d) =>
                                d.id === doc.id
                                  ? {
                                      ...d,
                                      status: newStatus,
                                      received_date:
                                        newStatus === "received"
                                          ? new Date()
                                              .toISOString()
                                              .split("T")[0]
                                          : undefined,
                                    }
                                  : d,
                            );
                            persist({ docs: updated });
                          }}
                          className="text-[11px] font-bold text-slate-600 hover:text-slate-900 underline"
                        >
                          Toggle
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Bottom Proceed Bar */}
            <div className="flex justify-end p-4 bg-white rounded-xl border border-slate-200 shadow-sm">
              <button
                type="button"
                onClick={() => switchStep(7)}
                className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-md transition-all flex items-center gap-2"
              >
                Compile Package &amp; Submit to Client (Step 7) →
              </button>
            </div>
          </div>
        ) : currentStep === 7 ? (
          /* ========================================================================= */
          /* STEP 7: SUBMIT TO PROJECT TEAM / CLIENT                                   */
          /* ========================================================================= */
          <div className="p-6 max-w-[1200px] mx-auto space-y-6">
            {/* Header Strip */}
            <div className="flex flex-wrap items-center justify-between gap-4 bg-white p-4 rounded-xl border border-slate-200 shadow-sm">
              <div>
                <h3 className="text-base font-bold text-slate-900 mt-0.5">
                  Submit Technical Documents to Project Team &amp; Client
                </h3>
                <p className="text-xs text-slate-500">
                  Check and bundle received drawings &amp; QAP for formal client
                  approval (AAI / Consultant).
                </p>
              </div>

              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => switchStep(6)}
                  className="px-3.5 py-1.5 text-xs font-bold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg border border-slate-300"
                >
                  ← Back to Step 6
                </button>
                <button
                  type="button"
                  onClick={() => switchStep(8)}
                  className="px-5 py-1.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-lg shadow-sm"
                >
                  Proceed to Step 8: Client Approval →
                </button>
              </div>
            </div>

            {/* Submission Form Card */}
            <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-sm space-y-6">
              <h4 className="text-sm font-bold text-slate-900 border-b border-slate-200 pb-3">
                Transmittal &amp; Client Submission Details
              </h4>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Transmittal Reference Number:
                    <ValidatedInput
                      type="text"
                      value={state.transmittal_no}
                      onChange={(e) =>
                        persist({ transmittal_no: e.target.value })
                      }
                      className="w-full border border-slate-300 rounded-lg p-2.5 text-xs font-mono"
                    />
                  </ValidatedLabel>
                </div>

                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Reviewing Authority / Client:
                    <ValidatedInput
                      type="text"
                      value={state.reviewing_authority}
                      onChange={(e) =>
                        persist({ reviewing_authority: e.target.value })
                      }
                      className="w-full border border-slate-300 rounded-lg p-2.5 text-xs"
                    />
                  </ValidatedLabel>
                </div>

                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Submission Date:
                    <ValidatedInput
                      type="date"
                      value={state.submission_date}
                      onChange={(e) =>
                        persist({ submission_date: e.target.value })
                      }
                      className="w-full border border-slate-300 rounded-lg p-2.5 text-xs"
                    />
                  </ValidatedLabel>
                </div>

                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Target Approval SLA Date:
                    <ValidatedInput
                      type="date"
                      value={state.target_approval_date}
                      onChange={(e) =>
                        persist({ target_approval_date: e.target.value })
                      }
                      className="w-full border border-slate-300 rounded-lg p-2.5 text-xs"
                    />
                  </ValidatedLabel>
                </div>
              </div>

              {/* Document Bundle Included */}
              <div className="p-4 bg-slate-50 rounded-xl border border-slate-200 space-y-3">
                <span className="text-xs font-bold text-slate-800">
                  Documents Bundled in this Transmittal:
                </span>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 text-xs">
                  {state.docs
                    .filter(
                      (d) => d.status === "received" || d.status === "approved",
                    )
                    .map((d) => (
                      <div
                        key={d.id}
                        className="flex items-center gap-2 p-2 bg-white rounded border border-slate-200"
                      >
                        <span className="text-emerald-600 font-bold">✓</span>
                        <span className="font-semibold text-slate-800">
                          {d.title}
                        </span>
                        <span className="text-[10px] text-slate-400 font-mono">
                          ({d.revision})
                        </span>
                      </div>
                    ))}
                </div>
              </div>

              {/* Transmittal Package Upload */}
              <div className="p-4 border-2 border-dashed border-slate-200 rounded-xl text-center space-y-2">
                <span className="text-3xl">📦</span>
                <div className="text-xs font-bold text-slate-800">
                  {state.transmittal_file_name ||
                    "Upload Signed Transmittal Letter / Bundle"}
                </div>
                <ValidatedLabel className="inline-block px-4 py-2 bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold rounded-lg cursor-pointer border border-slate-300">
                  <span>Browse File</span>
                  <ValidatedInput
                    type="file"
                    className="hidden"
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      if (f) persist({ transmittal_file_name: f.name });
                    }}
                  />
                </ValidatedLabel>
              </div>

              <div className="flex justify-end pt-4 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => switchStep(8)}
                  className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-md transition-all flex items-center gap-2"
                >
                  Record Client Technical Approval (Step 8) →
                </button>
              </div>
            </div>
          </div>
        ) : currentStep === 8 ? (
          /* ========================================================================= */
          /* STEP 8: CLIENT TECHNICAL APPROVAL & QAP SIGN-OFF                          */
          /* ========================================================================= */
          <div className="p-6 max-w-[1200px] mx-auto space-y-6">
            {/* Header Strip */}
            <div className="flex flex-wrap items-center justify-between gap-4 bg-white p-4 rounded-xl border border-slate-200 shadow-sm">
              <div>
                <h3 className="text-base font-bold text-slate-900 mt-0.5">
                  Client &amp; Project Technical Approval (QAP Included)
                </h3>
                <p className="text-xs text-slate-500">
                  Record formal technical clearance and approved Quality
                  Assurance Plan (QAP) from AAI / Project Consultant.
                </p>
              </div>

              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => switchStep(7)}
                  className="px-3.5 py-1.5 text-xs font-bold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg border border-slate-300"
                >
                  ← Back to Step 7
                </button>
                <button
                  type="button"
                  onClick={() => switchStep(9)}
                  className="px-5 py-1.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-lg shadow-sm"
                >
                  Proceed to Step 9: PO Release →
                </button>
              </div>
            </div>

            {/* Approval Details Card */}
            <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-sm space-y-6">
              <h4 className="text-sm font-bold text-slate-900 border-b border-slate-200 pb-3">
                Formal Technical Approval Status &amp; Sign-Off
              </h4>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                <div
                  onClick={() => persist({ approval_status: "approved" })}
                  className={`p-4 rounded-xl border cursor-pointer transition-all ${
                    state.approval_status === "approved"
                      ? "bg-emerald-50 border-emerald-500 ring-2 ring-emerald-500"
                      : "bg-white border-slate-200 hover:bg-slate-50"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-xs text-slate-900">
                      Category A / Code 1
                    </span>
                    <span className="text-emerald-600 font-bold">✓</span>
                  </div>
                  <h5 className="font-bold text-sm text-emerald-800 mt-1">
                    Approved
                  </h5>
                  <p className="text-[11px] text-slate-500 mt-1">
                    Clean technical clearance without changes. Ready for PO.
                  </p>
                </div>

                <div
                  onClick={() =>
                    persist({ approval_status: "approved_with_comments" })
                  }
                  className={`p-4 rounded-xl border cursor-pointer transition-all ${
                    state.approval_status === "approved_with_comments"
                      ? "bg-blue-50 border-blue-500 ring-2 ring-blue-500"
                      : "bg-white border-slate-200 hover:bg-slate-50"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-xs text-slate-900">
                      Category B / Code 2
                    </span>
                    <span className="text-blue-600 font-bold">⚠️</span>
                  </div>
                  <h5 className="font-bold text-sm text-blue-800 mt-1">
                    Approved with Comments
                  </h5>
                  <p className="text-[11px] text-slate-500 mt-1">
                    Minor comments to incorporate during fabrication.
                  </p>
                </div>

                <div
                  onClick={() =>
                    persist({ approval_status: "revision_required" })
                  }
                  className={`p-4 rounded-xl border cursor-pointer transition-all ${
                    state.approval_status === "revision_required"
                      ? "bg-red-50 border-red-500 ring-2 ring-red-500"
                      : "bg-white border-slate-200 hover:bg-slate-50"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-bold text-xs text-slate-900">
                      Category C / Code 3
                    </span>
                    <span className="text-red-600 font-bold">✕</span>
                  </div>
                  <h5 className="font-bold text-sm text-red-800 mt-1">
                    Revision Required
                  </h5>
                  <p className="text-[11px] text-slate-500 mt-1">
                    Technical rejection. Vendor must resubmit corrected GA/TDS.
                  </p>
                </div>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-6 pt-2">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Client Approval Letter / Reference Number:
                    <ValidatedInput
                      type="text"
                      value={state.approval_ref_no}
                      onChange={(e) =>
                        persist({ approval_ref_no: e.target.value })
                      }
                      className="w-full border border-slate-300 rounded-lg p-2.5 text-xs font-mono"
                    />
                  </ValidatedLabel>
                </div>

                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Approval Date:
                    <ValidatedInput
                      type="date"
                      value={state.approval_date}
                      onChange={(e) =>
                        persist({ approval_date: e.target.value })
                      }
                      className="w-full border border-slate-300 rounded-lg p-2.5 text-xs"
                    />
                  </ValidatedLabel>
                </div>

                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Approved By (Official Authority):
                    <ValidatedInput
                      type="text"
                      value={state.approver_name}
                      onChange={(e) =>
                        persist({ approver_name: e.target.value })
                      }
                      className="w-full border border-slate-300 rounded-lg p-2.5 text-xs"
                    />
                  </ValidatedLabel>
                </div>

                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Designation:
                    <ValidatedInput
                      type="text"
                      value={state.approver_designation}
                      onChange={(e) =>
                        persist({ approver_designation: e.target.value })
                      }
                      className="w-full border border-slate-300 rounded-lg p-2.5 text-xs"
                    />
                  </ValidatedLabel>
                </div>
              </div>

              {/* QAP Approval Confirmation Checkbox */}
              <div className="p-4 bg-emerald-50 rounded-xl border border-emerald-200 flex items-center justify-between">
                <div>
                  <h5 className="font-bold text-xs text-emerald-900">
                    Quality Assurance Plan (QAP) Witness Points Clearance
                  </h5>
                  <p className="text-[11px] text-emerald-700 mt-0.5">
                    Confirms factory stage inspection and test plan have been
                    formally ratified by client inspectors.
                  </p>
                </div>
                <ValidatedLabel className="flex items-center gap-2 cursor-pointer font-bold text-xs text-emerald-900">
                  <ValidatedInput
                    type="checkbox"
                    checked={state.qap_approved}
                    onChange={(e) =>
                      persist({ qap_approved: e.target.checked })
                    }
                    className="w-5 h-5 text-emerald-600 rounded"
                  />
                  <span>QAP Fully Signed</span>
                </ValidatedLabel>
              </div>

              {/* Upload Signed Approval Letter */}
              <div className="p-4 border-2 border-dashed border-slate-200 rounded-xl text-center space-y-2">
                <span className="text-3xl">📝</span>
                <div className="text-xs font-bold text-slate-800">
                  {state.approval_file_name ||
                    "Upload Stamped Approval Letter / Approved GA Drawings"}
                </div>
                <ValidatedLabel className="inline-block px-4 py-2 bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-bold rounded-lg cursor-pointer border border-slate-300">
                  <span>Browse Stamped Document</span>
                  <ValidatedInput
                    type="file"
                    className="hidden"
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      if (f) persist({ approval_file_name: f.name });
                    }}
                  />
                </ValidatedLabel>
              </div>

              <div className="flex justify-end pt-4 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => switchStep(9)}
                  className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-md transition-all flex items-center gap-2"
                >
                  Generate &amp; Release Full Kit Purchase Order (Step 9) →
                </button>
              </div>
            </div>
          </div>
        ) : (
          /* ========================================================================= */
          /* STEP 9: PURCHASE ORDER (PO) RELEASE & PRINT                               */
          /* ========================================================================= */
          <div className="p-6 max-w-[1000px] mx-auto space-y-6">
            {/* Management Authorization Banner (Hidden on print) */}
            <div className="no-print bg-white p-5 rounded-xl border border-slate-200 shadow-sm space-y-3">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-base font-bold text-slate-900 mt-0.5">
                    Full Kit Purchase Order (PO) Release
                  </h3>
                </div>

                <div className="flex items-center gap-3">
                  <span className="px-3 py-1 bg-emerald-100 text-emerald-800 font-extrabold text-xs rounded-full border border-emerald-300">
                    ✓ Management Approved
                  </span>
                  <button
                    type="button"
                    onClick={handlePrintPDF}
                    className="px-4 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs rounded-lg shadow-sm"
                  >
                    🖨️ Print / Download PO
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-[11px] pt-2 border-t border-slate-100">
                <div className="p-2 rounded bg-slate-50 border border-slate-200">
                  <span className="text-slate-400">PO No:</span>{" "}
                  <strong className="font-mono">{state.po_number}</strong>
                </div>
                <div className="p-2 rounded bg-slate-50 border border-slate-200">
                  <span className="text-slate-400">Awarded Vendor:</span>{" "}
                  <strong>{currentSupplier?.name || "Vendor"}</strong>
                </div>
                <div className="p-2 rounded bg-slate-50 border border-slate-200">
                  <span className="text-slate-400">Total PO Value:</span>{" "}
                  <strong className="text-emerald-700">
                    ₹{" "}
                    {grandTotal.toLocaleString("en-IN", {
                      minimumFractionDigits: 2,
                    })}
                  </strong>
                </div>
                <div className="p-2 rounded bg-slate-50 border border-slate-200">
                  <span className="text-slate-400">Client Approval:</span>{" "}
                  <strong className="text-emerald-700">
                    ✓ Signed ({state.approval_ref_no})
                  </strong>
                </div>
              </div>
            </div>

            {/* PO Configuration Strip (Hidden on print) */}
            <div className="no-print bg-white p-4 rounded-xl border border-slate-200 shadow-sm flex flex-wrap items-center justify-between gap-4 text-xs">
              <div className="flex items-center gap-3">
                <span className="text-slate-500 font-semibold">PO Number:</span>
                <ValidatedInput
                  type="text"
                  value={state.po_number}
                  onChange={(e) => persist({ po_number: e.target.value })}
                  className="font-mono border border-slate-300 rounded px-2.5 py-1 w-52"
                />
              </div>

              <div className="flex items-center gap-3">
                <span className="text-slate-500 font-semibold">PO Date:</span>
                <ValidatedInput
                  type="text"
                  value={state.po_date}
                  onChange={(e) => persist({ po_date: e.target.value })}
                  className="font-mono border border-slate-300 rounded px-2.5 py-1 w-32"
                />
              </div>

              <div className="flex items-center gap-3">
                <span className="text-slate-500 font-semibold">
                  Delivery (Days):
                </span>
                <ValidatedInput
                  type="number"
                  value={state.delivery_timeline_days}
                  onChange={(e) =>
                    persist({
                      delivery_timeline_days: Number(e.target.value) || 21,
                    })
                  }
                  className="border border-slate-300 rounded px-2.5 py-1 w-20"
                />
              </div>
            </div>

            {/* ========================================================================= */}
            {/* THE FORMAL PURCHASE ORDER DOCUMENT (Letterhead & Print Target)             */}
            {/* ========================================================================= */}
            <div
              id="purchase-order-document"
              className="bg-white p-8 sm:p-12 rounded-xl shadow-lg border border-slate-300 text-slate-900 font-serif leading-relaxed text-xs space-y-6"
            >
              {/* Letterhead Header */}
              <div className="border-b-2 border-slate-900 pb-5">
                <div className="flex items-start justify-between gap-4">
                  <div className="flex items-center gap-4">
                    <img
                      src="/vishal_technopower.png"
                      alt="Vishal Technopower Logo"
                      className="h-16 w-auto object-contain"
                    />
                    <div>
                      <h1 className="text-xl font-bold tracking-tight text-slate-900 uppercase font-sans">
                        Vishal Technopower Private Limited
                      </h1>
                      <p className="text-[11px] font-sans text-slate-600 mt-0.5">
                        Empowering Your Electrical Needs · ISO 9001:2015
                        Certified EPC Contractor
                      </p>
                      <p className="text-[10px] font-sans text-slate-500 mt-0.5">
                        Regd. Office: Plot No. 42, Sector-10, Industrial Area,
                        Ghaziabad, Uttar Pradesh - 201001
                      </p>
                      <p className="text-[10px] font-sans text-slate-500">
                        CIN: U31900UP2016PTC085412 · GSTIN: 09AABCV4921C1Z4 ·
                        Email: purchase@vishaltechnopower.com
                      </p>
                    </div>
                  </div>
                  <div className="text-right font-sans">
                    <span className="inline-block bg-slate-900 text-white text-[11px] font-extrabold uppercase px-3 py-1 rounded">
                      Purchase Order
                    </span>
                    <p className="text-[10px] text-slate-500 mt-1">
                      Original for Consignee
                    </p>
                  </div>
                </div>
              </div>

              {/* PO Metadata & Addresses Grid */}
              <div className="grid grid-cols-2 gap-6 font-sans text-xs pt-2">
                {/* Left: Supplier Details */}
                <div className="p-3 bg-slate-50 border border-slate-200 rounded-lg space-y-1">
                  <p className="text-[10px] font-extrabold text-slate-500 uppercase tracking-wider">
                    Supplier / Vendor Details:
                  </p>
                  <p className="font-bold text-sm text-slate-900">
                    {currentSupplier?.name}
                  </p>
                  <p className="text-slate-600">
                    {currentSupplier?.data?.location ||
                      "Industrial Area, Halol, Gujarat"}
                  </p>
                  <p className="text-slate-600">
                    Contact:{" "}
                    <strong>
                      {currentSupplier?.data?.contact_person_name ||
                        "Sales Manager"}
                    </strong>{" "}
                    · {currentSupplier?.data?.phone || "+91 98765 43210"}
                  </p>
                  <p className="text-slate-600">
                    Email:{" "}
                    <strong>
                      {currentSupplier?.data?.email || "sales@vendor.com"}
                    </strong>
                  </p>
                  <p className="text-slate-600 font-mono">
                    GSTIN:{" "}
                    {currentSupplier?.data?.gst_number || "24AABCS1234F1Z8"} ·
                    PAN: {currentSupplier?.data?.pan_number || "AABCS1234F"}
                  </p>
                </div>

                {/* Right: PO & Consignee Details */}
                <div className="p-3 bg-slate-50 border border-slate-200 rounded-lg space-y-1">
                  <div className="flex justify-between">
                    <span className="text-slate-500">PO Number:</span>
                    <span className="font-bold font-mono text-slate-900">
                      {state.po_number}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">PO Date:</span>
                    <span className="font-semibold text-slate-800">
                      {state.po_date}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">LOI Reference:</span>
                    <span className="font-mono text-slate-800">
                      {state.loi_ref_no}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Client Approval Ref:</span>
                    <span className="font-mono text-emerald-700">
                      {state.approval_ref_no}
                    </span>
                  </div>
                  <div className="flex justify-between border-t border-slate-200 pt-1 mt-1">
                    <span className="text-slate-500">Project / Site:</span>
                    <span className="font-bold text-slate-900">
                      {projectName || "AAI Amritsar"}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Delivery Location:</span>
                    <span className="font-medium text-slate-800">
                      Sri Guru Ram Dass Jee Int. Airport, Amritsar
                    </span>
                  </div>
                </div>
              </div>

              {/* Order Scope Narrative */}
              <div>
                <p>Dear Sir/Madam,</p>
                <p className="mt-1.5">
                  With reference to your quotation and our subsequent Letter of
                  Intent (<strong>{state.loi_ref_no}</strong>) followed by
                  client technical approval (
                  <strong>{state.approval_ref_no}</strong>), we are pleased to
                  place this formal <strong>Purchase Order</strong> for the
                  supply of the following materials strictly in accordance with
                  the specifications, agreed rates, terms, and delivery
                  schedules stipulated herein:
                </p>
              </div>

              {/* Items & Commercial Schedule Table */}
              <div className="overflow-x-auto font-sans">
                <table className="vtpl-data-table w-full text-left border-collapse border border-slate-300 text-xs table-fixed">
                  <thead>
                    <tr className="bg-slate-100 text-slate-800 font-bold border-b border-slate-300">
                      <th className="p-2 border border-slate-300 w-10 text-center">
                        #
                      </th>
                      <th className="p-2 border border-slate-300 min-w-[200px] break-words">
                        Item Code &amp; Detailed Specification
                      </th>
                      <th className="p-2 border border-slate-300 w-16 text-center">
                        Qty
                      </th>
                      <th className="p-2 border border-slate-300 w-14 text-center">
                        Unit
                      </th>
                      <th className="p-2 border border-slate-300 text-right w-24">
                        Basic Rate (₹)
                      </th>
                      <th className="p-2 border border-slate-300 text-right w-16">
                        GST %
                      </th>
                      <th className="p-2 border border-slate-300 text-right w-28">
                        Total Amount (₹)
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr>
                      <td className="p-2 border border-slate-300 text-center font-bold">
                        1
                      </td>
                      <td className="p-2 border border-slate-300 break-words overflow-hidden">
                        <div className="font-bold text-slate-900 break-all">
                          {itemSku}
                        </div>
                        <div className="text-[11px] text-slate-700 mt-0.5 leading-relaxed break-words whitespace-normal">
                          {itemDescription}
                        </div>
                        <div className="text-[10px] text-slate-500 mt-1 italic break-words">
                          Approved Make: {currentSupplier?.name} · Approved
                          Drawings: {state.docs[0]?.revision || "Rev 0"}
                        </div>
                      </td>
                      <td className="p-2 border border-slate-300 text-center font-bold">
                        {targetQuantity}
                      </td>
                      <td className="p-2 border border-slate-300 text-center">
                        {targetUom}
                      </td>
                      <td className="p-2 border border-slate-300 text-right font-mono">
                        {unitBasic.toLocaleString("en-IN", {
                          minimumFractionDigits: 2,
                        })}
                      </td>
                      <td className="p-2 border border-slate-300 text-right font-mono">
                        {taxPct}%
                      </td>
                      <td className="p-2 border border-slate-300 text-right font-mono font-bold">
                        {(unitBasic * targetQuantity).toLocaleString("en-IN", {
                          minimumFractionDigits: 2,
                        })}
                      </td>
                    </tr>

                    {/* Freight Row */}
                    <tr>
                      <td className="p-2 border border-slate-300 text-center font-bold">
                        2
                      </td>
                      <td
                        className="p-2 border border-slate-300 text-slate-800"
                        colSpan={5}
                      >
                        Freight &amp; Transit Insurance (Door delivery at AAI
                        Amritsar Site)
                      </td>
                      <td className="p-2 border border-slate-300 text-right font-mono font-bold">
                        {freightTotal.toLocaleString("en-IN", {
                          minimumFractionDigits: 2,
                        })}
                      </td>
                    </tr>

                    {/* Total Summary Rows */}
                    <tr className="bg-slate-50 font-bold">
                      <td
                        className="p-2 border border-slate-300 text-right"
                        colSpan={6}
                      >
                        Sub-Total (Basic + Freight):
                      </td>
                      <td className="p-2 border border-slate-300 text-right font-mono">
                        {(
                          unitBasic * targetQuantity +
                          freightTotal
                        ).toLocaleString("en-IN", {
                          minimumFractionDigits: 2,
                        })}
                      </td>
                    </tr>
                    <tr className="bg-slate-50 font-bold">
                      <td
                        className="p-2 border border-slate-300 text-right"
                        colSpan={6}
                      >
                        Applicable GST (18%):
                      </td>
                      <td className="p-2 border border-slate-300 text-right font-mono">
                        {(unitBasic * targetQuantity * 0.18).toLocaleString(
                          "en-IN",
                          {
                            minimumFractionDigits: 2,
                          },
                        )}
                      </td>
                    </tr>
                    <tr className="bg-slate-200 font-extrabold text-sm text-slate-900">
                      <td
                        className="p-2.5 border border-slate-400 text-right"
                        colSpan={6}
                      >
                        Grand Total Landed Cost (F.O.R. Site):
                      </td>
                      <td className="p-2.5 border border-slate-400 text-right font-mono text-emerald-800">
                        ₹{" "}
                        {grandTotal.toLocaleString("en-IN", {
                          minimumFractionDigits: 2,
                        })}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>

              {/* Amount in words */}
              <div className="font-sans text-xs p-2.5 bg-slate-50 border border-slate-200 rounded">
                <span className="font-bold text-slate-700">
                  Amount in Words:
                </span>{" "}
                <span className="font-semibold text-slate-900 italic">
                  {numberToWords(grandTotal)}
                </span>
              </div>

              {/* Commercial Terms & Conditions */}
              <div className="space-y-2 pt-2 border-t border-slate-200">
                <h4 className="font-sans font-bold text-xs uppercase tracking-wider text-slate-900">
                  Key Commercial &amp; Operational Terms:
                </h4>
                <ol className="list-decimal pl-5 space-y-1.5 text-[11px] text-slate-800 leading-relaxed font-sans">
                  <li>
                    <strong>Price Basis:</strong> F.O.R. Site, Sri Guru Ram Dass
                    Jee International Airport, Amritsar, inclusive of packaging,
                    forwarding, transit insurance, and freight.
                  </li>
                  <li>
                    <strong>Delivery Schedule:</strong> Material must be
                    delivered at project site within{" "}
                    <strong>{state.delivery_timeline_days} days</strong> from
                    date of PO release.
                  </li>
                  <li>
                    <strong>Terms of Payment:</strong>{" "}
                    {getPaymentTermsString(currentSupplier)}
                  </li>
                  <li>
                    <strong>Inspection &amp; Dispatch Clearance:</strong>{" "}
                    Factory inspection will be carried out by client/consultant
                    as per approved QAP. Minimum 15 days advance notice required
                    for inspection call.
                  </li>
                  <li>
                    <strong>Warranty &amp; Guarantee:</strong> 24 months from
                    date of commissioning or 30 months from date of supply,
                    whichever is later, against all manufacturing defects and
                    corrosion.
                  </li>
                  <li>
                    <strong>Liquidated Damages (LD):</strong> In case of delay
                    beyond the agreed delivery schedule, LD will be levied @
                    0.5% per week of delay or part thereof, subject to a maximum
                    of 5% of total PO value.
                  </li>
                  <li>
                    <strong>Statutory Compliance &amp; Jurisdiction:</strong>{" "}
                    All disputes shall be subject to exclusive jurisdiction of
                    courts in Ghaziabad, Uttar Pradesh.
                  </li>
                </ol>
              </div>

              {/* Signatures & Execution Section */}
              <div className="pt-8 border-t border-slate-400 grid grid-cols-3 text-center text-xs font-sans font-bold text-slate-800">
                <div>
                  <p className="mb-10">Prepared by (Procurement)</p>
                  <p className="font-semibold text-slate-700">
                    {user.username}
                  </p>
                  <p className="text-[10px] text-slate-400 font-normal">
                    Purchase Officer
                  </p>
                </div>
                <div>
                  <p className="mb-10">Verified &amp; Checked by</p>
                  <p className="font-semibold text-slate-700">Dinesh Kumar</p>
                  <p className="text-[10px] text-slate-400 font-normal">
                    Head of Procurement
                  </p>
                </div>
                <div>
                  <p className="mb-10">For Vishal Technopower Pvt. Ltd.</p>
                  <p className="font-semibold text-slate-900">
                    Authorized Signatory
                  </p>
                  <p className="text-[10px] text-slate-400 font-normal">
                    Director / Management
                  </p>
                </div>
              </div>

              {/* Vendor Acceptance Block */}
              <div className="mt-6 pt-4 border-t border-dashed border-slate-300 font-sans text-xs text-slate-600 flex justify-between items-end">
                <div>
                  <p className="font-bold text-slate-800">
                    Vendor Acceptance &amp; Confirmation:
                  </p>
                  <p className="text-[10px] text-slate-500 mt-0.5">
                    We hereby confirm acceptance of this Purchase Order and all
                    enclosed terms &amp; conditions.
                  </p>
                </div>
                <div className="text-right">
                  <p className="text-[10px] text-slate-400">
                    Signature &amp; Company Stamp
                  </p>
                </div>
              </div>

              {/* Proceed to Phase 4 Button */}
              <div className="mt-8 pt-4 border-t border-slate-200 flex justify-end gap-3 print:hidden font-sans">
                <button
                  type="button"
                  onClick={() => onNavigateStep(10)}
                  className="px-5 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white rounded-md font-bold text-xs shadow-xs flex items-center gap-2 transition"
                >
                  Proceed to Phase 4 (Step 10: Vendor PO Acceptance &amp;
                  Inspection) →
                </button>
              </div>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
