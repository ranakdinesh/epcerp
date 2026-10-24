export type ReceiptFile = { file_id:string;client_file_key:string;filename:string;version:number;state:string;size_bytes:number;sha256:string|null;error_code:string|null };
export type Receipt = {id:string;version:number;state:string;error_code:string|null;expires_at:string;files:ReceiptFile[];member_count:number;unresolved_count:number};
export type CapturedTender = {can_edit:boolean;can_assign:boolean;manager_display_name:string;id:string;name:string;manager_id:string;business_version:number;content_revision:number;assignment_version:number;provisioning_state:string;blocker_code:string|null;project_id:string|null;estimate_id:string|null;deliveries:{source_action_id:string;notice_id:string|null;email_job_id:string|null;email_state:string}[]};
export type Candidate = {id:string;display_name:string};
export function packageFileError(files:readonly Pick<File,"name"|"size">[]):string {
 if (!files.length) return "Select tender documents or a ZIP file.";
 if (files.length>100) return "Select at most 100 files.";
 let total=0;const names=new Set<string>();
 for (const file of files) {const zip=/\.zip$/i.test(file.name);if(!/\.(pdf|docx|xlsx|zip)$/i.test(file.name))return "Use PDF, DOCX, XLSX or ZIP. Convert older Office documents before uploading.";if(file.size<1||file.size>(zip?64:32)*1024*1024)return zip?"Each ZIP must be nonempty and no larger than 64 MiB.":"Each document must be nonempty and no larger than 32 MiB.";const name=file.name.toLowerCase();if(names.has(name))return "Rename duplicate filenames before uploading.";names.add(name);total+=file.size;}
 return total>250*1024*1024?"Selected files exceed the 250 MiB package limit.":"";
}
export function verificationMessage(receipt:Receipt):string {
 if(receipt.state==="ready")return `Verified · ${receipt.member_count} document${receipt.member_count===1?"":"s"}`;
 if(receipt.state==="failed")return receipt.error_code==="dependency_unavailable"?"Document scanning is unavailable. Ask your administrator to restore scanning, then retry verification.":receipt.error_code==="malware_detected"?"This package was quarantined by malware inspection. Do not reuse it; contact your administrator.":"Verification failed. Check the documents, archive contents and supported limits before retrying.";
 if(receipt.state==="expired")return "This upload receipt has expired. Prepare the files again.";
 return receipt.state==="verifying"?"Checking file integrity, archive safety and malware…":"Upload the declared files to continue.";
}
