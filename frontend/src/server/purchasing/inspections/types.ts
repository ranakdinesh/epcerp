import type { PackageRecord } from '../core/package-store';
export type Capabilities={read:boolean;write:boolean;scoped:boolean;work:boolean};
export type Facts={project_id:string;package_id:string;order_id:string;order_version:number;number:string;date:string;supplier_id:string;supplier_name:string;description:string;quantity:string;uom:string;acceptance_id:string;acceptance_version:number;technical_id:string;technical_version:number;technical_revision:number;qap_reference:string;qap_date:string;qap_file_id:string;signed_approval_file_id:string};
export type Source=Facts&{available:boolean;blockers:string[]};
export type Point={code:string;title:string;is_witness:boolean;is_cleared:boolean;remarks:string};
export type Inspection={status:string;call_notice_ref:string;call_date:string;scheduled_date:string;actual_date:string;inspection_agency:string;inspector_name:string;factory_location:string;witness_points:Point[];no_witness_points_reason:string;findings:string};
export type InspectionInput=Omit<Inspection,'witness_points'>&{witness_points:(Omit<Point,'is_witness'|'is_cleared'>&{is_witness:boolean|null;is_cleared:boolean|null})[]};
export type Record=PackageRecord&{revision:number;source_snapshot:Source;inspection:Inspection};
export type Command={version:number;action:string;reason:string;inspection:InspectionInput|null};
