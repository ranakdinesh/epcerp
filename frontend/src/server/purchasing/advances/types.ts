export type Capabilities={read:boolean;write:boolean;work:boolean;review:boolean;disburse:boolean;scoped:boolean};
export type Facts={project_id:string;package_id:string;order_id:string;order_version:number;number:string;date:string;supplier_id:string;supplier_name:string;currency:string;total:string;payment_terms:string;acceptance_id:string;acceptance_version:number;supplier_verification:string;calculation_basis:string};
export type Source=Facts&{available:boolean;blockers:string[]};
export type Details={kind:string;request_no:string;request_date:string;advance_pct:string;proforma_invoice_ref:string;proforma_invoice_date:string;bank_account_ref:string;notes:string};
export type Payment={voucher_ref:string;disbursed_date:string;notes:string};
export type Record={id:string;project_id:string;package_id:string;version:number;state:string;source_snapshot:Source;details:Details;amount:string;submitted_by_id:string;submitted_at:string|null;submitted_version:number;decision_id:string;reviewed_by_id:string;reviewed_at:string|null;payment:Payment|null;is_active:boolean;created_at:string;updated_at:string;created_by_id:string;updated_by_id:string};
export type Command={version:number;action:string;reason:string;details:Details|null;payment:Payment|null};
