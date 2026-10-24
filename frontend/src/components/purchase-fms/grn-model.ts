export type GRNStatus="pending"|"issued";
export const grnLabels:Record<GRNStatus,string>={pending:"Pending GRN",issued:"Issued GRN"};
export type GRN={status:GRNStatus;grn_number:string;grn_date:string;received_qty:string;accepted_qty:string;rejected_qty:string;uom:string;store_keeper_name:string;remarks:string;warranty_ref:string;warranty_valid_until:string;file_name:string};
export type GRNSource={receipt_id:string;receipt_version:number;receipt_revision:number;arrival_date:string;receiving_officer:string;packages_received:number;packages_condition:string;transit_damage_notes:string;gate_entry_ref:string;dispatch_id:string;dispatch_version:number;dispatch_revision:number;dispatch_date:string;lr_number:string;transporter_name:string;project_id:string;package_id:string;clearance_id:string;clearance_version:number;clearance_revision:number;mdcc_ref:string;clearance_date:string;quality_id:string;quality_version:number;quality_revision:number;report_id:string;report_version:number;report_revision:number;ir_number:string;irn_number:string;issue_date:string;number:string;supplier_name:string;description:string;cleared_quantity:string;uom:string;available:boolean;blockers:string[]};
export type GRNRecord={id:string;project_id:string;package_id:string;version:number;revision:number;source_snapshot:GRNSource;grn:GRN;is_active:boolean;created_at:string;updated_at:string;created_by_id:string;updated_by_id:string};
export type GRNEvent={id:string;reason:string;action:string;before:GRNRecord|null;after:GRNRecord;at:string;actor_id:string};
export type GRNData={record:GRNRecord|null;source:GRNSource;actions:string[];history:GRNEvent[];task_only:boolean};
export const grnDecimalPattern="(0|[1-9][0-9]{0,11})(\\.[0-9]{1,6})?";
function units(v:string):bigint|null{if(!/^(0|[1-9][0-9]{0,11})(\.[0-9]{1,6})?$/.test(v))return null;const [whole,fraction=""]=v.split(".");return BigInt(whole)*BigInt(1000000)+BigInt(fraction.padEnd(6,"0"));}
export function grnQuantityErrors(status:string,received:string,accepted:string,rejected:string,allowance:string):Record<string,string>{
 const errors:Record<string,string>={};const values={received_qty:received,accepted_qty:accepted,rejected_qty:rejected};
 for(const[key,value]of Object.entries(values)){if(value&&units(value)===null)errors[key]="Use a nonnegative decimal with at most six decimal places and twelve integral digits.";}
 if(status!=="issued")return errors;
 const r=units(received),a=units(accepted||"0"),j=units(rejected||"0"),limit=units(allowance);
 if(!received||r!==null&&r<=BigInt(0))errors.received_qty="Issued GRN requires a positive received quantity.";
 if(r!==null&&limit!==null&&r>limit)errors.received_qty="Received quantity cannot exceed the current cleared allowance.";
 if(r!==null&&a!==null&&j!==null&&a+j>r)errors.accepted_qty="Accepted and rejected quantities together cannot exceed received quantity.";
 return errors;
}
export function grnCommand(version:number,action:"record"|"revise",f:FormData){const text=(name:string)=>String(f.get(name)??"");const base={version,action,reason:text("reason")};if(action==="revise")return base;return {...base,grn:{status:text("status"),grn_number:text("grn_number"),grn_date:text("grn_date"),received_qty:text("received_qty")||"0",accepted_qty:text("accepted_qty")||"0",rejected_qty:text("rejected_qty")||"0",store_keeper_name:text("store_keeper_name"),remarks:text("remarks"),warranty_ref:text("warranty_ref"),warranty_valid_until:text("warranty_valid_until"),file_name:text("file_name")}};}
