"use client";
import {
  ValidatedInput,
  ValidatedLabel,
  ValidatedSelect,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";

import React, { useState, useEffect } from "react";

export interface PurchaseInspectionWorkspaceProps {
  project: string;
  projectName?: string;
  lines: Array<{
    id: string;
    item_id: string;
    item_code: string;
    item_name: string;
    quantity: number;
    unit_of_measure: string;
    category?: string;
  }>;
  boId?: string;
  initialStep?: number;
  onNavigateStep: (stepNumber: number) => void;
}

interface WitnessPoint {
  code: string;
  title: string;
  is_witness: boolean;
  is_cleared: boolean;
  remarks?: string | null;
}

interface QualityCertificate {
  id: string;
  cert_type: string;
  title: string;
  cert_number: string;
  issued_by: string;
  valid_until?: string | null;
  verified: boolean;
  verified_at?: string | null;
  file_name?: string | null;
}

interface InspectionCaseData {
  id: string;
  project_id: string;
  bo_id: string;
  supplier_id: string | null;
  po_number: string;
  version: number;
  po_acceptance: {
    status: string;
    accepted_date: string | null;
    signatory_name: string | null;
    file_name: string | null;
    remarks: string | null;
  };
  advance_payment: {
    status: string;
    request_no: string | null;
    request_date: string | null;
    amount: number;
    advance_pct: number;
    proforma_invoice_ref: string | null;
    proforma_invoice_date: string | null;
    bank_account_ref: string | null;
    voucher_ref: string | null;
    disbursed_date: string | null;
    notes: string | null;
  };
  inspection: {
    status: string;
    call_notice_ref: string | null;
    call_date: string | null;
    scheduled_date: string | null;
    actual_date: string | null;
    inspection_agency: string | null;
    inspector_name: string | null;
    factory_location: string | null;
    witness_points: WitnessPoint[];
    findings: string | null;
  };
  inspection_report: {
    status: string;
    ir_number: string | null;
    irn_number: string | null;
    issue_date: string | null;
    issuing_authority: string | null;
    cleared_quantity: number;
    uom: string;
    file_name: string | null;
  };
  certificates: QualityCertificate[];
}

export function PurchaseInspectionWorkspace({
  project,
  projectName,
  lines,
  boId,
  initialStep = 10,
  onNavigateStep,
}: PurchaseInspectionWorkspaceProps) {
  const [currentStep, setCurrentStep] = useState<number>(initialStep);
  const [loading, setLoading] = useState(true);
  const [reloadRevision, setReloadRevision] = useState(0);
  const [isPersisted, setIsPersisted] = useState(false);
  const [poNumber, setPoNumber] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  const [eligibleForDispatch, setEligibleForDispatch] =
    useState<boolean>(false);
  const [dispatchBlockers, setDispatchBlockers] = useState<string[]>([]);

  // Step 10 Local edit state
  const [acceptanceStatus, setAcceptanceStatus] = useState("pending");
  const [acceptedDate, setAcceptedDate] = useState("");
  const [signatoryName, setSignatoryName] = useState("");
  const [acceptanceFileName, setAcceptanceFileName] = useState("");
  const [acceptanceRemarks, setAcceptanceRemarks] = useState("");

  // Step 11 Local edit state
  const [advanceStatus, setAdvanceStatus] = useState("draft");
  const [advanceReqNo, setAdvanceReqNo] = useState("");
  const [advanceReqDate, setAdvanceReqDate] = useState("");
  const [advanceAmount, setAdvanceAmount] = useState(0);
  const [advancePct, setAdvancePct] = useState(0);
  const [piRef, setPiRef] = useState("");
  const [piDate, setPiDate] = useState("");
  const [bankRef, setBankRef] = useState("");
  const [voucherRef, setVoucherRef] = useState("");
  const [disbursedDate, setDisbursedDate] = useState("");
  const [advanceNotes, setAdvanceNotes] = useState("");

  // Step 12 Local edit state
  const [inspectionStatus, setInspectionStatus] = useState("not_scheduled");
  const [callNoticeRef, setCallNoticeRef] = useState("");
  const [callDate, setCallDate] = useState("");
  const [scheduledDate, setScheduledDate] = useState("");
  const [actualDate, setActualDate] = useState("");
  const [inspectionAgency, setInspectionAgency] = useState("");
  const [inspectorName, setInspectorName] = useState("");
  const [factoryLocation, setFactoryLocation] = useState("");
  const [witnessPoints, setWitnessPoints] = useState<WitnessPoint[]>([]);
  const [findings, setFindings] = useState("");

  // Step 13 Local edit state
  const [irnStatus, setIrnStatus] = useState("pending");
  const [irNumber, setIrNumber] = useState("");
  const [irnNumber, setIrnNumber] = useState("");
  const [irnIssueDate, setIrnIssueDate] = useState("");
  const [irnAuthority, setIrnAuthority] = useState("");
  const [clearedQty, setClearedQty] = useState(0);
  const [uom, setUom] = useState("Unit");
  const [irnFileName, setIrnFileName] = useState("");

  // Step 14 Local edit state
  const [certificates, setCertificates] = useState<QualityCertificate[]>([]);

  // View IRN Letterhead preview modal/tab
  const [showIRNPreview, setShowIRNPreview] = useState(false);

  // Sync prop changes during render
  const [prevInitialStep, setPrevInitialStep] = useState(initialStep);
  if (initialStep !== prevInitialStep) {
    setPrevInitialStep(initialStep);
    if (initialStep >= 10 && initialStep <= 14) {
      setCurrentStep(initialStep);
    }
  }

  // Load Inspection Case Data from backend
  useEffect(() => {
    let live = true;
    async function loadCase() {
      if (!boId) {
        setLoading(false);
        setError("Save a BO list before entering purchase execution details.");
        return;
      }
      setLoading(true);
      setError(null);
      try {
        const res = await fetch(
          `/api/projects/${project}/purchase-inspection?bo_id=${boId}`,
        );
        if (!res.ok) {
          throw new Error(
            `Failed to load purchase inspection data (${res.status})`,
          );
        }
        const data = await res.json();
        if (live && data.case) {
          setIsPersisted(Boolean(data.is_persisted));
          setPoNumber(data.case.po_number ?? "");
          const c: InspectionCaseData = data.case;
          setEligibleForDispatch(Boolean(data.eligible_for_dispatch));
          setDispatchBlockers(data.dispatch_blockers || []);

          // Sync step 10
          if (c.po_acceptance) {
            setAcceptanceStatus(c.po_acceptance.status ?? "pending");
            setAcceptedDate(c.po_acceptance.accepted_date ?? "");
            setSignatoryName(c.po_acceptance.signatory_name ?? "");
            setAcceptanceFileName(c.po_acceptance.file_name ?? "");
            setAcceptanceRemarks(c.po_acceptance.remarks ?? "");
          }

          // Sync step 11
          if (c.advance_payment) {
            setAdvanceStatus(c.advance_payment.status ?? "draft");
            setAdvanceReqNo(c.advance_payment.request_no ?? "");
            setAdvanceReqDate(c.advance_payment.request_date ?? "");
            setAdvanceAmount(c.advance_payment.amount ?? 0);
            setAdvancePct(c.advance_payment.advance_pct ?? 0);
            setPiRef(c.advance_payment.proforma_invoice_ref ?? "");
            setPiDate(c.advance_payment.proforma_invoice_date ?? "");
            setBankRef(c.advance_payment.bank_account_ref ?? "");
            setVoucherRef(c.advance_payment.voucher_ref ?? "");
            setDisbursedDate(c.advance_payment.disbursed_date ?? "");
            setAdvanceNotes(c.advance_payment.notes ?? "");
          }

          // Sync step 12
          if (c.inspection) {
            setInspectionStatus(c.inspection.status ?? "not_scheduled");
            setCallNoticeRef(c.inspection.call_notice_ref ?? "");
            setCallDate(c.inspection.call_date ?? "");
            setScheduledDate(c.inspection.scheduled_date ?? "");
            setActualDate(c.inspection.actual_date ?? "");
            setInspectionAgency(c.inspection.inspection_agency ?? "");
            setInspectorName(c.inspection.inspector_name ?? "");
            setFactoryLocation(c.inspection.factory_location ?? "");
            setWitnessPoints(c.inspection.witness_points ?? []);
            setFindings(c.inspection.findings ?? "");
          }

          // Sync step 13
          if (c.inspection_report) {
            setIrnStatus(c.inspection_report.status ?? "pending");
            setIrNumber(c.inspection_report.ir_number ?? "");
            setIrnNumber(c.inspection_report.irn_number ?? "");
            setIrnIssueDate(c.inspection_report.issue_date ?? "");
            setIrnAuthority(c.inspection_report.issuing_authority ?? "");
            setClearedQty(c.inspection_report.cleared_quantity ?? 0);
            setUom(c.inspection_report.uom ?? "Unit");
            setIrnFileName(c.inspection_report.file_name ?? "");
          }

          // Sync step 14
          setCertificates(c.certificates ?? []);
        }
      } catch (err: unknown) {
        if (live) {
          setError(
            err instanceof Error
              ? err.message
              : "Failed to load inspection case",
          );
        }
      } finally {
        if (live) setLoading(false);
      }
    }
    loadCase();
    return () => {
      live = false;
    };
  }, [project, boId, reloadRevision]);

  const showNotification = (msg: string) => {
    setSuccessMsg(msg);
    setTimeout(() => setSuccessMsg(null), 3500);
  };

  // Step 10: Save PO Acceptance
  const handleSavePOAcceptance = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-inspection/po-acceptance?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: acceptanceStatus,
            accepted_date: acceptedDate,
            signatory_name: signatoryName,
            file_name: acceptanceFileName,
            remarks: acceptanceRemarks,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to save PO acceptance");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification("Step 10: Vendor PO acceptance successfully recorded.");
      onNavigateStep(11);
      setCurrentStep(11);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to save PO acceptance",
      );
    } finally {
      setSaving(false);
    }
  };

  // Step 11: Save Advance Payment Request
  const handleSaveAdvancePayment = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-inspection/advance-payment?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: advanceStatus,
            request_no: advanceReqNo,
            request_date: advanceReqDate,
            amount: advanceAmount,
            advance_pct: advancePct,
            proforma_invoice_ref: piRef,
            proforma_invoice_date: piDate,
            bank_account_ref: bankRef,
            voucher_ref: voucherRef,
            disbursed_date: disbursedDate,
            notes: advanceNotes,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to save advance payment");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification("Step 11: Advance payment request record updated.");
      onNavigateStep(12);
      setCurrentStep(12);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to save advance payment",
      );
    } finally {
      setSaving(false);
    }
  };

  // Step 12: Save Inspection & Witness Points
  const handleSaveInspection = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-inspection/inspection?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: inspectionStatus,
            call_notice_ref: callNoticeRef,
            call_date: callDate,
            scheduled_date: scheduledDate,
            actual_date: actualDate,
            inspection_agency: inspectionAgency,
            inspector_name: inspectorName,
            factory_location: factoryLocation,
            witness_points: witnessPoints,
            findings: findings,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to save stage inspection");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification("Step 12: Inspection details saved.");
      onNavigateStep(13);
      setCurrentStep(13);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to save stage inspection",
      );
    } finally {
      setSaving(false);
    }
  };

  // Step 13: Save Inspection Report & IRN
  const handleSaveInspectionReport = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-inspection/inspection-report?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: irnStatus,
            ir_number: irNumber,
            irn_number: irnNumber,
            issue_date: irnIssueDate,
            issuing_authority: irnAuthority,
            cleared_quantity: clearedQty,
            uom: uom,
            file_name: irnFileName,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to issue IRN");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification("Step 13: Inspection report details saved.");
      onNavigateStep(14);
      setCurrentStep(14);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to issue IRN");
    } finally {
      setSaving(false);
    }
  };

  // Step 14: Verify Quality Certificate
  const handleToggleCertificateVerification = async (
    certId: string,
    currentVerified: boolean,
  ) => {
    if (!boId) return;
    const nextVerified = !currentVerified;
    const certificate = certificates.find((item) => item.id === certId);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-inspection/certificates?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            cert_id: certId,
            cert_number: certificate?.cert_number,
            issued_by: certificate?.issued_by,
            valid_until: certificate?.valid_until ?? "",
            verified: nextVerified,
            file_name: null,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to verify certificate");
      }
      setCertificates((prev) =>
        prev.map((c) =>
          c.id === certId
            ? {
                ...c,
                verified: nextVerified,
                verified_at: nextVerified
                  ? new Date().toISOString().split("T")[0]
                  : null,
              }
            : c,
        ),
      );
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification(
        `Certificate ${certId} status updated to ${nextVerified ? "Verified" : "Pending"}.`,
      );
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to verify certificate",
      );
    }
  };

  const primaryItem = lines[0] || {
    item_name: "No BO item selected",
    item_code: "—",
    unit_of_measure: "Unit",
  };

  return (
    <div className="bo-stage flex flex-col h-full bg-slate-50 text-slate-800 text-sm overflow-hidden">
      <div className="bo-stage-toolbar">
        <span
          className={
            eligibleForDispatch
              ? "text-emerald-800 font-semibold"
              : "text-amber-900 font-semibold"
          }
        >
          {eligibleForDispatch
            ? "Ready for dispatch clearance"
            : "Inspection / verification pending"}
        </span>
        <button
          type="button"
          onClick={() => setShowIRNPreview(!showIRNPreview)}
        >
          {showIRNPreview ? "Close document preview" : "IRN document preview"}
        </button>
      </div>
      <details className="bo-stage-reference">
        <summary>
          {isPersisted ? "Saved record" : "New draft"} · Document references
        </summary>
        <p>
          Save each step to retain edits. Document references are text only;
          entering a filename does not upload evidence.
        </p>
      </details>
      {/* Notification and Error Banners */}
      {error && (
        <div className="mx-6 mt-3 p-3 rounded bg-rose-50 border border-rose-200 text-rose-800 text-xs flex justify-between items-center shrink-0">
          <span>
            <strong>Error:</strong> {error}
          </span>
          <button
            onClick={() => setError(null)}
            className="text-rose-500 hover:text-rose-700 font-bold ml-2"
          >
            ×
          </button>
        </div>
      )}
      {successMsg && (
        <div className="mx-6 mt-3 p-3 rounded bg-emerald-50 border border-emerald-200 text-emerald-800 text-xs flex justify-between items-center shrink-0">
          <span>✓ {successMsg}</span>
          <button
            onClick={() => setSuccessMsg(null)}
            className="text-emerald-500 hover:text-emerald-700 font-bold ml-2"
          >
            ×
          </button>
        </div>
      )}

      {/* Main Content Workspace */}
      <div className="flex-1 overflow-y-auto p-6">
        {loading ? (
          <div className="flex items-center justify-center h-48 text-slate-500">
            <span className="inline-block animate-spin mr-2">⚙</span> Loading
            Phase 4 Inspection &amp; Execution records...
          </div>
        ) : showIRNPreview ? (
          /* Live High-Fidelity Letterhead Inspection Release Note Preview */
          <div className="max-w-4xl mx-auto bg-white border border-slate-300 rounded-lg shadow-md p-8 font-serif print:p-0">
            <div className="flex justify-between items-start pb-4 border-b-2 border-slate-900">
              <div>
                <h2 className="text-xl font-black tracking-wide text-slate-900 font-sans">
                  VISHAL TECHNOPOWER PRIVATE LIMITED
                </h2>
                <p className="text-xs text-slate-600 font-sans">
                  AN ISO 9001:2015 CERTIFIED EPC POWER &amp; INFRASTRUCTURE
                  COMPANY
                </p>
                <p className="text-[11px] text-slate-500 font-sans mt-0.5">
                  Regd. Office: 204, City Centre, Near Central Bus Station,
                  Vadodara, Gujarat - 390001
                </p>
              </div>
              <div className="text-right font-sans">
                <span className="inline-block bg-blue-900 text-white font-bold text-xs px-2.5 py-1 rounded">
                  INSPECTION RELEASE NOTE
                </span>
                <p className="text-xs font-mono font-bold text-slate-800 mt-1">
                  IRN No: {irnNumber}
                </p>
                <p className="text-xs text-slate-600">Date: {irnIssueDate}</p>
              </div>
            </div>

            <div className="my-6 grid grid-cols-2 gap-4 text-xs font-sans">
              <div className="p-3 bg-slate-50 border border-slate-200 rounded">
                <p className="font-bold text-slate-900 mb-1">
                  PROJECT DETAILS:
                </p>
                <p>
                  <span className="text-slate-500">Project:</span> {projectName}
                </p>
                <p>
                  <span className="text-slate-500">Client / Authority:</span>{" "}
                  {irnAuthority}
                </p>
                <p>
                  <span className="text-slate-500">Inspection Agency:</span>{" "}
                  {inspectionAgency}
                </p>
                <p>
                  <span className="text-slate-500">Lead Inspector:</span>{" "}
                  {inspectorName}
                </p>
              </div>
              <div className="p-3 bg-slate-50 border border-slate-200 rounded">
                <p className="font-bold text-slate-900 mb-1">
                  PURCHASE &amp; FACTORY DETAILS:
                </p>
                <p>
                  <span className="text-slate-500">PO Number:</span>{" "}
                  {poNumber || "Not linked"}
                </p>
                <p>
                  <span className="text-slate-500">Inspection Report No:</span>{" "}
                  {irNumber}
                </p>
                <p>
                  <span className="text-slate-500">Inspection Date:</span>{" "}
                  {actualDate}
                </p>
                <p>
                  <span className="text-slate-500">Factory Works:</span>{" "}
                  {factoryLocation}
                </p>
              </div>
            </div>

            {/* Cleared Material Summary Table */}
            <div className="my-6 font-sans">
              <table className="vtpl-data-table w-full text-xs border-collapse border border-slate-300">
                <thead>
                  <tr className="bg-slate-100 text-slate-900 font-bold">
                    <th className="border border-slate-300 p-2 text-center w-12">
                      Sr.
                    </th>
                    <th className="border border-slate-300 p-2 text-left">
                      Equipment / Material Description
                    </th>
                    <th className="border border-slate-300 p-2 text-center w-24">
                      Item Code
                    </th>
                    <th className="border border-slate-300 p-2 text-center w-24">
                      Offered Qty
                    </th>
                    <th className="border border-slate-300 p-2 text-center w-24">
                      Inspected Qty
                    </th>
                    <th className="border border-slate-300 p-2 text-center w-24 text-emerald-800">
                      Cleared Qty
                    </th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td className="border border-slate-300 p-2 text-center">
                      1
                    </td>
                    <td className="border border-slate-300 p-2 font-medium">
                      {primaryItem.item_name}
                    </td>
                    <td className="border border-slate-300 p-2 text-center font-mono">
                      {primaryItem.item_code}
                    </td>
                    <td className="border border-slate-300 p-2 text-center">
                      {clearedQty} {uom}
                    </td>
                    <td className="border border-slate-300 p-2 text-center">
                      {clearedQty} {uom}
                    </td>
                    <td className="border border-slate-300 p-2 text-center font-bold text-emerald-700">
                      {clearedQty} {uom}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            {/* Witness Tests Cleared Summary */}
            <div className="my-4 font-sans text-xs">
              <p className="font-bold text-slate-900 mb-2">
                SUMMARY OF WITNESS &amp; ROUTINE TESTS WITNESSED:
              </p>
              <div className="space-y-1.5">
                {witnessPoints.map((wp) => (
                  <div
                    key={wp.code}
                    className="flex items-start gap-2 bg-slate-50 p-2 border border-slate-200 rounded"
                  >
                    <span className="font-mono font-bold text-emerald-700">
                      ✓ {wp.code}:
                    </span>
                    <span className="font-semibold text-slate-800">
                      {wp.title}
                    </span>
                    <span className="text-slate-500 italic ml-auto">
                      {wp.remarks}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            <div className="my-4 p-3 bg-emerald-50 border border-emerald-200 rounded font-sans text-xs text-emerald-900">
              <p className="font-bold">
                INSPECTION RELEASE CLEARANCE STATEMENT:
              </p>
              <p className="mt-1">
                This is to certify that the materials detailed above have been
                jointly inspected at manufacturer works and tested as per
                approved QAP, tender technical specifications, and relevant
                Indian Standards (IS). Stage inspection clearance is hereby
                granted for dispatch.
              </p>
            </div>

            {/* Signatures */}
            <div className="mt-12 pt-6 border-t border-slate-400 grid grid-cols-3 gap-6 text-center font-sans text-xs">
              <div>
                <p className="font-bold text-slate-800">{inspectorName}</p>
                <p className="text-slate-500 text-[10px]">
                  Client / Consultant Inspector
                </p>
                <p className="text-slate-400 text-[9px] mt-8">
                  Official Stamp &amp; Sign
                </p>
              </div>
              <div>
                <p className="font-bold text-slate-800">
                  Er. Rajesh Nair (QA/QC Head)
                </p>
                <p className="text-slate-500 text-[10px]">
                  VTPL Quality Assurance
                </p>
                <p className="text-slate-400 text-[9px] mt-8">
                  Authorized Signatory
                </p>
              </div>
              <div>
                <p className="font-bold text-slate-800">{signatoryName}</p>
                <p className="text-slate-500 text-[10px]">
                  Manufacturer / Vendor QA
                </p>
                <p className="text-slate-400 text-[9px] mt-8">
                  Works Inspector
                </p>
              </div>
            </div>

            <div className="mt-8 flex justify-end gap-3 print:hidden font-sans">
              <button
                onClick={() => window.print()}
                className="px-4 py-2 bg-slate-800 text-white rounded font-semibold text-xs hover:bg-slate-900"
              >
                🖨 Print Official IRN
              </button>
              <button
                onClick={() => setShowIRNPreview(false)}
                className="px-4 py-2 bg-slate-200 text-slate-700 rounded font-semibold text-xs hover:bg-slate-300"
              >
                Back to Workspace
              </button>
            </div>
          </div>
        ) : (
          <div className="max-w-5xl mx-auto space-y-6">
            {/* STEP 10: VENDOR PO ACCEPTANCE */}
            {currentStep === 10 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4 mb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Vendor Purchase Order Acceptance &amp; Acknowledgment
                    </h2>
                  </div>
                  <span
                    className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                      acceptanceStatus === "accepted"
                        ? "bg-emerald-100 text-emerald-800"
                        : "bg-amber-100 text-amber-800"
                    }`}
                  >
                    Status: {acceptanceStatus}
                  </span>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Acceptance Status <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="acceptance_status"
                        value={acceptanceStatus}
                        onChange={(e) => setAcceptanceStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded focus:ring-1 focus:ring-blue-500 bg-white"
                      >
                        <option value="pending">
                          Pending Vendor Confirmation
                        </option>
                        <option value="accepted">
                          Accepted without Exceptions (Standard)
                        </option>
                        <option value="accepted_with_exceptions">
                          Accepted with Exceptions
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Date of Vendor Acceptance{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="accepted_date"
                        type="date"
                        value={acceptedDate}
                        onChange={(e) => setAcceptedDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded focus:ring-1 focus:ring-blue-500"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Vendor Authorized Signatory Name &amp; Title
                      <ValidatedInput
                        name="signatory_name"
                        type="text"
                        value={signatoryName}
                        onChange={(e) => setSignatoryName(e.target.value)}
                        placeholder="e.g. Ashok Singhania (Head of Industrial Sales)"
                        className="w-full text-xs p-2 border border-slate-300 rounded focus:ring-1 focus:ring-blue-500"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Signed PO document reference (file not uploaded)
                    </ValidatedLabel>
                    <div className="flex gap-2">
                      <ValidatedInput
                        name="acceptance_file_name"
                        type="text"
                        value={acceptanceFileName}
                        onChange={(e) => setAcceptanceFileName(e.target.value)}
                        className="flex-1 text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </div>
                  </div>

                  <div className="col-span-2">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Acceptance Remarks / Exceptions Note
                      <ValidatedTextarea
                        name="acceptance_remarks"
                        rows={3}
                        value={acceptanceRemarks}
                        onChange={(e) => setAcceptanceRemarks(e.target.value)}
                        placeholder="Enter acceptance details, payment terms confirmation, delivery schedule alignment..."
                        className="w-full text-xs p-2 border border-slate-300 rounded focus:ring-1 focus:ring-blue-500"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                <div className="mt-6 flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSavePOAcceptance}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Save PO Acceptance & Proceed to Step 11 →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 11: ADVANCE PAYMENT REQUEST */}
            {currentStep === 11 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4 mb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Advance Payment Request to Accounts Department
                    </h2>
                  </div>
                  <span
                    className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                      advanceStatus === "disbursed"
                        ? "bg-emerald-100 text-emerald-800"
                        : advanceStatus === "requested"
                          ? "bg-blue-100 text-blue-800"
                          : "bg-slate-100 text-slate-800"
                    }`}
                  >
                    Status: {advanceStatus}
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Payment Status
                      <ValidatedSelect
                        name="advance_status"
                        value={advanceStatus}
                        onChange={(e) => setAdvanceStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded focus:ring-1 focus:ring-blue-500 bg-white"
                      >
                        <option value="draft">Draft Request</option>
                        <option value="requested">Requested to Accounts</option>
                        <option value="approved">Approved by Accounts</option>
                        <option value="disbursed">
                          Disbursed (UTR Issued)
                        </option>
                        <option value="not_applicable">
                          Not Applicable (0% Advance)
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Advance Request Ref No.
                      <ValidatedInput
                        name="advance_req_no"
                        type="text"
                        value={advanceReqNo}
                        onChange={(e) => setAdvanceReqNo(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Request Date
                      <ValidatedInput
                        name="advance_req_date"
                        type="date"
                        value={advanceReqDate}
                        onChange={(e) => setAdvanceReqDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Agreed Advance Percentage (%)
                      <ValidatedInput
                        name="advance_pct"
                        type="number"
                        value={advancePct}
                        onChange={(e) => setAdvancePct(Number(e.target.value))}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Advance Payable Amount (₹)
                      <ValidatedInput
                        name="advance_amount"
                        type="number"
                        value={advanceAmount}
                        onChange={(e) =>
                          setAdvanceAmount(Number(e.target.value))
                        }
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono font-bold text-slate-900"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Proforma Invoice Ref
                      <ValidatedInput
                        name="pi_ref"
                        type="text"
                        value={piRef}
                        onChange={(e) => setPiRef(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Proforma Invoice Date
                      <ValidatedInput
                        name="pi_date"
                        type="date"
                        value={piDate}
                        onChange={(e) => setPiDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Accounts Payment Voucher / UTR Ref
                      <ValidatedInput
                        name="voucher_ref"
                        type="text"
                        value={voucherRef}
                        onChange={(e) => setVoucherRef(e.target.value)}
                        placeholder="e.g. PV-VTP-2026-09-441 / RTGS UTR"
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Disbursal Date
                      <ValidatedInput
                        name="disbursed_date"
                        type="date"
                        value={disbursedDate}
                        onChange={(e) => setDisbursedDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Vendor Bank Account Details for RTGS/NEFT
                      <ValidatedInput
                        name="bank_ref"
                        type="text"
                        value={bankRef}
                        onChange={(e) => setBankRef(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Accounts Processing Notes
                      <ValidatedTextarea
                        name="advance_notes"
                        rows={2}
                        value={advanceNotes}
                        onChange={(e) => setAdvanceNotes(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                <div className="mt-6 flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSaveAdvancePayment}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Save Advance Payment & Proceed to Step 12 →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 12: MATERIAL FACTORY STAGE INSPECTION (24%) */}
            {currentStep === 12 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6 space-y-6">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Material Factory Stage Inspection (as per approved QAP)
                    </h2>
                  </div>
                  <span
                    className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                      inspectionStatus === "cleared"
                        ? "bg-emerald-100 text-emerald-800"
                        : inspectionStatus === "scheduled"
                          ? "bg-blue-100 text-blue-800"
                          : "bg-amber-100 text-amber-800"
                    }`}
                  >
                    Status: {inspectionStatus}
                  </span>
                </div>

                {/* Inspection Logistics */}
                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Inspection Clearance Status{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="inspection_status"
                        value={inspectionStatus}
                        onChange={(e) => setInspectionStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded focus:ring-1 focus:ring-blue-500 bg-white"
                      >
                        <option value="not_scheduled">Not Scheduled</option>
                        <option value="call_received">
                          Inspection Call Received
                        </option>
                        <option value="scheduled">Inspection Scheduled</option>
                        <option value="cleared">
                          Stage Inspection Cleared (Pass)
                        </option>
                        <option value="rejected">
                          Inspection Rejected / Rectification Needed
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Vendor Inspection Call Ref
                      <ValidatedInput
                        name="call_notice_ref"
                        type="text"
                        value={callNoticeRef}
                        onChange={(e) => setCallNoticeRef(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Call Received Date
                      <ValidatedInput
                        name="call_date"
                        type="date"
                        value={callDate}
                        onChange={(e) => setCallDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Actual Inspection Date{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="actual_date"
                        type="date"
                        value={actualDate}
                        onChange={(e) => setActualDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Inspection Agency
                      <ValidatedSelect
                        name="inspection_agency"
                        value={inspectionAgency}
                        onChange={(e) => setInspectionAgency(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded bg-white"
                      >
                        <option value="Client (AAI) & Consultant QC Joint Inspection">
                          Client (AAI) &amp; Consultant QC Joint Inspection
                        </option>
                        <option value="Third Party Inspection (TPI - SGS/BV/TUV)">
                          Third Party Inspection (TPI - SGS/BV/TUV)
                        </option>
                        <option value="Internal VTPL Quality Assurance Team">
                          Internal VTPL Quality Assurance Team
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Lead Inspector Name &amp; Designation
                      <ValidatedInput
                        name="inspector_name"
                        type="text"
                        value={inspectorName}
                        onChange={(e) => setInspectorName(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Factory / Works Location{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="factory_location"
                        type="text"
                        value={factoryLocation}
                        onChange={(e) => setFactoryLocation(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                {/* Witness Points Checklist as per approved QAP */}
                <div className="border border-slate-200 rounded-lg overflow-hidden">
                  <div className="bg-slate-100 px-4 py-2.5 border-b border-slate-200 flex justify-between items-center">
                    <h3 className="text-xs font-bold text-slate-900 uppercase tracking-wide">
                      Approved QAP Stage Witness Points Verification
                    </h3>
                    <span className="text-[11px] text-slate-500">
                      Total Points: {witnessPoints.length} | Cleared:{" "}
                      {witnessPoints.filter((w) => w.is_cleared).length}
                    </span>
                  </div>
                  <table className="vtpl-data-table w-full text-xs border-collapse">
                    <thead>
                      <tr className="bg-slate-50 text-slate-700 text-left border-b border-slate-200">
                        <th className="p-2.5 w-16 text-center">Code</th>
                        <th className="p-2.5">
                          Witness Checkpoint &amp; Acceptance Criteria
                        </th>
                        <th className="p-2.5 w-24 text-center">Mandatory</th>
                        <th className="p-2.5 w-24 text-center">Status</th>
                        <th className="p-2.5 w-72">
                          Recorded Observations / Measured Values
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200">
                      {witnessPoints.map((wp, idx) => (
                        <tr key={wp.code} className="hover:bg-slate-50/80">
                          <td className="p-2.5 text-center font-mono font-bold text-slate-700">
                            {wp.code}
                          </td>
                          <td className="p-2.5 font-medium text-slate-800">
                            {wp.title}
                          </td>
                          <td className="p-2.5 text-center">
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-blue-100 text-blue-800 font-semibold">
                              Yes
                            </span>
                          </td>
                          <td className="p-2.5 text-center">
                            <button
                              type="button"
                              onClick={() => {
                                const next = [...witnessPoints];
                                next[idx].is_cleared = !next[idx].is_cleared;
                                setWitnessPoints(next);
                              }}
                              className={`px-2.5 py-1 rounded text-[11px] font-bold transition ${
                                wp.is_cleared
                                  ? "bg-emerald-100 text-emerald-800 hover:bg-emerald-200"
                                  : "bg-rose-100 text-rose-800 hover:bg-rose-200"
                              }`}
                            >
                              {wp.is_cleared ? "✓ PASS" : "✕ FAIL"}
                            </button>
                          </td>
                          <td className="p-2.5">
                            <ValidatedInput
                              type="text"
                              value={wp.remarks || ""}
                              onChange={(e) => {
                                const next = [...witnessPoints];
                                next[idx].remarks = e.target.value;
                                setWitnessPoints(next);
                              }}
                              className="w-full text-xs p-1.5 border border-slate-300 rounded"
                            />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                <div>
                  <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                    Overall Inspection Findings &amp; Stage Clearance
                    Certificate Notes
                    <ValidatedTextarea
                      name="findings"
                      rows={2}
                      value={findings}
                      onChange={(e) => setFindings(e.target.value)}
                      className="w-full text-xs p-2 border border-slate-300 rounded"
                    />
                  </ValidatedLabel>
                </div>

                <div className="flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSaveInspection}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Grant Inspection Clearance & Proceed to Step 13 →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 13: INSPECTION REPORT (IR) & INSPECTION RELEASE NOTE (IRN) */}
            {currentStep === 13 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6 space-y-6">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Inspection Report (IR) &amp; Inspection Release Note (IRN)
                    </h2>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => setShowIRNPreview(true)}
                      className="px-3 py-1.5 bg-blue-50 text-blue-700 border border-blue-200 rounded text-xs font-semibold hover:bg-blue-100"
                    >
                      📄 Preview Letterhead Document
                    </button>
                    <span
                      className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                        irnStatus === "issued"
                          ? "bg-emerald-100 text-emerald-800"
                          : "bg-amber-100 text-amber-800"
                      }`}
                    >
                      Status: {irnStatus}
                    </span>
                  </div>
                </div>

                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      IRN Issue Status <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="irn_status"
                        value={irnStatus}
                        onChange={(e) => setIrnStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded bg-white"
                      >
                        <option value="pending">
                          Pending Document Issuance
                        </option>
                        <option value="issued">
                          Officially Issued &amp; Signed
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Inspection Report (IR) Number{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="ir_number"
                        type="text"
                        value={irNumber}
                        onChange={(e) => setIrNumber(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Inspection Release Note (IRN) Number{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="irn_number"
                        type="text"
                        value={irnNumber}
                        onChange={(e) => setIrnNumber(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono font-bold text-slate-900"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      IRN Issue Date <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="irn_issue_date"
                        type="date"
                        value={irnIssueDate}
                        onChange={(e) => setIrnIssueDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Issuing Authority
                      <ValidatedInput
                        name="irn_authority"
                        type="text"
                        value={irnAuthority}
                        onChange={(e) => setIrnAuthority(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Cleared Quantity &amp; UOM
                    </ValidatedLabel>
                    <div className="flex gap-2">
                      <ValidatedInput
                        name="cleared_qty"
                        type="number"
                        value={clearedQty}
                        onChange={(e) => setClearedQty(Number(e.target.value))}
                        className="w-2/3 text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                      <ValidatedInput
                        name="uom"
                        type="text"
                        value={uom}
                        onChange={(e) => setUom(e.target.value)}
                        className="w-1/3 text-xs p-2 border border-slate-300 rounded text-center"
                      />
                    </div>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Signed IRN document reference (file not uploaded)
                    </ValidatedLabel>
                    <div className="flex gap-2">
                      <ValidatedInput
                        name="irn_file_name"
                        type="text"
                        value={irnFileName}
                        onChange={(e) => setIrnFileName(e.target.value)}
                        className="flex-1 text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </div>
                  </div>
                </div>

                <div className="flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSaveInspectionReport}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Save IRN & Proceed to Step 14 (Certificates) →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 14: QUALITY CERTIFICATES TRACKING */}
            {currentStep === 14 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6 space-y-6">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Manufacturer Quality Certificates Verification
                      (TC/TR/MTC/RTC/Warranty)
                    </h2>
                  </div>
                  <div className="text-xs font-semibold text-slate-600">
                    Verified:{" "}
                    <span className="text-emerald-700 font-bold">
                      {certificates.filter((c) => c.verified).length} /{" "}
                      {certificates.length}
                    </span>
                  </div>
                </div>

                <div className="border border-slate-200 rounded-lg overflow-hidden">
                  <table className="vtpl-data-table w-full text-xs border-collapse">
                    <thead>
                      <tr className="bg-slate-50 text-slate-700 text-left border-b border-slate-200">
                        <th className="p-2.5 w-16 text-center">Type</th>
                        <th className="p-2.5">
                          Certificate Title &amp; Description
                        </th>
                        <th className="p-2.5 w-48">
                          Cert No. &amp; Issuing Lab/Agency
                        </th>
                        <th className="p-2.5 w-24 text-center">Valid Until</th>
                        <th className="p-2.5 w-32 text-center">
                          Verification Status
                        </th>
                        <th className="p-2.5 w-44">Document reference</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-200">
                      {certificates.map((cert) => (
                        <tr key={cert.id} className="hover:bg-slate-50/80">
                          <td className="p-2.5 text-center font-mono font-bold text-blue-800">
                            <span className="px-1.5 py-0.5 rounded bg-blue-100 text-blue-800">
                              {cert.cert_type}
                            </span>
                          </td>
                          <td className="p-2.5 font-medium text-slate-800">
                            {cert.title}
                          </td>
                          <td className="p-2.5 text-slate-600">
                            <ValidatedInput
                              aria-label={`${cert.cert_type} certificate number`}
                              value={cert.cert_number}
                              onChange={(e) =>
                                setCertificates((items) =>
                                  items.map((item) =>
                                    item.id === cert.id
                                      ? { ...item, cert_number: e.target.value }
                                      : item,
                                  ),
                                )
                              }
                              className="w-full border rounded p-1"
                              placeholder="Certificate number"
                            />
                            <ValidatedInput
                              aria-label={`${cert.cert_type} issuer`}
                              value={cert.issued_by}
                              onChange={(e) =>
                                setCertificates((items) =>
                                  items.map((item) =>
                                    item.id === cert.id
                                      ? { ...item, issued_by: e.target.value }
                                      : item,
                                  ),
                                )
                              }
                              className="w-full border rounded p-1 mt-1"
                              placeholder="Issuing organisation"
                            />
                          </td>
                          <td className="p-2.5 text-center font-mono text-slate-600">
                            <ValidatedInput
                              type="date"
                              aria-label={`${cert.cert_type} valid until`}
                              value={cert.valid_until ?? ""}
                              onChange={(e) =>
                                setCertificates((items) =>
                                  items.map((item) =>
                                    item.id === cert.id
                                      ? { ...item, valid_until: e.target.value }
                                      : item,
                                  ),
                                )
                              }
                              className="w-full border rounded p-1"
                            />
                          </td>
                          <td className="p-2.5 text-center">
                            <button
                              type="button"
                              disabled={loading || !boId}
                              onClick={() =>
                                handleToggleCertificateVerification(
                                  cert.id,
                                  cert.verified,
                                )
                              }
                              className={`px-3 py-1.5 rounded text-xs font-bold transition ${
                                cert.verified
                                  ? "bg-emerald-100 text-emerald-800 hover:bg-emerald-200"
                                  : "bg-slate-100 text-slate-700 hover:bg-slate-200"
                              }`}
                            >
                              {cert.verified ? "✓ Verified" : "○ Pending"}
                            </button>
                          </td>
                          <td className="p-2.5 font-mono text-[11px] text-slate-500 truncate max-w-xs">
                            {cert.file_name ? (
                              <span className="text-blue-700 hover:underline cursor-pointer">
                                📎 {cert.file_name}
                              </span>
                            ) : (
                              <span className="text-slate-400 italic">
                                No document reference
                              </span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Final Gating & Step 15 Handover Readiness */}
                <div
                  className={`p-4 rounded-lg border ${
                    eligibleForDispatch
                      ? "bg-emerald-50 border-emerald-300 text-emerald-900"
                      : "bg-amber-50 border-amber-300 text-amber-900"
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <h4 className="font-bold text-sm">
                        {eligibleForDispatch
                          ? "✓ Phase 4 Inspection & Quality Clearance Complete!"
                          : "Phase 4 Execution in Progress"}
                      </h4>
                      <p className="text-xs mt-1">
                        {eligibleForDispatch
                          ? "Saved inspection details meet the current readiness checks."
                          : "Complete and save the outstanding inspection and certificate checks."}
                      </p>
                      {dispatchBlockers.length > 0 && (
                        <ul className="mt-2 text-xs text-amber-800 list-disc list-inside">
                          {dispatchBlockers.map((b, i) => (
                            <li key={i}>{b}</li>
                          ))}
                        </ul>
                      )}
                    </div>

                    <button
                      onClick={() => onNavigateStep(15)}
                      className="px-4 py-2 bg-emerald-600 hover:bg-emerald-700 text-white rounded font-bold text-xs shadow-xs transition"
                    >
                      Proceed to Step 15: Client Clearance →
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
