export type AdvanceState="draft"|"requested"|"approved"|"rejected"|"not_applicable"|"disbursed";
export type AdvanceAction="prepare"|"submit"|"revise"|"approve"|"reject"|"disburse";
export type AdvanceSource={project_id:string;package_id:string;order_id:string;order_version:number;number:string;date:string;supplier_id:string;supplier_name:string;currency:string;total:string;payment_terms:string;acceptance_id:string;acceptance_version:number;supplier_verification:string;calculation_basis:string;available:boolean;blockers:string[]};
export type AdvanceDetails={kind:"payment"|"not_applicable";request_no:string;request_date:string;advance_pct:string;proforma_invoice_ref:string;proforma_invoice_date:string;bank_account_ref:string;notes:string};
export type AdvancePayment={voucher_ref:string;disbursed_date:string;notes:string};
export type AdvanceRecord={id:string;project_id:string;package_id:string;version:number;state:AdvanceState;source_snapshot:AdvanceSource;details:AdvanceDetails;amount:string;submitted_by_id:string;submitted_at:string|null;submitted_version:number;decision_id:string;reviewed_by_id:string;reviewed_at:string|null;payment:AdvancePayment|null;created_at:string;updated_at:string;created_by_id:string;updated_by_id:string};
export type AdvanceEvent={id:string;action:AdvanceAction;reason:string;before:AdvanceRecord|null;after:AdvanceRecord;at:string;actor_id:string};
export type AdvanceData={record:AdvanceRecord|null;source:AdvanceSource;actions:AdvanceAction[];history:AdvanceEvent[];task_only:boolean};
export type AdvanceQueueRow={id:string;project_id:string;package_id:string;project_name:string;project_number:string;po_number:string;request_no:string;state:AdvanceState;amount:string;currency:string;updated_at:string};
export const advanceLabels:Record<AdvanceState,string>={draft:"Draft",requested:"Awaiting Finance",approved:"Finance approved · payment pending",rejected:"Finance rejected",not_applicable:"No advance · Finance confirmed",disbursed:"Payment recorded"};
export const actionLabels:Record<AdvanceAction,string>={prepare:"Save changes",submit:"Submit request",revise:"Start revision",approve:"Approve request",reject:"Reject request",disburse:"Record payment"};
export function advanceCommand(version:number,action:AdvanceAction,f:FormData){
 const text=(name:string)=>String(f.get(name)??"");
 const base={version,action,reason:text("reason")};
 if(action==="prepare"){
  const kind=text("kind") as AdvanceDetails["kind"];
  return {...base,details:{kind,request_no:text("request_no"),request_date:text("request_date"),advance_pct:kind==="not_applicable"?"0":text("advance_pct"),proforma_invoice_ref:kind==="not_applicable"?"":text("proforma_invoice_ref"),proforma_invoice_date:kind==="not_applicable"?"":text("proforma_invoice_date"),bank_account_ref:kind==="not_applicable"?"":text("bank_account_ref"),notes:text("notes")}};
 }
 if(action==="disburse")return {...base,payment:{voucher_ref:text("voucher_ref"),disbursed_date:text("disbursed_date"),notes:text("notes")}};
 return base;
}
/** Preview only. The API derives the saved amount from its actual PO. */
export function advancePreview(total:string,pct:string):string{
 const cents=(v:string)=>{if(!/^(0|[1-9][0-9]{0,13})(\.[0-9]{1,2})?$/.test(v))return null;const [i,f=""]=v.split(".");return BigInt(i)*BigInt(100)+BigInt(f.padEnd(2,"0"));};
 const t=cents(total),p=cents(pct);if(t===null||p===null||p>BigInt(10000))return "Pending valid percentage";
 const amount=(t*p+BigInt(5000))/BigInt(10000);return `${amount/BigInt(100)}.${String(amount%BigInt(100)).padStart(2,"0")}`;
}
export function advanceSubmissionMissing(r:AdvanceRecord):string[]{
 const d=r.details,missing:string[]=[];
 if(!d.request_no.trim())missing.push("Request number");if(!d.request_date)missing.push("Request date");
 if(d.kind==="payment"){
  if(!d.advance_pct||!r.amount||r.amount==="0.00")missing.push("Positive agreed percentage");
  if(!d.proforma_invoice_ref.trim())missing.push("Proforma invoice reference");if(!d.proforma_invoice_date)missing.push("Proforma invoice date");if(!d.bank_account_ref.trim())missing.push("Bank-account reference");
 }else if(!d.notes.trim())missing.push("No-advance reason");
 return missing;
}
