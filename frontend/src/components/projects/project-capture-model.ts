export const captureNames=["short_name","project_type","site_name","site_state","site_city","scope_summary","procurement_executive_id","contract_value","currency","loi_number","po_number","schedule_start_basis","planned_start_date","planned_completion_date","loi_date","po_date","actual_start_date","actual_end_date","site_handover_date","planned_duration_days","safety_buffer_percent","safety_buffer_days","remarks"] as const;
export const metadataNames=captureNames.filter(n=>n!=="procurement_executive_id"&&n!=="planned_duration_days"&&n!=="safety_buffer_days");
export function metadataPayload(data:FormData,version:number){
 return {purchase_enabled:data.get("purchase_enabled")==="on",name:data.get("name")??"",client_id:data.get("client_id")??"",reason:data.get("reason")??"",expected_version:version,...Object.fromEntries(metadataNames.map(n=>[n,data.get(n)??""]))};
}
