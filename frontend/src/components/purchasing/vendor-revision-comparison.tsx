"use client";
import {PurchaseSectionHeader} from "@/components/purchase-fms/purchase-layout";
import {ComparisonPrint,GroupNominations,DraftShortlist,FinanceSubmissionStatus} from "./quotation-shortlist";
import { useState } from "react";
import { ItemComparisonMatrix } from "./quotation-item-comparison";
import type { Comparison } from "./group-quotation-workspace";
import { draftComparison } from "./quotation-comparison-selection";
import { AccountsReview, VendorFinanceReview, approvalButton, useQuotationApprovals } from "./quotation-approval-controls";

export function VendorRevisionComparison({revisions,money,onDetails,project,boId,onNext}:{revisions:Comparison[];money:(value:string|null|undefined)=>string;onDetails:(quote:Comparison)=>void;project:string;boId:string;onNext:()=>void}) {
 const [tab,setTab]=useState("comparison");
 const [choices,setChoices]=useState<Record<string,string>>({}),[verifying,setVerifying]=useState<string|null>(null);
 const url=`/api/projects/${project}/purchase-bo/${boId}/quotation-approvals`;
 const approval=useQuotationApprovals(url,project);
 const drafts=Object.fromEntries((approval.workspace?.drafts??[]).filter(d=>d.is_active).map(d=>[d.group_id,d.proposals]));
 const savedChoices=Object.fromEntries((approval.workspace?.attempts??[]).map(attempt=>[attempt.quote_key.split(":")[0],attempt.quote_key]));
 const selected=draftComparison(revisions,{...savedChoices,...choices});
 const vendors=[...new Set(revisions.map(quote=>quote.document_id))].map(id=>({id,latest:revisions.filter(quote=>quote.document_id===id).sort((a,b)=>(b.revision??1)-(a.revision??1))[0],chosen:selected.find(quote=>quote.document_id===id)}));
 const sheet=[...selected,...vendors.filter(row=>!row.chosen).map(row=>row.latest)];
 const latest=new Map(approval.workspace?.attempts.map(attempt=>[attempt.group_id,attempt]));
 return <section className="purchase-quotation-comparison min-w-0 space-y-5">
 <PurchaseSectionHeader title="Compare & finalise vendor quotations"/>
 {Array.from(latest.values()).filter(attempt=>attempt.effective_state==="pending_accounts").map(attempt=>
  <FinanceSubmissionStatus key={attempt.id} attempt={attempt}/>)}
 {approval.error&&<p role="alert" className="quotation-error"><strong>Error. </strong>{approval.error}</p>}
 <nav className="purchase-stage-tabs" aria-label="Quotation comparison views"><button className={approvalButton} aria-pressed={tab==="comparison"} onClick={()=>setTab("comparison")}>Comparison</button><button className={approvalButton} aria-pressed={tab==="shortlist"} onClick={()=>setTab("shortlist")}>Shortlisted</button></nav>
 {tab==="shortlist"&&approval.workspace&&<>{Object.entries(drafts).filter(([group])=>!["pending_accounts","approved"].includes(latest.get(group)?.effective_state||"")).map(([group,rows])=><DraftShortlist onRefresh={approval.reload} onDocuments={setVerifying} key={group} rows={rows} quotes={revisions} workspace={approval.workspace!} url={url} busy={approval.busy} draft={approval.workspace!.drafts?.find(d=>d.group_id===group)} onSave={p=>approval.command("draft",p)} onSubmit={p=>approval.command("submit",p)}/>)}{Array.from(latest.values()).map(a=><div className="space-y-3 rounded-xl border p-4" key={a.id}><h3 className="font-bold">{a.group_name} · {a.effective_state.replaceAll("_"," ")}</h3><p>{a.message}</p>{a.review_comment&&<p className="text-sm">Finance notes: {a.review_comment}</p>}{a.lines.map(l=>l.comment&&<p className="text-sm" key={l.line_id}>{l.item.description||l.item.sku}: {l.comment}</p>)}<ComparisonPrint url={url} group={a.group_id} attempt={a.id}/>{(a.proposals||[]).map(p=><p className="text-sm" key={p.quote_key}>{p.snapshot.vendor} · {p.snapshot.total} · {p.best_price?`Best price: ${p.best_price_comment}`:""} · {p.preferred?`Preferred: ${p.preferred_comment}`:""}</p>)}{a.effective_state==="rejected"&&<a className={approvalButton} href={`/dashboard?screen=purchase-fms&project=${project}&step=${a.return_action==="change_vendor"?"PUR-02":"PUR-03"}&bo=${boId}&group=${a.group_id}${a.return_action==="change_vendor"?"&indentTab=vendors":`&quote=${encodeURIComponent(a.quote_key)}`}`}>{a.return_action==="change_vendor"?"Change vendor":"Negotiate quotation"}</a>}</div>)}{Object.keys(drafts).length===0&&latest.size===0&&<p>Choose best-price and preferred offers in Comparison to prepare a shortlist.</p>}</>}
 {tab==="comparison"&&sheet.length>0&&<ItemComparisonMatrix comparisons={sheet} revisions={revisions} money={money} onRevision={(document,key)=>setChoices(current=>({...current,[document]:key}))} onDetails={onDetails} quoteStatus={quote=>{const attempts=approval.workspace?.attempts.filter(attempt=>attempt.quote_key===`${quote.document_id}:${quote.revision??1}`);const attempt=attempts?.at(-1);return attempt?`${attempt.effective_state.replaceAll("_"," ")} — ${attempt.message}`:"Not finalised";}} groupShortlist={(quotes,renderGrid)=>{
  const group=quotes[0].group_id;
  const draft=approval.workspace?.drafts?.find(d=>d.group_id===group);
  return <GroupNominations key={draft?.version??0} initial={drafts[group]} quotes={quotes} revisions={revisions} renderGrid={renderGrid} attempt={latest.get(group)} busy={approval.busy} canSubmit={approval.workspace?.can_submit===true} receiptHref={`/dashboard?screen=purchase-fms&project=${encodeURIComponent(project)}&step=PUR-03&bo=${encodeURIComponent(boId)}`} onSend={async (proposals,reason)=>{
   if(approval.workspace) await approval.command("submit",{version:approval.workspace.version,reason,proposals});
  }} onSave={async rows=>{
   if(await approval.command("draft",{group_id:group,version:draft?.version??0,proposals:rows,comment:draft?.comment??""}))setTab("shortlist");
  }}/>;
 }} groupFooter={quotes=><><ComparisonPrint url={url} group={quotes[0].group_id} keys={quotes.map(q=>`${q.document_id}:${q.revision??1}`)}/>{approval.workspace&&latest.get(quotes[0].group_id)&&<>{approval.workspace.can_review_read?<AccountsReview key={latest.get(quotes[0].group_id)!.id} attempt={latest.get(quotes[0].group_id)!} workspace={approval.workspace} busy={approval.busy} onReview={payload=>approval.command("review",payload)} money={money} printUrl={url}/>:<div className="rounded-xl border bg-white p-4"><h4 className="font-bold">Accounts item decisions</h4>{latest.get(quotes[0].group_id)!.lines.map(line=><p key={line.line_id} className="mt-2 text-sm">{line.item.description??line.item.sku} · {line.decision??"Pending"}{line.comment?` — ${line.comment}`:""}</p>)}</div>}</>}</>}/>}
 {!revisions.length&&<p>No quotations recorded yet. Enter vendor item prices in Step 3.</p>}
 {approval.workspace&&(approval.workspace.ready
  ? <div className="quotation-success flex flex-wrap items-center justify-between gap-3">
     <p role="status"><strong>Finance approved every group.</strong></p>
     <button type="button" className="min-h-11 rounded-lg px-5 py-2 font-semibold" onClick={onNext}>Continue to vendor onboarding &amp; LOI</button>
    </div>
  : <div className="quotation-warning" role="status">
     <p className="font-bold">Finance approval required</p>
     <p className="mt-1">Every group needs Finance approval before vendor onboarding and LOI can begin.</p>
    </div>)}
 {verifying&&<VendorFinanceReview key={verifying} vendor={verifying} vendorName={revisions.find(q=>q.vendor_id===verifying)?.vendor??"Supplier"} url={url} onClose={()=>setVerifying(null)} onSaved={approval.reload}/>}
 </section>;
}
