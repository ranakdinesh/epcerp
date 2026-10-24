// Reference metadata only. Not an operational timing or approval policy.
export const FMS_PHASES = [
  { code: "WO2I", title: "Work order → Indent", range: "01–02" },
  { code: "I2LOI", title: "Indent → LOI", range: "03–05" },
  { code: "LOI2PO", title: "LOI → Purchase order", range: "06–09" },
  { code: "PO2INS", title: "PO → Inspection", range: "10–14" },
  { code: "INS2D", title: "Inspection → Delivery", range: "15–20" },
] as const;
export const FMS_STEPS = [
  {
    id: "1",
    number: 1,
    phase: "WO2I",
    title: "Bought-out (BO) item list",
    tat: 1,
    weight: 2.5,
    evidence: "BO list and reviewed project material requirements",
    team: "Project / procurement",
  },
  {
    id: "2",
    number: 2,
    phase: "WO2I",
    title: "Purchase Indent",
    tat: 1,
    weight: 2.5,
    evidence: "Full-kit indent, delivery allocations and budget review",
    team: "Procurement",
  },
  {
    id: "3",
    number: 3,
    phase: "I2LOI",
    title: "Invite Quotations from Vendors",
    tat: 3,
    weight: 3.0,
    evidence: "Eligible supplier list and issued RFQ reference",
    team: "Procurement",
  },
  {
    id: "4",
    number: 4,
    phase: "I2LOI",
    title: "Price Comparative Table",
    tat: 2,
    weight: 1.0,
    evidence: "Validated quotations, comparison and selection reason",
    team: "Procurement / approver",
  },
  {
    id: "5",
    number: 5,
    phase: "I2LOI",
    title: "Vendor Onboarding + LOI Release",
    tat: 1,
    weight: 2.0,
    evidence: "Supplier onboarding and approved LOI",
    team: "Procurement / approver",
  },
  {
    id: "6",
    number: 6,
    phase: "LOI2PO",
    title: "Technical Documents from Vendor",
    tat: 3,
    weight: 2.5,
    evidence: "GA drawing, TDS, QAP and applicable test documents",
    team: "Vendor / procurement",
  },
  {
    id: "7",
    number: 7,
    phase: "LOI2PO",
    title: "Docs Submit to Project Team",
    tat: 2,
    weight: 1.5,
    evidence: "Document checking and project-team submission reference",
    team: "Project team",
  },
  {
    id: "8",
    number: 8,
    phase: "LOI2PO",
    title: "Stamp Sign By Client Approval",
    tat: 5,
    weight: 5.0,
    evidence: "Client stamped approval and approved QAP",
    team: "Client / project team",
  },
  {
    id: "9",
    number: 9,
    phase: "LOI2PO",
    title: "PO Release",
    tat: 2,
    weight: 2.0,
    evidence:
      "Management approval, full-kit PO and approved technical revision",
    team: "Management / procurement",
  },
  {
    id: "10",
    number: 10,
    phase: "PO2INS",
    title: "PO Acceptance",
    tat: 1,
    weight: 1.0,
    evidence: "Supplier acknowledgement of PO terms",
    team: "Vendor / procurement",
  },
  {
    id: "11",
    number: 11,
    phase: "PO2INS",
    title: "Advance Payment Request",
    tat: 2,
    weight: 2.0,
    evidence: "Advance request to Accounts, or reason not applicable",
    team: "Accounts / procurement",
  },
  {
    id: "12",
    number: 12,
    phase: "PO2INS",
    title: "Material Inspection",
    tat: 10,
    weight: 24.0,
    evidence: "Inspection report with findings and acceptance decision",
    team: "Quality / inspector",
  },
  {
    id: "13",
    number: 13,
    phase: "PO2INS",
    title: "IR / IRN",
    tat: 2,
    weight: 2.0,
    evidence: "Inspection report and inspection release note",
    team: "Quality",
  },
  {
    id: "14",
    number: 14,
    phase: "PO2INS",
    title: "Quality Certificates",
    tat: 1,
    weight: 1.0,
    evidence: "Applicable TC / TR / MTC / RTC certificates",
    team: "Quality / vendor",
  },
  {
    id: "15",
    number: 15,
    phase: "INS2D",
    title: "Dispatch Clearance (Client)",
    tat: 1,
    weight: 1.0,
    evidence: "Client dispatch clearance",
    team: "Client / project team",
  },
  {
    id: "16",
    number: 16,
    phase: "INS2D",
    title: "Material Dispatch",
    tat: 2,
    weight: 1.0,
    evidence: "LR, invoice, e-way bill and dispatch reference",
    team: "Vendor / logistics",
  },
  {
    id: "17",
    number: 17,
    phase: "INS2D",
    title: "Received at Site",
    tat: 5,
    weight: 5.0,
    evidence: "Receipt, quantity/condition record and POD / gate entry",
    team: "Site stores",
  },
  {
    id: "18",
    number: 18,
    phase: "INS2D",
    title: "Warranty / Guarantee Letter",
    tat: 1,
    weight: 1.0,
    evidence: "Vendor warranty / guarantee letter",
    team: "Vendor / procurement",
  },
  {
    id: "19",
    number: 19,
    phase: "INS2D",
    title: "PE Site Confirmation",
    tat: 1,
    weight: 0.5,
    evidence: "Project engineer site-inspection confirmation",
    team: "Project engineer",
  },
  {
    id: "20",
    number: 20,
    phase: "INS2D",
    title: "Accounts Acceptance Confirmation",
    tat: 1,
    weight: 0.5,
    evidence: "Accounts acknowledgement of material acceptance",
    team: "Accounts",
  },
] as const;
export type StepState = "pending" | "done" | "not_applicable";
export type ExampleStep = {
  number: number;
  state: StepState;
  planned: string;
  actual: string;
  evidence: string;
  note: string;
};
export type ExamplePackage = {
  id: string;
  item: string;
  vendor: string;
  owner: string;
  quantity: string;
  blocked: string;
  steps: ExampleStep[];
  history: string[];
};
export const EXAMPLE_TODAY = "2026-09-18";
export function currentStep(pkg: ExamplePackage) {
  return pkg.steps.find((s) => s.state === "pending");
}
export function packageStatus(pkg: ExamplePackage) {
  const current = currentStep(pkg);
  if (!current) return "Completed";
  if (pkg.blocked) return "Blocked";
  return current.planned < EXAMPLE_TODAY ? "Overdue" : "On track";
}
export function examplePackages(): ExamplePackage[] {
  return [
    {
      id: "DEMO-001",
      item: "LED floodlights · 200 W",
      vendor: "Example Lighting Supplier",
      owner: "Example procurement executive",
      quantity: "48 nos",
      done: 3,
      planned: "2026-09-16",
      blocked: "",
    },
    {
      id: "DEMO-002",
      item: "HT panel · 11 kV",
      vendor: "Example Panel Supplier",
      owner: "Example project engineer",
      quantity: "2 sets",
      done: 7,
      planned: "2026-09-17",
      blocked: "Revised QAP awaited from supplier",
    },
    {
      id: "DEMO-003",
      item: "LT power cable · 4C × 120 mm²",
      vendor: "Example Cable Supplier",
      owner: "Example quality engineer",
      quantity: "600 m",
      done: 11,
      planned: "2026-09-21",
      blocked: "",
    },
    {
      id: "DEMO-004",
      item: "Earthing electrodes",
      vendor: "Example Earthing Supplier",
      owner: "Example site engineer",
      quantity: "24 nos",
      done: 16,
      planned: "2026-09-18",
      blocked: "",
    },
    {
      id: "DEMO-005",
      item: "Cable trays · 300 mm",
      vendor: "Example Fabrication Supplier",
      owner: "Example accounts executive",
      quantity: "150 m",
      done: 20,
      planned: "2026-09-18",
      blocked: "",
    },
  ].map(({ done, planned, ...pkg }) => ({
    ...pkg,
    steps: FMS_STEPS.map((step) => ({
      number: step.number,
      state: step.number <= done ? "done" : "pending",
      planned:
        step.number <= done
          ? "2026-09-10"
          : step.number === done + 1
            ? planned
            : "2026-09-30",
      actual: step.number <= done ? "2026-09-10" : "",
      evidence: step.number <= done ? `Example evidence S${step.number}` : "",
      note: "",
    })),
    history: ["Fictional example loaded. No operational records are linked."],
  }));
}
export type ExampleAction = "complete" | "skip" | "block" | "resume";
export function updateExample(
  pkg: ExamplePackage,
  number: number,
  action: ExampleAction,
  evidence: string,
  note: string,
): ExamplePackage {
  const step = currentStep(pkg);
  if (!step || step.number !== number)
    throw new Error(
      "Update the current step first. Earlier steps must be resolved.",
    );
  if (action === "resume") {
    if (!pkg.blocked) throw new Error("This package is not blocked.");
    if (!note.trim()) throw new Error("Explain how the blocker was resolved.");
  } else if (pkg.blocked)
    throw new Error("Resolve the blocker before updating the step.");
  if ((action === "block" || action === "skip") && !note.trim())
    throw new Error("Enter a reason before continuing.");
  if (action === "skip" && number !== 11)
    throw new Error(
      "Only the advance payment example supports not applicable.",
    );
  if (action === "complete" && !evidence.trim())
    throw new Error(
      "Enter an example evidence reference before completing this step.",
    );
  const title = FMS_STEPS[number - 1].title;
  return {
    ...pkg,
    blocked: action === "block" ? note.trim() : "",
    steps: pkg.steps.map((s) =>
      s.number === number && (action === "complete" || action === "skip")
        ? {
            ...s,
            state: action === "skip" ? "not_applicable" : "done",
            actual: EXAMPLE_TODAY,
            evidence: evidence.trim(),
            note: note.trim(),
          }
        : s,
    ),
    history: [
      `${EXAMPLE_TODAY} · Preview ${action} · S${number} ${title}${note.trim() ? ` · ${note.trim()}` : ""}`,
      ...pkg.history,
    ],
  };
}
