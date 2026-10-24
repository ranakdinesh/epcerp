import type { SourcingOffer, SourcingWorkspace } from "./sourcing-data";
const escape = (v: unknown) =>
  String(v ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ]!,
  );
export function sourcingDocument(
  w: SourcingWorkspace,
  kind: "RFQ" | "CST" | "LOI",
  offer?: SourcingOffer,
) {
  const c = w.case;
  if (!c) return;
  const s =
    kind === "LOI"
      ? c.decision_snapshot.source || c.source_snapshot
      : c.source_snapshot;
  const chosen = kind === "LOI" ? c.decision_snapshot.offer : offer;
  const title =
    kind === "LOI"
      ? c.loi_reference || "DRAFT LOI — " + s.package_code
      : kind +
        " — " +
        s.package_code +
        (offer ? " — " + offer.supplier_snapshot.code : "");
  const rows = (pairs: [string, unknown][]) =>
    pairs
      .map(([k, v]) => `<tr><th>${escape(k)}</th><td>${escape(v)}</td></tr>`)
      .join("");
  let body = `<h1>${escape(title)}</h1><p>${escape(c.state === "released" && kind === "LOI" ? "Released copy" : "Review copy — external dispatch must be recorded separately")}</p><table>${rows(
    [
      ["Project", s.project_name],
      ["Site", s.site],
      ["Package", s.package_code],
      ["Indent", s.indent_number],
      ["Material", s.description],
      ["Specification", s.full_kit],
      ["Quantity", s.quantity + " " + s.uom],
      ["Required date", s.required_date],
      ["Currency", s.currency],
    ],
  )}</table>`;
  if (chosen)
    body += `<h2>Supplier</h2><table>${rows([
      ["Name", chosen.supplier_snapshot.name],
      ["Registration", chosen.supplier_snapshot.registration],
      ["Contact", chosen.supplier_snapshot.contact],
      ["Location", chosen.supplier_snapshot.location],
    ])}</table>`;
  if (kind === "RFQ" && chosen)
    body += `<table>${rows([
      ["RFQ ID", chosen.id],
      ["Due date", chosen.due_date],
      ["Commercial requirements", chosen.rfq_terms],
      ["Dispatch reference", chosen.dispatch_reference],
    ])}</table>`;
  if (kind === "CST")
    body += `<h2>Comparative statement</h2><p>Line base less discount + explicit tax + freight + other charges. Tax credit is not assumed.</p><table><tr><th>Supplier</th><th>Quote</th><th>Landed total</th><th>Budget variance</th><th>Rank</th><th>Terms / review</th></tr>${c.offers
      .map((o) => {
        const r = w.comparison.find((r) => r.offer_id === o.id);
        return `<tr><td>${escape(o.supplier_snapshot.name)}</td><td>${escape(o.quote_reference || o.state)}</td><td>${escape(r?.total)}</td><td>${escape(r?.variance)}</td><td>${escape(r?.rank ? "L" + r.rank : "—")}</td><td>${escape([o.compliance_notes, o.payment_terms, o.warranty, o.delivery_days + " days", o.evidence_reference].join(" · "))}</td></tr>`;
      })
      .join("")}</table><p>Selection: ${escape(c.selection_reason)}</p>`;
  if (kind === "LOI" && chosen)
    body += `<h2>Approved commercial terms</h2><table>${rows([
      ["Quote reference", chosen.quote_reference],
      [
        "Landed total",
        c.decision_snapshot.comparison?.find((r) => r.offer_id === chosen.id)
          ?.total,
      ],
      ["Quote valid until", chosen.valid_until],
      ["Unit price", chosen.unit_price],
      ["Discount", chosen.discount],
      ["Tax total", chosen.tax],
      ["Freight", chosen.freight],
      ["Other charges", chosen.other],
      ["Delivery days", chosen.delivery_days],
      ["Payment terms", chosen.payment_terms],
      ["Warranty", chosen.warranty],
      ["LOI terms", c.decision_snapshot.terms],
      ["Approval timestamp", c.approved_at],
      ["Approver ID", c.approved_by_id],
      ["Release date", c.loi_sent_date],
      ["Dispatch reference", c.loi_dispatch_reference],
    ])}</table>`;
  const html = `<!doctype html><html><head><meta charset="utf-8"><title>${escape(title)}</title><style>body{font:14px Arial,sans-serif;color:#172033;margin:32px}table{border-collapse:collapse;width:100%;margin:20px 0}th,td{border:1px solid #cbd5e1;padding:9px;text-align:left;white-space:pre-wrap;overflow-wrap:anywhere}th{background:#f1f5f9}h1{font-size:24px}@media print{button{display:none}body{margin:0}tr{break-inside:avoid}}</style></head><body><p>Use your browser’s Print → Save as PDF to create a PDF copy.</p>${body}</body></html>`;
  const url = URL.createObjectURL(
    new Blob([html], { type: "text/html;charset=utf-8" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = title.replace(/[^a-z0-9-]/gi, "_") + ".html";
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
