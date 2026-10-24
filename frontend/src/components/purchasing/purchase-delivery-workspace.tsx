"use client";
import {
  ValidatedInput,
  ValidatedLabel,
  ValidatedSelect,
  ValidatedTextarea,
} from "@/components/ui/validated-controls";

import React, { useState, useEffect } from "react";

export interface PurchaseDeliveryWorkspaceProps {
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

interface DeliveryCaseData {
  id: string;
  project_id: string;
  bo_id: string;
  supplier_id: string | null;
  po_number: string;
  version: number;
  client_clearance: {
    status: string;
    mdcc_ref: string | null;
    clearance_date: string | null;
    cleared_by_authority: string | null;
    remarks: string | null;
    file_name: string | null;
  };
  dispatch: {
    status: string;
    dispatch_date: string | null;
    transporter_name: string | null;
    lr_number: string | null;
    lr_date: string | null;
    vehicle_number: string | null;
    eway_bill_number: string | null;
    eway_bill_valid_until: string | null;
    invoice_number: string | null;
    invoice_date: string | null;
    driver_contact: string | null;
    file_name: string | null;
  };
  site_delivery: {
    status: string;
    arrival_date: string | null;
    receiving_officer: string | null;
    packages_received: number;
    packages_condition: string;
    transit_damage_notes: string | null;
    gate_entry_ref: string | null;
  };
  grn: {
    status: string;
    grn_number: string | null;
    grn_date: string | null;
    received_qty: number;
    accepted_qty: number;
    rejected_qty: number;
    uom: string;
    store_keeper_name: string | null;
    remarks: string | null;
    warranty_ref: string | null;
    warranty_valid_until: string | null;
    file_name: string | null;
  };
  pe_confirmation: {
    status: string;
    confirmation_date: string | null;
    engineer_name: string | null;
    designation: string | null;
    mbook_ref: string | null;
    work_package_ref: string | null;
    remarks: string | null;
  };
  accounts_acceptance: {
    status: string;
    acceptance_date: string | null;
    officer_name: string | null;
    voucher_ref: string | null;
    booked_amount: number;
    notes: string | null;
  };
}

export function PurchaseDeliveryWorkspace({
  project,
  projectName,
  lines,
  boId,
  initialStep = 15,
  onNavigateStep,
}: PurchaseDeliveryWorkspaceProps) {
  const [currentStep, setCurrentStep] = useState<number>(initialStep);
  const [loading, setLoading] = useState(true);
  const [reloadRevision, setReloadRevision] = useState(0);
  const [isPersisted, setIsPersisted] = useState(false);
  const [poNumber, setPoNumber] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  const [fmsStats, setFmsStats] = useState<{
    step_scores: Record<number, number>;
    phase5_earned_weight: number;
    phase5_total_weight: number;
    phase5_completion_pct: number;
    is_fully_completed: boolean;
  }>({
    step_scores: {},
    phase5_earned_weight: 0,
    phase5_total_weight: 9.0,
    phase5_completion_pct: 0,
    is_fully_completed: false,
  });

  // Step 15 state
  const [clearanceStatus, setClearanceStatus] = useState("pending");
  const [mdccRef, setMdccRef] = useState("");
  const [clearanceDate, setClearanceDate] = useState("");
  const [clearedByAuth, setClearedByAuth] = useState("");
  const [clearanceRemarks, setClearanceRemarks] = useState("");
  const [clearanceFileName, setClearanceFileName] = useState("");

  // Step 16 state
  const [dispatchStatus, setDispatchStatus] = useState("pending");
  const [dispatchDate, setDispatchDate] = useState("");
  const [transporterName, setTransporterName] = useState("");
  const [lrNumber, setLrNumber] = useState("");
  const [lrDate, setLrDate] = useState("");
  const [vehicleNumber, setVehicleNumber] = useState("");
  const [ewayBillNo, setEwayBillNo] = useState("");
  const [ewayBillValid, setEwayBillValid] = useState("");
  const [invoiceNumber, setInvoiceNumber] = useState("");
  const [invoiceDate, setInvoiceDate] = useState("");
  const [driverContact, setDriverContact] = useState("");
  const [dispatchFileName, setDispatchFileName] = useState("");

  // Step 17 state
  const [deliveryStatus, setDeliveryStatus] = useState("pending");
  const [arrivalDate, setArrivalDate] = useState("");
  const [receivingOfficer, setReceivingOfficer] = useState("");
  const [packagesReceived, setPackagesReceived] = useState(0);
  const [packagesCondition, setPackagesCondition] = useState("not_checked");
  const [transitNotes, setTransitNotes] = useState("");
  const [gateEntryRef, setGateEntryRef] = useState("");

  // Step 18 state
  const [grnStatus, setGrnStatus] = useState("pending");
  const [grnNumber, setGrnNumber] = useState("");
  const [grnDate, setGrnDate] = useState("");
  const [receivedQty, setReceivedQty] = useState(0);
  const [acceptedQty, setAcceptedQty] = useState(0);
  const [rejectedQty, setRejectedQty] = useState(0);
  const [uom, setUom] = useState("Unit");
  const [storeKeeperName, setStoreKeeperName] = useState("");
  const [grnRemarks, setGrnRemarks] = useState("");
  const [warrantyRef, setWarrantyRef] = useState("");
  const [warrantyValidUntil, setWarrantyValidUntil] = useState("");
  const [grnFileName, setGrnFileName] = useState("");

  // Step 19 state
  const [peStatus, setPeStatus] = useState("pending");
  const [peConfirmationDate, setPeConfirmationDate] = useState("");
  const [peEngineerName, setPeEngineerName] = useState("");
  const [peDesignation, setPeDesignation] = useState("");
  const [mbookRef, setMbookRef] = useState("");
  const [workPackageRef, setWorkPackageRef] = useState("");
  const [peRemarks, setPeRemarks] = useState("");

  // Step 20 state
  const [accountsStatus, setAccountsStatus] = useState("pending");
  const [accountsDate, setAccountsDate] = useState("");
  const [accountsOfficerName, setAccountsOfficerName] = useState("");
  const [accountsVoucherRef, setAccountsVoucherRef] = useState("");
  const [bookedAmount, setBookedAmount] = useState(0);
  const [accountsNotes, setAccountsNotes] = useState("");

  // View GRN Voucher preview modal/tab
  const [showGRNPreview, setShowGRNPreview] = useState(false);

  // Sync prop changes during render
  const [prevInitialStep, setPrevInitialStep] = useState(initialStep);
  if (initialStep !== prevInitialStep) {
    setPrevInitialStep(initialStep);
    if (initialStep >= 15 && initialStep <= 20) {
      setCurrentStep(initialStep);
    }
  }

  // Load Delivery Case Data from backend
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
          `/api/projects/${project}/purchase-delivery?bo_id=${boId}`,
        );
        if (!res.ok) {
          throw new Error(`Failed to load delivery data (${res.status})`);
        }
        const data = await res.json();
        if (live && data.case) {
          setIsPersisted(Boolean(data.is_persisted));
          setPoNumber(data.case.po_number ?? "");
          const c: DeliveryCaseData = data.case;
          if (data.fms_stats) {
            setFmsStats(data.fms_stats);
          }

          // Sync step 15
          if (c.client_clearance) {
            setClearanceStatus(c.client_clearance.status ?? "pending");
            setMdccRef(c.client_clearance.mdcc_ref ?? "");
            setClearanceDate(c.client_clearance.clearance_date ?? "");
            setClearedByAuth(c.client_clearance.cleared_by_authority ?? "");
            setClearanceRemarks(c.client_clearance.remarks ?? "");
            setClearanceFileName(c.client_clearance.file_name ?? "");
          }

          // Sync step 16
          if (c.dispatch) {
            setDispatchStatus(c.dispatch.status ?? "pending");
            setDispatchDate(c.dispatch.dispatch_date ?? "");
            setTransporterName(c.dispatch.transporter_name ?? "");
            setLrNumber(c.dispatch.lr_number ?? "");
            setLrDate(c.dispatch.lr_date ?? "");
            setVehicleNumber(c.dispatch.vehicle_number ?? "");
            setEwayBillNo(c.dispatch.eway_bill_number ?? "");
            setEwayBillValid(c.dispatch.eway_bill_valid_until ?? "");
            setInvoiceNumber(c.dispatch.invoice_number ?? "");
            setInvoiceDate(c.dispatch.invoice_date ?? "");
            setDriverContact(c.dispatch.driver_contact ?? "");
            setDispatchFileName(c.dispatch.file_name ?? "");
          }

          // Sync step 17
          if (c.site_delivery) {
            setDeliveryStatus(c.site_delivery.status ?? "pending");
            setArrivalDate(c.site_delivery.arrival_date ?? "");
            setReceivingOfficer(c.site_delivery.receiving_officer ?? "");
            setPackagesReceived(c.site_delivery.packages_received ?? 0);
            setPackagesCondition(
              c.site_delivery.packages_condition ?? "not_checked",
            );
            setTransitNotes(c.site_delivery.transit_damage_notes ?? "");
            setGateEntryRef(c.site_delivery.gate_entry_ref ?? "");
          }

          // Sync step 18
          if (c.grn) {
            setGrnStatus(c.grn.status ?? "pending");
            setGrnNumber(c.grn.grn_number ?? "");
            setGrnDate(c.grn.grn_date ?? "");
            setReceivedQty(c.grn.received_qty ?? 0);
            setAcceptedQty(c.grn.accepted_qty ?? 0);
            setRejectedQty(c.grn.rejected_qty ?? 0);
            setUom(c.grn.uom ?? "Unit");
            setStoreKeeperName(c.grn.store_keeper_name ?? "");
            setGrnRemarks(c.grn.remarks ?? "");
            setWarrantyRef(c.grn.warranty_ref ?? "");
            setWarrantyValidUntil(c.grn.warranty_valid_until ?? "");
            setGrnFileName(c.grn.file_name ?? "");
          }

          // Sync step 19
          if (c.pe_confirmation) {
            setPeStatus(c.pe_confirmation.status ?? "pending");
            setPeConfirmationDate(c.pe_confirmation.confirmation_date ?? "");
            setPeEngineerName(c.pe_confirmation.engineer_name ?? "");
            setPeDesignation(c.pe_confirmation.designation ?? "");
            setMbookRef(c.pe_confirmation.mbook_ref ?? "");
            setWorkPackageRef(c.pe_confirmation.work_package_ref ?? "");
            setPeRemarks(c.pe_confirmation.remarks ?? "");
          }

          // Sync step 20
          if (c.accounts_acceptance) {
            setAccountsStatus(c.accounts_acceptance.status ?? "pending");
            setAccountsDate(c.accounts_acceptance.acceptance_date ?? "");
            setAccountsOfficerName(c.accounts_acceptance.officer_name ?? "");
            setAccountsVoucherRef(c.accounts_acceptance.voucher_ref ?? "");
            setBookedAmount(c.accounts_acceptance.booked_amount ?? 0);
            setAccountsNotes(c.accounts_acceptance.notes ?? "");
          }
        }
      } catch (err: unknown) {
        if (live) {
          setError(
            err instanceof Error ? err.message : "Failed to load delivery case",
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

  // Step 15: Save Client Dispatch Clearance (MDCC)
  const handleSaveClearance = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-delivery/client-clearance?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: clearanceStatus,
            mdcc_ref: mdccRef,
            clearance_date: clearanceDate,
            cleared_by_authority: clearedByAuth,
            remarks: clearanceRemarks,
            file_name: clearanceFileName,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to save client clearance");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification(
        "Step 15: Client Dispatch Clearance (MDCC) successfully recorded.",
      );
      onNavigateStep(16);
      setCurrentStep(16);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to save client clearance",
      );
    } finally {
      setSaving(false);
    }
  };

  // Step 16: Save Material Dispatch
  const handleSaveDispatch = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-delivery/dispatch?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: dispatchStatus,
            dispatch_date: dispatchDate,
            transporter_name: transporterName,
            lr_number: lrNumber,
            lr_date: lrDate,
            vehicle_number: vehicleNumber,
            eway_bill_number: ewayBillNo,
            eway_bill_valid_until: ewayBillValid,
            invoice_number: invoiceNumber,
            invoice_date: invoiceDate,
            driver_contact: driverContact,
            file_name: dispatchFileName,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to save material dispatch");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification(
        "Step 16: Material dispatch and transporter LR recorded.",
      );
      onNavigateStep(17);
      setCurrentStep(17);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to save material dispatch",
      );
    } finally {
      setSaving(false);
    }
  };

  // Step 17: Save Site Delivery
  const handleSaveDelivery = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-delivery/site-delivery?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: deliveryStatus,
            arrival_date: arrivalDate,
            receiving_officer: receivingOfficer,
            packages_received: packagesReceived,
            packages_condition: packagesCondition,
            transit_damage_notes: transitNotes,
            gate_entry_ref: gateEntryRef,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to save site delivery");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification(
        "Step 17: Site arrival and unloading verification recorded.",
      );
      onNavigateStep(18);
      setCurrentStep(18);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to save site delivery",
      );
    } finally {
      setSaving(false);
    }
  };

  // Step 18: Save Goods Receipt Note (GRN)
  const handleSaveGRN = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-delivery/grn?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: grnStatus,
            grn_number: grnNumber,
            grn_date: grnDate,
            received_qty: receivedQty,
            accepted_qty: acceptedQty,
            rejected_qty: rejectedQty,
            uom: uom,
            store_keeper_name: storeKeeperName,
            remarks: grnRemarks,
            warranty_ref: warrantyRef,
            warranty_valid_until: warrantyValidUntil,
            file_name: grnFileName,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to issue GRN");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification("Step 18: Goods receipt details saved.");
      onNavigateStep(19);
      setCurrentStep(19);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to issue GRN");
    } finally {
      setSaving(false);
    }
  };

  // Step 19: Save PE Confirmation
  const handleSavePE = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-delivery/pe-confirmation?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: peStatus,
            confirmation_date: peConfirmationDate,
            engineer_name: peEngineerName,
            designation: peDesignation,
            mbook_ref: mbookRef,
            work_package_ref: workPackageRef,
            remarks: peRemarks,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || "Failed to save PE confirmation");
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification(
        "Step 19: Project Engineer site confirmation & M-Book entry recorded.",
      );
      onNavigateStep(20);
      setCurrentStep(20);
    } catch (err: unknown) {
      setError(
        err instanceof Error ? err.message : "Failed to save PE confirmation",
      );
    } finally {
      setSaving(false);
    }
  };

  // Step 20: Save Accounts Acceptance & Final Closure
  const handleSaveAccounts = async () => {
    if (!boId) return;
    setSaving(true);
    setError(null);
    try {
      const res = await fetch(
        `/api/projects/${project}/purchase-delivery/accounts-acceptance?bo_id=${boId}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            status: accountsStatus,
            acceptance_date: accountsDate,
            officer_name: accountsOfficerName,
            voucher_ref: accountsVoucherRef,
            booked_amount: bookedAmount,
            notes: accountsNotes,
          }),
        },
      );
      if (!res.ok) {
        const data = await res.json();
        throw new Error(
          data.detail || "Failed to finalize accounts acceptance",
        );
      }
      setReloadRevision((value) => value + 1);
      window.dispatchEvent(
        new CustomEvent("purchase-progress-saved", { detail: project }),
      );
      showNotification("Step 20: Accounts acceptance details saved.");
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to finalize accounts acceptance",
      );
    } finally {
      setSaving(false);
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
        <span className="text-emerald-800 font-semibold">
          Delivery progress {fmsStats.phase5_completion_pct.toFixed(1)}%
        </span>
        <button
          type="button"
          onClick={() => setShowGRNPreview(!showGRNPreview)}
        >
          {showGRNPreview ? "Close document preview" : "GRN document preview"}
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
            Phase 5 Delivery &amp; Handover records...
          </div>
        ) : showGRNPreview ? (
          /* Live High-Fidelity Letterhead Goods Receipt Note (GRN) Preview */
          <div className="max-w-4xl mx-auto bg-white border border-slate-300 rounded-lg shadow-md p-8 font-serif print:p-0">
            <div className="flex justify-between items-start pb-4 border-b-2 border-slate-900">
              <div>
                <h2 className="text-xl font-black tracking-wide text-slate-900 font-sans">
                  VISHAL TECHNOPOWER PRIVATE LIMITED
                </h2>
                <p className="text-xs text-slate-600 font-sans">
                  MATERIAL MANAGEMENT &amp; CENTRAL STORE INWARD VOUCHER
                </p>
                <p className="text-[11px] text-slate-500 font-sans mt-0.5">
                  Regd. Office: 204, City Centre, Near Central Bus Station,
                  Vadodara, Gujarat - 390001
                </p>
              </div>
              <div className="text-right font-sans">
                <span className="inline-block bg-emerald-800 text-white font-bold text-xs px-2.5 py-1 rounded">
                  GOODS RECEIPT NOTE (GRN)
                </span>
                <p className="text-xs font-mono font-bold text-slate-800 mt-1">
                  GRN No: {grnNumber}
                </p>
                <p className="text-xs text-slate-600">Date: {grnDate}</p>
              </div>
            </div>

            <div className="my-6 grid grid-cols-2 gap-4 text-xs font-sans">
              <div className="p-3 bg-slate-50 border border-slate-200 rounded">
                <p className="font-bold text-slate-900 mb-1">
                  PROJECT &amp; SITE DETAILS:
                </p>
                <p>
                  <span className="text-slate-500">Project:</span> {projectName}
                </p>
                <p>
                  <span className="text-slate-500">Site Location:</span>{" "}
                  {projectName || "Not specified"}
                </p>
                <p>
                  <span className="text-slate-500">Gate Entry Ref:</span>{" "}
                  {gateEntryRef}
                </p>
                <p>
                  <span className="text-slate-500">Receiving Officer:</span>{" "}
                  {receivingOfficer}
                </p>
              </div>
              <div className="p-3 bg-slate-50 border border-slate-200 rounded">
                <p className="font-bold text-slate-900 mb-1">
                  DISPATCH &amp; TRANSPORTER DETAILS:
                </p>
                <p>
                  <span className="text-slate-500">PO Number:</span>{" "}
                  {poNumber || "Not linked"}
                </p>
                <p>
                  <span className="text-slate-500">Transporter:</span>{" "}
                  {transporterName}
                </p>
                <p>
                  <span className="text-slate-500">LR No. &amp; Date:</span>{" "}
                  {lrNumber} (Dt: {lrDate})
                </p>
                <p>
                  <span className="text-slate-500">Vehicle No:</span>{" "}
                  {vehicleNumber} | e-Way Bill: {ewayBillNo}
                </p>
              </div>
            </div>

            {/* Inward Items Table */}
            <div className="my-6 font-sans">
              <table className="vtpl-data-table w-full text-xs border-collapse border border-slate-300">
                <thead>
                  <tr className="bg-slate-100 text-slate-900 font-bold">
                    <th className="border border-slate-300 p-2 text-center w-12">
                      Sr.
                    </th>
                    <th className="border border-slate-300 p-2 text-left">
                      Item Description
                    </th>
                    <th className="border border-slate-300 p-2 text-center w-24">
                      Item Code
                    </th>
                    <th className="border border-slate-300 p-2 text-center w-20">
                      Received
                    </th>
                    <th className="border border-slate-300 p-2 text-center w-20 text-emerald-800">
                      Accepted
                    </th>
                    <th className="border border-slate-300 p-2 text-center w-20 text-rose-800">
                      Rejected
                    </th>
                    <th className="border border-slate-300 p-2 text-left w-48">
                      Store Remarks
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
                      {receivedQty} {uom}
                    </td>
                    <td className="border border-slate-300 p-2 text-center font-bold text-emerald-700">
                      {acceptedQty} {uom}
                    </td>
                    <td className="border border-slate-300 p-2 text-center font-bold text-rose-600">
                      {rejectedQty} {uom}
                    </td>
                    <td className="border border-slate-300 p-2 text-slate-600 text-[11px]">
                      {grnRemarks}
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div className="my-4 p-3 bg-slate-50 border border-slate-200 rounded font-sans text-xs">
              <p className="font-bold text-slate-900">
                WARRANTY &amp; GUARANTEE COMPLIANCE:
              </p>
              <p className="mt-1 text-slate-700">
                Covered under OEM Warranty Certificate Ref:{" "}
                <strong className="font-mono">{warrantyRef}</strong> valid until{" "}
                <strong>{warrantyValidUntil}</strong> against manufacturing and
                galvanizing defects.
              </p>
            </div>

            {/* Signatures */}
            <div className="mt-12 pt-6 border-t border-slate-400 grid grid-cols-3 gap-6 text-center font-sans text-xs">
              <div>
                <p className="font-bold text-slate-800">{storeKeeperName}</p>
                <p className="text-slate-500 text-[10px]">
                  Site Store Executive
                </p>
                <p className="text-slate-400 text-[9px] mt-8">
                  Material Received &amp; Verified
                </p>
              </div>
              <div>
                <p className="font-bold text-slate-800">{peEngineerName}</p>
                <p className="text-slate-500 text-[10px]">
                  Project Engineer (Quality &amp; M-Book)
                </p>
                <p className="text-slate-400 text-[9px] mt-8">
                  Technical Acceptance Certified
                </p>
              </div>
              <div>
                <p className="font-bold text-slate-800">
                  {accountsOfficerName}
                </p>
                <p className="text-slate-500 text-[10px]">
                  Commercial &amp; Accounts Authorization
                </p>
                <p className="text-slate-400 text-[9px] mt-8">
                  Booked in ERP Ledger
                </p>
              </div>
            </div>

            <div className="mt-8 flex justify-end gap-3 print:hidden font-sans">
              <button
                onClick={() => window.print()}
                className="px-4 py-2 bg-emerald-800 text-white rounded font-semibold text-xs hover:bg-emerald-900"
              >
                🖨 Print Official GRN Voucher
              </button>
              <button
                onClick={() => setShowGRNPreview(false)}
                className="px-4 py-2 bg-slate-200 text-slate-700 rounded font-semibold text-xs hover:bg-slate-300"
              >
                Back to Workspace
              </button>
            </div>
          </div>
        ) : (
          <div className="max-w-5xl mx-auto space-y-6">
            {/* STEP 15: CLIENT DISPATCH CLEARANCE (MDCC) */}
            {currentStep === 15 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4 mb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Client Dispatch Clearance / Material Dispatch Clearance
                      Certificate (MDCC)
                    </h2>
                  </div>
                  <span
                    className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                      clearanceStatus === "cleared"
                        ? "bg-emerald-100 text-emerald-800"
                        : "bg-amber-100 text-amber-800"
                    }`}
                  >
                    Status: {clearanceStatus}
                  </span>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Clearance Status <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="clearance_status"
                        value={clearanceStatus}
                        onChange={(e) => setClearanceStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded focus:ring-1 focus:ring-blue-500 bg-white"
                      >
                        <option value="pending">
                          Pending Client Formal Clearance
                        </option>
                        <option value="cleared">
                          MDCC Issued &amp; Clearance Granted
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      MDCC Reference Number{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="mdcc_ref"
                        type="text"
                        value={mdccRef}
                        onChange={(e) => setMdccRef(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono font-bold"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Clearance Date <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="clearance_date"
                        type="date"
                        value={clearanceDate}
                        onChange={(e) => setClearanceDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Clearing Authority (Client / Consultant)
                      <ValidatedInput
                        name="cleared_by_auth"
                        type="text"
                        value={clearedByAuth}
                        onChange={(e) => setClearedByAuth(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Client MDCC document reference (file not uploaded)
                    </ValidatedLabel>
                    <div className="flex gap-2">
                      <ValidatedInput
                        name="clearance_file_name"
                        type="text"
                        value={clearanceFileName}
                        onChange={(e) => setClearanceFileName(e.target.value)}
                        className="flex-1 text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </div>
                  </div>

                  <div className="col-span-2">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Clearance Remarks / Dispatch Instructions
                      <ValidatedTextarea
                        name="clearance_remarks"
                        rows={2}
                        value={clearanceRemarks}
                        onChange={(e) => setClearanceRemarks(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                <div className="mt-6 flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSaveClearance}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Record MDCC Clearance & Proceed to Step 16 →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 16: MATERIAL DISPATCH & LOGISTICS TRACKING */}
            {currentStep === 16 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4 mb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Vendor Material Dispatch Advice &amp; Logistics Tracking
                    </h2>
                  </div>
                  <span
                    className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                      dispatchStatus === "dispatched"
                        ? "bg-emerald-100 text-emerald-800"
                        : "bg-amber-100 text-amber-800"
                    }`}
                  >
                    Status: {dispatchStatus}
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Dispatch Status <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="dispatch_status"
                        value={dispatchStatus}
                        onChange={(e) => setDispatchStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded bg-white"
                      >
                        <option value="pending">Pending Vendor Loading</option>
                        <option value="dispatched">
                          Dispatched with Transporter LR
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Dispatch Date <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="dispatch_date"
                        type="date"
                        value={dispatchDate}
                        onChange={(e) => setDispatchDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Transporter / Logistics Carrier Name{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="transporter_name"
                        type="text"
                        value={transporterName}
                        onChange={(e) => setTransporterName(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Lorry Receipt (LR) Number{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="lr_number"
                        type="text"
                        value={lrNumber}
                        onChange={(e) => setLrNumber(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono font-bold text-slate-900"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      LR Booking Date
                      <ValidatedInput
                        name="lr_date"
                        type="date"
                        value={lrDate}
                        onChange={(e) => setLrDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Truck / Vehicle Registration No.
                      <ValidatedInput
                        name="vehicle_number"
                        type="text"
                        value={vehicleNumber}
                        onChange={(e) => setVehicleNumber(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      e-Way Bill Number
                      <ValidatedInput
                        name="eway_bill_no"
                        type="text"
                        value={ewayBillNo}
                        onChange={(e) => setEwayBillNo(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      e-Way Bill Valid Until
                      <ValidatedInput
                        name="eway_bill_valid"
                        type="date"
                        value={ewayBillValid}
                        onChange={(e) => setEwayBillValid(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Vendor Invoice No. &amp; Date
                    </ValidatedLabel>
                    <div className="flex gap-2">
                      <ValidatedInput
                        name="invoice_number"
                        type="text"
                        value={invoiceNumber}
                        onChange={(e) => setInvoiceNumber(e.target.value)}
                        className="w-2/3 text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                      <ValidatedInput
                        name="invoice_date"
                        type="date"
                        value={invoiceDate}
                        onChange={(e) => setInvoiceDate(e.target.value)}
                        className="w-1/3 text-xs p-2 border border-slate-300 rounded"
                      />
                    </div>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Driver Name &amp; Mobile Contact for Tracking
                      <ValidatedInput
                        name="driver_contact"
                        type="text"
                        value={driverContact}
                        onChange={(e) => setDriverContact(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                <div className="mt-6 flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSaveDispatch}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Record Dispatch & Proceed to Step 17 (Site Delivery) →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 17: SITE DELIVERY & UNLOADING CHECK (5.00%) */}
            {currentStep === 17 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6 space-y-4">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4 mb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Material Receipt at Site &amp; Physical Damage
                      Verification
                    </h2>
                  </div>
                  <span
                    className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                      deliveryStatus === "delivered_at_site"
                        ? "bg-emerald-100 text-emerald-800"
                        : "bg-blue-100 text-blue-800"
                    }`}
                  >
                    Status: {deliveryStatus}
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Site Delivery Status{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="delivery_status"
                        value={deliveryStatus}
                        onChange={(e) => setDeliveryStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded bg-white"
                      >
                        <option value="pending">
                          Not dispatched / pending
                        </option>
                        <option value="in_transit">In Transit to Site</option>
                        <option value="delivered_at_site">
                          Delivered &amp; Unloaded at Site Store
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Actual Site Arrival Date{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="arrival_date"
                        type="date"
                        value={arrivalDate}
                        onChange={(e) => setArrivalDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Site Gate Entry Reference No.
                      <ValidatedInput
                        name="gate_entry_ref"
                        type="text"
                        value={gateEntryRef}
                        onChange={(e) => setGateEntryRef(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Receiving Site Officer Name &amp; Designation{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="receiving_officer"
                        type="text"
                        value={receivingOfficer}
                        onChange={(e) => setReceivingOfficer(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Packages / Bundles Received Count
                      <ValidatedInput
                        name="packages_received"
                        type="number"
                        value={packagesReceived}
                        onChange={(e) =>
                          setPackagesReceived(Number(e.target.value))
                        }
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Condition of Consignment Upon Unloading
                      <ValidatedSelect
                        name="packages_condition"
                        value={packagesCondition}
                        onChange={(e) => setPackagesCondition(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded bg-white"
                      >
                        <option value="not_checked">Not checked</option>
                        <option value="intact">
                          Intact - Zero Transit Damage
                        </option>
                        <option value="minor_damage">
                          Minor Cosmetic Scratches (Acceptable)
                        </option>
                        <option value="quarantine">
                          Transit Damage / Quarantined for Review
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Unloading Verification Notes &amp; Observations
                      <ValidatedTextarea
                        name="transit_notes"
                        rows={2}
                        value={transitNotes}
                        onChange={(e) => setTransitNotes(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                <div className="mt-6 flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSaveDelivery}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Verify Site Receipt & Proceed to Step 18 (GRN) →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 18: GOODS RECEIPT NOTE (GRN) & WARRANTY LETTER */}
            {currentStep === 18 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6 space-y-4">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4 mb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Store Goods Receipt Note (GRN) Inwarding &amp; Warranty
                      Letter
                    </h2>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => setShowGRNPreview(true)}
                      className="px-3 py-1.5 bg-emerald-50 text-emerald-800 border border-emerald-200 rounded text-xs font-semibold hover:bg-emerald-100"
                    >
                      📄 Preview Official GRN Voucher
                    </button>
                    <span
                      className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                        grnStatus === "issued"
                          ? "bg-emerald-100 text-emerald-800"
                          : "bg-amber-100 text-amber-800"
                      }`}
                    >
                      Status: {grnStatus}
                    </span>
                  </div>
                </div>

                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      GRN Status <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="grn_status"
                        value={grnStatus}
                        onChange={(e) => setGrnStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded bg-white"
                      >
                        <option value="pending">Draft GRN</option>
                        <option value="issued">
                          Officially Inwarded &amp; Issued
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Official GRN Number{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="grn_number"
                        type="text"
                        value={grnNumber}
                        onChange={(e) => setGrnNumber(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono font-bold text-slate-900"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      GRN Date <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="grn_date"
                        type="date"
                        value={grnDate}
                        onChange={(e) => setGrnDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Received Quantity <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="received_qty"
                        type="number"
                        value={receivedQty}
                        onChange={(e) => setReceivedQty(Number(e.target.value))}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Accepted Quantity
                      <ValidatedInput
                        name="accepted_qty"
                        type="number"
                        value={acceptedQty}
                        onChange={(e) => setAcceptedQty(Number(e.target.value))}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono font-bold text-emerald-700"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Rejected / Shortage Qty
                      <ValidatedInput
                        name="rejected_qty"
                        type="number"
                        value={rejectedQty}
                        onChange={(e) => setRejectedQty(Number(e.target.value))}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono text-rose-600"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Store Keeper / Issuer Name
                      <ValidatedInput
                        name="store_keeper_name"
                        type="text"
                        value={storeKeeperName}
                        onChange={(e) => setStoreKeeperName(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Manufacturer Warranty Letter Ref
                      <ValidatedInput
                        name="warranty_ref"
                        type="text"
                        value={warrantyRef}
                        onChange={(e) => setWarrantyRef(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Warranty Guarantee Period Valid Until
                      <ValidatedInput
                        name="warranty_valid_until"
                        type="date"
                        value={warrantyValidUntil}
                        onChange={(e) => setWarrantyValidUntil(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Store Inward Observations &amp; Binning Remarks
                      <ValidatedTextarea
                        name="grn_remarks"
                        rows={2}
                        value={grnRemarks}
                        onChange={(e) => setGrnRemarks(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                <div className="mt-6 flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSaveGRN}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Issue GRN & Proceed to Step 19 (PE Confirmation) →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 19: PROJECT ENGINEER SITE CONFIRMATION & M-BOOK */}
            {currentStep === 19 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6 space-y-4">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4 mb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Project Engineer (PE) Site Confirmation &amp; Measurement
                      Book (M-Book)
                    </h2>
                  </div>
                  <span
                    className={`px-2.5 py-1 rounded text-xs font-bold uppercase ${
                      peStatus === "confirmed"
                        ? "bg-emerald-100 text-emerald-800"
                        : "bg-amber-100 text-amber-800"
                    }`}
                  >
                    Status: {peStatus}
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      PE Confirmation Status{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="pe_status"
                        value={peStatus}
                        onChange={(e) => setPeStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded bg-white"
                      >
                        <option value="pending">
                          Pending Site Engineering Inspection
                        </option>
                        <option value="confirmed">
                          Technical Acceptance Certified &amp; Recorded
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Confirmation Date <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="pe_confirmation_date"
                        type="date"
                        value={peConfirmationDate}
                        onChange={(e) => setPeConfirmationDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Project Engineer Name{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="pe_engineer_name"
                        type="text"
                        value={peEngineerName}
                        onChange={(e) => setPeEngineerName(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Designation
                      <ValidatedInput
                        name="pe_designation"
                        type="text"
                        value={peDesignation}
                        onChange={(e) => setPeDesignation(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Measurement Book (M-Book) Ref No.
                      <ValidatedInput
                        name="mbook_ref"
                        type="text"
                        value={mbookRef}
                        onChange={(e) => setMbookRef(e.target.value)}
                        placeholder="e.g. MB-04/Page-88/Item-14"
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Allocated Work Package Ref
                      <ValidatedInput
                        name="work_package_ref"
                        type="text"
                        value={workPackageRef}
                        onChange={(e) => setWorkPackageRef(e.target.value)}
                        placeholder="e.g. WP-AAI-AIRFIELD-LIGHTING-P1"
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Site Physical Check &amp; Installation Readiness
                      Observations
                      <ValidatedTextarea
                        name="pe_remarks"
                        rows={2}
                        value={peRemarks}
                        onChange={(e) => setPeRemarks(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                <div className="mt-6 flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSavePE}
                    className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded font-semibold text-xs shadow-xs transition"
                  >
                    {saving
                      ? "Saving..."
                      : "Sign Off PE Confirmation & Proceed to Step 20 (Final Accounts) →"}
                  </button>
                </div>
              </div>
            )}

            {/* STEP 20: ACCOUNTS ACCEPTANCE & FINAL 100% CLOSURE */}
            {currentStep === 20 && (
              <div className="bg-white rounded-lg border border-slate-200 shadow-xs p-6 space-y-6">
                <div className="flex items-center justify-between border-b border-slate-200 pb-4">
                  <div>
                    <h2 className="text-base font-bold text-slate-900">
                      Accounts Acceptance, Ledger Booking &amp; Complete
                      Handover
                    </h2>
                  </div>
                  <span
                    className={`px-3 py-1 rounded text-xs font-bold uppercase ${
                      accountsStatus === "accepted"
                        ? "bg-emerald-100 text-emerald-800"
                        : "bg-blue-100 text-blue-800"
                    }`}
                  >
                    Status: {accountsStatus}
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-4">
                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Accounts Acceptance Status{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedSelect
                        name="accounts_status"
                        value={accountsStatus}
                        onChange={(e) => setAccountsStatus(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded bg-white"
                      >
                        <option value="pending">
                          Pending Accounts Verification
                        </option>
                        <option value="accepted">
                          Accepted &amp; Booked in Books of Accounts
                        </option>
                      </ValidatedSelect>
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Final Acceptance Date{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="accounts_date"
                        type="date"
                        value={accountsDate}
                        onChange={(e) => setAccountsDate(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Accounts Officer Name{" "}
                      <span className="text-rose-500">*</span>
                      <ValidatedInput
                        name="accounts_officer_name"
                        type="text"
                        value={accountsOfficerName}
                        onChange={(e) => setAccountsOfficerName(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Journal Voucher (JV) / Ledger Booking Ref
                      <ValidatedInput
                        name="accounts_voucher_ref"
                        type="text"
                        value={accountsVoucherRef}
                        onChange={(e) => setAccountsVoucherRef(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Total Inward Booked Amount (₹)
                      <ValidatedInput
                        name="booked_amount"
                        type="number"
                        value={bookedAmount}
                        onChange={(e) =>
                          setBookedAmount(Number(e.target.value))
                        }
                        className="w-full text-xs p-2 border border-slate-300 rounded font-mono font-bold text-slate-900"
                      />
                    </ValidatedLabel>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Saved Delivery Checks
                      <ValidatedInput
                        type="text"
                        disabled
                        value={
                          fmsStats.is_fully_completed
                            ? "Delivery steps 15–20 saved as complete"
                            : "Delivery checks pending"
                        }
                        className="w-full text-xs p-2 bg-slate-100 border border-slate-300 rounded font-semibold text-emerald-800"
                      />
                    </ValidatedLabel>
                  </div>

                  <div className="col-span-3">
                    <ValidatedLabel className="block text-xs font-semibold text-slate-700 mb-1">
                      Accounts Notes &amp; Supplier Final Invoice Settlement
                      Instructions
                      <ValidatedTextarea
                        name="accounts_notes"
                        rows={2}
                        value={accountsNotes}
                        onChange={(e) => setAccountsNotes(e.target.value)}
                        className="w-full text-xs p-2 border border-slate-300 rounded"
                      />
                    </ValidatedLabel>
                  </div>
                </div>

                {fmsStats.is_fully_completed && (
                  <p className="p-4 bg-emerald-50 border border-emerald-300 rounded text-emerald-900">
                    Saved delivery steps 15–20 are complete. Earlier purchase
                    stages are tracked separately.
                  </p>
                )}

                <div className="flex justify-end gap-3 border-t border-slate-200 pt-4">
                  <button
                    disabled={saving || loading || !boId}
                    onClick={handleSaveAccounts}
                    className="px-6 py-2.5 bg-emerald-700 hover:bg-emerald-800 text-white rounded font-bold text-xs shadow-md transition"
                  >
                    {saving ? "Finalizing..." : "Save accounts acceptance"}
                  </button>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
