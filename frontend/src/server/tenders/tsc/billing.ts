/** Only a conclusive Google rejection resolves a hold. Unknown outcomes and timeouts remain reserved. */
export function confirmedRejection(provider:string,status:number,raw:string):boolean{
 if(provider!=='gemini'||![429,503].includes(status))return false;
 try{
  const body=JSON.parse(raw),error=body.error;if(error?.code!==status||body.candidates||body.usageMetadata)return false;
  if(status===503)return error.status==='UNAVAILABLE';
  return error.status==='RESOURCE_EXHAUSTED'&&Array.isArray(error.details)&&error.details.some((d:{'@type'?:string;violations?:{quotaMetric?:string}[]})=>d['@type']==='type.googleapis.com/google.rpc.QuotaFailure'&&d.violations?.length&&d.violations.every(v=>v.quotaMetric?.startsWith('generativelanguage.googleapis.com/generate_content_')));
 }catch{return false}
}
