"use client";
import { quotationMoney } from "./quotation-money";
import { VendorRevisionComparison } from "./vendor-revision-comparison";
import { useProjectUser } from "@/components/projects/project-permissions";
import { purchaseStepAllowed } from "@/components/projects/workspace-permissions";
import { useEffect, useState } from "react";
import { DataGrid } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { GroupQuotationReceipt, type QuotationFile, type QuotationRevision } from "./group-quotation-receipt";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import type { VendorIndent } from "./purchase-indent-documents";
import { quotationAmountFields } from "./quotation-costs";
type Entry = {
    line_id: string;
    unit_price: string | null;
    pricing_mode?: "unit_price" | "line_total";
    line_total?: string | null;
    coverage_type?: "warranty" | "guarantee" | "both";
    warranty_months?: number | null;
    discount_pct: string;
    tax_pct: string;
    compliant: boolean;
    notes: string;
};
export type Quote = {
    document_id: string;
    amount_mode?: "itemized" | "total_only";
    total_amount?: string | null;
    reference: string;
    evidence_reference: string;
    attachment_id?: string | null;
    document_not_provided?: boolean;
    no_document_reason?: string;
    calculation?: { total: string };
    received_date: string;
    valid_until: string;
    currency: string;
    freight: string;
    other_charges: string;
    delivery_days: number;
    payment_terms: string;
    delivery_terms?: string;
    lines: Entry[];
};
export type Comparison = {
    revision?: number;
    received_date?: string;
    valid_until?: string;
    recorded_at?: string;
    reason?: string;
    scope_revision?: number;
    document_id: string;
    amount_mode?: "itemized" | "total_only";
    total_amount?: string | null;
    group_id: string;
    group_name: string;
    vendor: string;
    vendor_id?: string;
    reference: string;
    total: string;
    eligible: boolean;
    exclusions: string[];
    delivery_days: number;
    payment_terms?: string;
    delivery_terms?: string;
    rank: number | null;
    suggested: boolean;
    lines: (Entry & {
        source: VendorIndent["data"]["lines"][number];
        quantity: string;
        net: string | null;
        tax_amount: string | null;
        landed: string | null;
        allocated_charges?: string;
        item_lowest?: boolean;
    })[];
};
export type Workspace = {
 tax_rate: string;
    record: {
        version: number;
        quotes: Quote[];
    } | null;
    documents: VendorIndent[];
    currency: string;
    can_write: boolean;
    comparisons: Comparison[];
    comparison_revisions?: Comparison[];
    attachments: QuotationFile[];
    revisions: QuotationRevision[];
    history: {
        reason: string;
        created_at: string;
    }[];
};
export function GroupQuotationWorkspace({ project, boId, initialStep, onNavigateStep }: {
    project: string;
    boId?: string;
    initialStep: number;
    onNavigateStep: (step: number) => void;
}) {
    const user = useProjectUser();
    const authorised = purchaseStepAllowed(user.permissions,initialStep);
    const [ws, setWs] = useState<Workspace | null>(null), [error, setError] = useState(""), [busy, setBusy] = useState(false), [details, setDetails] = useState<Comparison | null>(null);
    const [loadedSection,setLoadedSection]=useState("");
    const url = `/api/projects/${project}/purchase-bo/${boId}/group-quotations`;
    useEffect(() => { if (!boId || !authorised)
        return; const c = new AbortController(); fetch(`${url}?section=${initialStep===5?"award":initialStep===4?"comparison":"receipt"}`, { cache: "no-store", signal: c.signal }).then(async (r) => { const p = await r.json(); if (!r.ok)
        throw Error(typeof p.detail === "string" ? p.detail : "Cannot load grouped quotations"); setWs(p); setLoadedSection(`${url}:${initialStep}`); setError(""); }).catch(e => { if (e.name !== "AbortError")
        setError(e.message); }); return () => c.abort(); }, [boId, url, initialStep, authorised]);
    const money = (v: string | null | undefined) => v == null ? "Not set" : quotationMoney(v,ws?.currency||"INR");
    async function save(form: HTMLFormElement, doc: VendorIndent) { const f = new FormData(form); const totalOnly=f.has("total_only"); const quote: Quote = { document_id: doc.id, ...quotationAmountFields(totalOnly,f.get("total_amount"),f.get("freight"),f.get("other_charges")), attachment_id: f.get("attachment_id") ? String(f.get("attachment_id")) : null, document_not_provided: f.get("document_not_provided")==="true", no_document_reason: String(f.get("no_document_reason")??""), reference: String(f.get("reference")), evidence_reference: String(f.get("evidence_reference")), received_date: String(f.get("received_date")), valid_until: String(f.get("valid_until")), currency: ws!.currency, delivery_days: Number(f.get("delivery_days")), payment_terms: String(f.get("payment_terms")), delivery_terms: String(f.get("delivery_terms")), lines: doc.data.lines.map(l => totalOnly?{line_id:l.id,unit_price:null,pricing_mode:"unit_price",line_total:null,coverage_type:"warranty",warranty_months:null,discount_pct:"0",tax_pct:"0",compliant:false,notes:""}:{ line_id: l.id, pricing_mode: f.get(`pricing_mode_${l.id}`) === "line_total" ? "line_total" : "unit_price", unit_price: f.get(`pricing_mode_${l.id}`)==="line_total" || !f.get(`price_${l.id}`) ? null : String(f.get(`price_${l.id}`)), line_total: f.get(`pricing_mode_${l.id}`)!=="line_total" || !f.get(`line_total_${l.id}`) ? null : String(f.get(`line_total_${l.id}`)), coverage_type: String(f.get(`coverage_type_${l.id}`)) as "warranty" | "guarantee" | "both", warranty_months: f.get(`warranty_months_${l.id}`) ? Number(f.get(`warranty_months_${l.id}`)) : null, discount_pct: String(f.get(`discount_${l.id}`)), tax_pct: String(f.get(`tax_${l.id}`)), compliant: f.has(`compliant_${l.id}`), notes: String(f.get(`notes_${l.id}`)) }) }; setBusy(true); setError(""); try {
        const r = await fetch(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ version: ws?.record?.version || 0, reason: String(f.get("reason")), quote }) });
        const p = await r.json();
        if (!r.ok && displayServerErrors(form,p)) return false;
        if (!r.ok)
            throw Error(typeof p.detail === "string" ? p.detail : "Invalid quotation entries; check dates, prices and scope");
        setWs(p);
        window.dispatchEvent(new CustomEvent("purchase-progress-saved", {detail:project}));
        return true;
    }
    catch (e) {
        setError(e instanceof Error ? e.message : "Save failed");
        return false;
    }
    finally {
        setBusy(false);
    } }
    async function upload(document: VendorIndent, file: File): Promise<QuotationFile|null> {
        if(file.size>10*1024*1024){setError("Quotation files must be at most 10 MiB");return null;}
        setBusy(true);setError("");
        try {
            const body=new FormData();body.append("file",file);
            const response=await fetch(`${url}/${document.id}/attachments`,{method:"POST",body});
            const result=await response.json();
            if(!response.ok)throw Error(typeof result.detail === "string"?result.detail:result.message||"Unable to upload quotation");
            const attachment={...result,document_id:document.id} as QuotationFile;
            setWs(previous=>previous?{...previous,attachments:[...previous.attachments,attachment]}:previous);
            return attachment;
        } catch(error){setError(error instanceof Error?error.message:"Upload failed");return null;}
        finally{setBusy(false);}
    }
    if (!authorised) return <p role="status" className="p-6">You do not have permission to open this quotation tab.</p>;

    if(initialStep===5){
        if(error)return <p role="alert" className="p-6 text-red-800">{error}</p>;
        if(loadedSection!==`${url}:${initialStep}`)return <p role="status" className="p-6">Checking Accounts quotation approval…</p>;
        return <p role="status" className="rounded-xl bg-emerald-50 p-6">Accounts approved the current quotations. LOI issuance is the next capability to implement.</p>;
    }
    if (!boId)
        return <p>Select a saved BO list first.</p>;
    return <section aria-label="Quotation workspace" className="min-h-0 min-w-0 flex-1 space-y-4">
 {error && <p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p>}
 {!ws && !error && <p role="status">Loading vendor quotations…</p>}
 {ws && <>
 {initialStep === 5 ? <p className="rounded-xl bg-amber-50 p-5">Accounts approved the current quotations. LOI issuance is the next capability to implement.</p> : initialStep === 3 ? <GroupQuotationReceipt workspace={ws} busy={busy} error={error} url={url} onSave={save} onUpload={upload} money={money}/> : <VendorRevisionComparison project={project} boId={boId} onNext={()=>onNavigateStep(5)} key={`${project}:${boId}`} revisions={ws.comparison_revisions ?? ws.comparisons} money={money} onDetails={setDetails}/>}
 <details className="rounded-xl border bg-white p-4"><summary className="cursor-pointer font-semibold">Quotation change history ({ws.history.length})</summary>{ws.history.map((h, i) => <p className="mt-2 text-sm" key={i}>{new Date(h.created_at).toLocaleString()} · {h.reason}</p>)}</details></>}
 {details && <RecordDrawer centered wide title={`${details.amount_mode==="total_only"?"Overall quotation":"Item price breakdown"} — ${details.group_name} / ${details.vendor} · quote revision ${details.revision??1}`} onClose={() => setDetails(null)}><p className="mb-2 text-sm">Quotation {details.reference} · valid until {details.valid_until||"Not recorded"}</p><p className="mb-2">Payment terms: {details.payment_terms||"Not recorded"} · Delivery terms: {details.delivery_terms||"Not recorded"} · Delivery timeline: {details.delivery_days} days</p><p className="mb-4 font-semibold">Group total: {money(details.total)} · {details.suggested ? "L1 — lowest eligible group cost" : details.exclusions.join("; ") || `L${details.rank}`}</p>{details.amount_mode==="total_only"?<p className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm">The vendor supplied one all-inclusive indent total. No item prices, taxes, discounts, warranty values or charge allocations were recorded, so this revision cannot be sent for item-level Accounts approval.</p>:<DataGrid label="Quotation item price breakdown" actions={() => null} rows={details.lines.map(l => ({ ...l, id: l.line_id }))} columns={[{ key: "item", label: "Item", value: l => l.source.item_snapshot.description || l.source.item_snapshot.sku || "Item" }, { key: "quantity", label: "Quantity", value: l => l.quantity }, { key: "rate", label: "Unit price", value: l => money(l.unit_price) }, { key: "coverage", label: "Warranty / guarantee", value: l => l.warranty_months ? `${l.coverage_type||"warranty"} · ${l.warranty_months} months` : "Not recorded" }, { key: "base", label: "Entry basis", value: l => l.pricing_mode === "line_total" ? `Direct amount ${money(l.line_total)}` : "Unit price × quantity" }, { key: "net", label: "After discount", value: l => money(l.net) }, { key: "tax", label: "Tax", value: l => money(l.tax_amount) }, { key: "charges", label: "Allocated charges", value: l => money(l.allocated_charges) }, { key: "landed", label: "Item landed cost", value: l => money(l.landed) + (l.item_lowest ? " · lowest eligible item cost" : "") }, { key: "notes", label: "Compliance / notes", value: l => `${l.compliant ? "Confirmed" : "Not confirmed"} · ${l.notes}` }]}/>}</RecordDrawer>}
 </section>;
}
