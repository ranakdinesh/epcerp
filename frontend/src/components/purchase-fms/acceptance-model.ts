export type AcceptanceStatus="pending"|"accepted"|"accepted_with_exceptions";
export type AcceptanceResponse={status:AcceptanceStatus;accepted_date:string;signatory_name:string;document_reference:string;remarks:string};
export type AcceptedOrder={id:string;project_id:string;package_id:string;version:number;number:string;date:string;supplier_id:string;supplier_name:string;description:string;quantity:string;uom:string;currency:string;total:string;payment_terms:string;delivery_days:number;warranty:string;released_at:string;released_by_id:string};
export type AcceptanceRecord={id:string;project_id:string;package_id:string;version:number;order_snapshot:AcceptedOrder;response:AcceptanceResponse;is_active:boolean;created_at:string;updated_at:string;created_by_id:string;updated_by_id:string};
export type AcceptanceEvent={id:string;action:string;reason:string;before:AcceptanceRecord|null;after:AcceptanceRecord;at:string;actor_id:string};
export type AcceptanceWorkspace={record:AcceptanceRecord|null;source:{project_id:string;package_id:string;order:AcceptedOrder|null;available:boolean;blockers:string[]};actions:string[];history:AcceptanceEvent[];task_only:boolean};
export const acceptanceLabels:Record<AcceptanceStatus,string>={pending:"Pending vendor confirmation",accepted:"Accepted without exceptions",accepted_with_exceptions:"Accepted with exceptions"};
export function acceptanceCommand(version:number,f:FormData){
 const text=(name:string)=>String(f.get(name)??"");
 const status=text("status") as AcceptanceStatus;
 return {version,action:"record",reason:text("reason"),response:{status,accepted_date:status==="pending"?"":text("accepted_date"),signatory_name:text("signatory_name"),document_reference:text("document_reference"),remarks:text("remarks")}};
}
export function indiaDate(now=new Date()){
 const parts=new Intl.DateTimeFormat("en-GB",{timeZone:"Asia/Kolkata",year:"numeric",month:"2-digit",day:"2-digit"}).formatToParts(now);
 const value=(type:string)=>parts.find(p=>p.type===type)?.value??"";
 return `${value("year")}-${value("month")}-${value("day")}`;
}
