export type OrderAction="prepare"|"release"|"submit_client"|"client_decision";
export type OrderDetails={date:string;issuer_name:string;issuer_address:string;issuer_tax_reference:string;issuer_registration:string;issuer_email:string;bill_to:string;delivery_address:string};
export type OrderSource={project_id:string;package_id:string;project_name:string;site:string;sourcing_id:string;sourcing_version:number;loi_reference:string;loi_date:string;financial_reference:string;supplier_verification:string;supplier:{id:string;name:string;code:string;address:string;contact:string;email:string;phone:string;tax_reference:string;registration:string};technical:{case_id:string;version:number;revision:number;reference:string;approval_date:string;approval_file_id:string;qap_approved:boolean;conditions_closed:boolean};item_id:string;sku:string;description:string;uom:string;quantity:string;currency:string;pricing_mode:string;unit_price:string;base:string;discount:string;tax:string;freight:string;other:string;total:string;delivery_days:number;payment_terms:string;warranty:string;loi_terms:string;available:boolean;blockers:string[]};
export type ClientPOApproval={status:"not_submitted"|"submitted"|"approved"|"rejected";draft_revision:number;submission?:{reference:string;date:string};decision?:{status:string;reference:string;date:string;signatory:string;designation:string;file_id:string};recorded_by_id:string;recorded_at?:string};
export type PurchaseOrder={draft_revision:number;client_approval?:ClientPOApproval;id:string;project_id:string;package_id:string;version:number;state:"draft"|"released";number:string;source_snapshot:OrderSource;details:OrderDetails;released_at:string|null;released_by_id:string;is_active:boolean;created_at:string;updated_at:string;created_by_id:string;updated_by_id:string};
export type OrderWorkspace={order:PurchaseOrder|null;source:OrderSource;actions:OrderAction[];history:{id:string;order_id:string;action:OrderAction;reason:string;at:string;actor_id:string}[];task_only:boolean};
export function purchaseOrderCommand(action:OrderAction,version:number,f:FormData){
 const text=(name:string)=>String(f.get(name)??"");
 const base={action,version,reason:text("reason")};
 if(action==="release")return base;
 if(action==="submit_client")return {...base,submission:{reference:text("reference"),date:text("date")}};
 if(action==="client_decision")return {...base,client_decision:{status:text("status"),reference:text("reference"),date:text("date"),signatory:text("signatory"),designation:text("designation"),file_id:text("file_id")}};
 const details:OrderDetails={date:text("date"),issuer_name:text("issuer_name"),issuer_address:text("issuer_address"),issuer_tax_reference:text("issuer_tax_reference"),issuer_registration:text("issuer_registration"),issuer_email:text("issuer_email"),bill_to:text("bill_to"),delivery_address:text("delivery_address")};
 return {...base,details};
}
