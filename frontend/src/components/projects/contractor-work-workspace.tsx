"use client";
import { ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";

import { useEffect, useState, type FormEvent } from "react";
import { useProjectUser } from "@/components/projects/project-permissions";

export type TradeHeadcount = {
  id: string;
  trade_category: "highly_skilled" | "skilled" | "semi_skilled" | "unskilled";
  trade_role: string;
  headcount: number;
  work_front: string;
  mobilization_target_date?: string | null;
  equipment_needed: string[];
};

export type LaborBOQItem = {
  id: string;
  item_code: string;
  description_en: string;
  description_hi: string;
  uom: string;
  quantity: number;
  estimated_rate: number;
  work_type: "electrical" | "civil" | "mechanical" | "solar";
  drawing_ref?: string | null;
};

export type ContractorEnquiry = {
  enquiry_no: string;
  floated_date: string;
  due_date: string;
  contractor_ids: string[];
  status: "draft" | "floated" | "closed";
  notes: string;
};

export type ContractorQuotation = {
  contractor_id: string;
  contractor_name: string;
  quote_ref: string;
  quote_date: string;
  item_rates: Record<string, number>;
  total_basic: number;
  gst_pct: number;
  grand_total: number;
  rank: string;
  negotiated_total?: number | null;
  selection_status: "under_review" | "recommended" | "selected" | "rejected";
  remarks: string;
};

export type ContractorWorkOrder = {
  wo_number: string;
  wo_date: string;
  contractor_id: string;
  contractor_name: string;
  contractor_address: string;
  work_duration_days: number;
  start_date: string;
  completion_date: string;
  total_contract_value: number;
  advance_amount: number;
  retention_pct: number;
  payment_terms_desc: string;
  statutory_clauses_accepted: boolean;
  status: "draft" | "issued" | "accepted_by_contractor" | "active" | "completed";
  signatory_name: string;
};

export type SiteMobilisation = {
  status: "pending" | "mobilised" | "verified_at_site";
  actual_arrival_date?: string | null;
  reported_headcount: number;
  site_supervisor: string;
  supervisor_phone: string;
  ppe_verified: boolean;
  tools_verified: boolean;
  safety_induction_date?: string | null;
  gate_pass_issued: boolean;
  verified_by_pe: string;
  remarks: string;
};

export type ContractorWorkOrderCase = {
  id: string;
  project_id: string;
  version: number;
  manpower: TradeHeadcount[];
  labor_boq: LaborBOQItem[];
  enquiries: ContractorEnquiry[];
  quotations: ContractorQuotation[];
  work_order?: ContractorWorkOrder | null;
  mobilisation: SiteMobilisation;
  is_active: boolean;
  created_at?: string;
};

type HistoryEvent = {
  id: string;
  action: string;
  reason: string;
  created_at: string;
  created_by_id: string | null;
  before: Record<string, unknown>;
  after: Record<string, unknown>;
};

type StepTab =
  | "p08_manpower"
  | "p09_boq"
  | "p10_enquiries"
  | "p11_quotations"
  | "p13_wo"
  | "p14_mobilisation"
  | "history";

const BOQ_PRESETS = [
  {
    code: "LBR-ELEC-01",
    en: "Laying, dressing, clamping and testing of 1.1kV / 11kV XLPE HT/LT power cables in trench/tray including megger testing.",
    hi: "ट्रेंच अथवा केबल ट्रे में 1.1kV / 11kV XLPE केबल बिछाना, ड्रेसिंग, क्लैंपिंग एवं मेगर टेस्टिंग।",
    uom: "Mtr",
    rate: 45,
    type: "electrical" as const,
  },
  {
    code: "LBR-ELEC-02",
    en: "End termination of 3.5C / 4C XLPE aluminium armored cable using heavy duty brass glands and crimping lugs.",
    hi: "ब्रास ग्लैंड्स एवं क्रिम्पिंग लग्स द्वारा XLPE एल्युमीनियम आर्मर्ड केबल का एंड टर्मिनेशन कार्य।",
    uom: "Set",
    rate: 350,
    type: "electrical" as const,
  },
  {
    code: "LBR-SOL-01",
    en: "Assembly, bolting, torque checking and alignment of solar PV ground-mount MMS structures as per drawing.",
    hi: "ड्राइंग के अनुसार सोलर पीवी स्ट्रक्चर (MMS) की असेंबली, बोल्टिंग, टॉर्क चेकिंग और सटीक एलाइनमेंट।",
    uom: "KWp",
    rate: 220,
    type: "solar" as const,
  },
  {
    code: "LBR-SOL-02",
    en: "Installation, securing, MC4 string connector crimping and VOC checking of bifacial PV modules.",
    hi: "सोलर पीवी मॉड्यूल्स की माउंटिंग, फिक्सिंग, MC4 स्ट्रिंग कनेक्टर क्रिम्पिंग व वोल्टेज चेकिंग।",
    uom: "Nos",
    rate: 110,
    type: "solar" as const,
  },
  {
    code: "LBR-CIV-01",
    en: "RCC pile foundation / inverter station civil plinth casting including shuttering and reinforcement.",
    hi: "शटरिंग और रीइन्फोर्समेंट सरिया के साथ RCC पाइल फाउंडेशन एवं इन्वर्टर चबूतरा सिविल निर्माण।",
    uom: "Cum",
    rate: 1850,
    type: "civil" as const,
  },
  {
    code: "LBR-ELEC-03",
    en: "Installation of maintenance-free chemical earthing pit with GI earth electrode and backfill compound.",
    hi: "GI अर्थ इलेक्ट्रोड एवं अर्थिंग कंपाउंड के साथ मेंटेनेंस-फ्री केमिकल अर्थिंग पिट का निर्माण।",
    uom: "Nos",
    rate: 1200,
    type: "electrical" as const,
  },
];

export function ContractorWorkWorkspace({
  projectId,
  projectName,
}: {
  projectId: string;
  projectName: string;
}) {
  const user = useProjectUser();
  const canWrite =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("projects.contractor_work.write");

  const canApprove =
    user.permissions.includes("system:admin") ||
    user.permissions.includes("projects.contractor_work.approve");

  const [activeStep, setActiveStep] = useState<StepTab>("p08_manpower");
  const [caseData, setCaseData] = useState<ContractorWorkOrderCase | null>(null);
  const [contractorMaster, setContractorMaster] = useState<
    Array<{ id: string; code: string; name: string; phone: string; trade_capabilities: string[] }>
  >([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Modals / Dialogs
  const [showAddTradeModal, setShowAddTradeModal] = useState(false);
  const [showAddBOQModal, setShowAddBOQModal] = useState(false);
  const [showQuoteModal, setShowQuoteModal] = useState(false);
  const [historyEvents, setHistoryEvents] = useState<HistoryEvent[]>([]);

  // Trade form
  const [tradeCategory, setTradeCategory] = useState<"skilled" | "semi_skilled" | "unskilled" | "highly_skilled">("skilled");
  const [tradeRole, setTradeRole] = useState("");
  const [tradeHeadcount, setTradeHeadcount] = useState(4);
  const [tradeWorkFront, setTradeWorkFront] = useState("");
  const [tradeTargetDate, setTradeTargetDate] = useState("");

  // BOQ form
  const [boqCode, setBoqCode] = useState("");
  const [boqEn, setBoqEn] = useState("");
  const [boqHi, setBoqHi] = useState("");
  const [boqUom, setBoqUom] = useState("Mtr");
  const [boqQty, setBoqQty] = useState(100);
  const [boqRate, setBoqRate] = useState(50);
  const [boqType, setBoqType] = useState<"electrical" | "civil" | "mechanical" | "solar">("electrical");
  const [boqDrawing, setBoqDrawing] = useState("");

  // Enquiry state
  const [enqNo, setEnqNo] = useState("");
  const [enqFloatedDate, setEnqFloatedDate] = useState("");
  const [enqDueDate, setEnqDueDate] = useState("");
  const [enqSelectedContractors, setEnqSelectedContractors] = useState<string[]>([]);
  const [enqNotes, setEnqNotes] = useState("");

  // Quotation form
  const [quoteContractorId, setQuoteContractorId] = useState("");
  const [quoteRef, setQuoteRef] = useState("");
  const [quoteDate, setQuoteDate] = useState("");
  const [quoteBasicTotal, setQuoteBasicTotal] = useState(0);
  const [quoteGstPct, setQuoteGstPct] = useState(18);
  const [quoteNegotiated, setQuoteNegotiated] = useState<number | "">("");
  const [quoteRemarks, setQuoteRemarks] = useState("");

  // Work Order form
  const [woNumber, setWoNumber] = useState("");
  const [woDate, setWoDate] = useState(new Date().toISOString().split("T")[0]);
  const [woContractorId, setWoContractorId] = useState("");
  const [woStartDate, setWoStartDate] = useState("");
  const [woDurationDays, setWoDurationDays] = useState(30);
  const [woContractValue, setWoContractValue] = useState(0);
  const [woAdvanceAmount, setWoAdvanceAmount] = useState(0);
  const [woRetentionPct, setWoRetentionPct] = useState(10);
  const [woPaymentTerms, setWoPaymentTerms] = useState(
    "10% mobilization advance against security, 80% progressive RA bills on certified joint measurement, 10% retention upon final testing and PE sign-off."
  );
  const [woSignatory, setWoSignatory] = useState(user.email || "VTPL Project Director");

  // Mobilisation state
  const [mobArrivalDate, setMobArrivalDate] = useState("");
  const [mobHeadcount, setMobHeadcount] = useState(0);
  const [mobSupervisor, setMobSupervisor] = useState("");
  const [mobSupervisorPhone, setMobSupervisorPhone] = useState("");
  const [mobPPE, setMobPPE] = useState(false);
  const [mobTools, setMobTools] = useState(false);
  const [mobInductionDate, setMobInductionDate] = useState("");
  const [mobGatePass, setMobGatePass] = useState(false);
  const [mobPEVerification, setMobPEVerification] = useState("");
  const [mobRemarks, setMobRemarks] = useState("");
  const [reload, setReload] = useState(0);

  useEffect(() => {
    let live = true;
    Promise.all([
      fetch(`/api/projects/${projectId}/contractor-work-orders`, { cache: "no-store" }),
      fetch("/api/settings/contractors", { cache: "no-store" }),
    ])
      .then(async ([caseRes, masterRes]) => {
        if (!caseRes.ok) throw new Error("Failed to load contractor work order data");
        const [cData, mData] = await Promise.all([
          caseRes.json(),
          masterRes.ok ? masterRes.json() : [],
        ]);
        if (live) {
          setCaseData(cData);
          setContractorMaster(mData);
          if (cData.work_order) {
            setWoNumber(cData.work_order.wo_number);
            setWoDate(cData.work_order.wo_date);
            setWoContractorId(cData.work_order.contractor_id);
            setWoStartDate(cData.work_order.start_date);
            setWoDurationDays(cData.work_order.work_duration_days);
            setWoContractValue(cData.work_order.total_contract_value);
            setWoAdvanceAmount(cData.work_order.advance_amount);
            setWoRetentionPct(cData.work_order.retention_pct);
            setWoPaymentTerms(cData.work_order.payment_terms_desc);
            setWoSignatory(cData.work_order.signatory_name);
          } else {
            setWoNumber(`VTPL/WO/${projectId.substring(0, 5).toUpperCase()}/${new Date().getFullYear()}/001`);
          }

          if (cData.mobilisation) {
            setMobArrivalDate(cData.mobilisation.actual_arrival_date || "");
            setMobHeadcount(cData.mobilisation.reported_headcount || 0);
            setMobSupervisor(cData.mobilisation.site_supervisor || "");
            setMobSupervisorPhone(cData.mobilisation.supervisor_phone || "");
            setMobPPE(cData.mobilisation.ppe_verified);
            setMobTools(cData.mobilisation.tools_verified);
            setMobInductionDate(cData.mobilisation.safety_induction_date || "");
            setMobGatePass(cData.mobilisation.gate_pass_issued);
            setMobPEVerification(cData.mobilisation.verified_by_pe || "");
            setMobRemarks(cData.mobilisation.remarks || "");
          }
          setError(null);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (live) {
          setError(err instanceof Error ? err.message : "Error loading data");
          setLoading(false);
        }
      });
    return () => {
      live = false;
    };
  }, [projectId, reload]);

  useEffect(() => {
    if (activeStep !== "history") return;
    let live = true;
    fetch(`/api/projects/${projectId}/contractor-work-orders/history`)
      .then((res) => (res.ok ? res.json() : []))
      .then((hData) => {
        if (live) setHistoryEvents(hData);
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [activeStep, projectId, reload]);

  const loadCase = () => {
    setLoading(true);
    setReload((r) => r + 1);
  };

  // Calculate total estimated BOQ value
  const totalBOQValue = (caseData?.labor_boq || []).reduce(
    (acc, item) => acc + item.quantity * item.estimated_rate,
    0
  );

  // Calculate total assessed manpower
  const totalAssessedManpower = (caseData?.manpower || []).reduce(
    (acc, item) => acc + item.headcount,
    0
  );

  // Handle Save Manpower
  const handleSaveManpower = async (updatedList: TradeHeadcount[]) => {
    if (!caseData) return;
    const reason = window.prompt("Audit reason for updating Manpower Assessment plan:");
    if (!reason || !reason.trim()) return;

    setSubmitting(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/contractor-work-orders/manpower`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          items: updatedList,
          version: caseData.version,
          reason: reason.trim(),
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Failed to update manpower");
      }
      const updatedCase = await res.json();
      setCaseData(updatedCase);
      setNotice("Manpower Assessment plan updated successfully.");
      setShowAddTradeModal(false);
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : "Failed to update manpower");
    } finally {
      setSubmitting(false);
    }
  };

  // Add Trade item
  const handleAddTrade = (e: FormEvent) => {
    e.preventDefault();
    if (!tradeRole.trim() || !tradeWorkFront.trim() || tradeHeadcount <= 0) {
      return;
    }
    const newItem: TradeHeadcount = {
      id: `MH-${Date.now().toString().slice(-4)}`,
      trade_category: tradeCategory,
      trade_role: tradeRole.trim(),
      headcount: Number(tradeHeadcount),
      work_front: tradeWorkFront.trim(),
      mobilization_target_date: tradeTargetDate || null,
      equipment_needed: [],
    };
    const currentList = caseData?.manpower || [];
    handleSaveManpower([...currentList, newItem]);
  };

  // Delete Trade item
  const handleDeleteTrade = (id: string) => {
    if (!confirm("Are you sure you want to remove this trade headcount row?")) return;
    const currentList = (caseData?.manpower || []).filter((item) => item.id !== id);
    handleSaveManpower(currentList);
  };

  // Handle Save Labor BOQ
  const handleSaveLaborBOQ = async (updatedList: LaborBOQItem[]) => {
    if (!caseData) return;
    const reason = window.prompt("Audit reason for updating Scope of Work & Labor BOQ:");
    if (!reason || !reason.trim()) return;

    setSubmitting(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/contractor-work-orders/labor-boq`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          items: updatedList,
          version: caseData.version,
          reason: reason.trim(),
        }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Failed to update labor BOQ");
      }
      const updatedCase = await res.json();
      setCaseData(updatedCase);
      setNotice("Labor BOQ updated successfully.");
      setShowAddBOQModal(false);
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : "Failed to update labor BOQ");
    } finally {
      setSubmitting(false);
    }
  };

  // Add BOQ Item
  const handleAddBOQItem = (e: FormEvent) => {
    e.preventDefault();
    if (!boqEn.trim() || !boqHi.trim() || boqQty <= 0) {
      return;
    }
    const newItem: LaborBOQItem = {
      id: `BOQ-${Date.now().toString().slice(-4)}`,
      item_code: boqCode.trim() || `LBR-${Date.now().toString().slice(-3)}`,
      description_en: boqEn.trim(),
      description_hi: boqHi.trim(),
      uom: boqUom.trim(),
      quantity: Number(boqQty),
      estimated_rate: Number(boqRate),
      work_type: boqType,
      drawing_ref: boqDrawing.trim() || null,
    };
    const currentList = caseData?.labor_boq || [];
    handleSaveLaborBOQ([...currentList, newItem]);
  };

  // Apply Preset
  const applyPreset = (preset: (typeof BOQ_PRESETS)[0]) => {
    setBoqCode(preset.code);
    setBoqEn(preset.en);
    setBoqHi(preset.hi);
    setBoqUom(preset.uom);
    setBoqRate(preset.rate);
    setBoqType(preset.type);
  };

  // Delete BOQ Item
  const handleDeleteBOQItem = (id: string) => {
    if (!confirm("Are you sure you want to remove this labor BOQ item?")) return;
    const currentList = (caseData?.labor_boq || []).filter((item) => item.id !== id);
    handleSaveLaborBOQ(currentList);
  };

  const [enquiryError, setEnquiryError] = useState("");
  // Float Enquiry
  const handleFloatEnquiry = async (e: FormEvent) => {
    e.preventDefault();
    if (!caseData) return;
    if (enqSelectedContractors.length === 0) {
      setEnquiryError("Select at least one contractor from the master bank.");
      return;
    }
    const reason = window.prompt("Audit reason for floating enquiry / RFQ:");
    if (!reason || !reason.trim()) return;

    setSubmitting(true);
    try {
      const payload = {
        enquiry: {
          enquiry_no: enqNo.trim() || `RFQ-${Date.now().toString().slice(-4)}`,
          floated_date: enqFloatedDate || new Date().toISOString().split("T")[0],
          due_date: enqDueDate || new Date(Date.now() + 7 * 86400000).toISOString().split("T")[0],
          contractor_ids: enqSelectedContractors,
          status: "floated",
          notes: enqNotes.trim(),
        },
        version: caseData.version,
        reason: reason.trim(),
      };

      const res = await fetch(`/api/projects/${projectId}/contractor-work-orders/enquiries`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Failed to float enquiry");
      }

      const updatedCase = await res.json();
      setCaseData(updatedCase);
      setNotice("Enquiries floated to contractors successfully.");
      setActiveStep("p11_quotations");
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : "Failed to float enquiry");
    } finally {
      setSubmitting(false);
    }
  };

  // Add / Update Quotation
  const handleAddQuotation = async (e: FormEvent) => {
    e.preventDefault();
    if (!caseData || !quoteContractorId) {
      return;
    }
    const contractor = contractorMaster.find((c) => c.id === quoteContractorId);
    const contractorName = contractor ? contractor.name : "Unknown Contractor";

    const reason = window.prompt("Audit reason for recording contractor quotation:");
    if (!reason || !reason.trim()) return;

    const basic = Number(quoteBasicTotal);
    const gstVal = basic * (Number(quoteGstPct) / 100);
    const grand = basic + gstVal;
    const negotiated = quoteNegotiated !== "" ? Number(quoteNegotiated) : null;

    const newQuote: ContractorQuotation = {
      contractor_id: quoteContractorId,
      contractor_name: contractorName,
      quote_ref: quoteRef.trim() || `QUO-${Date.now().toString().slice(-4)}`,
      quote_date: quoteDate || new Date().toISOString().split("T")[0],
      item_rates: {},
      total_basic: basic,
      gst_pct: Number(quoteGstPct),
      grand_total: grand,
      rank: "L1", // Will be auto-ranked by backend
      negotiated_total: negotiated,
      selection_status: "under_review",
      remarks: quoteRemarks.trim(),
    };

    // Filter out existing quote from this contractor if updating
    const existingQuotes = (caseData.quotations || []).filter(
      (q) => q.contractor_id !== quoteContractorId
    );
    const updatedQuotes = [...existingQuotes, newQuote];

    setSubmitting(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/contractor-work-orders/quotations`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          quotes: updatedQuotes,
          version: caseData.version,
          reason: reason.trim(),
        }),
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Failed to save quotation");
      }

      const updatedCase = await res.json();
      setCaseData(updatedCase);
      setNotice("Quotation and Comparative Statement updated.");
      setShowQuoteModal(false);
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : "Failed to save quotation");
    } finally {
      setSubmitting(false);
    }
  };

  // Issue Work Order
  const handleIssueWorkOrder = async (e: FormEvent) => {
    e.preventDefault();
    if (!caseData || !woContractorId || woContractValue <= 0) {
      return;
    }
    const contractor = contractorMaster.find((c) => c.id === woContractorId);
    const contractorName = contractor ? contractor.name : "Contractor";

    const reason = window.prompt("Audit reason for issuing Official Contractor Work Order:");
    if (!reason || !reason.trim()) return;

    const completion = new Date(new Date(woStartDate).getTime() + woDurationDays * 86400000)
      .toISOString()
      .split("T")[0];

    const payload = {
      work_order: {
        wo_number: woNumber.trim(),
        wo_date: woDate,
        contractor_id: woContractorId,
        contractor_name: contractorName,
        contractor_address: "Registered labor contractor facility",
        work_duration_days: Number(woDurationDays),
        start_date: woStartDate,
        completion_date: completion,
        total_contract_value: Number(woContractValue),
        advance_amount: Number(woAdvanceAmount),
        retention_pct: Number(woRetentionPct),
        payment_terms_desc: woPaymentTerms.trim(),
        statutory_clauses_accepted: true,
        status: "issued",
        signatory_name: woSignatory.trim(),
      },
      version: caseData.version,
      reason: reason.trim(),
    };

    setSubmitting(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/contractor-work-orders/work-order`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Failed to issue work order");
      }

      const updatedCase = await res.json();
      setCaseData(updatedCase);
      setNotice("Contractor Work Order successfully issued on VTPL letterhead.");
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : "Failed to issue work order");
    } finally {
      setSubmitting(false);
    }
  };

  // Update Mobilisation
  const handleUpdateMobilisation = async (e: FormEvent) => {
    e.preventDefault();
    if (!caseData) return;
    const reason = window.prompt("Audit reason for recording Site Mobilisation sign-off:");
    if (!reason || !reason.trim()) return;

    const payload = {
      mobilisation: {
        status: mobPEVerification ? "verified_at_site" : mobHeadcount > 0 ? "mobilised" : "pending",
        actual_arrival_date: mobArrivalDate || null,
        reported_headcount: Number(mobHeadcount),
        site_supervisor: mobSupervisor.trim(),
        supervisor_phone: mobSupervisorPhone.trim(),
        ppe_verified: mobPPE,
        tools_verified: mobTools,
        safety_induction_date: mobInductionDate || null,
        gate_pass_issued: mobGatePass,
        verified_by_pe: mobPEVerification.trim(),
        remarks: mobRemarks.trim(),
      },
      version: caseData.version,
      reason: reason.trim(),
    };

    setSubmitting(true);
    try {
      const res = await fetch(`/api/projects/${projectId}/contractor-work-orders/mobilisation`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.detail || "Failed to update mobilisation");
      }

      const updatedCase = await res.json();
      setCaseData(updatedCase);
      setNotice("Site Mobilisation recorded and verified successfully.");
    } catch (err: unknown) {
      alert(err instanceof Error ? err.message : "Failed to update mobilisation");
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="rounded-2xl border border-slate-200 bg-white p-12 text-center text-slate-500">
        Loading Contractor Work Orders & Manpower workspace...
      </div>
    );
  }

  if (error || !caseData) {
    return (
      <div className="rounded-2xl border border-red-200 bg-red-50 p-6 text-sm text-red-700">
        <p className="font-bold">Error loading workspace:</p>
        <p>{error || "No data returned."}</p>
        <button
          onClick={loadCase}
          className="mt-3 rounded-lg bg-red-600 px-4 py-1.5 text-xs font-bold text-white hover:bg-red-700"
        >
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Workspace Header */}
      <div className="rounded-2xl border border-slate-200 bg-white p-5 sm:p-6 shadow-sm">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <span className="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-amber-50 text-amber-700 font-bold text-sm">
                ⚡
              </span>
              <h2 className="text-xl font-extrabold text-slate-900">
                Contractor Work Orders & Manpower · {projectName}
              </h2>
            </div>
            <p className="mt-1 text-sm text-slate-600 max-w-3xl">
              End-to-end execution of erection & labor contracts: from headcount assessment and
              dual-language Hindi/English labor BOQ, through RFQ floatation, L1/L2/L3 commercial
              comparative statements, official work order issuance, to site mobilisation verification.
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <div className="rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-xs">
              <span className="text-slate-500 block">Assessed Manpower</span>
              <span className="text-base font-extrabold text-slate-900">
                {totalAssessedManpower} workers
              </span>
            </div>
            <div className="rounded-xl border border-slate-200 bg-slate-50 px-3 py-2 text-xs">
              <span className="text-slate-500 block">Labor BOQ Estimate</span>
              <span className="text-base font-extrabold text-blue-700">
                ₹{totalBOQValue.toLocaleString("en-IN")}
              </span>
            </div>
          </div>
        </div>

        {notice && (
          <div className="mt-4 rounded-xl bg-emerald-50 border border-emerald-200 p-3 text-xs font-bold text-emerald-800 flex items-center justify-between">
            <span>✓ {notice}</span>
            <button onClick={() => setNotice(null)} className="text-emerald-600 hover:text-emerald-900">
              ✕
            </button>
          </div>
        )}
      </div>

      {/* Process Stepper Bar */}
      <div className="flex overflow-x-auto border-b border-slate-200 bg-white rounded-xl shadow-sm px-2">
        {[
          { key: "p08_manpower", label: "P08. Manpower", badge: `${caseData.manpower.length} trades` },
          { key: "p09_boq", label: "P09. Labor BOQ (हिन्दी/Eng)", badge: `${caseData.labor_boq.length} items` },
          { key: "p10_enquiries", label: "P10. RFQ Enquiries", badge: `${caseData.enquiries.length} floated` },
          { key: "p11_quotations", label: "P11-12. CS & Rates", badge: `${caseData.quotations.length} quotes` },
          {
            key: "p13_wo",
            label: "P13. Work Order",
            badge: caseData.work_order ? caseData.work_order.status : "Pending",
          },
          {
            key: "p14_mobilisation",
            label: "P14. Mobilisation",
            badge: caseData.mobilisation.status,
          },
          { key: "history", label: "Audit Trail", badge: "Log" },
        ].map((step) => {
          const active = activeStep === step.key;
          return (
            <button
              key={step.key}
              onClick={() => setActiveStep(step.key as StepTab)}
              className={`flex items-center gap-2 border-b-2 py-3.5 px-4 text-xs font-bold whitespace-nowrap transition ${
                active
                  ? "border-blue-700 text-blue-700 bg-blue-50/40"
                  : "border-transparent text-slate-600 hover:text-slate-900 hover:bg-slate-50"
              }`}
            >
              <span>{step.label}</span>
              <span
                className={`rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                  active ? "bg-blue-600 text-white" : "bg-slate-100 text-slate-600"
                }`}
              >
                {step.badge}
              </span>
            </button>
          );
        })}
      </div>

      {/* STEP 1: P08 Manpower Assessment */}
      {activeStep === "p08_manpower" && (
        <div className="space-y-4">
          <div className="flex items-center justify-between bg-white p-4 rounded-xl border border-slate-200">
            <div>
              <h3 className="font-extrabold text-slate-900 text-sm">
                Process 8: Manpower & Trade Headcount Assessment
              </h3>
              <p className="text-xs text-slate-500">
                Plan trade requirements, gang compositions, and work fronts for site execution.
              </p>
            </div>
            {canWrite && (
              <button
                onClick={() => setShowAddTradeModal(true)}
                className="rounded-lg bg-blue-700 px-3 py-1.5 text-xs font-bold text-white hover:bg-blue-800"
              >
                + Add Trade Gang
              </button>
            )}
          </div>

          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
            <table className="vtpl-data-table w-full text-left text-sm text-slate-700">
              <thead className="border-b border-slate-200 bg-slate-50 text-xs font-bold uppercase text-slate-500">
                <tr>
                  <th className="px-4 py-3">Category</th>
                  <th className="px-4 py-3">Trade Role</th>
                  <th className="px-4 py-3 text-center">Headcount</th>
                  <th className="px-4 py-3">Work Front</th>
                  <th className="px-4 py-3">Mobilization Date</th>
                  {canWrite && <th className="px-4 py-3 text-right">Actions</th>}
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {caseData.manpower.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-slate-400 italic text-xs">
                      No trade gangs planned yet. Click &quot;+ Add Trade Gang&quot; to begin.
                    </td>
                  </tr>
                ) : (
                  caseData.manpower.map((m) => (
                    <tr key={m.id} className="hover:bg-slate-50/70">
                      <td className="px-4 py-3 font-semibold text-xs capitalize text-slate-900">
                        <span
                          className={`inline-block rounded px-2 py-0.5 font-mono text-[11px] ${
                            m.trade_category === "highly_skilled"
                              ? "bg-purple-100 text-purple-800"
                              : m.trade_category === "skilled"
                              ? "bg-blue-100 text-blue-800"
                              : m.trade_category === "semi_skilled"
                              ? "bg-amber-100 text-amber-800"
                              : "bg-slate-100 text-slate-800"
                          }`}
                        >
                          {m.trade_category.replace("_", " ")}
                        </span>
                      </td>
                      <td className="px-4 py-3 font-medium text-slate-800">{m.trade_role}</td>
                      <td className="px-4 py-3 text-center font-bold text-blue-700">{m.headcount}</td>
                      <td className="px-4 py-3 text-slate-600">{m.work_front}</td>
                      <td className="px-4 py-3 text-xs text-slate-500">
                        {m.mobilization_target_date || "As per schedule"}
                      </td>
                      {canWrite && (
                        <td className="px-4 py-3 text-right">
                          <button
                            onClick={() => handleDeleteTrade(m.id)}
                            className="text-xs font-semibold text-red-600 hover:text-red-800"
                          >
                            Remove
                          </button>
                        </td>
                      )}
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* STEP 2: P09 Labor BOQ with Dual-Language Hindi & English */}
      {activeStep === "p09_boq" && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-white p-4 rounded-xl border border-slate-200">
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-extrabold text-slate-900 text-sm">
                  Process 9: Scope of Work & Labor BOQ (द्विभाषी - हिन्दी एवं English)
                </h3>
                <span className="rounded bg-indigo-50 border border-indigo-200 text-indigo-800 text-[10px] font-bold px-2 py-0.5">
                  Dual-Language Field Standard
                </span>
              </div>
              <p className="text-xs text-slate-500">
                Detailed scope descriptions in both English and Hindi for accurate site execution and
                clear labor subcontractor agreements.
              </p>
            </div>
            {canWrite && (
              <button
                onClick={() => setShowAddBOQModal(true)}
                className="rounded-lg bg-blue-700 px-3 py-1.5 text-xs font-bold text-white hover:bg-blue-800 self-start sm:self-auto"
              >
                + Add Labor BOQ Item
              </button>
            )}
          </div>

          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
            <table className="vtpl-data-table w-full text-left text-sm text-slate-700">
              <thead className="border-b border-slate-200 bg-slate-50 text-xs font-bold uppercase text-slate-500">
                <tr>
                  <th className="px-4 py-3 w-28">Item Code</th>
                  <th className="px-4 py-3">Description (English & हिन्दी)</th>
                  <th className="px-4 py-3 w-20 text-center">UOM</th>
                  <th className="px-4 py-3 w-24 text-right">Qty</th>
                  <th className="px-4 py-3 w-28 text-right">Est. Rate</th>
                  <th className="px-4 py-3 w-32 text-right">Total Est.</th>
                  {canWrite && <th className="px-4 py-3 w-16 text-right">Action</th>}
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {caseData.labor_boq.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="px-4 py-8 text-center text-slate-400 italic text-xs">
                      No labor BOQ items configured yet. Click &quot;+ Add Labor BOQ Item&quot; to begin.
                    </td>
                  </tr>
                ) : (
                  caseData.labor_boq.map((item) => (
                    <tr key={item.id} className="hover:bg-slate-50/70">
                      <td className="px-4 py-3 align-top font-mono text-xs font-bold text-blue-700">
                        {item.item_code}
                      </td>
                      <td className="px-4 py-3 align-top space-y-1">
                        <div className="font-semibold text-slate-900 text-xs">
                          {item.description_en}
                        </div>
                        <div className="font-medium text-slate-600 text-xs font-serif bg-amber-50/50 p-1 rounded border border-amber-100/60">
                          <span className="text-[10px] text-amber-800 font-sans font-bold mr-1">
                            हिन्दी:
                          </span>
                          {item.description_hi}
                        </div>
                      </td>
                      <td className="px-4 py-3 align-top text-center text-xs font-mono font-medium">
                        {item.uom}
                      </td>
                      <td className="px-4 py-3 align-top text-right font-medium text-xs">
                        {item.quantity.toLocaleString()}
                      </td>
                      <td className="px-4 py-3 align-top text-right font-mono text-xs">
                        ₹{item.estimated_rate.toLocaleString("en-IN")}
                      </td>
                      <td className="px-4 py-3 align-top text-right font-mono font-bold text-xs text-slate-900">
                        ₹{(item.quantity * item.estimated_rate).toLocaleString("en-IN")}
                      </td>
                      {canWrite && (
                        <td className="px-4 py-3 align-top text-right">
                          <button
                            onClick={() => handleDeleteBOQItem(item.id)}
                            className="text-xs font-semibold text-red-600 hover:text-red-800"
                          >
                            ✕
                          </button>
                        </td>
                      )}
                    </tr>
                  ))
                )}
              </tbody>
              <tfoot className="border-t-2 border-slate-200 bg-slate-50 font-bold text-slate-900 text-xs">
                <tr>
                  <td colSpan={5} className="px-4 py-3 text-right">
                    Total Estimated Labor Budget:
                  </td>
                  <td className="px-4 py-3 text-right font-mono text-blue-700 text-sm">
                    ₹{totalBOQValue.toLocaleString("en-IN")}
                  </td>
                  {canWrite && <td></td>}
                </tr>
              </tfoot>
            </table>
          </div>
        </div>
      )}

      {/* STEP 3: P10 Float Enquiries */}
      {activeStep === "p10_enquiries" && (
        <div className="space-y-4">
          <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm space-y-4">
            <div>
              <h3 className="font-extrabold text-slate-900 text-sm">
                Process 10: Float Enquiries / RFQs to Qualified Contractors
              </h3>
              <p className="text-xs text-slate-500">
                Select registered contractors from the Contractor Bank to invite competitive labor
                bids against the approved Hindi/English BOQ.
              </p>
            </div>

            <ValidatedForm onSubmit={handleFloatEnquiry} className="space-y-4 pt-2">
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Enquiry / RFQ Number

                  <ValidatedInput name="enq_no"
                    value={enqNo}
                    onChange={(e) => setEnqNo(e.target.value)}
                    placeholder="e.g. VTPL/RFQ/LABOR/2026/01"
                    className="w-full rounded-lg border border-slate-300 px-3 py-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Floated Date

                  <ValidatedInput name="enq_floated_date"
                    type="date"
                    value={enqFloatedDate}
                    onChange={(e) => setEnqFloatedDate(e.target.value)}
                    className="w-full rounded-lg border border-slate-300 px-3 py-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Quote Due Date

                  <ValidatedInput name="enq_due_date"
                    type="date"
                    value={enqDueDate}
                    onChange={(e) => setEnqDueDate(e.target.value)}
                    className="w-full rounded-lg border border-slate-300 px-3 py-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Select Qualified Contractors from Master Bank *
                </ValidatedLabel>
                <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2 max-h-48 overflow-y-auto p-2 border rounded-lg bg-slate-50">
                  {contractorMaster.length === 0 ? (
                    <p className="text-xs text-slate-400 p-2 italic">
                      No contractors found in settings. Please register contractors in Contractor Master.
                    </p>
                  ) : (
                    contractorMaster.map((c) => {
                      const selected = enqSelectedContractors.includes(c.id);
                      return (
                        <ValidatedLabel
                          key={c.id}
                          className={`flex items-start gap-2 p-2 rounded-lg border cursor-pointer text-xs ${
                            selected
                              ? "bg-blue-50 border-blue-400 text-blue-900"
                              : "bg-white border-slate-200 text-slate-700"
                          }`}
                        >
                          <ValidatedInput
                            type="checkbox"
                            checked={selected}
                            onChange={(e) => {
                              if (e.target.checked) {
                                setEnqSelectedContractors([...enqSelectedContractors, c.id]);
                              } else {
                                setEnqSelectedContractors(
                                  enqSelectedContractors.filter((id) => id !== c.id)
                                );
                              }
                            }}
                            className="mt-0.5"
                          />
                          <div>
                            <div className="font-bold">{c.name}</div>
                            <div className="text-[10px] text-slate-500">{c.phone}</div>
                          </div>
                        </ValidatedLabel>
                      );
                    })
                  )}
                </div>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Notes / Instructions
                <ValidatedTextarea name="enq_notes"
                  rows={2}
                  value={enqNotes}
                  onChange={(e) => setEnqNotes(e.target.value)}
                  placeholder="e.g. Quotations must include all labor, tools & tackles, ESI/PF, and PPE compliance."
                  className="w-full rounded-lg border border-slate-300 px-3 py-2 text-xs outline-none focus:border-blue-500"
                /></ValidatedLabel>
              </div>

              {canWrite && (
                <div className="flex justify-end">
                  {enquiryError && !enqSelectedContractors.length ? <p role="alert" className="text-sm text-red-600">{enquiryError}</p> : null}
                  <button
                    type="submit"
                    disabled={submitting}
                    className="rounded-xl bg-blue-700 px-5 py-2 text-xs font-bold text-white hover:bg-blue-800 shadow"
                  >
                    {submitting ? "Processing..." : "Float RFQ to Contractors"}
                  </button>
                </div>
              )}
            </ValidatedForm>
          </div>

          {/* Floated RFQs list */}
          {caseData.enquiries.length > 0 && (
            <div className="space-y-3">
              <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500">
                Previously Floated Enquiries
              </h4>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {caseData.enquiries.map((enq, idx) => (
                  <div key={idx} className="rounded-xl border border-slate-200 bg-white p-4 text-xs space-y-2 shadow-sm">
                    <div className="flex items-center justify-between">
                      <span className="font-bold text-slate-900">{enq.enquiry_no}</span>
                      <span className="rounded-full bg-blue-50 border border-blue-200 text-blue-700 px-2 py-0.5 font-semibold capitalize">
                        {enq.status}
                      </span>
                    </div>
                    <div className="text-slate-600">
                      Floated on: <strong>{enq.floated_date}</strong> · Due: <strong>{enq.due_date}</strong>
                    </div>
                    <div className="text-slate-500">
                      Invited Contractors: {enq.contractor_ids.length}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* STEP 4: P11 & P12 Commercial CS Matrix & Rate Negotiation */}
      {activeStep === "p11_quotations" && (
        <div className="space-y-4">
          <div className="flex items-center justify-between bg-white p-4 rounded-xl border border-slate-200">
            <div>
              <h3 className="font-extrabold text-slate-900 text-sm">
                Processes 11 & 12: Comparative Statement (CS) & Rate Negotiation
              </h3>
              <p className="text-xs text-slate-500">
                Automated L1 / L2 / L3 bidder rankings based on negotiated grand total, with budget
                variance tracking.
              </p>
            </div>
            {canWrite && (
              <button
                onClick={() => setShowQuoteModal(true)}
                className="rounded-lg bg-blue-700 px-3 py-1.5 text-xs font-bold text-white hover:bg-blue-800"
              >
                + Record Quotation / Negotiation
              </button>
            )}
          </div>

          <div className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
            <table className="vtpl-data-table w-full text-left text-sm text-slate-700">
              <thead className="border-b border-slate-200 bg-slate-50 text-xs font-bold uppercase text-slate-500">
                <tr>
                  <th className="px-4 py-3">Rank</th>
                  <th className="px-4 py-3">Contractor Name</th>
                  <th className="px-4 py-3">Quote Ref & Date</th>
                  <th className="px-4 py-3 text-right">Quoted Basic</th>
                  <th className="px-4 py-3 text-right">GST %</th>
                  <th className="px-4 py-3 text-right">Quoted Total</th>
                  <th className="px-4 py-3 text-right">Negotiated Total</th>
                  <th className="px-4 py-3 text-center">Status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-100">
                {caseData.quotations.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="px-4 py-8 text-center text-slate-400 italic text-xs">
                      No contractor quotations recorded yet. Click &quot;+ Record Quotation&quot; to add rates.
                    </td>
                  </tr>
                ) : (
                  caseData.quotations.map((q) => {
                    const isL1 = q.rank === "L1";
                    return (
                      <tr
                        key={q.contractor_id}
                        className={isL1 ? "bg-emerald-50/50 hover:bg-emerald-50" : "hover:bg-slate-50/70"}
                      >
                        <td className="px-4 py-3">
                          <span
                            className={`inline-flex items-center justify-center font-bold px-2 py-0.5 rounded text-xs ${
                              isL1
                                ? "bg-emerald-600 text-white shadow-sm"
                                : q.rank === "L2"
                                ? "bg-amber-100 text-amber-800 border border-amber-300"
                                : "bg-slate-100 text-slate-700"
                            }`}
                          >
                            {q.rank}
                          </span>
                        </td>
                        <td className="px-4 py-3 font-bold text-slate-900">{q.contractor_name}</td>
                        <td className="px-4 py-3 text-xs text-slate-600">
                          <div>{q.quote_ref}</div>
                          <div className="text-slate-400">{q.quote_date}</div>
                        </td>
                        <td className="px-4 py-3 text-right font-mono text-xs">
                          ₹{q.total_basic.toLocaleString("en-IN")}
                        </td>
                        <td className="px-4 py-3 text-right text-xs">{q.gst_pct}%</td>
                        <td className="px-4 py-3 text-right font-mono text-xs">
                          ₹{q.grand_total.toLocaleString("en-IN")}
                        </td>
                        <td className="px-4 py-3 text-right font-mono font-bold text-xs text-slate-900">
                          {q.negotiated_total != null ? (
                            <span className="text-emerald-700">
                              ₹{q.negotiated_total.toLocaleString("en-IN")}
                            </span>
                          ) : (
                            <span className="text-slate-400">—</span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-center">
                          <span
                            className={`rounded-full px-2 py-0.5 text-[11px] font-semibold capitalize ${
                              q.selection_status === "recommended" || q.selection_status === "selected"
                                ? "bg-emerald-100 text-emerald-800"
                                : q.selection_status === "rejected"
                                ? "bg-red-100 text-red-800"
                                : "bg-slate-100 text-slate-700"
                            }`}
                          >
                            {q.selection_status.replace("_", " ")}
                          </span>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* STEP 5: P13 Official Contractor Work Order on Letterhead */}
      {activeStep === "p13_wo" && (
        <div className="space-y-6">
          <div className="flex items-center justify-between bg-white p-4 rounded-xl border border-slate-200">
            <div>
              <h3 className="font-extrabold text-slate-900 text-sm">
                Process 13: Official Contractor Work Order (VTPL Letterhead)
              </h3>
              <p className="text-xs text-slate-500">
                Legally binding work contract with milestones, retention, and statutory ESI/PF &
                safety compliance.
              </p>
            </div>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => window.print()}
                className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-bold text-slate-700 hover:bg-slate-50 shadow-sm"
              >
                🖨 Print / Export PDF
              </button>
            </div>
          </div>

          {/* Letterhead Preview / Editor */}
          <div className="bg-white rounded-2xl border border-slate-200 p-8 shadow-sm space-y-6 print:border-none print:shadow-none print:p-0">
            {/* VTPL Corporate Header */}
            <div className="border-b-2 border-blue-900 pb-4 flex items-start justify-between">
              <div>
                <h1 className="text-2xl font-black tracking-tight text-blue-950">
                  VISHAL TECHNO POWER LIMITED
                </h1>
                <p className="text-xs text-slate-600 font-medium">
                  EPC Contractors & Engineers · Electrical, Solar & Power Distribution
                </p>
                <p className="text-[11px] text-slate-500 font-mono mt-0.5">
                  CIN: U40106GJ2020PLC114000 · GSTIN: 24AAACV1234A1Z5
                </p>
              </div>
              <div className="text-right">
                <span className="inline-block bg-blue-900 text-white font-extrabold text-xs px-3 py-1 rounded">
                  WORK ORDER
                </span>
                <div className="font-mono text-xs font-bold text-slate-900 mt-1">{woNumber}</div>
                <div className="text-[11px] text-slate-500">Date: {woDate}</div>
              </div>
            </div>

            {/* Editable or Issued WO Details */}
            <ValidatedForm onSubmit={handleIssueWorkOrder} className="space-y-6">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6 bg-slate-50/70 p-4 rounded-xl border border-slate-200">
                <div className="space-y-3">
                  <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500">
                    To Contractor:
                  </h4>
                  <div>
                    <ValidatedLabel className="block text-[11px] font-bold text-slate-700 mb-0.5">
                      Awarded Contractor *

                    <ValidatedSelect name="wo_contractor_id" required
                      value={woContractorId}
                      onChange={(e) => {
                        setWoContractorId(e.target.value);
                        // Also pre-fill contract value if quote exists
                        const q = caseData.quotations.find((item) => item.contractor_id === e.target.value);
                        if (q) {
                          setWoContractValue(q.negotiated_total || q.grand_total);
                        }
                      }}
                      className="w-full rounded-lg border border-slate-300 p-2 text-xs bg-white outline-none focus:border-blue-500"
                    >
                      <option value="">Select Contractor...</option>
                      {contractorMaster.map((c) => (
                        <option key={c.id} value={c.id}>
                          {c.name} ({c.code})
                        </option>
                      ))}
                    </ValidatedSelect></ValidatedLabel>
                  </div>

                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <ValidatedLabel className="block text-[11px] font-bold text-slate-700 mb-0.5">
                        Commencement Date *

                      <ValidatedInput name="wo_start_date"
                        type="date"
                        required
                        value={woStartDate}
                        onChange={(e) => setWoStartDate(e.target.value)}
                        className="w-full rounded-lg border border-slate-300 p-1.5 text-xs outline-none focus:border-blue-500"
                      /></ValidatedLabel>
                    </div>
                    <div>
                      <ValidatedLabel className="block text-[11px] font-bold text-slate-700 mb-0.5">
                        Duration (Days) *

                      <ValidatedInput name="wo_duration_days"
                        type="number"
                        min="1"
                        required
                        value={woDurationDays}
                        onChange={(e) => setWoDurationDays(Number(e.target.value))}
                        className="w-full rounded-lg border border-slate-300 p-1.5 text-xs outline-none focus:border-blue-500"
                      /></ValidatedLabel>
                    </div>
                  </div>
                </div>

                <div className="space-y-3">
                  <h4 className="text-xs font-bold uppercase tracking-wider text-slate-500">
                    Commercial & Payment Terms:
                  </h4>
                  <div className="grid grid-cols-2 gap-2">
                    <div>
                      <ValidatedLabel className="block text-[11px] font-bold text-slate-700 mb-0.5">
                        Agreed Contract Value (₹) *

                      <ValidatedInput name="wo_contract_value"
                        type="number"
                        min="1"
                        required
                        value={woContractValue}
                        onChange={(e) => setWoContractValue(Number(e.target.value))}
                        className="w-full rounded-lg border border-slate-300 p-1.5 text-xs font-mono font-bold text-blue-900 outline-none focus:border-blue-500"
                      /></ValidatedLabel>
                    </div>
                    <div>
                      <ValidatedLabel className="block text-[11px] font-bold text-slate-700 mb-0.5">
                        Advance Amount (₹)

                      <ValidatedInput name="wo_advance_amount"
                        type="number"
                        min="0"
                        value={woAdvanceAmount}
                        onChange={(e) => setWoAdvanceAmount(Number(e.target.value))}
                        className="w-full rounded-lg border border-slate-300 p-1.5 text-xs font-mono outline-none focus:border-blue-500"
                      /></ValidatedLabel>
                    </div>
                  </div>

                  <div>
                    <ValidatedLabel className="block text-[11px] font-bold text-slate-700 mb-0.5">
                      Retention Percentage (%)

                    <ValidatedInput name="wo_retention_pct"
                      type="number"
                      min="0"
                      max="20"
                      value={woRetentionPct}
                      onChange={(e) => setWoRetentionPct(Number(e.target.value))}
                      className="w-full rounded-lg border border-slate-300 p-1.5 text-xs outline-none focus:border-blue-500"
                    /></ValidatedLabel>
                  </div>
                </div>
              </div>

              {/* Payment Terms Description */}
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Payment Milestones & Release Conditions

                <ValidatedTextarea name="wo_payment_terms"
                  rows={2}
                  value={woPaymentTerms}
                  onChange={(e) => setWoPaymentTerms(e.target.value)}
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none focus:border-blue-500"
                /></ValidatedLabel>
              </div>

              {/* Standard Statutory Conditions */}
              <div className="rounded-xl border border-slate-200 bg-slate-50 p-4 space-y-2 text-xs text-slate-600">
                <h5 className="font-bold text-slate-800 uppercase tracking-wider text-[11px]">
                  Mandatory Statutory & Safety Clauses:
                </h5>
                <ul className="list-disc list-inside space-y-1 text-[11px] leading-relaxed">
                  <li>
                    <strong>Statutory Compliance:</strong> Contractor must ensure 100% compliance
                    with ESI, EPF, and Labour License acts. Monthly challans must be submitted prior
                    to RA bill release.
                  </li>
                  <li>
                    <strong>Site Safety & PPE:</strong> Helmets, safety shoes, high-visibility
                    jackets, and full-body harnesses (for heights &gt; 1.8m) are mandatory. Zero
                    tolerance on site violations.
                  </li>
                  <li>
                    <strong>Tools & Tackles:</strong> All electrical testing equipment (megger, earth
                    testers) must carry valid calibration certificates.
                  </li>
                  <li>
                    <strong>Liquidated Damages:</strong> Delay beyond completion date shall attract
                    LD @ 0.5% per week up to a maximum of 10% of contract value.
                  </li>
                </ul>
              </div>

              {/* Signatures Block */}
              <div className="grid grid-cols-2 gap-8 pt-8 border-t border-slate-300">
                <div>
                  <div className="text-xs font-bold text-slate-700">Accepted & Agreed by:</div>
                  <div className="h-14 border-b border-dashed border-slate-300 mt-2"></div>
                  <div className="text-[11px] text-slate-500 mt-1">Authorized Contractor Signatory & Seal</div>
                </div>

                <div className="text-right">
                  <div className="text-xs font-bold text-slate-700">For Vishal Techno Power Limited:</div>
                  <div className="h-14 border-b border-dashed border-slate-300 mt-2 flex items-end justify-end">
                    <span className="font-serif italic text-sm text-blue-900 font-bold">
                      {woSignatory}
                    </span>
                  </div>
                  <div className="text-[11px] text-slate-500 mt-1">Authorized Signatory</div>
                </div>
              </div>

              {canApprove && (
                <div className="flex justify-end pt-4 print:hidden">
                  <button
                    type="submit"
                    disabled={submitting}
                    className="rounded-xl bg-blue-700 px-6 py-2.5 text-xs font-bold text-white hover:bg-blue-800 shadow"
                  >
                    {submitting ? "Issuing..." : "Issue & Record Official Work Order"}
                  </button>
                </div>
              )}
            </ValidatedForm>
          </div>
        </div>
      )}

      {/* STEP 6: P14 Site Mobilisation */}
      {activeStep === "p14_mobilisation" && (
        <div className="space-y-4">
          <div className="bg-white p-5 rounded-2xl border border-slate-200 shadow-sm space-y-4">
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-extrabold text-slate-900 text-sm">
                  Process 14: Site Mobilisation & Safety Readiness Sign-off
                </h3>
                <span
                  className={`rounded-full px-2.5 py-0.5 text-[11px] font-bold capitalize ${
                    caseData.mobilisation.status === "verified_at_site"
                      ? "bg-emerald-100 text-emerald-800 border border-emerald-300"
                      : caseData.mobilisation.status === "mobilised"
                      ? "bg-blue-100 text-blue-800"
                      : "bg-slate-100 text-slate-700"
                  }`}
                >
                  {caseData.mobilisation.status.replace("_", " ")}
                </span>
              </div>
              <p className="text-xs text-slate-500">
                Verify actual worker arrival, site supervisor contact, mandatory PPE compliance, and
                secure Project Engineer (PE) verification before starting site work.
              </p>
            </div>

            <ValidatedForm onSubmit={handleUpdateMobilisation} className="space-y-4 pt-2">
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Actual Site Arrival Date

                  <ValidatedInput name="mob_arrival_date"
                    type="date"
                    value={mobArrivalDate}
                    onChange={(e) => setMobArrivalDate(e.target.value)}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Reported Crew Headcount

                  <ValidatedInput name="mob_headcount"
                    type="number"
                    min="0"
                    value={mobHeadcount}
                    onChange={(e) => setMobHeadcount(Number(e.target.value))}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Safety Induction Date

                  <ValidatedInput name="mob_induction_date"
                    type="date"
                    value={mobInductionDate}
                    onChange={(e) => setMobInductionDate(e.target.value)}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Contractor Site Supervisor Name

                  <ValidatedInput name="mob_supervisor"
                    value={mobSupervisor}
                    onChange={(e) => setMobSupervisor(e.target.value)}
                    placeholder="e.g. Ramesh Patel"
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Supervisor Contact Phone

                  <ValidatedInput name="mob_supervisor_phone"
                    value={mobSupervisorPhone}
                    onChange={(e) => setMobSupervisorPhone(e.target.value)}
                    placeholder="+91 98765 00000"
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
              </div>

              {/* Safety & Tools Verification Checklist */}
              <div className="rounded-xl border border-slate-200 bg-slate-50 p-4 space-y-3">
                <h5 className="font-bold text-slate-800 text-xs">
                  Site Readiness Checklist (Physical Verification):
                </h5>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <ValidatedLabel className="flex items-center gap-2 p-2.5 rounded-lg bg-white border border-slate-200 text-xs font-semibold cursor-pointer">
                    <ValidatedInput
                      type="checkbox"
                      checked={mobPPE}
                      onChange={(e) => setMobPPE(e.target.checked)}
                      className="h-4 w-4 rounded"
                    />
                    <span>🦺 PPE Verified (Helmets, Shoes, Vests)</span>
                  </ValidatedLabel>

                  <ValidatedLabel className="flex items-center gap-2 p-2.5 rounded-lg bg-white border border-slate-200 text-xs font-semibold cursor-pointer">
                    <ValidatedInput
                      type="checkbox"
                      checked={mobTools}
                      onChange={(e) => setMobTools(e.target.checked)}
                      className="h-4 w-4 rounded"
                    />
                    <span>🛠 Tools & Tackles Inspected</span>
                  </ValidatedLabel>

                  <ValidatedLabel className="flex items-center gap-2 p-2.5 rounded-lg bg-white border border-slate-200 text-xs font-semibold cursor-pointer">
                    <ValidatedInput
                      type="checkbox"
                      checked={mobGatePass}
                      onChange={(e) => setMobGatePass(e.target.checked)}
                      className="h-4 w-4 rounded"
                    />
                    <span>🎫 Security Gate Passes Issued</span>
                  </ValidatedLabel>
                </div>
              </div>

              {/* PE Sign-off */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    VTPL Project Engineer (PE) Verification Sign-off

                  <ValidatedInput name="mob_p_e_verification"
                    value={mobPEVerification}
                    onChange={(e) => setMobPEVerification(e.target.value)}
                    placeholder="Enter PE Name to confirm site readiness sign-off"
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none focus:border-blue-500 font-medium text-emerald-900"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Remarks
                  <ValidatedInput name="mob_remarks"
                    value={mobRemarks}
                    onChange={(e) => setMobRemarks(e.target.value)}
                    placeholder="Additional site notes..."
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none focus:border-blue-500"
                  /></ValidatedLabel>
                </div>
              </div>

              {canWrite && (
                <div className="flex justify-end pt-2">
                  <button
                    type="submit"
                    disabled={submitting}
                    className="rounded-xl bg-blue-700 px-6 py-2 text-xs font-bold text-white hover:bg-blue-800 shadow"
                  >
                    {submitting ? "Saving..." : "Record & Sign-off Mobilisation"}
                  </button>
                </div>
              )}
            </ValidatedForm>
          </div>
        </div>
      )}

      {/* STEP 7: Audit History */}
      {activeStep === "history" && (
        <div className="space-y-4">
          <div className="bg-white p-4 rounded-xl border border-slate-200">
            <h3 className="font-extrabold text-slate-900 text-sm">
              Audit Trail & Lifecycle Change Log
            </h3>
            <p className="text-xs text-slate-500">
              Immutable event log for all manpower updates, BOQ modifications, enquiry floatations,
              quotations, work orders, and mobilisation sign-offs.
            </p>
          </div>

          <div className="space-y-3">
            {historyEvents.length === 0 ? (
              <div className="p-8 text-center text-xs text-slate-400 italic bg-white rounded-xl border">
                No recorded change events yet.
              </div>
            ) : (
              historyEvents.map((ev: HistoryEvent) => (
                <div
                  key={ev.id}
                  className="rounded-xl border border-slate-200 bg-white p-4 text-xs space-y-1 shadow-sm"
                >
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
                      <strong>Audit Reason:</strong> {ev.reason}
                    </p>
                  )}
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* Modal: Add Trade Gang */}
      {showAddTradeModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 p-4 backdrop-blur-sm">
          <div className="vtpl-modal w-full max-w-md rounded-2xl bg-white p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b pb-2">
              <h3 className="font-bold text-slate-900 text-sm">Add Trade Gang / Headcount</h3>
              <button
                onClick={() => setShowAddTradeModal(false)}
                className="text-slate-400 hover:text-slate-600"
              >
                ✕
              </button>
            </div>
            <ValidatedForm onSubmit={handleAddTrade} className="space-y-3">
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Trade Category *

                <ValidatedSelect name="trade_category"
                  value={tradeCategory}
                  onChange={(e) =>
                    setTradeCategory(
                      e.target.value as "skilled" | "semi_skilled" | "unskilled" | "highly_skilled"
                    )
                  }
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs bg-white outline-none"
                >
                  <option value="highly_skilled">Highly Skilled (Engineer / Specialist)</option>
                  <option value="skilled">Skilled (Electrician / Welder / Cable Jointer)</option>
                  <option value="semi_skilled">Semi-Skilled (Assistant / Helper)</option>
                  <option value="unskilled">Unskilled (General Site Labor)</option>
                </ValidatedSelect></ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Trade Role *
                <ValidatedInput name="trade_role"
                  required
                  value={tradeRole}
                  onChange={(e) => setTradeRole(e.target.value)}
                  placeholder="e.g. HT Cable Jointer"
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                /></ValidatedLabel>
              </div>

              <div className="grid grid-cols-2 gap-2">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Headcount *

                  <ValidatedInput name="trade_headcount"
                    type="number"
                    min="1"
                    required
                    value={tradeHeadcount}
                    onChange={(e) => setTradeHeadcount(Number(e.target.value))}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Target Date
                  <ValidatedInput name="trade_target_date"
                    type="date"
                    value={tradeTargetDate}
                    onChange={(e) => setTradeTargetDate(e.target.value)}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Work Front *
                <ValidatedInput name="trade_work_front"
                  required
                  value={tradeWorkFront}
                  onChange={(e) => setTradeWorkFront(e.target.value)}
                  placeholder="e.g. Substation Switchyard Yard A"
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                /></ValidatedLabel>
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t">
                <button
                  type="button"
                  onClick={() => setShowAddTradeModal(false)}
                  className="rounded-lg border px-3 py-1.5 text-xs font-semibold text-slate-600"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="rounded-lg bg-blue-700 px-4 py-1.5 text-xs font-bold text-white hover:bg-blue-800"
                >
                  Save Trade Gang
                </button>
              </div>
            </ValidatedForm>
          </div>
        </div>
      )}

      {/* Modal: Add Labor BOQ Item (Dual Language) */}
      {showAddBOQModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 p-4 backdrop-blur-sm">
          <div className="vtpl-modal w-full max-w-2xl rounded-2xl bg-white p-6 shadow-xl space-y-4 max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between border-b pb-2">
              <h3 className="font-bold text-slate-900 text-sm">
                Add Labor BOQ Item (English & हिन्दी Scope)
              </h3>
              <button
                onClick={() => setShowAddBOQModal(false)}
                className="text-slate-400 hover:text-slate-600"
              >
                ✕
              </button>
            </div>

            {/* Quick Presets */}
            <div className="space-y-1">
              <span className="text-[11px] font-bold text-slate-500 uppercase tracking-wider">
                Quick Industry Presets:
              </span>
              <div className="flex flex-wrap gap-1.5 pt-1">
                {BOQ_PRESETS.map((preset) => (
                  <button
                    type="button"
                    key={preset.code}
                    onClick={() => applyPreset(preset)}
                    className="rounded bg-slate-100 hover:bg-blue-50 hover:text-blue-700 px-2 py-1 text-[11px] font-medium text-slate-700 border border-slate-200"
                  >
                    + {preset.code}
                  </button>
                ))}
              </div>
            </div>

            <ValidatedForm onSubmit={handleAddBOQItem} className="space-y-3 pt-2">
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Item Code *
                  <ValidatedInput name="boq_code"
                    required
                    value={boqCode}
                    onChange={(e) => setBoqCode(e.target.value)}
                    placeholder="LBR-ELEC-01"
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Work Type
                  <ValidatedSelect name="boq_type"
                    value={boqType}
                    onChange={(e) =>
                      setBoqType(e.target.value as "electrical" | "civil" | "mechanical" | "solar")
                    }
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs bg-white outline-none"
                  >
                    <option value="electrical">Electrical</option>
                    <option value="solar">Solar</option>
                    <option value="civil">Civil</option>
                    <option value="mechanical">Mechanical</option>
                  </ValidatedSelect></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Drawing Ref
                  <ValidatedInput name="boq_drawing"
                    value={boqDrawing}
                    onChange={(e) => setBoqDrawing(e.target.value)}
                    placeholder="DWG-VTPL-E-01"
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Scope of Work (English) *

                <ValidatedTextarea name="boq_en"
                  required
                  rows={2}
                  value={boqEn}
                  onChange={(e) => setBoqEn(e.target.value)}
                  placeholder="Detailed description in English..."
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                /></ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  कार्य का विस्तृत विवरण (हिन्दी / Hindi) *

                <ValidatedTextarea name="boq_hi"
                  required
                  rows={2}
                  value={boqHi}
                  onChange={(e) => setBoqHi(e.target.value)}
                  placeholder="कार्य का विस्तृत हिन्दी विवरण यहाँ लिखें..."
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none font-serif"
                /></ValidatedLabel>
              </div>

              <div className="grid grid-cols-3 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">UOM *
                  <ValidatedInput name="boq_uom"
                    required
                    value={boqUom}
                    onChange={(e) => setBoqUom(e.target.value)}
                    placeholder="Mtr / Nos / Set"
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Quantity *
                  <ValidatedInput name="boq_qty"
                    type="number"
                    min="0.1"
                    step="0.1"
                    required
                    value={boqQty}
                    onChange={(e) => setBoqQty(Number(e.target.value))}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">Est. Rate (₹)
                  <ValidatedInput name="boq_rate"
                    type="number"
                    min="0"
                    step="0.5"
                    value={boqRate}
                    onChange={(e) => setBoqRate(Number(e.target.value))}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t">
                <button
                  type="button"
                  onClick={() => setShowAddBOQModal(false)}
                  className="rounded-lg border px-3 py-1.5 text-xs font-semibold text-slate-600"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="rounded-lg bg-blue-700 px-4 py-1.5 text-xs font-bold text-white hover:bg-blue-800"
                >
                  Add to BOQ
                </button>
              </div>
            </ValidatedForm>
          </div>
        </div>
      )}

      {/* Modal: Record Quotation / Negotiation */}
      {showQuoteModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 p-4 backdrop-blur-sm">
          <div className="vtpl-modal w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl space-y-4">
            <div className="flex items-center justify-between border-b pb-2">
              <h3 className="font-bold text-slate-900 text-sm">
                Record Contractor Quotation & Negotiation
              </h3>
              <button
                onClick={() => setShowQuoteModal(false)}
                className="text-slate-400 hover:text-slate-600"
              >
                ✕
              </button>
            </div>

            <ValidatedForm onSubmit={handleAddQuotation} className="space-y-3">
              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Contractor *

                <ValidatedSelect name="quote_contractor_id"
                  required
                  value={quoteContractorId}
                  onChange={(e) => setQuoteContractorId(e.target.value)}
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs bg-white outline-none"
                >
                  <option value="">Select Contractor...</option>
                  {contractorMaster.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name} ({c.code})
                    </option>
                  ))}
                </ValidatedSelect></ValidatedLabel>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Quote Reference *

                  <ValidatedInput name="quote_ref"
                    required
                    value={quoteRef}
                    onChange={(e) => setQuoteRef(e.target.value)}
                    placeholder="e.g. SEW/QUO/2026/12"
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Quote Date

                  <ValidatedInput name="quote_date"
                    type="date"
                    value={quoteDate}
                    onChange={(e) => setQuoteDate(e.target.value)}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                    Total Basic Quoted (₹) *

                  <ValidatedInput name="quote_basic_total"
                    type="number"
                    min="0"
                    step="1"
                    required
                    value={quoteBasicTotal}
                    onChange={(e) => setQuoteBasicTotal(Number(e.target.value))}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs font-mono outline-none"
                  /></ValidatedLabel>
                </div>
                <div>
                  <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">GST (%)
                  <ValidatedInput name="quote_gst_pct"
                    type="number"
                    min="0"
                    max="28"
                    value={quoteGstPct}
                    onChange={(e) => setQuoteGstPct(Number(e.target.value))}
                    className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                  /></ValidatedLabel>
                </div>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Negotiated Final Amount (₹)

                <ValidatedInput name="quote_negotiated"
                  type="number"
                  min="0"
                  step="1"
                  value={quoteNegotiated}
                  onChange={(e) =>
                    setQuoteNegotiated(e.target.value === "" ? "" : Number(e.target.value))
                  }
                  placeholder="Final agreed amount after negotiations..."
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs font-mono font-bold text-emerald-800 outline-none"
                /></ValidatedLabel>
              </div>

              <div>
                <ValidatedLabel className="block text-xs font-bold text-slate-700 mb-1">
                  Remarks / Terms

                <ValidatedInput name="quote_remarks"
                  value={quoteRemarks}
                  onChange={(e) => setQuoteRemarks(e.target.value)}
                  placeholder="Special conditions or negotiation notes..."
                  className="w-full rounded-lg border border-slate-300 p-2 text-xs outline-none"
                /></ValidatedLabel>
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t">
                <button
                  type="button"
                  onClick={() => setShowQuoteModal(false)}
                  className="rounded-lg border px-3 py-1.5 text-xs font-semibold text-slate-600"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="rounded-lg bg-blue-700 px-4 py-1.5 text-xs font-bold text-white hover:bg-blue-800"
                >
                  Save Quotation
                </button>
              </div>
            </ValidatedForm>
          </div>
        </div>
      )}
    </div>
  );
}
