import type { PackageRecord } from '../core/package-store';
export type Source={project_id:string;package_id:string;inspection_id:string;inspection_version:number;inspection_revision:number;actual_date:string;order_id:string;order_version:number;number:string;supplier_name:string;description:string;quantity:string;uom:string;qap_reference:string;available:boolean;blockers:string[]};
export type Report={status:string;ir_number:string;irn_number:string;issue_date:string;issuing_authority:string;cleared_quantity:string;uom:string;file_name:string};
export type Record=PackageRecord&{revision:number;source_snapshot:Source;report:Report};
export type Command={version:number;action:string;reason:string;report:Omit<Report,'uom'>|null};
