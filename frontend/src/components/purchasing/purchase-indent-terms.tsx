import { supplierPaymentTerms } from "./quotation-terms";
import type { Supplier } from "./purchase-indent-workspace";

export type IndentTerms = { source: "vendor_master" | "legacy_template_v1"; notes: string[]; conditions: string[]; delivery_basis: string };
const lines = (value?: string) => (value??"").split(/\r?\n/).map(line=>line.trim()).filter(Boolean);
export function vendorIndentDefaults(supplier?: Supplier): IndentTerms {
  return {source:"vendor_master",notes:lines(supplier?.data.indent_notes),conditions:lines(supplier?.data.indent_terms),delivery_basis:supplier?.data.delivery_terms?.trim()??""};
}

/** Render frozen document text; never replace an issued snapshot with current master data. */
export function PurchaseIndentTerms({terms,supplier}:{terms?:IndentTerms;supplier?:Supplier}) {
  const payment=supplierPaymentTerms(supplier);
  if(!terms)return <p role="alert" className="no-print text-sm text-red-700">This indent has no saved terms snapshot. Reload the document before printing.</p>;
  return <section aria-label="Purchase indent terms" className="mb-8 space-y-5 text-[11px] font-sans leading-relaxed text-slate-800">
    <p className="no-print text-xs text-slate-500">{terms.source==="legacy_template_v1"?"Saved historical template terms":"Saved vendor terms"}</p>
    {terms.notes.length>0&&<div><h3 className="mb-2 text-xs font-bold uppercase underline">Important notes</h3><ol className="list-decimal space-y-1 pl-5">{terms.notes.map((note,index)=><li key={index} className="whitespace-pre-wrap">{note}</li>)}</ol></div>}
    <div><h3 className="mb-2 text-xs font-bold uppercase underline">Terms &amp; conditions</h3>{terms.conditions.length>0?<ol className="list-decimal space-y-1 pl-5">{terms.conditions.map((condition,index)=><li key={index} className="whitespace-pre-wrap">{condition}</li>)}</ol>:<p className="no-print text-amber-800">Configure this vendor&apos;s purchase indent conditions before generation.</p>}</div>
    <dl className="space-y-2"><div><dt className="font-bold">Delivery basis</dt><dd className="whitespace-pre-wrap">{terms.delivery_basis||"Not configured"}</dd></div><div><dt className="font-bold">Payment terms</dt><dd className="whitespace-pre-wrap">{payment||"Not recorded"}</dd></div></dl>
  </section>;
}
