export type TechnicalAction = "configure" | "document" | "submit" | "team_review" | "client_approval" | "revise";
export type TechnicalFile = {id:string;case_id:string;purpose:string;name:string;media_type:string;size:number;is_active:boolean};
export type TechnicalDocument = {id:string;kind:string;title:string;category:string;standard:string;required:boolean;status:string;revision:string;received_date:string;file_id:string;notes:string;na_reason:string;is_active:boolean};
export type TechnicalSubmission = {revision:number;reference:string;date:string;target_date:string;authority:string;file_id:string;submitted_by_id:string};
export type TechnicalApproval = {revision:number;status:string;reference:string;date:string;approver_name:string;approver_designation:string;file_id:string;qap_approved:boolean;conditions_closed:boolean;closure_file_id:string;remarks:string};
export type TechnicalWorkspace = {
 case:null | {id:string;version:number;revision:number;state:string;documents:TechnicalDocument[];submission:TechnicalSubmission|null;team_review:null|{reviewer_id:string;approved:boolean;remarks:string};approval:TechnicalApproval|null};
 source:{supplier_name?:string;loi_reference:string;description:string;quantity:string;currency:string;total:string;project_available:boolean;blockers:string[]};
 actions:TechnicalAction[];files:TechnicalFile[];history:{id:string;action:string;reason:string;actor_id:string;at:string}[];ready:boolean;task_only:boolean;
};
export const technicalLabels:Record<TechnicalAction,string>={configure:"Create technical kit",document:"Save document",submit:"Submit full kit",team_review:"Record team review",client_approval:"Record client decision",revise:"Revise technical kit"};
export function technicalError(detail:unknown,fallback:string):string{return typeof detail==="string"?detail:fallback;}
export function technicalCommand(action:TechnicalAction,version:number,f:FormData,doc?:TechnicalDocument){
 const text=(key:string)=>String(f.get(key)??"");
 const base={action,version,reason:text("reason")};
 if(action==="document")return {...base,document:{id:doc!.id,kind:doc!.kind,title:text("title"),category:text("category"),standard:text("standard"),required:text("required")==="yes",is_active:text("is_active")==="yes",status:text("status"),revision:text("revision"),received_date:text("received_date"),file_id:text("file_id"),notes:text("notes"),na_reason:text("na_reason")}};
 if(action==="submit")return {...base,submission:{reference:text("reference"),date:text("date"),target_date:text("target_date"),authority:text("authority"),file_id:text("file_id")}};
 if(action==="team_review")return {...base,approve:text("approve")==="yes",remarks:text("remarks")};
 if(action==="client_approval")return {...base,qap_approved:text("qap_approved")==="yes",conditions_closed:text("conditions_closed")==="yes",approval:{status:text("status"),reference:text("reference"),date:text("date"),approver_name:text("approver_name"),approver_designation:text("approver_designation"),file_id:text("file_id"),closure_file_id:text("closure_file_id"),remarks:text("remarks")}};
 return base;
}
