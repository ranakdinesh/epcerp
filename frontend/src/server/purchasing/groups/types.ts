import type { Line as BOLine } from '../bo/types';
import type { purchaseVendors } from '../../master/vendors/purchase-reader';
export type Vendor=Awaited<ReturnType<typeof purchaseVendors>>[number];
export type Line=Omit<BOLine,'item_snapshot'>&{item_snapshot:Omit<BOLine['item_snapshot'],'demand_version'>};
export type Invitation={vendor_id:string;scope_revision:number;line_ids:string[];sent_date:string;channel:string;dispatch_reference:string;recorded_by_id:string;recorded_at:string};
export type GroupInput={id:string;name:string;line_ids:string[];vendor_ids:string[]};
export type Group=GroupInput&{invitations:Invitation[];scope_revision:number};
export type GroupRecord={bo_id:string;project_id:string;version:number;state:string;bo_version:number;groups:Group[];source_lines:Line[];number:string;source_vendors:Vendor[];selected_line_ids:string[];is_active:boolean;created_at:string;updated_at:string;created_by_id:string|null;updated_by_id:string|null};
export type Command={version:number;bo_version:number;action:string;reason:string;groups:GroupInput[];group_id:string|null;line_id:string|null;line_ids:string[];vendor_ids:string[];selected_line_ids:string[];vendor_id:string|null;sent_date:string|null;channel:string;dispatch_reference:string;invitation_scope_revision?:number;invitation_line_ids?:string[]};
