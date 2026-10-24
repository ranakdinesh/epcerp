"use client";
import {
  ValidatedLabel,
  ValidatedSelect,
} from "@/components/ui/validated-controls";

import { PurchaseIndentTerms, vendorIndentDefaults } from "./purchase-indent-terms";
import { VendorIndentRegister, type DocumentWorkspace, type DocumentCommand } from "./purchase-indent-documents";
import { PurchaseIndentGroups, type GroupWorkspace, type GroupCommand } from "./purchase-indent-groups";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { displayServerErrors } from "@/components/ui/server-field-errors";

import {useRouter,useSearchParams} from "next/navigation";
import { useEffect, useState } from "react";
import { useProjectUser } from "@/components/projects/project-permissions";
import { permitted, canOpen, indentTabs } from "@/components/projects/workspace-permissions";

export type Supplier = {
  id: string;
  code: string;
  name: string;
  is_active: boolean;
  data: {
    contact_person_name?: string;
    email?: string;
    phone?: string;
    location?: string;
    gst_number?: string;
    pan_number?: string;
    payment_terms?: {
      advance_pct?: number;
      qc_pct?: number;
      delivery_pct?: number;
      retention_pct?: number;
      credit_days?: number;
      description?: string;
    };
    indent_notes?: string;
    indent_terms?: string;
    delivery_terms?: string;
    category_ids?: string[];
    item_ids?: string[];
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
  demand_id?: string | null;
};

export function PurchaseIndentWorkspace({
  project,
  projectName,
  lines,
  boId,
  boVersion = 0,
  onBackToBO,
  onProceedToSourcing,
}: {
  project: string;
  projectName?: string;
  lines: BOLine[];
  boId?: string;
  boVersion?: number;
  onBackToBO: () => void;
  onProceedToSourcing?: () => void;
}) {
  const router=useRouter(),params=useSearchParams();
  const user = useProjectUser();
  const [suppliers, setSuppliers] = useState<Supplier[]>([]);
  const [loading, setLoading] = useState(true);
  const allowedTabs = indentTabs(user.permissions);
  const [activeTab, setActiveTab] = useState<"assignment" | "vendors" | "report" | "preview" | "document">(
    params.get("indentTab")==="vendors"&&allowedTabs.includes("vendors")?"vendors":allowedTabs[0] ?? "assignment",
  );
  const [selectedSupplierId, setSelectedSupplierId] = useState<string>("");
  const [workspace, setWorkspace] = useState<GroupWorkspace | null>(null);
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [selectedGroupId, setSelectedGroupId] = useState("");
  const [isDocumentOpen, setIsDocumentOpen] = useState(false);
  const [documentOpener, setDocumentOpener] = useState<HTMLElement | null>(null);
  const [printNotice, setPrintNotice] = useState("");
  const [documents, setDocuments] = useState<DocumentWorkspace|null>(null);
  const canGenerate = permitted(user.permissions,"projects.purchase_indent_documents.generate");
  const canPrint = permitted(user.permissions,"projects.purchase_indent_documents.print");
  const canDispatch = permitted(user.permissions,"projects.purchase_indent_documents.dispatch");
  const canReadDocuments = allowedTabs.includes("preview") || allowedTabs.includes("document");
  const groupUrl = boId ? `/api/projects/${project}/purchase-bo/${boId}/indent-groups` : "";
  const documentUrl = boId ? `/api/projects/${project}/purchase-bo/${boId}/indent-documents` : "";
  useEffect(() => {
    let live = true;
    async function load() {
      setLoading(true);
      try {
        if (!groupUrl) throw new Error("Save a BO list before grouping its items.");
        const res = await fetch(groupUrl);
        const data = await res.json();
        if (!res.ok) throw new Error(typeof data.detail === "string" ? data.detail : "Unable to load item groups.");
        if (live) { setWorkspace(data); setSuppliers(data.vendors); setNotice(""); }
        if (!canReadDocuments) { if(live)setDocuments(null); return; }
        const response=await fetch(documentUrl);
        const documentData=await response.json();
        if(!response.ok)throw new Error(typeof documentData.detail === "string" ? documentData.detail : "Unable to load vendor indent logs.");
        if(live)setDocuments(documentData);
      } catch (error) {
        if (live) setNotice(error instanceof Error ? error.message : "Unable to load item groups.");
      } finally { if (live) setLoading(false); }
    }
    void load();
    return () => { live = false; };
  }, [groupUrl, documentUrl, boVersion, canReadDocuments]);

  async function saveGroups(command: GroupCommand, form?: HTMLFormElement) {
    setBusy(true); setNotice("");
    try {
      const response = await fetch(groupUrl, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ ...command, groups: command.groups?.map(({ id, name, line_ids, vendor_ids }) => ({ id, name, line_ids, vendor_ids })), version: workspace?.record?.version ?? 0, bo_version: boVersion }) });
      const data = await response.json();
      if (!response.ok) {
        if (form && displayServerErrors(form, data)) return false;
        throw new Error(typeof data.detail === "string" ? data.detail : "Unable to save item groups.");
      }
      setWorkspace(data); setSuppliers(data.vendors);
      window.dispatchEvent(new CustomEvent("purchase-progress-saved", {detail:project}));
      const saved = data as GroupWorkspace;
      const savedGroups = saved.record?.groups ?? [];
      const assigned = new Set(savedGroups.flatMap(group => group.line_ids));
      const allGrouped = validLines.length > 0 && validLines.every(line => assigned.has(line.id));
      const allVendorsReady = savedGroups.length > 0 && savedGroups.every(group =>
        group.vendor_ids.filter(id => saved.vendors.some(vendor => vendor.id === id && vendor.is_active)).length >= 3);
      if ((command.action === "create_group" || command.action === "add_items") && allGrouped && allowedTabs.includes("vendors")) setActiveTab("vendors");
      if ((command.action === "set_vendors" || command.action === "add_vendors") && allVendorsReady && allowedTabs.includes("preview")) setActiveTab("preview");
      if (command.action === "reset_item") setActiveTab("assignment");
      setSelectedDocumentId("");
      setDocuments(previous=>previous?{...previous,documents:previous.documents.map(d=>({...d,data:{...d.data,is_current:(d.data.scope_revision??1)===(data.record?.groups.find((g:{id:string;scope_revision?:number})=>g.id===d.group_id)?.scope_revision??1)}}))}:null);
      return true;
    } catch (error) { setNotice(error instanceof Error ? error.message : "Unable to save item groups."); return false; }
    finally { setBusy(false); }
  }
  async function documentAction(command:DocumentCommand, form?:HTMLFormElement) {
    if (!(command.action === "generate" ? canGenerate : command.action === "print" ? canPrint : canDispatch)) return false;
    setBusy(true);setNotice("");
    try {
      const response=await fetch(documentUrl,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({...command,group_version:workspace?.record?.version??0,bo_version:boVersion})});
      const data=await response.json();
      if(!response.ok){if(form&&displayServerErrors(form,data))return false;throw new Error(typeof data.detail==="string"?data.detail:"Unable to save indent log.");}
      setDocuments(data);
      window.dispatchEvent(new CustomEvent("purchase-progress-saved", {detail:project}));
      const updated=await fetch(groupUrl);const groupsData=await updated.json();
      if(!updated.ok)throw new Error("Indent saved. Refresh to reload the updated group report.");
      setWorkspace(groupsData);setSuppliers(groupsData.vendors);
      if(command.action==="generate"){
        try {
          const summary=await fetch(`/api/purchase-fms/projects/${encodeURIComponent(project)}`,{cache:"no-store"});
          if(summary.ok){const saved=await summary.json();if(saved.project.completed_steps?.includes(2)){setIsDocumentOpen(false);router.push(`/dashboard?screen=purchase-fms&project=${encodeURIComponent(project)}&step=PUR-03`);}}
        } catch {setNotice("Indent saved. Refresh to check stage completion.");}
      }

      window.dispatchEvent(new Event("indent-dispatch-changed"));
      return true;
    } catch(error){setNotice(error instanceof Error?error.message:"Unable to save indent log.");return false;}
    finally{setBusy(false);}
  }
  const created = workspace?.record?.state === "created";
  const frozenLines = created ? workspace?.record?.source_lines ?? [] : [];
  const frozenIds = new Set(frozenLines.map(l=>l.id));
  const validLines = [...frozenLines, ...lines.filter(l=>!frozenIds.has(l.id))].filter(l => l.item_id || l.item_snapshot?.description || l.quantity);
  const groups = workspace?.record?.groups ?? [];
  const ready = groups.length>0 && groups.every(g=>g.vendor_ids.filter(id=>suppliers.some(v=>v.id===id&&v.is_active)).length>=3);
  const currentGroup = groups.find(g => g.id === selectedGroupId) ?? groups[0];
  const assignedSuppliers = suppliers.filter(s => currentGroup?.vendor_ids.includes(s.id));
  const selectedVendor = assignedSuppliers.find(s => s.id === selectedSupplierId) ?? assignedSuppliers[0];
  const [selectedDocumentId,setSelectedDocumentId]=useState("");
  const currentDocument = documents?.documents.find(d=>d.group_id===currentGroup?.id && d.vendor_id===selectedVendor?.id && (selectedDocumentId ? d.id===selectedDocumentId : (d.data.scope_revision??1)===(currentGroup?.scope_revision??1)));
  const vendorDocuments = documents?.documents.filter(d=>d.group_id===currentGroup?.id && d.vendor_id===selectedVendor?.id)??[];
  const currentSupplier = currentDocument?.data.vendor ?? selectedVendor;
  const supplierLines = currentDocument?.data.lines ?? (currentSupplier ? validLines.filter(l => currentGroup?.line_ids.includes(l.id)) : []);
  const indentDate = currentDocument ? new Date(currentDocument.data.generated_at).toLocaleDateString("en-GB",{timeZone:"Asia/Kolkata"}) : "Not generated";
  const itemNumber = currentDocument?.data.group_name ?? currentGroup?.name ?? "";

  async function handlePrintPDF() {
    setPrintNotice("");
    if (currentDocument?.data.terms && await documentAction({action:"print", document_id:currentDocument.id, version:currentDocument.version})) {
      setPrintNotice("Print requested. Finish printing or saving the PDF, then close this popup. Record the actual sent date in the register after dispatch.");
      try { window.print(); } catch { setPrintNotice("The print request was saved, but the print dialog could not open. Try Print / save PDF again."); }
    }
  }

  function closeDocument() {
    if (busy) return;
    setIsDocumentOpen(false);
    setPrintNotice("");
  }

  const indentDocument = (
            <div
              id="purchase-indent-document"
              className="bg-white p-10 rounded-xl shadow-lg border border-slate-300 text-slate-900 font-serif"
              style={{
                minHeight: "1050px",
                fontFamily: '"Times New Roman", Times, Georgia, serif',
              }}
            >
              {/* Header: Company Logo & Tagline */}
              <div className="flex items-start justify-between border-b-2 border-slate-800 pb-4">
                <div className="flex items-center gap-4">
                  {/* Stylized Logo mark */}
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
                    Date: <span className="font-semibold">{indentDate}</span>
                  </p>
                </div>
              </div>

              {/* Title & Numbers */}
              <div className="text-center my-6">
                <h2 className="text-xl font-black uppercase tracking-wider text-slate-900 underline underline-offset-4 font-sans">
                  Purchase Indent
                </h2>
              </div>

              <div className="flex justify-between text-xs font-sans font-bold text-slate-800 mb-4 px-1">
                <div>
                  <p>
                    PI No:{" "}
                    <span className="font-normal font-mono">{currentDocument?.data.number || "Draft — not generated"} / {currentGroup?.name}</span>
                  </p>
                  <p>
                    Item No:{" "}
                    <span className="font-normal font-mono">{itemNumber}</span>
                  </p>
                </div>
                <div className="text-right">
                  <p>
                    Vendor:{" "}
                    <span className="font-bold text-blue-900">
                      {currentSupplier?.name || "Selected Vendor"}
                    </span>
                  </p>
                  {currentSupplier?.data?.contact_person_name && (
                    <p className="font-normal text-slate-600">
                      Attn: {currentSupplier.data.contact_person_name}
                    </p>
                  )}
                  {currentSupplier?.data?.phone && (
                    <p className="font-normal text-slate-600">
                      Ph: {currentSupplier.data.phone}
                    </p>
                  )}
                </div>
              </div>

              {/* Project Name Strip */}
              <div className="border border-slate-800 p-2 text-xs font-sans font-bold bg-slate-50 mb-6">
                Name of Project:{" "}
                <span className="font-normal uppercase tracking-wide">
                  {currentDocument?.data.project_name || projectName || project}
                </span>
              </div>

              {/* Table of Items */}
              <table className="vtpl-data-table w-full border-collapse border border-slate-800 text-xs font-sans mb-8">
                <thead>
                  <tr className="bg-slate-100 border-b border-slate-800 font-bold text-center">
                    <th className="border border-slate-800 p-2 w-12">S. No.</th>
                    <th className="border border-slate-800 p-2 text-left">
                      Description
                    </th>
                    <th className="border border-slate-800 p-2 w-16">Qty.</th>
                    <th className="border border-slate-800 p-2 w-16">Unit</th>
                  </tr>
                </thead>
                <tbody>
                  {supplierLines.length > 0 ? (
                    supplierLines.map((line, idx) => (
                      <tr key={line.id} className="border-b border-slate-800">
                        <td className="border border-slate-800 p-2 text-center align-top font-bold">
                          {idx + 1}
                        </td>
                        <td className="border border-slate-800 p-2 align-top text-justify leading-relaxed">
                          <p className="font-semibold text-slate-900">
                            {line.item_snapshot?.description ||
                              "Detailed specification as per project contract"}
                          </p>
                          {line.notes && (
                            <p className="mt-1 text-[11px] text-slate-700 italic">
                              Scope / Drawing Ref: {line.notes}
                            </p>
                          )}
                        </td>
                        <td className="border border-slate-800 p-2 text-center align-top font-bold">
                          {line.quantity || "1"}
                        </td>
                        <td className="border border-slate-800 p-2 text-center align-top">
                          {line.item_snapshot?.uom || "Each"}
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td
                        colSpan={4}
                        className="p-4 text-center text-slate-500 italic"
                      >
                        No items assigned to this vendor yet. Please return to
                        &apos;Items &amp; Vendor Mapping&apos; to assign items.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>

              <PurchaseIndentTerms terms={currentDocument ? currentDocument.data.terms : vendorIndentDefaults(currentSupplier)} supplier={currentSupplier}/>

              {/* Sign-off Blocks */}
              <div className="pt-8 border-t border-slate-400 grid grid-cols-3 text-center text-xs font-sans font-bold text-slate-800">
                <div>
                  <p className="mb-8">Indent Raised by</p>
                  <p className="font-semibold text-slate-600">
                    {currentDocument?.data.generated_by_name || user.username}
                  </p>
                </div>
                <div>
                  <p className="mb-8">Checked by</p>
                  <p className="font-semibold text-slate-400">
                    ___________________
                  </p>
                </div>
                <div>
                  <p className="mb-8">Approved by</p>
                  <p className="font-semibold text-slate-400">
                    ___________________
                  </p>
                </div>
              </div>
            </div>
  );

  return (
    <div className="bo-stage flex flex-col flex-1 min-w-0 bg-slate-100 text-slate-900 overflow-hidden">
      {/* Print stylesheet to isolate the document exactly */}
      <style>{`
        @media print {
          body * {
            visibility: hidden !important;
          }
          #purchase-indent-document,
          #purchase-indent-document * {
            visibility: visible !important;
          }
          #purchase-indent-document {
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
            font-size: 10pt !important;
          }
          .no-print {
            display: none !important;
          }
          dialog:has(#purchase-indent-document) {
            position: absolute !important;
            inset: 0 !important;
            width: 100% !important;
            max-width: none !important;
            height: auto !important;
            max-height: none !important;
            overflow: visible !important;
            margin: 0 !important;
            padding: 0 !important;
            background: white !important;
          }
          dialog:has(#purchase-indent-document)::backdrop { display: none !important; }
          dialog:has(#purchase-indent-document) > div { overflow: visible !important; padding: 0 !important; }
          dialog:has(#purchase-indent-document) > div:first-child,
          dialog:has(#purchase-indent-document) > div:last-child { display: none !important; }
          @page {
            size: A4 portrait;
            margin: 10mm;
          }
        }
      `}</style>

      {/* Top Workspace Toolbar - Compact & Clean */}
      <header className="no-print bo-stage-toolbar">
        <div className="flex items-center gap-3">
          {/* Mode toggle */}
          <div className="bo-segmented flex-wrap" role="navigation" aria-label="Purchase indent steps">
            {([ ["assignment","Items & groups"], ["vendors","Group vendors"], ["report","Item vendor report"], ["preview","Vendor indent register"], ["document","Indent document"] ] as const).filter(([tab])=>allowedTabs.includes(tab)).map(([tab,label])=><button key={tab} type="button" aria-pressed={activeTab===tab} disabled={loading || busy || (tab!=="assignment" && !groups.length)} onClick={event=>{if(tab === "document"){setDocumentOpener(event.currentTarget);setPrintNotice("");setIsDocumentOpen(true);}else setActiveTab(tab);}} className={`px-3 py-1.5 text-xs font-bold rounded-md transition-all ${activeTab===tab?"bg-blue-600 text-white shadow-sm":"text-slate-700 hover:text-blue-900"}`}>{label}</button>)}
          </div>

          {activeTab === "document" && canPrint && (
            <button
              onClick={()=>void handlePrintPDF()}
              disabled={busy || !currentDocument?.data.terms || !canPrint}
              type="button"
              className="flex items-center gap-2 px-4 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white font-bold text-xs rounded-lg shadow-sm transition-all"
            >
              Print indent / save PDF
            </button>
          )}

          {onProceedToSourcing && canOpen(user.permissions,"projects.group_quotations") && (
            <button
              disabled={!created}
              onClick={onProceedToSourcing}
              type="button"
              className="flex items-center gap-1.5 px-3.5 py-1.5 bg-blue-600 hover:bg-blue-500 text-white font-bold text-xs rounded-lg shadow-sm transition-all"
            >
              Continue to quotations
            </button>
          )}
        </div>
      </header>

      {/* Main Workspace Body */}
      <main className="flex-1 min-w-0 overflow-y-auto">
        {notice && <div role="alert" className="m-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">{notice}</div>}
        {loading ? (
          <div className="flex items-center justify-center p-16 text-slate-500 font-semibold text-sm">
            Loading project items and vendor bank...
          </div>
        ) : validLines.length === 0 ? (
          <div className="max-w-xl mx-auto my-16 bg-white p-8 rounded-2xl border border-slate-200 text-center shadow-sm">
            <span className="text-4xl">📦</span>
            <h3 className="text-lg font-bold mt-4 text-slate-900">
              No BO Items Ready for Indent
            </h3>
            <p className="text-sm text-slate-600 mt-2">
              Step 2 requires items defined in Step 1 (BO List). Return to Step
              1 and add items with required quantities and dates.
            </p>
            <button
              onClick={onBackToBO}
              type="button"
              className="mt-6 px-5 py-2.5 bg-blue-600 hover:bg-blue-700 text-white text-sm font-bold rounded-xl"
            >
              Go to Step 1 (BO List)
            </button>
          </div>
        ) : !allowedTabs.includes(activeTab) ? <p role="status" className="p-6">You do not have permission to open this indent tab.</p> : activeTab === "assignment" || activeTab === "vendors" || activeTab === "report" ? (
          <PurchaseIndentGroups initialVendorGroup={params.get("group")??undefined} key={boId} stage={activeTab === "assignment" ? "items" : activeTab} onStageChange={stage=>setActiveTab(stage === "items" ? "assignment" : stage)} lines={validLines} suppliers={suppliers} record={workspace?.record ?? null} report={workspace?.item_report ?? []} canWrite={workspace?.can_write ?? false} canManageVendors={workspace?.can_manage_vendors ?? false} canViewVendors={allowedTabs.includes("vendors")} canViewRegister={allowedTabs.includes("preview")} busy={busy} notice={notice} history={workspace?.history ?? []} onSave={saveGroups} onPreview={id => { setSelectedDocumentId("");setSelectedGroupId(id); setSelectedSupplierId(""); setActiveTab("preview"); }} />
        ) : activeTab === "preview" ? (
          <div className="p-6 max-w-[1000px] mx-auto space-y-6">
            {!ready && <div role="status" className="no-print rounded-lg bg-amber-50 p-3 text-sm"><p>Choose at least three active vendors for every saved group before generating indents.</p>{allowedTabs.includes("vendors") && <button type="button" disabled={busy || !groups.length} className="mt-2 min-h-11 rounded-lg border border-slate-300 bg-white px-4 py-2 font-semibold" onClick={()=>setActiveTab("vendors")}>Choose group vendors</button>}</div>}
            <VendorIndentRegister groups={groups} suppliers={suppliers} workspace={documents} canGenerate={canGenerate && (created || ready)} canWrite={canDispatch} canOpenDocument={allowedTabs.includes("document")} busy={busy} notice={notice} onAction={documentAction} onOpen={(group,vendor,documentId,opener)=>{setDocumentOpener(opener??null);setSelectedDocumentId(documentId||"");setSelectedGroupId(group);setSelectedSupplierId(vendor);setPrintNotice("");setIsDocumentOpen(true);}}/>
          </div>
        ) : (
          /* TAB 3: SUPPLIER INDENT PREVIEW & PDF */
          <div className="p-6 max-w-[1000px] mx-auto space-y-6">
            {!ready && <div role="status" className="no-print rounded-lg bg-amber-50 p-3 text-sm"><p>Choose at least three active vendors for every saved group before generating indents.</p>{allowedTabs.includes("vendors") && <button type="button" disabled={busy || !groups.length} className="mt-2 min-h-11 rounded-lg border border-slate-300 bg-white px-4 py-2 font-semibold" onClick={()=>setActiveTab("vendors")}>Choose group vendors</button>}</div>}
            {/* Vendor Selector & Indent Settings Bar (Hidden on print) */}
            <div className="no-print bg-white p-4 rounded-xl border border-slate-200 shadow-sm flex flex-wrap items-center justify-between gap-4">
              <div className="flex flex-wrap items-center gap-3">
                <ValidatedLabel className="text-xs font-bold text-slate-700">Item group
                  <ValidatedSelect value={currentGroup?.id ?? ""} onChange={e => { setSelectedGroupId(e.target.value); setSelectedSupplierId(""); setSelectedDocumentId(""); }} className="ml-2 rounded-lg border border-slate-300 px-3 py-2">
                    {groups.map(g => <option key={g.id} value={g.id}>{g.name}</option>)}
                  </ValidatedSelect>
                </ValidatedLabel>
                <ValidatedLabel className="text-xs font-bold text-slate-700">
                  Indent for vendor:
                  <ValidatedSelect
                    value={currentSupplier?.id || ""}
                    onChange={(e) => {setSelectedSupplierId(e.target.value);setSelectedDocumentId("");}}
                    className="text-xs font-bold bg-slate-50 border border-slate-300 rounded-lg px-3 py-2 text-slate-900 focus:ring-2 focus:ring-blue-500"
                  >
                    {assignedSuppliers.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name} ({s.code}) —{" "}
                        {
                          supplierLines.length
                        }{" "}
                        items assigned
                      </option>
                    ))}
                  </ValidatedSelect>
                </ValidatedLabel>
              </div>

              <div className="flex flex-wrap items-center gap-3 text-sm">
                {vendorDocuments.length>0 && <ValidatedLabel className="no-print block text-sm">Indent scope revision<ValidatedSelect className="ml-2 rounded-lg border p-2" value={selectedDocumentId} onChange={event=>setSelectedDocumentId(event.target.value)}><option value="">Current group scope (revision {currentGroup?.scope_revision??1})</option>{vendorDocuments.map(d=><option key={d.id} value={d.id}>Revision {d.data.scope_revision??1} · {d.data.number}{d.data.is_current===false?" · previous scope":""}</option>)}</ValidatedSelect></ValidatedLabel>}
                {currentDocument ? <p>Generated {new Date(currentDocument.data.generated_at).toLocaleString()} · {currentDocument.data.sent_date ? `Sent ${currentDocument.data.sent_date}` : "Sent date not recorded"}</p> : <button type="button" disabled={busy || (!created && !ready) || !currentSupplier?.is_active || !canGenerate} onClick={()=>void documentAction({action:"generate",group_id:currentGroup?.id,vendor_id:currentSupplier?.id})} className="min-h-11 rounded-lg bg-blue-600 px-4 py-2 font-semibold text-white disabled:opacity-50">{busy ? "Generating…" : "Generate selected vendor indent"}</button>}
              </div>
            </div>
            {currentDocument?.data.is_current===false && <p className="no-print rounded-lg bg-amber-50 p-3 text-sm">Previous scope revision {currentDocument.data.scope_revision??1}. Current group scope is revision {currentGroup?.scope_revision??1}; this document retains its original item scope.</p>}
            {!currentDocument && <p className="no-print text-sm text-slate-600">The layout below is a draft. Generate this vendor indent to save its number, timestamp and document snapshot before printing.</p>}

            {!isDocumentOpen && indentDocument}
          </div>
        )}
      </main>
      {isDocumentOpen && allowedTabs.includes("document") && <RecordDrawer centered wide returnFocusTo={documentOpener} title={`Purchase indent — ${itemNumber} / ${currentSupplier?.name || "Vendor"}`} onClose={closeDocument} footer={<div className="no-print flex flex-wrap items-center justify-between gap-3"><p className="text-sm text-slate-600">Close to return to the register and continue with the next vendor.</p><div className="flex flex-wrap gap-2">{currentDocument ? <button type="button" disabled={busy || !currentDocument.data.terms || !canPrint || documents?.can_print === false} onClick={()=>void handlePrintPDF()} className="min-h-11 rounded-lg bg-blue-700 px-4 py-2 font-semibold text-white disabled:opacity-50">{busy ? "Saving print request…" : "Print / save PDF"}</button> : <button type="button" disabled={busy || !canGenerate || (!created && !ready) || !currentSupplier?.is_active} onClick={()=>void documentAction({action:"generate",group_id:currentGroup?.id,vendor_id:currentSupplier?.id})} className="min-h-11 rounded-lg bg-blue-700 px-4 py-2 font-semibold text-white disabled:opacity-50">Generate selected vendor indent</button>}<button type="button" disabled={busy} onClick={closeDocument} className="min-h-11 rounded-lg border border-slate-300 px-4 py-2 font-semibold disabled:opacity-50">Close and return to register</button></div></div>}>
        <div className="no-print space-y-2 rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900"><p className="font-semibold">1. Review this indent · 2. Print or save PDF · 3. Close and return</p><p>{currentDocument ? `Saved indent ${currentDocument.data.number}.` : "Draft preview — generate the indent before printing."} Printing does not record dispatch. After sending it to the vendor, use Record sent date in the register.</p>{currentDocument?.data.is_current === false && <p className="font-semibold">Previous scope revision {currentDocument.data.scope_revision ?? 1}; this indent retains its original items.</p>}</div>
        {notice && <p role="alert" className="no-print rounded-lg bg-red-50 p-3 text-sm text-red-700">{notice}</p>}
        {printNotice && <p role="status" className="no-print rounded-lg bg-emerald-50 p-3 text-sm text-emerald-800">{printNotice}</p>}
        <div className="min-w-0 overflow-x-auto">{indentDocument}</div>
      </RecordDrawer>}
    </div>
  );
}
