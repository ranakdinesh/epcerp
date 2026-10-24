import type {PurchaseProject} from "@/components/purchase-fms/data";
import type {PersonalTask} from "@/components/tasks/personal-tasks";
export const projectStates = [
  {key:"draft",label:"Draft",tone:"blue"},
  {key:"active",label:"Active",tone:"teal"},
  {key:"on_hold",label:"On hold",tone:"amber"},
  {key:"closed",label:"Closed",tone:"muted"},
  {key:"inactive",label:"Inactive",tone:"muted"},
] as const;
export function projectStatus(row:PurchaseProject){return row.is_active?row.lifecycle_status:"inactive";}
// Charts describe the displayed server page, never the full scope or completion.
export function projectChart(rows:readonly PurchaseProject[]){
 const counts=new Map<string,number>();
 for(const row of rows){const key=projectStatus(row);counts.set(key,(counts.get(key)??0)+1);}
 return [...projectStates.map(s=>({...s,count:counts.get(s.key)??0})),...[...counts].filter(([key])=>!projectStates.some(s=>s.key===key)).map(([key,count])=>({key,label:key.replaceAll("_"," "),tone:"muted",count}))];
}
export function progressCoverage(rows:readonly PurchaseProject[]){
 const recorded=rows.filter(r=>r.progress_percent!==null&&Number.isFinite(r.progress_percent)&&r.progress_percent>=0&&r.progress_percent<=100).length;
 return {recorded,missing:rows.length-recorded,total:rows.length};
}
export function taskAttention(rows:readonly PersonalTask[]){
 return {overdue:rows.filter(t=>t.is_overdue).length,blocked:rows.filter(t=>t.coordination_state==="blocked"||!t.available).length,
 rows:[...rows].sort((a,b)=>Number(b.is_overdue)-Number(a.is_overdue)||Number(b.coordination_state==="blocked")-Number(a.coordination_state==="blocked"))};
}
export function greetingAt(now:Date){const hour=Number(new Intl.DateTimeFormat("en-IN",{timeZone:"Asia/Kolkata",hour:"numeric",hourCycle:"h23"}).format(now));return hour<12?"Good morning":hour<17?"Good afternoon":"Good evening";}

export function averagePurchaseProgress(rows:readonly PurchaseProject[]){
 const values=rows.map(r=>r.progress_percent).filter((v):v is number=>v!==null&&Number.isFinite(v)&&v>=0&&v<=100);
 return values.length?Math.round(values.reduce((a,b)=>a+b,0)/values.length*100)/100:null;
}
