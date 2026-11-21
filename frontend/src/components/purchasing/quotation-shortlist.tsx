"use client";
import {useId,useState,type ReactNode} from "react";
import {DocumentArrowDownIcon} from "@heroicons/react/24/outline";
import {ValidatedForm,ValidatedLabel,ValidatedInput,ValidatedTextarea} from "@/components/ui/validated-controls";
import type {Comparison} from "./group-quotation-workspace";
import type {ApprovalWorkspace,Attempt} from "./quotation-approval-controls";
import {quotationMoney} from "./quotation-money";
import {lowestRecordedQuotes,quoteKey} from "./quotation-comparison-selection";
import {DataGrid,GridAction} from "@/components/ui/data-grid";
export type Nomination={quote_key:string;best_price:boolean;preferred:boolean;best_price_comment:string;preferred_comment:string};
const button="min-h-11 rounded-lg border px-4 py-2 text-sm font-semibold disabled:opacity-50";
export function ComparisonPrint({url,group,attempt,keys}:{url:string;group:string;attempt?:string;keys?:string[]}){return <a className={`quotation-action ${button} inline-flex items-center gap-2`} href={`${url}/print?group_id=${group}${attempt?`&attempt_id=${attempt}`:keys?`&quote_keys=${encodeURIComponent(keys.join(","))}`:""}`} title="Download landscape PDF"><DocumentArrowDownIcon className="h-5 w-5"/>Print PDF</a>;}
export function FinanceSubmissionStatus({attempt}:{attempt:Attempt}) {
 const sentAt=new Intl.DateTimeFormat("en-IN",{dateStyle:"medium",timeStyle:"short",timeZone:"Asia/Kolkata"}).format(new Date(attempt.submitted_at));
 return <p role="status" className="quotation-success">
  <strong>{attempt.group_name} was sent to Finance for approval.</strong>{" "}
  Awaiting Finance review. Submitted <time dateTime={attempt.submitted_at}>{sentAt} IST</time>.
 </p>;
}
export function GroupNominations({quotes,revisions,renderGrid,attempt,busy,canSubmit,onSave,onSend,receiptHref,initial=[]}:{quotes:Comparison[];revisions:Comparison[];renderGrid:(footer?:ReactNode)=>ReactNode;attempt?:Attempt;receiptHref?:string;busy:boolean;canSubmit:boolean;initial?:Nomination[];onSave:(rows:Nomination[])=>void|Promise<void>;onSend:(rows:Nomination[],reason:string)=>void|Promise<void>}){
 const locked=attempt&&["pending_accounts","approved"].includes(attempt.effective_state);
 const saved=locked?attempt.proposals??initial:initial;
 const savedBest=saved.find(n=>n.best_price);
 const savedPreferred=saved.find(n=>n.preferred);
 const [preferredChoice,setPreferred]=useState(savedPreferred?.quote_key??"");
 const [lowestChoice,setLowest]=useState<{automatic:string;key:string}>();
 const group=quotes[0].group_id;
 const eligibleLowest=new Set(revisions.filter(q=>q.group_id===group&&q.eligible&&q.suggested).map(quoteKey));
 const visibleLowest=quotes.filter(q=>eligibleLowest.has(quoteKey(q)));
 const candidates=(eligibleLowest.size?visibleLowest:lowestRecordedQuotes(quotes)).map(quoteKey);
 const automatic=(savedBest&&candidates.includes(savedBest.quote_key)?savedBest.quote_key:candidates[0])??"";
 const best=locked?(savedBest?.quote_key??automatic):
  lowestChoice?.automatic===automatic&&(!lowestChoice.key||candidates.includes(lowestChoice.key))?lowestChoice.key:automatic;
 const preferred=locked?(savedPreferred?.quote_key??""):preferredChoice;
 const preferredQuote=quotes.find(q=>quoteKey(q)===preferred);
 const bestQuote=quotes.find(q=>quoteKey(q)===best);
 const blockerId=useId();
 const selections=[{role:"Lowest quote",quote:bestQuote},{role:"Preferred partner",quote:preferredQuote}];
 const exclusions=selections.flatMap(({role,quote})=>quote&&!quote.eligible?[{role,quote}]:[]);
 const blockers=[
  ...(!bestQuote?[eligibleLowest.size?"Select the lowest eligible quotation revision.":"No quotation in this group is eligible yet."]:[]),
  ...(!preferredQuote?["Select a preferred partner."]:[]),
  ...(!canSubmit&&!busy?["Freeze vendor quotations before submitting. If they are already frozen, check your submission access."]:[]),
 ];
 const showBlockers=exclusions.length>0||blockers.length>0;
 const canSave=!busy&&canSubmit&&!locked&&eligibleLowest.has(best)&&Boolean(preferredQuote?.eligible);
 const rows=(["Lowest quote","Preferred partner"] as const).map(label=>{
  const isLowest=label==="Lowest quote";
  return <tr key={label}>
   <th scope="row" colSpan={2} className="px-4 py-2 text-left">{label}</th>
   {quotes.map(q=>{
    const key=quoteKey(q);
    return <td key={key} className="px-4 text-center"><ValidatedLabel className="inline-flex min-h-11 min-w-11 items-center justify-center">
     <ValidatedInput type="checkbox" className="accent-blue-700" style={{width:18,height:18,minHeight:0,padding:0,margin:0}} aria-label={`${label} · ${q.vendor}`} checked={isLowest?best===key:preferred===key} disabled={busy||!!locked||(isLowest&&!candidates.includes(key))} onChange={event=>{
      const choice=event.target.checked?key:"";
      if(isLowest)setLowest({automatic,key:choice});
      else setPreferred(choice);
     }}/>
    </ValidatedLabel></td>;
   })}
  </tr>;
 });
 return <ValidatedForm className="min-w-0 space-y-3" onSubmit={event=>{
  event.preventDefault();
  if(!canSave||!best)return;
  const form=new FormData(event.currentTarget);
  const bestReason=String(form.get("best_comment")).trim();
  const preferredReason=String(form.get("preferred_comment")).trim();
  const proposals=[...new Set([best,preferred])].map(key=>({
   quote_key:key,best_price:key===best,preferred:key===preferred,
   best_price_comment:key===best?bestReason:"",preferred_comment:key===preferred?preferredReason:"",
  }));
  if((event.nativeEvent as SubmitEvent).submitter?.getAttribute("value")==="send") void onSend(proposals,preferredReason);
  else void onSave(proposals);
 }}>
 {renderGrid(rows)}
 <div className="space-y-2 rounded-xl border bg-white p-3 text-sm">
  <h4 className="text-sm font-bold">Shortlist · {quotes[0].group_name}</h4>
  {locked?<p role="status" className={attempt.effective_state==="pending_accounts"?"quotation-warning":"quotation-success"}>{attempt.effective_state==="pending_accounts"?"Sent to Finance · awaiting approval":"Finance approved this group."}</p>:<>
   <div className="grid gap-2 md:grid-cols-2"><ValidatedLabel required>Best-price reason<ValidatedInput key={best} name="best_comment" defaultValue={savedBest&&savedBest.quote_key===best?savedBest.best_price_comment:""} required maxLength={2000} disabled={busy} className="min-h-11 w-full rounded-lg border p-3"/></ValidatedLabel><ValidatedLabel required>Preferred-offer reason<ValidatedInput key={preferred} name="preferred_comment" defaultValue={savedPreferred?.quote_key===preferred?savedPreferred.preferred_comment:""} required maxLength={2000} disabled={busy} className="min-h-11 w-full rounded-lg border p-3"/></ValidatedLabel></div>
   {showBlockers&&<div id={blockerId} role="status" className="quotation-warning">
    <p className="font-bold">Approval is blocked for this group</p>
    <ul className="mt-1 list-disc space-y-1 pl-5">
     {exclusions.map(({role,quote})=><li key={role}><strong>{role} · {quote.vendor}</strong> — {quote.exclusions.join(". ") || "This quotation is ineligible."}</li>)}
     {blockers.map(blocker=><li key={blocker}>{blocker}</li>)}
    </ul>
    {receiptHref&&exclusions.length>0&&<a className="mt-2 inline-block font-semibold underline" href={receiptHref}>Open Receive vendor quotations</a>}
   </div>}
   <div className="flex flex-wrap justify-end gap-2">
    <button type="submit" value="draft" className={button} disabled={!canSave}>Save shortlist draft</button>
    <button type="submit" value="send" className={`primary ${button}`} aria-describedby={showBlockers?blockerId:undefined} disabled={!canSave}>Send for approval</button>
   </div>
  </>}
  {attempt?.effective_state==="rejected"&&<p role="status" className="quotation-warning">Finance returned this group: {attempt.return_action?.replaceAll("_"," ")}. {attempt.message}</p>}
 </div>
 </ValidatedForm>;
}
export function DraftShortlist({rows,quotes,workspace,url,busy,draft,onSave,onSubmit,onRefresh,onDocuments}:{rows:Nomination[];quotes:Comparison[];workspace:ApprovalWorkspace;url:string;busy:boolean;draft?:{version:number;comment:string};onSave:(p:object)=>Promise<boolean>;onSubmit:(p:object)=>Promise<boolean>;onRefresh:()=>void;onDocuments:(vendor:string)=>void}){
 const [error,setError]=useState(""),[comment,setComment]=useState(draft?.comment??""),[editVersion,setEditVersion]=useState(draft?.version??0),[notice,setNotice]=useState(""),[printing,setPrinting]=useState(false);
 const changed=(draft?.version??0)!==editVersion,group=quotes.find(q=>quoteKey(q)===rows[0]?.quote_key)?.group_id;
 const offers=rows.map(n=>{const quote=quotes.find(q=>quoteKey(q)===n.quote_key);return {...n,id:n.quote_key,quote};});
 const blocked=offers.some(r=>!r.quote?.eligible);
 async function print(){setError("");setPrinting(true);try{const r=await fetch(`${url}/print?group_id=${group}`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({proposals:rows})});if(!r.ok){const d=await r.json();throw Error(d.detail||"Cannot print shortlist")};const href=URL.createObjectURL(await r.blob());const a=document.createElement("a");a.href=href;a.download="shortlisted-quotations.pdf";a.click();URL.revokeObjectURL(href);}catch(e){setError(e instanceof Error?e.message:"Cannot print shortlist");}finally{setPrinting(false);}}
 return <ValidatedForm className="space-y-4 rounded-xl border border-slate-200 bg-white p-4" onSubmit={async e=>{e.preventDefault();if(blocked||changed||busy||!workspace.can_submit)return;await onSubmit({version:workspace.version,reason:comment,proposals:rows});}}>
 <header className="flex flex-wrap items-center justify-between gap-3"><div><h3 className="font-bold">{quotes.find(q=>q.group_id===group)?.group_name} · Shortlist</h3><p className="mt-1 text-xs text-slate-500">{offers.length} {offers.length===1?"offer":"offers"} selected</p></div><div className="flex flex-wrap gap-2"><button type="button" className={button} disabled={busy} onClick={onRefresh}>Refresh status</button><button type="button" className={`${button} inline-flex items-center gap-2`} onClick={print} disabled={printing||!group}><DocumentArrowDownIcon className="h-4 w-4"/>{printing?"Preparing PDF…":"Print PDF"}</button></div></header>
 <DataGrid label="Shortlisted offers" rows={offers} searchable={false} paginate={false} columns={[
 {key:"vendor",label:"Supplier / offer",value:r=>r.quote?.vendor??"Unavailable offer",render:r=><div className="whitespace-normal"><p className="font-semibold">{r.quote?.vendor??"Unavailable offer"}</p><p className="mt-1 text-xs text-slate-500">{r.quote?.reference} · R{r.quote?.revision}</p></div>},
 {key:"selection",label:"Selection",value:r=>[r.best_price?"Lowest price":"",r.preferred?"Preferred":""].filter(Boolean).join(" + "),render:r=><div className="space-y-1 whitespace-normal"><div className="flex flex-wrap gap-1">{r.best_price&&<span className="rounded-full bg-green-50 px-2 py-1 text-xs font-semibold text-green-800">Lowest price</span>}{r.preferred&&<span className="rounded-full bg-yellow-50 px-2 py-1 text-xs font-semibold text-yellow-900">Preferred</span>}</div><details><summary className="cursor-pointer py-2 text-xs text-slate-500">Selection reasons</summary>{r.best_price&&<p className="max-w-80 text-xs text-slate-600">Lowest price: {r.best_price_comment}</p>}{r.preferred&&<p className="mt-1 max-w-80 text-xs text-slate-600">Preferred: {r.preferred_comment}</p>}</details></div>},
 {key:"total",label:"Total incl. tax",className:"text-right",value:r=>r.quote?.total??"",render:r=><span className="font-semibold tabular-nums">{r.quote?quotationMoney(r.quote.total,workspace.currency):"—"}</span>},

 ]} actions={r=>r.quote?.vendor_id?<GridAction kind="view" label={`View documents for ${r.quote.vendor}`} onClick={()=>onDocuments(r.quote!.vendor_id!)}/>:null}/>
 <ValidatedLabel required>Submission comment<ValidatedTextarea className="mt-1 min-h-20 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm" name="reason" disabled={busy||!workspace.can_submit} value={comment} onChange={e=>{setComment(e.target.value);setNotice("");}} required maxLength={2000}/></ValidatedLabel>
 {error&&<p role="alert" className="text-sm text-red-700">{error}</p>}{changed&&<p role="alert" className="text-sm text-amber-800">This draft changed in another session. Refresh before saving or submitting.</p>}
 <footer className="flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 pt-3"><p role="status" className="text-xs text-slate-500">{notice|| (blocked?"Refresh the shortlist and select eligible quotations before submitting.":"Submission sends these offers to Finance for price approval. Supplier documents are verified before PO release.")}</p><div className="flex flex-wrap gap-2"><button type="button" className={button} disabled={busy||!workspace.can_submit||changed} onClick={async()=>{if(await onSave({group_id:group,version:editVersion,proposals:rows,comment})){setEditVersion(editVersion+1);setNotice("Draft saved.");}}}>Save draft</button><button className="primary min-h-11 rounded-lg bg-blue-700 px-5 py-2 font-semibold text-white disabled:opacity-50" disabled={busy||!workspace.can_submit||changed||blocked}>{busy?"Saving…":"Send for approval"}</button></div></footer>
 </ValidatedForm>;
}
