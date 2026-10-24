export type InspectionStatus="not_scheduled"|"call_received"|"scheduled"|"cleared"|"rejected";
export const inspectionLabels:Record<InspectionStatus,string>={not_scheduled:"Not scheduled",call_received:"Vendor call received",scheduled:"Scheduled",cleared:"Cleared",rejected:"Rejected"};
export const agencyLabels:Record<string,string>={client_consultant:"Client / Consultant",tpi:"Third Party Inspection",internal_qc:"Internal Quality Assurance"};
export type WitnessPoint={code:string;title:string;is_witness:boolean;is_cleared:boolean;remarks:string};
export type Inspection={status:InspectionStatus;call_notice_ref:string;call_date:string;scheduled_date:string;actual_date:string;inspection_agency:string;inspector_name:string;factory_location:string;witness_points:WitnessPoint[];no_witness_points_reason:string;findings:string};
export type InspectionSource={project_id:string;package_id:string;order_id:string;order_version:number;number:string;date:string;supplier_id:string;supplier_name:string;description:string;quantity:string;uom:string;acceptance_id:string;acceptance_version:number;technical_id:string;technical_version:number;technical_revision:number;qap_reference:string;qap_date:string;qap_file_id:string;signed_approval_file_id:string;available:boolean;blockers:string[]};
export type InspectionRecord={id:string;project_id:string;package_id:string;version:number;revision:number;source_snapshot:InspectionSource;inspection:Inspection;is_active:boolean;created_at:string;created_by_id:string;updated_at:string;updated_by_id:string};
export type InspectionEvent={id:string;reason:string;action:"record"|"revise";before:InspectionRecord|null;after:InspectionRecord;at:string;actor_id:string};
export type InspectionData={record:InspectionRecord|null;source:InspectionSource;actions:string[];history:InspectionEvent[];task_only:boolean};
export function inspectionCommand(version:number,action:"record"|"revise",f:FormData,points:WitnessPoint[]){
 const text=(name:string)=>String(f.get(name)??"");
 const base={version,action,reason:text("reason")};
 if(action==="revise")return base;
 return {...base,inspection:{status:text("status"),call_notice_ref:text("call_notice_ref"),call_date:text("call_date"),scheduled_date:text("scheduled_date"),actual_date:text("actual_date"),inspection_agency:text("inspection_agency"),inspector_name:text("inspector_name"),factory_location:text("factory_location"),witness_points:points.map(p=>({code:p.code,title:p.title,is_witness:p.is_witness,is_cleared:p.is_cleared,remarks:p.remarks})),no_witness_points_reason:points.length?"":text("no_witness_points_reason"),findings:text("findings")}};
}
