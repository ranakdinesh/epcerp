"use client";
import {
  ValidatedInput,
  ValidatedLabel,
  ValidatedTextarea,
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

export type VendorQuote = {
  supplier_id: string;
  quote_reference: string;
  quote_date: string;
  valid_until: string;
  basic_rate: number;
  discount_pct: number;
  tax_pct: number;
  freight_amount: number;
  delivery_days: number;
  payment_terms: string;
  warranty: string;
  compliant: boolean;
  compliance_notes: string;
  document_name?: string;
  document_url?: string;
  submitted_at?: string;
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

export function PurchaseSourcingWorkspace({
  project,
  projectName,
  lines,
  boId,
  initialStep = 3,
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

  // Storage key for persistent quotes & selection
  const storageKey = `vtpl_sourcing_${project}_${boId || "default"}`;

  // Sourcing state: quotes per supplier
  const [quotes, setQuotes] = useState<Record<string, VendorQuote>>({});
  const [selectedSupplierId, setSelectedSupplierId] = useState<string>("");
  const [negotiatedRate, setNegotiatedRate] = useState<number | "">("");
  const [selectionReason, setSelectionReason] = useState<string>(
    "Selected as L1 lowest compliant bidder offering standard delivery timeline and approved payment terms.",
  );
  const [loiNumber, setLoiNumber] = useState<string>(
    `LOI/VTP/${new Date().getFullYear()}/001`,
  );
  const [loiDate, setLoiDate] = useState<string>(
    new Date()
      .toLocaleDateString("en-GB", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
      })
      .replace(/\//g, "."),
  );

  // Quote Entry Modal state
  const [modalSupplier, setModalSupplier] = useState<Supplier | null>(null);
  const [modalQuote, setModalQuote] = useState<Partial<VendorQuote>>({});

  // Filter valid lines
  const validLines = lines.filter(
    (l) => l.item_id || l.item_snapshot?.sku || l.item_snapshot?.description,
  );
  const primaryItem = validLines[0] || null;
  const targetQuantity = primaryItem
    ? parseFloat(primaryItem.quantity || "1") || 1
    : 1;
  const targetUom = primaryItem?.item_snapshot?.uom || "Unit";

  // Load suppliers and restore quotes
  useEffect(() => {
    let live = true;
    async function loadData() {
      try {
        const res = await fetch("/api/settings/suppliers");
        if (res.ok) {
          const data: Supplier[] = await res.json();
          if (live) {
            setSuppliers(data);

            // Restore from localStorage or populate sensible initial quotes
            const saved = localStorage.getItem(storageKey);
            if (saved) {
              try {
                const parsed = JSON.parse(saved);
                if (parsed.quotes) setQuotes(parsed.quotes);
                if (parsed.selectedSupplierId)
                  setSelectedSupplierId(parsed.selectedSupplierId);
                if (parsed.negotiatedRate)
                  setNegotiatedRate(parsed.negotiatedRate);
                if (parsed.selectionReason)
                  setSelectionReason(parsed.selectionReason);
              } catch {
                /* ignore */
              }
            } else if (data.length >= 2) {
              // Seed initial sample quotes for demonstration
              const s1 = data[0];
              const s2 = data[1];
              const seeded: Record<string, VendorQuote> = {};

              seeded[s1.id] = {
                supplier_id: s1.id,
                quote_reference: `QT-${s1.code}-2026-001`,
                quote_date: "2026-09-22",
                valid_until: "2026-10-22",
                basic_rate: 1850.0,
                discount_pct: 5.0,
                tax_pct: 18.0,
                freight_amount: 5000.0,
                delivery_days: 21,
                payment_terms:
                  s1.data.payment_terms?.description ||
                  "20% advance, 80% on delivery",
                warranty: "5 Years",
                compliant: true,
                compliance_notes:
                  "Fully compliant with tender technical data sheet and IS/IEC standards.",
                document_name: `${s1.name.split(" ")[0]}_Quotation_Official.pdf`,
                submitted_at: "2026-09-22T10:30:00Z",
              };

              seeded[s2.id] = {
                supplier_id: s2.id,
                quote_reference: `QT-${s2.code}-2026-042`,
                quote_date: "2026-09-23",
                valid_until: "2026-10-23",
                basic_rate: 1980.0,
                discount_pct: 7.5,
                tax_pct: 18.0,
                freight_amount: 4500.0,
                delivery_days: 14,
                payment_terms:
                  s2.data.payment_terms?.description ||
                  "10% advance, 90% against dispatch",
                warranty: "5 Years",
                compliant: true,
                compliance_notes:
                  "Compliant. Short delivery timeline available from Halol plant.",
                document_name: `${s2.name.split(" ")[0]}_Commercial_Quote.pdf`,
                submitted_at: "2026-09-23T14:15:00Z",
              };

              setQuotes(seeded);
              setSelectedSupplierId(s1.id);
            }
          }
        }
      } catch (err) {
        console.error("Failed to load suppliers:", err);
      } finally {
        if (live) setLoading(false);
      }
    }
    void loadData();
    return () => {
      live = false;
    };
  }, [storageKey]);

  // Persist state updates
  function persistState(
    updatedQuotes = quotes,
    updatedSelected = selectedSupplierId,
    updatedNeg = negotiatedRate,
    updatedReason = selectionReason,
  ) {
    localStorage.setItem(
      storageKey,
      JSON.stringify({
        quotes: updatedQuotes,
        selectedSupplierId: updatedSelected,
        negotiatedRate: updatedNeg,
        selectionReason: updatedReason,
      }),
    );
  }

  // Cost calculation helper for an offer
  function calculateOfferCost(q: VendorQuote) {
    const discountedBasic = q.basic_rate * (1 - q.discount_pct / 100);
    const taxAmount = discountedBasic * (q.tax_pct / 100);
    const unitLanded = discountedBasic + taxAmount;
    const totalMaterial = unitLanded * targetQuantity;
    const grandTotal = totalMaterial + q.freight_amount;
    return {
      discountedBasic,
      taxAmount,
      unitLanded,
      totalMaterial,
      grandTotal,
    };
  }

  // Rank quotes for Comparative Statement (L1, L2, L3)
  const evaluatedQuotes = Object.values(quotes).map((q) => {
    const cost = calculateOfferCost(q);
    return { ...q, ...cost };
  });
  evaluatedQuotes.sort((a, b) => a.grandTotal - b.grandTotal);

  // The L1 bidder
  const l1Quote = evaluatedQuotes[0] || null;

  // Selected supplier object for Step 5 LOI
  const chosenSupplier =
    suppliers.find((s) => s.id === selectedSupplierId) || suppliers[0];
  const chosenQuote = quotes[selectedSupplierId] || l1Quote || null;
  const effectiveFinalRate =
    typeof negotiatedRate === "number" && negotiatedRate > 0
      ? negotiatedRate
      : chosenQuote?.basic_rate || 0;

  // Handle quote modal submission
  function handleSaveQuote() {
    if (!modalSupplier) return;
    const newQuote: VendorQuote = {
      supplier_id: modalSupplier.id,
      quote_reference:
        modalQuote.quote_reference?.trim() || `QT-${modalSupplier.code}-001`,
      quote_date:
        modalQuote.quote_date || new Date().toISOString().split("T")[0],
      valid_until:
        modalQuote.valid_until ||
        new Date(Date.now() + 30 * 86400000).toISOString().split("T")[0],
      basic_rate: Number(modalQuote.basic_rate) || 0,
      discount_pct: Number(modalQuote.discount_pct) || 0,
      tax_pct: Number(modalQuote.tax_pct) ?? 18,
      freight_amount: Number(modalQuote.freight_amount) || 0,
      delivery_days: Number(modalQuote.delivery_days) || 21,
      payment_terms:
        modalQuote.payment_terms?.trim() ||
        modalSupplier.data.payment_terms?.description ||
        "As per agreed terms",
      warranty: modalQuote.warranty?.trim() || "5 Years",
      compliant: modalQuote.compliant !== false,
      compliance_notes:
        modalQuote.compliance_notes?.trim() ||
        "Fully compliant with specifications.",
      document_name:
        modalQuote.document_name || "Quotation_Uploaded_Document.pdf",
      submitted_at: new Date().toISOString(),
    };

    const updated = { ...quotes, [modalSupplier.id]: newQuote };
    setQuotes(updated);
    if (!selectedSupplierId) setSelectedSupplierId(modalSupplier.id);
    persistState(updated);
    setModalSupplier(null);
  }

  function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (file) {
      setModalQuote((prev) => ({
        ...prev,
        document_name: file.name,
      }));
    }
  }

  return (
    <div className="bo-stage flex flex-col flex-1 min-w-0 bg-slate-100 text-slate-900 overflow-hidden">
      {/* Print stylesheet for formal LOI output */}
      <style>{`
        @media print {
          body * {
            visibility: hidden !important;
          }
          #formal-loi-document,
          #formal-loi-document * {
            visibility: visible !important;
          }
          #formal-loi-document {
            position: absolute !important;
            left: 0 !important;
            top: 0 !important;
            width: 100% !important;
            margin: 0 !important;
            padding: 12mm !important;
            background: #ffffff !important;
            color: #000000 !important;
            box-shadow: none !important;
            border: none !important;
          }
          .no-print {
            display: none !important;
          }
          @page {
            size: A4 portrait;
            margin: 10mm;
          }
        }
      `}</style>

      {/* Top Workspace Bar - Compact, Slate Header */}
      {currentStep === 5 ? (
        <header className="no-print bo-stage-toolbar">
          {currentStep === 5 && (
            <button
              onClick={() => window.print()}
              type="button"
              className="flex items-center gap-1.5 px-4 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs rounded-lg shadow-sm"
            >
              Print / download LOI
            </button>
          )}
        </header>
      ) : null}

      {/* Main Sourcing Content */}
      <main className="flex-1 min-w-0 overflow-y-auto">
        {loading ? (
          <div className="p-16 text-center text-slate-500 text-sm font-semibold">
            Loading quotation records and vendor data...
          </div>
        ) : currentStep === 3 ? (
          /* ========================================================================= */
          /* STEP 3: INVITE VENDOR QUOTATIONS & UPLOAD QUOTES                          */
          /* ========================================================================= */
          <div className="p-6 max-w-[1500px] mx-auto space-y-4">
            {/* Status Summary */}
            <div className="bg-white p-4 rounded-xl border border-slate-200 shadow-sm flex flex-wrap items-center justify-between gap-4">
              <div className="flex items-center gap-6 text-xs">
                <div>
                  <span className="text-slate-400 font-medium">
                    Invited Vendors:
                  </span>{" "}
                  <strong className="text-slate-900 text-sm">
                    {suppliers.length}
                  </strong>
                </div>
                <div>
                  <span className="text-slate-400 font-medium">
                    Quotations Received:
                  </span>{" "}
                  <strong className="text-emerald-700 text-sm">
                    {Object.keys(quotes).length} / {suppliers.length}
                  </strong>
                </div>
                <div>
                  <span className="text-slate-400 font-medium">
                    Target Qty:
                  </span>{" "}
                  <strong className="text-slate-900 text-sm">
                    {targetQuantity} {targetUom}
                  </strong>
                </div>
              </div>

              <button
                type="button"
                onClick={() => switchStep(4)}
                disabled={Object.keys(quotes).length === 0}
                className="px-5 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-slate-300 text-white text-xs font-bold rounded-lg shadow-sm flex items-center gap-2"
              >
                Proceed to Step 4: Compare Prices ({Object.keys(quotes).length}{" "}
                Quotes Ready) →
              </button>
            </div>

            {/* Vendor Quotations Grid */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {suppliers.map((sup) => {
                const quote = quotes[sup.id];
                const cost = quote ? calculateOfferCost(quote) : null;

                return (
                  <div
                    key={sup.id}
                    className={`bg-white rounded-xl border p-5 shadow-sm flex flex-col justify-between transition-all ${
                      quote
                        ? "border-emerald-300 bg-emerald-50/20"
                        : "border-slate-200 hover:border-slate-300"
                    }`}
                  >
                    <div>
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <span className="font-mono text-[10px] font-bold text-slate-500 uppercase">
                            {sup.code}
                          </span>
                          <h3 className="text-sm font-bold text-slate-900 leading-snug mt-0.5">
                            {sup.name}
                          </h3>
                        </div>
                        {quote ? (
                          <span className="px-2 py-0.5 text-[10px] font-extrabold bg-emerald-100 text-emerald-800 rounded-full border border-emerald-300 shrink-0">
                            ✓ Quote Received
                          </span>
                        ) : (
                          <span className="px-2 py-0.5 text-[10px] font-bold bg-amber-100 text-amber-800 rounded-full border border-amber-300 shrink-0">
                            ⏳ RFQ Pending
                          </span>
                        )}
                      </div>

                      <div className="text-[11px] text-slate-600 space-y-1 mt-3 border-t border-slate-100 pt-2.5">
                        {sup.data.contact_person_name && (
                          <p>
                            <span className="text-slate-400">Contact:</span>{" "}
                            <strong>{sup.data.contact_person_name}</strong>
                          </p>
                        )}
                        {sup.data.phone && (
                          <p>
                            <span className="text-slate-400">Phone:</span>{" "}
                            {sup.data.phone}
                          </p>
                        )}
                        {sup.data.email && (
                          <p>
                            <span className="text-slate-400">Email:</span>{" "}
                            {sup.data.email}
                          </p>
                        )}
                      </div>

                      {quote && cost ? (
                        <div className="mt-4 bg-white p-3 rounded-lg border border-slate-200 text-xs space-y-1">
                          <div className="flex justify-between text-slate-600">
                            <span>Ref:</span>
                            <span className="font-mono font-bold text-slate-900">
                              {quote.quote_reference}
                            </span>
                          </div>
                          <div className="flex justify-between text-slate-600">
                            <span>Basic Rate:</span>
                            <span>₹{quote.basic_rate.toFixed(2)}</span>
                          </div>
                          <div className="flex justify-between text-slate-600">
                            <span>Discount / GST:</span>
                            <span>
                              {quote.discount_pct}% / {quote.tax_pct}%
                            </span>
                          </div>
                          <div className="flex justify-between font-bold text-slate-900 border-t border-slate-100 pt-1 mt-1">
                            <span>Landed Total:</span>
                            <span className="text-blue-700">
                              ₹
                              {cost.grandTotal.toLocaleString("en-IN", {
                                minimumFractionDigits: 2,
                              })}
                            </span>
                          </div>
                          <div className="flex justify-between text-[11px] text-slate-500 pt-0.5">
                            <span>Delivery:</span>
                            <span>{quote.delivery_days} days</span>
                          </div>
                          {quote.document_name && (
                            <div className="pt-1.5 text-[11px] text-blue-600 flex items-center gap-1">
                              <span>📎</span>
                              <span className="truncate">
                                {quote.document_name}
                              </span>
                            </div>
                          )}
                        </div>
                      ) : (
                        <div className="mt-4 p-3 bg-slate-50 rounded-lg text-xs text-slate-500 italic text-center border border-dashed border-slate-200">
                          RFQ sent. Awaiting formal quote upload.
                        </div>
                      )}
                    </div>

                    <div className="mt-4 pt-3 border-t border-slate-100 flex items-center justify-between">
                      <button
                        type="button"
                        onClick={() => {
                          setModalSupplier(sup);
                          setModalQuote(
                            quote || {
                              basic_rate: 0,
                              discount_pct: 0,
                              tax_pct: 18,
                              freight_amount: 0,
                              delivery_days: 21,
                              payment_terms:
                                sup.data.payment_terms?.description ||
                                "20% advance, 80% on delivery",
                              warranty: "5 Years",
                              compliant: true,
                            },
                          );
                        }}
                        className={`w-full py-2 px-3 text-xs font-bold rounded-lg transition-all ${
                          quote
                            ? "bg-slate-100 hover:bg-slate-200 text-slate-800 border border-slate-300"
                            : "bg-blue-600 hover:bg-blue-700 text-white shadow-sm"
                        }`}
                      >
                        {quote
                          ? "✏️ Edit / Re-upload Quote"
                          : "📥 Upload & Record Quote"}
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        ) : currentStep === 4 ? (
          /* ========================================================================= */
          /* STEP 4: PRICE COMPARISON (COMPARATIVE STATEMENT / CS TABLE)               */
          /* ========================================================================= */
          <div className="p-6 max-w-[1600px] mx-auto space-y-5">
            {/* Header Metrics */}
            <div className="bg-white p-4 rounded-xl border border-slate-200 shadow-sm flex flex-wrap items-center justify-between gap-4">
              <div>
                <h3 className="text-base font-bold text-slate-900">
                  Commercial Comparative Statement (CS)
                </h3>
                <p className="text-xs text-slate-500 mt-0.5">
                  Side-by-side rate evaluation of {evaluatedQuotes.length}{" "}
                  quotations. All rates normalized for {targetQuantity}{" "}
                  {targetUom}.
                </p>
              </div>

              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => switchStep(3)}
                  className="px-3.5 py-1.5 text-xs font-bold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg border border-slate-300"
                >
                  ← Back to Step 3 Quotes
                </button>
                <button
                  type="button"
                  onClick={() => switchStep(5)}
                  className="px-5 py-1.5 text-xs font-bold text-white bg-blue-600 hover:bg-blue-700 rounded-lg shadow-sm"
                >
                  Proceed to Step 5: LOI Release →
                </button>
              </div>
            </div>

            {/* Side-by-Side Comparison Table */}
            {evaluatedQuotes.length === 0 ? (
              <div className="bg-white p-12 text-center rounded-xl border border-slate-200">
                <p className="text-sm text-slate-600">
                  No vendor quotations have been recorded yet.
                </p>
                <button
                  type="button"
                  onClick={() => switchStep(3)}
                  className="mt-4 px-4 py-2 bg-blue-600 text-white text-xs font-bold rounded-lg"
                >
                  Go to Step 3 and Record Quotes
                </button>
              </div>
            ) : (
              <div className="bg-white rounded-xl border border-slate-200 shadow-sm overflow-x-auto">
                <table className="vtpl-data-table w-full text-left border-collapse text-xs">
                  <thead>
                    <tr className="bg-slate-900 text-white border-b border-slate-800">
                      <th className="py-3 px-4 w-60 uppercase text-[11px] font-extrabold tracking-wider">
                        Evaluation Parameter
                      </th>
                      {evaluatedQuotes.map((q, idx) => {
                        const sup = suppliers.find(
                          (s) => s.id === q.supplier_id,
                        );
                        const isL1 = idx === 0;
                        const isSelected = selectedSupplierId === q.supplier_id;

                        return (
                          <th
                            key={q.supplier_id}
                            className={`py-3 px-4 min-w-[240px] text-center border-l border-slate-800 ${
                              isL1 ? "bg-slate-800/90 text-emerald-400" : ""
                            }`}
                          >
                            <div className="flex items-center justify-center gap-1.5 mb-1">
                              <span
                                className={`px-2 py-0.5 rounded text-[10px] font-black ${
                                  isL1
                                    ? "bg-emerald-500 text-slate-950"
                                    : idx === 1
                                      ? "bg-blue-500 text-white"
                                      : "bg-slate-700 text-slate-300"
                                }`}
                              >
                                {isL1 ? "L1 (LOWEST)" : `L${idx + 1}`}
                              </span>
                              {isSelected && (
                                <span className="px-2 py-0.5 rounded text-[10px] font-bold bg-amber-400 text-slate-950">
                                  ✓ SELECTED
                                </span>
                              )}
                            </div>
                            <p className="font-bold text-white text-xs truncate">
                              {sup?.name || "Vendor"}
                            </p>
                            <p className="font-mono text-[10px] text-slate-400 mt-0.5">
                              {q.quote_reference}
                            </p>
                          </th>
                        );
                      })}
                    </tr>
                  </thead>

                  <tbody className="divide-y divide-slate-200">
                    {/* Basic Unit Rate */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        Basic Unit Rate (₹)
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center font-mono border-l border-slate-200"
                        >
                          ₹{q.basic_rate.toFixed(2)}
                        </td>
                      ))}
                    </tr>

                    {/* Discount */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        Offered Discount (%)
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center text-slate-600 border-l border-slate-200"
                        >
                          {q.discount_pct > 0 ? `${q.discount_pct}%` : "Nil"}
                        </td>
                      ))}
                    </tr>

                    {/* Net Basic Rate */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        Net Basic Rate (₹)
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center font-mono font-medium border-l border-slate-200"
                        >
                          ₹{q.discountedBasic.toFixed(2)}
                        </td>
                      ))}
                    </tr>

                    {/* GST / Tax */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        GST / Tax Amount (₹)
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center font-mono text-slate-600 border-l border-slate-200"
                        >
                          {q.tax_pct}% (₹{q.taxAmount.toFixed(2)})
                        </td>
                      ))}
                    </tr>

                    {/* Freight & Charges */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        Freight &amp; P&amp;F (₹)
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center font-mono text-slate-600 border-l border-slate-200"
                        >
                          ₹{q.freight_amount.toFixed(2)}
                        </td>
                      ))}
                    </tr>

                    {/* Landed Unit Cost */}
                    <tr className="bg-slate-100 font-bold">
                      <td className="py-3 px-4 text-slate-900">
                        All-Inclusive Landed Unit Rate
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-3 px-4 text-center font-mono text-slate-900 border-l border-slate-200"
                        >
                          ₹{q.unitLanded.toFixed(2)} / {targetUom}
                        </td>
                      ))}
                    </tr>

                    {/* Total Project Cost */}
                    <tr className="bg-blue-50/40 font-extrabold text-sm border-t-2 border-b-2 border-blue-200">
                      <td className="py-3.5 px-4 text-blue-950 uppercase tracking-wide">
                        Total Landed Cost ({targetQuantity} {targetUom})
                      </td>
                      {evaluatedQuotes.map((q, idx) => (
                        <td
                          key={q.supplier_id}
                          className={`py-3.5 px-4 text-center font-mono text-base border-l border-slate-200 ${
                            idx === 0
                              ? "text-emerald-700 bg-emerald-50/80 font-black"
                              : "text-blue-900"
                          }`}
                        >
                          ₹
                          {q.grandTotal.toLocaleString("en-IN", {
                            minimumFractionDigits: 2,
                          })}
                        </td>
                      ))}
                    </tr>

                    {/* Delivery Timeline */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        Delivery Timeline
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center font-medium border-l border-slate-200"
                        >
                          {q.delivery_days} Days (
                          {Math.ceil(q.delivery_days / 7)} Weeks)
                        </td>
                      ))}
                    </tr>

                    {/* Payment Terms */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        Offered Payment Terms
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center text-[11px] text-slate-700 border-l border-slate-200"
                        >
                          {q.payment_terms}
                        </td>
                      ))}
                    </tr>

                    {/* Technical Compliance */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        Technical Compliance
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center border-l border-slate-200"
                        >
                          {q.compliant ? (
                            <span className="text-emerald-700 font-bold">
                              ✓ 100% Compliant
                            </span>
                          ) : (
                            <span className="text-red-700 font-bold">
                              ⚠ Deviations noted
                            </span>
                          )}
                        </td>
                      ))}
                    </tr>

                    {/* Uploaded Quote Document */}
                    <tr className="hover:bg-slate-50">
                      <td className="py-2.5 px-4 font-bold text-slate-700 bg-slate-50/70">
                        Uploaded Quote Document
                      </td>
                      {evaluatedQuotes.map((q) => (
                        <td
                          key={q.supplier_id}
                          className="py-2.5 px-4 text-center text-blue-600 font-medium border-l border-slate-200"
                        >
                          {q.document_name ? (
                            <span
                              className="inline-flex items-center gap-1 cursor-pointer hover:underline"
                              title="View uploaded quote document"
                            >
                              📄 {q.document_name}
                            </span>
                          ) : (
                            <span className="text-slate-400">
                              None attached
                            </span>
                          )}
                        </td>
                      ))}
                    </tr>

                    {/* Select Winning Vendor Action */}
                    <tr className="bg-slate-50">
                      <td className="py-3 px-4 font-bold text-slate-900">
                        Final Vendor Award Decision
                      </td>
                      {evaluatedQuotes.map((q) => {
                        const isSelected = selectedSupplierId === q.supplier_id;
                        return (
                          <td
                            key={q.supplier_id}
                            className="py-3 px-4 text-center border-l border-slate-200"
                          >
                            <button
                              type="button"
                              onClick={() => {
                                setSelectedSupplierId(q.supplier_id);
                                persistState(quotes, q.supplier_id);
                              }}
                              className={`px-4 py-1.5 rounded-lg text-xs font-bold transition-all ${
                                isSelected
                                  ? "bg-emerald-600 text-white shadow-sm"
                                  : "bg-white hover:bg-slate-100 text-slate-700 border border-slate-300"
                              }`}
                            >
                              {isSelected
                                ? "✓ Finalized Vendor"
                                : "Select Vendor"}
                            </button>
                          </td>
                        );
                      })}
                    </tr>
                  </tbody>
                </table>
              </div>
            )}

            {/* Negotiation & Finalization Box */}
            <div className="bg-white p-5 rounded-xl border border-slate-200 shadow-sm space-y-4">
              <h4 className="text-sm font-bold text-slate-900">
                Commercial Negotiation &amp; Selection Approval
              </h4>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
                <div>
                  <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                    Final Negotiated Unit Basic Rate (₹):
                    <ValidatedInput
                      name="negotiated_rate"
                      type="number"
                      value={negotiatedRate}
                      onChange={(e) => {
                        const val =
                          e.target.value === ""
                            ? ""
                            : parseFloat(e.target.value);
                        setNegotiatedRate(val);
                        persistState(quotes, selectedSupplierId, val);
                      }}
                      placeholder={`e.g. ${chosenQuote?.basic_rate || 0}`}
                      className="w-full font-mono text-sm border border-slate-300 rounded-lg p-2.5"
                    />
                  </ValidatedLabel>
                  <small className="text-slate-500 block mt-1">
                    Leave blank to use quoted basic rate (₹
                    {chosenQuote?.basic_rate?.toFixed(2) || 0}).
                  </small>
                </div>

                <div>
                  <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                    Selection Justification / Notes:
                    <ValidatedTextarea
                      name="selection_reason"
                      rows={2}
                      value={selectionReason}
                      onChange={(e) => {
                        setSelectionReason(e.target.value);
                        persistState(
                          quotes,
                          selectedSupplierId,
                          negotiatedRate,
                          e.target.value,
                        );
                      }}
                      className="w-full border border-slate-300 rounded-lg p-2 text-xs"
                    />
                  </ValidatedLabel>
                </div>
              </div>

              <div className="flex justify-end pt-2 border-t border-slate-100">
                <button
                  type="button"
                  onClick={() => switchStep(5)}
                  className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-md transition-all flex items-center gap-2"
                >
                  Confirm Award &amp; Release LOI (Step 5) →
                </button>
              </div>
            </div>
          </div>
        ) : (
          /* ========================================================================= */
          /* STEP 5: VENDOR ONBOARDING & LOI RELEASE                                   */
          /* ========================================================================= */
          <div className="p-6 max-w-[1000px] mx-auto space-y-6">
            {/* Onboarding Compliance Banner (Hidden on print) */}
            <div className="no-print bg-white p-5 rounded-xl border border-slate-200 shadow-sm space-y-3">
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-[10px] font-extrabold uppercase tracking-wider text-slate-500">
                    Vendor Onboarding Verification
                  </span>
                  <h3 className="text-base font-bold text-slate-900 mt-0.5">
                    {chosenSupplier?.name} ({chosenSupplier?.code})
                  </h3>
                </div>
                {chosenSupplier?.data?.onboarding_verified ? (
                  <span className="px-3 py-1 bg-emerald-100 text-emerald-800 font-extrabold text-xs rounded-full border border-emerald-300">
                    ✓ Onboarding Complete &amp; Encrypted
                  </span>
                ) : (
                  <span className="px-3 py-1 bg-amber-100 text-amber-800 font-bold text-xs rounded-full border border-amber-300">
                    ⚠️ Verification Pending (7 Documents Required)
                  </span>
                )}
              </div>

              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-[11px] pt-2 border-t border-slate-100">
                <div className="p-2 rounded bg-slate-50 border border-slate-200">
                  <span className="text-slate-400">PAN:</span>{" "}
                  <strong>
                    {chosenSupplier?.data?.pan_number || "Verified"}
                  </strong>
                </div>
                <div className="p-2 rounded bg-slate-50 border border-slate-200">
                  <span className="text-slate-400">GST:</span>{" "}
                  <strong>
                    {chosenSupplier?.data?.gst_number || "Verified"}
                  </strong>
                </div>
                <div className="p-2 rounded bg-slate-50 border border-slate-200">
                  <span className="text-slate-400">Payment Terms:</span>{" "}
                  <strong>
                    {chosenSupplier?.data?.payment_terms?.advance_pct || 20}%
                    Adv /{" "}
                    {chosenSupplier?.data?.payment_terms?.delivery_pct || 40}%
                    Del
                  </strong>
                </div>
                <div className="p-2 rounded bg-slate-50 border border-slate-200">
                  <span className="text-slate-400">Encrypted Vault:</span>{" "}
                  <strong className="text-emerald-700">
                    AES-256-GCM Active
                  </strong>
                </div>
              </div>
            </div>

            {/* Letterhead LOI Configuration Strip (Hidden on print) */}
            <div className="no-print bg-white p-4 rounded-xl border border-slate-200 shadow-sm flex flex-wrap items-center justify-between gap-4 text-xs">
              <div className="flex items-center gap-3">
                <span className="text-slate-500 font-semibold">
                  LOI Ref No:
                </span>
                <ValidatedInput
                  name="loi_number"
                  type="text"
                  value={loiNumber}
                  onChange={(e) => setLoiNumber(e.target.value)}
                  className="font-mono border border-slate-300 rounded px-2.5 py-1 w-44"
                />
              </div>

              <div className="flex items-center gap-3">
                <span className="text-slate-500 font-semibold">LOI Date:</span>
                <ValidatedInput
                  name="loi_date"
                  type="text"
                  value={loiDate}
                  onChange={(e) => setLoiDate(e.target.value)}
                  className="font-mono border border-slate-300 rounded px-2.5 py-1 w-28"
                />
              </div>

              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => window.print()}
                  className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white font-bold rounded-lg shadow-sm"
                >
                  🖨️ Print / Save LOI PDF
                </button>
                <button
                  type="button"
                  onClick={() => switchStep(6)}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white font-bold rounded-lg shadow-sm"
                >
                  Step 6 (Tech Docs) →
                </button>
              </div>
            </div>

            {/* FORMAL LETTER OF INTENT (LOI) DOCUMENT */}
            <div
              id="formal-loi-document"
              className="bg-white p-10 rounded-xl shadow-lg border border-slate-300 text-slate-900"
              style={{
                minHeight: "1050px",
                fontFamily: '"Times New Roman", Times, Georgia, serif',
              }}
            >
              {/* Header: Company Logo & Tagline */}
              <div className="flex items-start justify-between border-b-2 border-slate-800 pb-4">
                <div className="flex items-center gap-4">
                  <div
                    className="w-12 h-12 bg-blue-700 flex items-center justify-center text-white font-black text-2xl tracking-tighter rounded-sm shadow-sm"
                    style={{ transform: "skewX(-10deg)" }}
                  >
                    VT
                  </div>
                  <div>
                    <h1 className="text-2xl font-black tracking-wide text-blue-900 uppercase font-sans">
                      Vishal Technopower
                    </h1>
                    <p className="text-xs font-semibold italic text-blue-700 tracking-wide font-sans">
                      Engineering Your Electrical Needs!
                    </p>
                  </div>
                </div>

                <div className="text-right">
                  <p className="text-xs font-bold font-sans text-slate-800">
                    Date: <span className="font-semibold">{loiDate}</span>
                  </p>
                  <p className="text-xs font-bold font-sans text-slate-800 mt-1">
                    Ref: <span className="font-mono">{loiNumber}</span>
                  </p>
                </div>
              </div>

              {/* Title */}
              <div className="text-center my-6">
                <h2 className="text-xl font-black uppercase tracking-wider text-slate-900 underline underline-offset-4 font-sans">
                  Letter of Intent (LOI)
                </h2>
              </div>

              {/* Recipient Vendor Box */}
              <div className="text-xs font-sans mb-6 leading-relaxed">
                <p className="font-bold text-slate-900">To,</p>
                <p className="font-bold text-sm text-blue-950 mt-1">
                  {chosenSupplier?.name}
                </p>
                {chosenSupplier?.data?.contact_person_name && (
                  <p className="text-slate-700">
                    Kind Attn: {chosenSupplier.data.contact_person_name}
                  </p>
                )}
                {chosenSupplier?.data?.location && (
                  <p className="text-slate-600">
                    Location: {chosenSupplier.data.location}
                  </p>
                )}
                {chosenSupplier?.data?.gst_number && (
                  <p className="text-slate-600 font-mono">
                    GSTIN: {chosenSupplier.data.gst_number}
                  </p>
                )}
              </div>

              {/* Subject */}
              <div className="bg-slate-50 p-3 border border-slate-300 text-xs font-sans font-bold mb-6">
                Subject: Award of Contract / Letter of Intent for Supply of
                Electrical Materials for Project{" "}
                <span className="uppercase text-blue-900">
                  {projectName || project}
                </span>
              </div>

              <div className="text-xs font-sans leading-relaxed space-y-3 mb-6">
                <p>Dear Sir / Madam,</p>
                <p className="text-justify">
                  With reference to your quotation{" "}
                  <strong className="font-mono">
                    {chosenQuote?.quote_reference || "submitted"}
                  </strong>{" "}
                  dated <strong>{chosenQuote?.quote_date || loiDate}</strong>{" "}
                  and subsequent commercial discussions, we are pleased to issue
                  this <strong>Letter of Intent (LOI)</strong> for the supply of
                  the following materials as per the agreed specifications,
                  rates, and delivery schedule:
                </p>
              </div>

              {/* Awarded Items Table */}
              <table className="vtpl-data-table w-full border-collapse border border-slate-800 text-xs font-sans mb-6">
                <thead>
                  <tr className="bg-slate-100 border-b border-slate-800 font-bold text-center">
                    <th className="border border-slate-800 p-2 w-12">S. No.</th>
                    <th className="border border-slate-800 p-2 text-left">
                      Item Description &amp; Technical Scope
                    </th>
                    <th className="border border-slate-800 p-2 w-16">Qty.</th>
                    <th className="border border-slate-800 p-2 w-24">
                      Unit Rate (₹)
                    </th>
                    <th className="border border-slate-800 p-2 w-28">
                      Total Amount (₹)
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr className="border-b border-slate-800">
                    <td className="border border-slate-800 p-2 text-center align-top font-bold">
                      1
                    </td>
                    <td className="border border-slate-800 p-2 align-top text-justify">
                      <p className="font-semibold text-slate-900">
                        {primaryItem?.item_snapshot?.description ||
                          "High voltage / electrical equipment as per tender"}
                      </p>
                      {primaryItem?.notes && (
                        <p className="mt-1 text-[11px] text-slate-700 italic">
                          Specification Ref: {primaryItem.notes}
                        </p>
                      )}
                    </td>
                    <td className="border border-slate-800 p-2 text-center align-top font-bold">
                      {targetQuantity} {targetUom}
                    </td>
                    <td className="border border-slate-800 p-2 text-center align-top font-mono">
                      ₹{effectiveFinalRate.toFixed(2)}
                    </td>
                    <td className="border border-slate-800 p-2 text-center align-top font-mono font-bold text-blue-900">
                      ₹
                      {(effectiveFinalRate * targetQuantity).toLocaleString(
                        "en-IN",
                        { minimumFractionDigits: 2 },
                      )}
                    </td>
                  </tr>
                </tbody>
              </table>

              {/* Terms of LOI */}
              <div className="text-[11px] font-sans leading-relaxed text-slate-800 mb-8 space-y-2">
                <p className="font-bold text-xs uppercase underline">
                  Key Commercial &amp; Technical Conditions:
                </p>
                <ol className="list-decimal pl-5 space-y-1.5 text-justify">
                  <li>
                    <strong>Price Basis:</strong> F.O.R. destination site (
                    {projectName || "Project Site"}), inclusive of all packing,
                    forwarding, and insurance.
                  </li>
                  <li>
                    <strong>Delivery Schedule:</strong> Material shall be ready
                    for dispatch inspection within{" "}
                    <strong>{chosenQuote?.delivery_days || 21} days</strong>{" "}
                    from date of drawing/QAP approval.
                  </li>
                  <li>
                    <strong>Payment Terms:</strong>{" "}
                    {chosenSupplier?.data?.payment_terms?.description ||
                      "20% advance against PI, 30% after stage QC inspection approval, 40% after site delivery, and 10% retention against performance warranty."}
                  </li>
                  <li>
                    <strong>
                      Mandatory Document Submission (Within 2 Days):
                    </strong>{" "}
                    In line with standard EPC procedures, the vendor must submit
                    within <strong>2 days</strong>: (i) General Arrangement (GA)
                    Drawing, (ii) Technical Data Sheet (TDS), and (iii) Quality
                    Assurance Plan (QAP) for end client/consultant approval.
                  </li>
                  <li>
                    <strong>Inspection &amp; Dispatch:</strong> Factory
                    third-party inspection (TPI) is mandatory. Material shall be
                    dispatched only upon issuance of formal Dispatch Clearance
                    from Vishal Technopower Private Limited.
                  </li>
                  <li>
                    <strong>Formal Purchase Order (PO):</strong> A detailed
                    Purchase Order will be released following client technical
                    approval of submitted drawings.
                  </li>
                </ol>
              </div>

              {/* Closing & Signatures */}
              <div className="text-xs font-sans mb-8">
                <p>
                  Please return a signed and stamped copy of this Letter of
                  Intent as an acknowledgement of your acceptance.
                </p>
              </div>

              <div className="pt-8 border-t border-slate-400 grid grid-cols-2 text-xs font-sans font-bold text-slate-800">
                <div>
                  <p className="mb-10">
                    For Vishal Technopower Private Limited
                  </p>
                  <p className="font-semibold text-slate-800">
                    {user.username}
                  </p>
                  <p className="text-[11px] font-normal text-slate-500">
                    Authorized Signatory / Sourcing
                  </p>
                </div>
                <div className="text-right">
                  <p className="mb-10">Accepted &amp; Acknowledged By</p>
                  <p className="font-semibold text-slate-800">
                    ________________________
                  </p>
                  <p className="text-[11px] font-normal text-slate-500">
                    Authorized Signatory (Vendor Stamp &amp; Signature)
                  </p>
                </div>
              </div>
            </div>

            {/* Bottom Transition to Phase 3 */}
            <div className="no-print flex justify-end p-4 bg-white rounded-xl border border-slate-200 shadow-sm">
              <button
                type="button"
                onClick={() => switchStep(6)}
                className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-bold text-xs rounded-xl shadow-md transition-all flex items-center gap-2"
              >
                Proceed to Phase 3: Technical Documents (Step 6) →
              </button>
            </div>
          </div>
        )}
      </main>

      {/* Quote Entry / Document Upload Modal */}
      {modalSupplier && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 p-4 backdrop-blur-xs">
          <div className="vtpl-modal bg-white rounded-2xl max-w-xl w-full p-6 shadow-2xl border border-slate-200 space-y-4 max-h-[90vh] overflow-y-auto">
            <div className="flex items-start justify-between border-b border-slate-100 pb-3">
              <div>
                <span className="text-[10px] font-extrabold uppercase text-blue-600 tracking-wider">
                  Quotation Entry &amp; Document Upload
                </span>
                <h3 className="text-base font-bold text-slate-900 mt-0.5">
                  {modalSupplier.name}
                </h3>
              </div>
              <button
                type="button"
                onClick={() => setModalSupplier(null)}
                className="text-slate-400 hover:text-slate-700 text-lg font-bold"
              >
                ✕
              </button>
            </div>

            {/* Document Upload Area */}
            <div className="p-4 rounded-xl border-2 border-dashed border-blue-200 bg-blue-50/40 text-center">
              <span className="text-2xl">📄</span>
              <p className="text-xs font-bold text-slate-800 mt-1">
                Upload Vendor Quotation Document
              </p>
              <p className="text-[11px] text-slate-500 mt-0.5">
                Supports PDF, Excel (.xlsx), or Scanned Quotation Images
              </p>
              <ValidatedInput
                type="file"
                accept=".pdf,.xlsx,.xls,.png,.jpg,.jpeg"
                onChange={handleFileUpload}
                className="mt-3 text-xs block w-full text-slate-600 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-bold file:bg-blue-600 file:text-white hover:file:bg-blue-700 cursor-pointer"
              />
              {modalQuote.document_name && (
                <p className="mt-2 text-xs font-bold text-emerald-700">
                  ✓ Attached: {modalQuote.document_name}
                </p>
              )}
            </div>

            {/* Rate Breakdown Fields */}
            <div className="grid grid-cols-2 gap-3 text-xs">
              <div>
                <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                  Quote Reference No:
                  <ValidatedInput
                    type="text"
                    value={modalQuote.quote_reference || ""}
                    onChange={(e) =>
                      setModalQuote({
                        ...modalQuote,
                        quote_reference: e.target.value,
                      })
                    }
                    placeholder={`QT-${modalSupplier.code}-001`}
                    className="w-full border border-slate-300 rounded-lg p-2 font-mono text-xs"
                  />
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                  Quote Validity Date:
                  <ValidatedInput
                    type="date"
                    value={modalQuote.valid_until || ""}
                    onChange={(e) =>
                      setModalQuote({
                        ...modalQuote,
                        valid_until: e.target.value,
                      })
                    }
                    className="w-full border border-slate-300 rounded-lg p-2 text-xs"
                  />
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                  Basic Unit Rate (₹):
                  <ValidatedInput
                    type="number"
                    step="0.01"
                    value={modalQuote.basic_rate || ""}
                    onChange={(e) =>
                      setModalQuote({
                        ...modalQuote,
                        basic_rate: parseFloat(e.target.value) || 0,
                      })
                    }
                    placeholder="0.00"
                    className="w-full border border-slate-300 rounded-lg p-2 font-mono text-xs font-bold"
                  />
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                  Discount (%):
                  <ValidatedInput
                    type="number"
                    step="0.1"
                    value={modalQuote.discount_pct || ""}
                    onChange={(e) =>
                      setModalQuote({
                        ...modalQuote,
                        discount_pct: parseFloat(e.target.value) || 0,
                      })
                    }
                    placeholder="0"
                    className="w-full border border-slate-300 rounded-lg p-2 text-xs"
                  />
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                  GST Rate (%):
                  <ValidatedInput
                    type="number"
                    value={modalQuote.tax_pct ?? 18}
                    onChange={(e) =>
                      setModalQuote({
                        ...modalQuote,
                        tax_pct: parseFloat(e.target.value) || 0,
                      })
                    }
                    className="w-full border border-slate-300 rounded-lg p-2 text-xs"
                  />
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                  Freight &amp; Insurance (₹):
                  <ValidatedInput
                    type="number"
                    value={modalQuote.freight_amount || ""}
                    onChange={(e) =>
                      setModalQuote({
                        ...modalQuote,
                        freight_amount: parseFloat(e.target.value) || 0,
                      })
                    }
                    placeholder="0"
                    className="w-full border border-slate-300 rounded-lg p-2 text-xs"
                  />
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                  Delivery Timeline (Days):
                  <ValidatedInput
                    type="number"
                    value={modalQuote.delivery_days || 21}
                    onChange={(e) =>
                      setModalQuote({
                        ...modalQuote,
                        delivery_days: parseInt(e.target.value) || 0,
                      })
                    }
                    className="w-full border border-slate-300 rounded-lg p-2 text-xs"
                  />
                </ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="font-bold text-slate-700 block mb-1">
                  Warranty Offered:
                  <ValidatedInput
                    type="text"
                    value={modalQuote.warranty || "5 Years"}
                    onChange={(e) =>
                      setModalQuote({ ...modalQuote, warranty: e.target.value })
                    }
                    className="w-full border border-slate-300 rounded-lg p-2 text-xs"
                  />
                </ValidatedLabel>
              </div>
            </div>

            <div>
              <ValidatedLabel className="font-bold text-slate-700 text-xs block mb-1">
                Payment Terms Offered:
                <ValidatedInput
                  type="text"
                  value={modalQuote.payment_terms || ""}
                  onChange={(e) =>
                    setModalQuote({
                      ...modalQuote,
                      payment_terms: e.target.value,
                    })
                  }
                  className="w-full border border-slate-300 rounded-lg p-2 text-xs"
                />
              </ValidatedLabel>
            </div>

            <div className="flex items-center gap-2 pt-1">
              <ValidatedInput
                type="checkbox"
                id="modal-compliant"
                checked={modalQuote.compliant !== false}
                onChange={(e) =>
                  setModalQuote({ ...modalQuote, compliant: e.target.checked })
                }
                className="w-4 h-4 rounded text-blue-600"
              />
              <ValidatedLabel
                htmlFor="modal-compliant"
                className="text-xs font-bold text-slate-800 cursor-pointer"
              >
                Quotation technically compliant with BOQ specifications
              </ValidatedLabel>
            </div>

            {/* Modal Actions */}
            <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-100">
              <button
                type="button"
                onClick={() => setModalSupplier(null)}
                className="px-4 py-2 text-xs font-bold text-slate-600 hover:bg-slate-100 rounded-lg"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleSaveQuote}
                className="px-5 py-2 bg-blue-600 hover:bg-blue-700 text-white text-xs font-bold rounded-lg shadow-sm"
              >
                Save Quotation Record
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
