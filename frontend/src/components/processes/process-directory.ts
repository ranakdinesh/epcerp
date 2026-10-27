export type ProcessStep = { code: string; group: string; number: number; label: string; available: boolean; purpose?: string; evidence?: readonly string[]; team?: string; sourceNumber?: number };
export type ProcessDefinition = { screen: string; label: string; steps: readonly ProcessStep[] };

// Tender FMS reference labels, not persisted task state or approval rules.
// Only an implemented, authorized Go workspace may become an available step.
export const tenderProcess: ProcessDefinition = {
 screen: "tender-fms", label: "Tender FMS", steps: [
  { code: "TS-01", group: "Tender Selection", number: 1, label: "Read documents", available: true },
  { code: "TS-TSC", group: "Tender Selection", number: 2, label: "Generate TSC", available: true },
  { code: "TS-02", group: "Tender Selection", number: 3, label: "Approval for Bidding the Tender", available: false },
  { code: "LG-01", group: "Lead Generation", number: 4, label: "Share tender and basic details", available: false },
  { code: "LG-02", group: "Lead Generation", number: 5, label: "Approval for Bidding", available: false },
  { code: "LG-03", group: "Lead Generation", number: 6, label: "Submit Pre-Bid Query over portal/E-mail", available: false },
  { code: "LG-04", group: "Lead Generation", number: 7, label: "Pre-bid query approved or rejected", available: false },
  { code: "CV-01", group: "Costing and Vendor Work", number: 8, label: "Read & understand tender document", available: false },
  { code: "CV-02", group: "Costing and Vendor Work", number: 9, label: "Float Inquiries to vendors as per Make list", available: false },
  { code: "CV-03", group: "Costing and Vendor Work", number: 10, label: "Receive Quotation from vendors", available: false },
  { code: "CV-04", group: "Costing and Vendor Work", number: 11, label: "Check Quotation for correctness & completeness", available: false },
  { code: "CV-05", group: "Costing and Vendor Work", number: 12, label: "Negotiate with vendors for L-1 Price", available: false },
  { code: "CV-06", group: "Costing and Vendor Work", number: 13, label: "Submit Technical Documents for Technical Bid preparation", available: false },
  { code: "CV-07", group: "Costing and Vendor Work", number: 14, label: "Prepare final working BOQ / costing sheet", available: false },
  { code: "CV-08", group: "Costing and Vendor Work", number: 15, label: "Submission of tender on online portal", available: false },
  { code: "TB-01", group: "Technical Bid Preparation", number: 16, label: "Prepare print-out file of tender documentation", available: false },
  { code: "TB-02", group: "Technical Bid Preparation", number: 17, label: "Read and understand tender in totality", available: false },
  { code: "TB-03", group: "Technical Bid Preparation", number: 18, label: "Review Bid cover", available: false },
  { code: "TB-04", group: "Technical Bid Preparation", number: 19, label: "Raise request to Accounts department", available: false },
  { code: "TB-05", group: "Technical Bid Preparation", number: 20, label: "Raise request to Operations executive for vendor documents", available: false },
  { code: "TB-06", group: "Technical Bid Preparation", number: 21, label: "Prepare Technical Bid", available: false },
  { code: "TB-07", group: "Technical Bid Preparation", number: 22, label: "Courier / Speed post Hard copy document", available: false },
  { code: "TB-08", group: "Technical Bid Preparation", number: 23, label: "Submit Technical Bid to Operations Executive", available: false },
  { code: "TBQ-01", group: "Tender Bid Query", number: 24, label: "Prepare query reply letter and supporting documents", available: false },
  { code: "TBQ-02", group: "Tender Bid Query", number: 25, label: "Mail to Procurement Executive for checking", available: false },
  { code: "TBQ-03", group: "Tender Bid Query", number: 26, label: "Forward for submission", available: false },
  { code: "TBQ-04", group: "Tender Bid Query", number: 27, label: "Check & Submission", available: false },
 ]
};
export const projectProcess: ProcessDefinition = {
 screen: "projects", label: "Projects", steps: [
  {code:"register",group:"Project workspace",number:0,label:"Project register & ownership",available:true},
  {"code": "order_receipt", "group": "Order, survey & design", "number": 1, "label": "Project order receipt", "available": false, "purpose": "Establish the awarded order and contractual starting point.", "evidence": ["Order / LOI / PO reference", "Contract scope and evidence", "Order acceptance and O2TE review"]},
  {"code": "site_survey", "group": "Order, survey & design", "number": 2, "label": "Basic site survey", "available": false, "purpose": "Record actual site conditions and constraints before design.", "evidence": ["Site survey record", "Measurements and site photographs", "Access and infrastructure observations"]},
  {"code": "risk_assessment", "group": "Order, survey & design", "number": 3, "label": "Risk assessment", "available": false, "purpose": "Identify delivery risks and accountable mitigation actions.", "evidence": ["Project risk assessment", "Mitigation owners and actions", "Required specialist reviews"]},
  {"code": "design_boq_drawings", "group": "Order, survey & design", "number": 4, "label": "Design, BOQ, and drawings", "available": false, "purpose": "Translate project requirements into controlled design and scope.", "evidence": ["Design package", "BOQ", "Drawing references and revisions"]},
  {"code": "bom_finalisation", "group": "Order, survey & design", "number": 5, "label": "BOM finalisation", "available": false, "purpose": "Establish the material quantities and specifications required for delivery.", "evidence": ["BOM revision", "Material specifications", "Quantity reconciliation with BOQ"]},
  {"code": "quantity_item_approval", "group": "Order, survey & design", "number": 6, "label": "Quantity and item approval", "available": false, "purpose": "Record the authorized quantities and material selections.", "evidence": ["Quantity approval reference", "Item / make approval", "Approved BOQ / BOM revision"]},
  {"code": "project_planning", "group": "Planning", "number": 7, "label": "Project planning", "available": false, "purpose": "Plan work packages, dates, accountability, and the execution network.", "evidence": ["Work packages", "Activity schedule", "Reviewed process dependency map"]},
  {"code": "manpower_assessment", "group": "Contractor & site preparation", "number": 8, "label": "Manpower assessment", "available": false, "purpose": "Determine the people and skills needed for the project.", "evidence": ["Skill and headcount requirements", "Deployment timing", "Accountable project team"]},
  {"code": "contract_scope_boq", "group": "Contractor & site preparation", "number": 9, "label": "Work-contract scope and BOQ", "available": false, "purpose": "Prepare the contractor scope and BOQ in Hindi and English.", "evidence": ["Hindi scope and BOQ", "English scope and BOQ", "Work package and acceptance requirements"]},
  {"code": "contractor_enquiries", "group": "Contractor & site preparation", "number": 10, "label": "Float enquiries to contractors", "available": false, "purpose": "Invite contractor quotations against a controlled scope.", "evidence": ["Enquiry scope revision", "Invited contractor list", "Enquiry issue and response dates"]},
  {"code": "contractor_rates", "group": "Contractor & site preparation", "number": 11, "label": "Receive rates from contractors", "available": false, "purpose": "Collect comparable contractor rates and qualifications.", "evidence": ["Received quotations", "Rate comparison", "Clarifications and exclusions"]},
  {"code": "contractor_negotiation", "group": "Contractor & site preparation", "number": 12, "label": "Negotiate with contractors", "available": false, "purpose": "Document negotiated terms and the selection recommendation.", "evidence": ["Negotiation record", "Final terms and rates", "Selection / approval reference"]},
  {"code": "contractor_work_order", "group": "Contractor & site preparation", "number": 13, "label": "Issue contractor work order", "available": false, "purpose": "Issue an authorized work order to the selected contractor.", "evidence": ["Approved work order", "Scope, rates and terms", "Contractor acceptance"]},
  {"code": "manpower_mobilisation", "group": "Contractor & site preparation", "number": 14, "label": "Mobilise manpower to site", "available": false, "purpose": "Bring the required people to site with readiness documented.", "evidence": ["Mobilisation record", "Deployment and responsibility confirmation", "Required site induction / readiness evidence"]},
  {"code": "site_preparation", "group": "Contractor & site preparation", "number": 15, "label": "Pre-material-delivery site work", "available": false, "purpose": "Complete site work needed before material delivery.", "evidence": ["Site preparation record", "Civil / mechanical readiness", "Readiness inspection reference"]},
  {"code": "material_delivery", "group": "Delivery, installation & handover", "number": 16, "label": "Deliver material to site", "available": false, "purpose": "Confirm material availability and acceptance at the site.", "evidence": ["Delivery and receipt references", "Quantity / condition verification", "Shortage or rejection records"]},
  {"code": "material_installation", "group": "Delivery, installation & handover", "number": 17, "label": "Install material", "available": false, "purpose": "Execute installation against the approved scope and drawings.", "evidence": ["Installation work records", "Measurements and progress evidence", "Inspection references"]},
  {"code": "commissioning", "group": "Delivery, installation & handover", "number": 18, "label": "Commission the system", "available": false, "purpose": "Document testing and commissioning against applicable requirements.", "evidence": ["Test / commissioning results", "Inspection and acceptance references", "Outstanding defects / punch-list resolution"]},
  {"code": "project_completion", "group": "Delivery, installation & handover", "number": 19, "label": "Complete the project", "available": false, "purpose": "Record completion and the handover evidence required by the project.", "evidence": ["Completion / acceptance reference", "Handover and as-built package", "Warranty / support handoff"]},
 ]
};
export const purchaseProcess: ProcessDefinition = {
 screen:"purchase-fms", label:"Purchase FMS", steps:[
  {"code": "PUR-01", "group": "WO2I \u00b7 Work order to indent", "number": 1, "label": "Items to purchase", "available": true, "evidence": ["Reviewed project purchase items and material requirements"], "team": "Project / procurement"},
  {"code": "PUR-02", "group": "WO2I \u00b7 Work order to indent", "number": 2, "label": "Purchase Indent", "available": true, "evidence": ["Full-kit indent, delivery allocations and budget review"], "team": "Procurement"},
  {"code": "PUR-03", "group": "I2LOI \u00b7 Indent to LOI", "number": 3, "label": "Receive vendor quotations", "available": true, "evidence": ["Eligible supplier list and issued RFQ reference"], "team": "Procurement"},
  {"code": "PUR-04", "group": "I2LOI \u00b7 Indent to LOI", "number": 4, "label": "Price Comparative Table", "available": true, "evidence": ["Validated quotations, comparison and selection reason"], "team": "Procurement / approver"},
  {"code": "PUR-05", "group": "I2LOI \u00b7 Indent to LOI", "number": 5, "label": "Vendor award & LOI", "available": true, "evidence": ["Existing approved vendor selection and released LOI"], "team": "Procurement / approver"},
  {"code": "PUR-06", "group": "LOI2PO \u00b7 LOI to purchase order", "number": 6, "label": "Technical Documents from Vendor", "available": true, "evidence": ["GA drawing, TDS, QAP and applicable test documents"], "team": "Vendor / procurement"},
  {"code": "PUR-07", "group": "LOI2PO \u00b7 LOI to purchase order", "number": 7, "label": "Technical review & submission", "available": true, "evidence": ["Document checking and project-team submission reference"], "team": "Project team"},
  {"code": "PUR-08", "group": "LOI2PO \u00b7 LOI to purchase order", "number": 8, "label": "Client technical approval", "available": true, "evidence": ["Client stamped approval and approved QAP"], "team": "Client / project team"},
  {code:"PUR-PO-CLIENT",group:"LOI2PO · LOI to purchase order",number:9,sourceNumber:21,label:"Draft PO & client stamp/sign",available:true,evidence:["Exact draft PO submission and client signed approval"],team:"Client / procurement"},
  {"code": "PUR-09", "group": "LOI2PO \u00b7 LOI to purchase order", "number": 10, "sourceNumber": 9, "label": "PO Release", "available": true, "evidence": ["Finance approval, approved technical revision and client-signed current PO"], "team": "Management / procurement"},
  {"code": "PUR-10", "group": "PO2INS \u00b7 PO to inspection", "number": 11, "sourceNumber": 10, "label": "PO Acceptance", "available": true, "evidence": ["Supplier acknowledgement of PO terms"], "team": "Vendor / procurement"},
  {"code": "PUR-11", "group": "PO2INS \u00b7 PO to inspection", "number": 12, "sourceNumber": 11, "label": "Advance Payment Request", "available": true, "evidence": ["Advance request to Accounts, or reason not applicable"], "team": "Accounts / procurement"},
  {"code": "PUR-12", "group": "PO2INS \u00b7 PO to inspection", "number": 13, "sourceNumber": 12, "label": "Material Inspection", "available": true, "evidence": ["Inspection report with findings and acceptance decision"], "team": "Quality / inspector"},
  {"code": "PUR-13", "group": "PO2INS \u00b7 PO to inspection", "number": 14, "sourceNumber": 13, "label": "IR / IRN", "available": true, "evidence": ["Inspection report and inspection release note"], "team": "Quality"},
  {"code": "PUR-14", "group": "PO2INS \u00b7 PO to inspection", "number": 15, "sourceNumber": 14, "label": "Quality Certificates", "available": true, "evidence": ["Required TC / TR / MTC / RTC / Warranty certificates"], "team": "Quality / vendor"},
  {"code": "PUR-15", "group": "INS2D \u00b7 Inspection to delivery", "number": 16, "sourceNumber": 15, "label": "Dispatch Clearance (Client)", "available": true, "evidence": ["Client dispatch clearance"], "team": "Client / project team"},
  {"code": "PUR-16", "group": "INS2D \u00b7 Inspection to delivery", "number": 17, "sourceNumber": 16, "label": "Material Dispatch", "available": true, "evidence": ["LR, invoice, e-way bill and dispatch reference"], "team": "Vendor / logistics"},
  {"code": "PUR-17", "group": "INS2D \u00b7 Inspection to delivery", "number": 18, "sourceNumber": 17, "label": "Received at Site", "available": true, "evidence": ["Receipt, quantity/condition record and POD / gate entry"], "team": "Site stores"},
  {"code": "PUR-18", "group": "INS2D \u00b7 Inspection to delivery", "number": 19, "sourceNumber": 18, "label": "GRN Inwarding & Warranty", "available": true, "evidence": ["Issued GRN, received/accepted/rejected quantities and optional warranty reference"], "team": "Site stores / procurement"},
  {"code": "PUR-19", "group": "INS2D \u00b7 Inspection to delivery", "number": 20, "sourceNumber": 19, "label": "PE Site Confirmation", "available": true, "evidence": ["Project engineer site-inspection confirmation"], "team": "Project engineer"},
  {"code": "PUR-20", "group": "INS2D \u00b7 Inspection to delivery", "number": 21, "sourceNumber": 20, "label": "Accounts Acceptance Confirmation", "available": true, "evidence": ["Accounts acknowledgement of material acceptance"], "team": "Accounts"},
 ]
};
export function processForScreen(screen: string): ProcessDefinition | undefined {
 return screen === tenderProcess.screen ? tenderProcess : screen === projectProcess.screen ? projectProcess : screen === purchaseProcess.screen ? purchaseProcess : undefined;
}
export function availableProcessStep(process: ProcessDefinition, requested: string | null): ProcessStep | undefined {
 return process.steps.find(step => step.available && step.code === requested) ?? process.steps.find(step => step.available);
}
export function processStepHref(process: ProcessDefinition, step: ProcessStep, projectId?: string, draftId?: string): string | undefined {
 if (!step.available || (process.screen === "purchase-fms" && !projectId)) return undefined;
 return `/dashboard?screen=${process.screen}&step=${encodeURIComponent(step.code)}${process.screen === "purchase-fms" ? `&project=${encodeURIComponent(projectId!)}` : process.screen === "tender-fms" && draftId ? `&draft=${encodeURIComponent(draftId)}` : ""}`;
}
export function filterProcessSteps(process: ProcessDefinition, query: string): readonly ProcessStep[] {
 const term = query.trim().toLowerCase();
 return term ? process.steps.filter(step => `${step.number} ${step.code} ${step.label} ${step.group}`.toLowerCase().includes(term)) : process.steps;
}

export function processMapHref(process: ProcessDefinition, projectId?: string): string {
 return `/dashboard?screen=${process.screen}&view=map${projectId ? `&project=${encodeURIComponent(projectId)}` : ""}`;
}
export const processHandoffs = [
 {from:"tender-fms",to:"projects",label:"Award → Project order and contractual evidence", detail:"Future award handoff; reading a tender does not create an execution project."},
 {from:"projects",to:"purchase-fms",label:"Approved BOM and quantities → Procurement", detail:"Procurement runs alongside contractor/site preparation, using approved requirements and required-at-site dates."},
 {from:"purchase-fms",to:"projects",label:"Receipt and site acceptance → Material delivery", detail:"Purchase receipt/acceptance evidence supports Project stage 16 before installation and commissioning."},
] as const;
