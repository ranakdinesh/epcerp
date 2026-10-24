"use client";

import { useState } from "react";
import { ValidatedInput, ValidatedLabel, ValidatedTextarea } from "@/components/ui/validated-controls";
import type { VendorIndent } from "./purchase-indent-documents";
import type { Quote } from "./group-quotation-workspace";
import { indentDeliveryTerms, supplierPaymentTerms } from "./quotation-terms";

const field = "mt-1 block min-h-11 w-full rounded-lg border border-slate-300 bg-white p-3";
export function QuotationCommonTerms({document, previous}: {document: VendorIndent; previous?: Quote}) {
  const payment = previous?.payment_terms?.trim() || supplierPaymentTerms(document.data.vendor);
  const delivery = previous?.delivery_terms?.trim() || indentDeliveryTerms(document);
  const [change, setChange] = useState(!payment || !delivery);
  const [paymentText, setPaymentText] = useState(payment);
  const [deliveryText, setDeliveryText] = useState(delivery);
  return <section aria-label="Quotation terms" className="space-y-4 rounded-xl border border-slate-200 bg-white p-4">
    <div><h3 className="font-bold">Terms &amp; conditions</h3><p className="mt-1 text-sm text-slate-600">{previous ? "Copied from the previous quotation revision." : "Payment terms from the vendor record saved with this indent; delivery basis from the printed indent."} Review these defaults against the vendor quotation.</p></div>
    <dl className="grid gap-4 text-sm sm:grid-cols-2"><div><dt className="font-semibold text-slate-600">Payment terms</dt><dd className="mt-1 whitespace-pre-wrap break-words">{payment || "No payment terms recorded — enter them below."}</dd></div><div><dt className="font-semibold text-slate-600">Delivery basis / location</dt><dd className="mt-1 whitespace-pre-wrap break-words">{delivery || "No delivery basis recorded — enter it below."}</dd></div></dl>
    {payment && delivery && <ValidatedLabel className="indent-selection flex min-h-11 items-center gap-2"><ValidatedInput type="checkbox" checked={change} onChange={event=>setChange(event.target.checked)}/>Change terms for this quotation</ValidatedLabel>}
    <div hidden={!change} className={`${change ? "grid" : "hidden"} gap-4 sm:grid-cols-2`}>
      <ValidatedLabel className="font-semibold">Payment terms for this quotation<ValidatedTextarea aria-label="Payment terms for this quotation" required={change} disabled={!change} name={change ? "payment_terms" : undefined} rows={3} maxLength={2000} value={paymentText} onChange={event=>setPaymentText(event.target.value)} className={field}/></ValidatedLabel>
      <ValidatedLabel className="font-semibold">Delivery terms / location for this quotation<ValidatedTextarea aria-label="Delivery terms / location for this quotation" required={change} disabled={!change} name={change ? "delivery_terms" : undefined} rows={3} maxLength={2000} value={deliveryText} onChange={event=>setDeliveryText(event.target.value)} className={field}/></ValidatedLabel>
      <p className="text-sm text-slate-600 sm:col-span-2">Changes are saved with this quotation revision. The vendor master and indent stay as recorded.</p>
    </div>
    {!change && <><ValidatedInput type="hidden" name="payment_terms" value={payment}/><ValidatedInput type="hidden" name="delivery_terms" value={delivery}/></>}
  </section>;
}
