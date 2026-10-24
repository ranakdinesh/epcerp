import type {Decision,SupplierDocument,Verification} from "./supplier-types";
export const documentTypeNames:Record<string,string>={pan:"PAN",cheque:"Cancelled cheque",gst:"GST",aadhar:"Aadhaar",other:"Other"};
export type DocumentReviewRow={id:string;title:string;type:string;status:"Missing"|"Pending"|"Approved"|"Rejected"|"Archived";document?:SupplierDocument;comment?:string};
export function currentDocumentDecisions(status:Verification|null):Decision[]{
 return status?.review?.fingerprint===status?.fingerprint?status?.review?.decisions??[]:[];
}
export function documentReviewRows(documents:SupplierDocument[],status:Verification|null,showArchived=false):DocumentReviewRow[]{
 const decisions=currentDocumentDecisions(status);
 return [
 ...documents.filter(d=>d.is_active||showArchived).map(d=>{const decision=decisions.find(v=>v.document_id===d.id)??(["accept","reject"].includes(d.approval_state??"")?{decision:d.approval_state,comment:d.approval_comment}:undefined);return {id:d.id,title:d.title||d.original_filename,type:d.doc_type,document:d,comment:decision?.comment,status:!d.is_active?"Archived":decision?.decision==="accept"?"Approved":decision?.decision==="reject"?"Rejected":"Pending"} as DocumentReviewRow;}),
 ...(status?.missing_types??[]).map(type=>({id:`missing-${type}`,title:"Not uploaded",type,status:"Missing" as const}))
 ];
}
export function documentDecisionUpdate(status:Verification,documentId:string,decision:"accept"|"reject",comment:string){
 return {version:status.review?.version??0,fingerprint:status.fingerprint,decisions:[...currentDocumentDecisions(status).filter(d=>d.document_id!==documentId),{document_id:documentId,decision,comment}],reason:`${decision==="accept"?"Approved":"Rejected"} supplier document: ${comment}`.slice(0,2000)};
}
