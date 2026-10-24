export type SourcingAction="configure"|"rfq"|"dispatch"|"no_bid"|"quote"|"propose"|"decide"|"reopen"|"release";
export type SourcingSupplier={id:string;name:string;code:string;version:number;eligible:boolean;registration:string;contact:string;location:string};
export type SourcingOffer={id:string;supplier_id:string;supplier_snapshot:SourcingSupplier;state:string;due_date:string;rfq_terms:string;sent_date:string;channel:string;dispatch_reference:string;quote_reference:string;evidence_reference:string;valid_until:string;pricing_mode?:string;line_total?:string;unit_price:string;discount:string;tax:string;freight:string;other:string;delivery_days:number;payment_terms:string;warranty:string;compliant:boolean;compliance_notes:string;no_bid_reason:string};
export type ApprovedEvidence={scoped_quantity?:string;scoped_total?:string;price?:{pricing_mode:string;unit_price:string;base:string;discount:string;tax:string;allocated_charges:string};id:string;vendor_id:string;quote_reference:string;evidence_reference:string;quantity:string;currency:string;total:string};
export type SourcingCase={award_reference?:string;id:string;project_id:string;package_id:string;version:number;state:string;approver_id:string;offers:SourcingOffer[];selected_offer_id:string;selection_reason:string;loi_terms:string;proposed_by_id:string;approved_by_id:string;approved_at:string|null;loi_reference:string;loi_sent_date:string;loi_channel:string;loi_dispatch_reference:string;released_at:string|null};
export type SourcingWorkspace={case:SourcingCase|null;source:{description:string;quantity:string;currency:string;project_available:boolean;eligible:boolean;blockers:string[]};suppliers:SourcingSupplier[];approvers:{id:string;label:string}[];actions:SourcingAction[];comparison:{offer_id:string;total:string;rank:number|null;expired:boolean}[];history:{id:string;action:string;reason:string;created_at:string;created_by_id:string}[];task_only:boolean;approved_evidence:ApprovedEvidence[]};
export const sourcingLabels:Record<SourcingAction,string>={configure:"Set selection approver",rfq:"Add RFQ",dispatch:"Record RFQ dispatch",no_bid:"Record no bid",quote:"Record quotation",propose:"Propose selection",decide:"Review selection",reopen:"Return to draft",release:"Release LOI"};
// Build only the selected command's permitted fields. Values stay decimal strings.
export function sourcingCommand(action:SourcingAction,version:number,fields:FormData,offer?:SourcingOffer):Record<string,unknown>{
 const command:Record<string,unknown>={action,version,reason:fields.get("reason")};
 const copy=(...names:string[])=>names.forEach(name=>command[name]=fields.get(name)??"");
 if(action==="configure")copy("approver_id");
 if(action==="rfq"){copy("supplier_id","due_date","rfq_terms");if(offer)command.offer_id=offer.id;}
 if(["dispatch","no_bid","quote","propose"].includes(action))command.offer_id=offer?.id;
 if(action==="dispatch"||action==="release")copy("sent_date","channel","dispatch_reference");
 if(action==="quote"){copy("quote_reference","evidence_reference","valid_until","discount","tax","freight","other","payment_terms","warranty","compliance_notes");command.pricing_mode=String(fields.get("pricing_mode")||"unit_price");copy(command.pricing_mode==="line_total"?"line_total":"unit_price");command.delivery_days=Number(fields.get("delivery_days"));command.compliant=fields.get("compliant")==="yes";}
 if(action==="propose")copy("loi_terms");
 if(action==="decide")command.approve=fields.get("approve")==="yes";
 return command;
}

export function sourcingError(detail:unknown,fallback:string):string{
 if(typeof detail==="string")return detail;
 if(Array.isArray(detail)){const messages=detail.slice(0,5).flatMap(issue=>issue&&typeof issue.msg==="string"?[issue.msg]:[]);if(messages.length)return messages.join("; ");}
 return fallback;
}
