import type { Supplier } from "./purchase-indent-workspace";

/** Same payment wording used in the printed indent, from its frozen vendor snapshot. */
export function supplierPaymentTerms(supplier?: Supplier): string {
  const terms = supplier?.data.payment_terms;
  if (!terms) return "";
  if (terms.description?.trim()) return terms.description.trim();
  const parts: string[] = [];
  if (terms.advance_pct) parts.push(`${terms.advance_pct}% advance against PI`);
  if (terms.qc_pct) parts.push(`${terms.qc_pct}% after stage inspection / QC clearance`);
  if (terms.delivery_pct) parts.push(`${terms.delivery_pct}% after material delivery at site`);
  if (terms.retention_pct) parts.push(`${terms.retention_pct}% retention / warranty`);
  const payment = parts.length ? `Payment shall be ${parts.join(", ")}.` : "";
  const credit = terms.credit_days ? `Credit period: ${terms.credit_days} days.` : "";
  return [payment, credit].filter(Boolean).join(" ");
}

export function indentDeliveryTerms(document: {data:{terms?:{delivery_basis:string}}}): string { return document.data.terms?.delivery_basis?.trim() || ""; }
