"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { ArrowLeftIcon, MapIcon, ClipboardDocumentListIcon, CheckCircleIcon } from "@heroicons/react/24/outline";
import { filterProcessSteps, processStepHref, processMapHref, type ProcessDefinition, type ProcessStep } from "./process-directory";

// Rendered inside the shared application shell: no duplicate account/theme/sidebar.
export function ProcessNavigation({ process, selected, mapSelected, projectId, draftId, workPackages=false, close }: {
  process: ProcessDefinition; selected?: ProcessStep; mapSelected: boolean; projectId?: string; draftId?: string; workPackages?: boolean; close: () => void;
}) {
  const [query, setQuery] = useState("");
  const [completion,setCompletion]=useState<{project?:string;steps:number[];prebidSkipped?:boolean;repliesSkipped?:boolean}>({steps:[]});
  const completed=completion.project===(projectId??draftId)?completion.steps:[];
  useEffect(()=>{
    if(process.screen==="purchase-fms"&&projectId){
      let controller:AbortController;
      const refresh=()=>{controller?.abort();controller=new AbortController();const active=controller;fetch(`/api/purchase-fms/projects/${encodeURIComponent(projectId)}`,{cache:"no-store",signal:active.signal}).then(async r=>{if(!r.ok)throw Error();return r.json()}).then(d=>{if(!active.signal.aborted)setCompletion({project:projectId,steps:d.project.completed_steps??[]})}).catch(()=>{if(!active.signal.aborted)setCompletion({project:projectId,steps:[]})});};
      refresh();window.addEventListener("purchase-progress-saved",refresh);window.addEventListener("focus",refresh);
      return()=>{controller?.abort();window.removeEventListener("purchase-progress-saved",refresh);window.removeEventListener("focus",refresh)};
    }
    if(process.screen==="tender-fms"&&draftId){
      let controller:AbortController;
      const refresh=()=>{controller?.abort();controller=new AbortController();const active=controller;fetch(`/api/tender-fms/drafts/${encodeURIComponent(draftId)}/flow-status`,{cache:"no-store",signal:active.signal}).then(async r=>{if(!r.ok)throw Error();return r.json()}).then(d=>{if(!active.signal.aborted)setCompletion({project:draftId,steps:d.completed_steps??[],prebidSkipped:d.prebid_review_skipped,repliesSkipped:d.prebid_replies_skipped})}).catch(()=>{if(!active.signal.aborted)setCompletion({project:draftId,steps:[]})});};
      refresh();window.addEventListener("tender-progress-saved",refresh);window.addEventListener("focus",refresh);
      return()=>{controller?.abort();window.removeEventListener("tender-progress-saved",refresh);window.removeEventListener("focus",refresh)};
    }
  },[process.screen,projectId,draftId]);
  const steps = filterProcessSteps(process, query);
  const groups = [...new Set(steps.map(step => step.group))];
  const dashboard = process.screen === "purchase-fms" ? "purchase" : process.screen === "tender-fms" ? "tender" : null;
  const backLabel = dashboard ? `Back to ${dashboard} dashboard` : "Back to workspace";
  return <>
    <Link href={dashboard ? `/dashboard?screen=${process.screen}` : "/dashboard"} title={backLabel}
      aria-label={backLabel} onClick={close}><ArrowLeftIcon aria-hidden="true" /><span>{backLabel}</span></Link>
    <div className="go-process-heading"><strong>{process.label}</strong></div>
    <Link href={processMapHref(process,projectId)} title="Process map" aria-label="Process map" aria-current={mapSelected ? "page" : undefined} onClick={close}><MapIcon aria-hidden="true" /><span>Process map</span></Link>
    {process.screen==="purchase-fms"&&projectId&&<Link href={`/dashboard?screen=purchase-fms&project=${encodeURIComponent(projectId)}&work=packages`} title="Purchase work packages" aria-label="Purchase work packages" aria-current={workPackages?"page":undefined} onClick={close}><ClipboardDocumentListIcon aria-hidden="true"/><span>Purchase work packages</span></Link>}
    <ValidatedLabel className="go-step-search">Find a step<ValidatedInput type="search" value={query} onChange={event => setQuery(event.target.value)} placeholder="Search steps…" /></ValidatedLabel>
    {groups.map(group => <div key={group} className="go-process-group"><p>{group}</p>{steps.filter(step => step.group === group).map(step => {
      const href = processStepHref(process, step,projectId,draftId);
      const skipped = process.screen === "tender-fms" && Boolean(draftId) &&
        completion.project === draftId &&
        ((step.code === "LG-03" && completion.prebidSkipped) ||
         (step.code === "LG-04" && completion.repliesSkipped));
      const done = !skipped && completed.includes(step.sourceNumber ?? step.number);
      const content = <><span className="go-step-number" aria-hidden="true">{done ? <CheckCircleIcon className="text-green-600" aria-hidden="true" /> : step.number || "R"}</span><span className="go-step-label">{step.label}{skipped && <small>Skipped</small>}{!step.available && <small>Planned</small>}</span></>;
      return href ? <Link key={step.code} href={href} title={`${step.number ? step.number + ". " : ""}${step.label}`} aria-label={`${step.number ? step.number + ". " : ""}${step.label}${skipped ? " · Skipped" : done ? " · Completed" : ""}`} aria-current={!mapSelected && !workPackages && selected?.code === step.code ? "page" : undefined} onClick={close}>{content}</Link>
        : <div key={step.code} className="go-planned-step" aria-disabled="true" title={`${step.number}. ${step.label} · Planned`}>{content}</div>;
    })}</div>)}
    {!steps.length && <div className="go-no-steps" role="status">No matching steps.</div>}
  </>;
}
