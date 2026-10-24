"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowRightIcon } from "@heroicons/react/24/outline";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { processForScreen, processHandoffs, processMapHref, processStepHref, type ProcessDefinition, type ProcessStep } from "./process-directory";

type ReferenceRow = ProcessStep & { id: string };
export function ProcessMap({ process, projectId, allowedScreens }: { process: ProcessDefinition; projectId?: string; allowedScreens: readonly string[] }) {
  const [selected, setSelected] = useState<ReferenceRow | null>(null);
  const stages = process.steps.filter(step => step.number > 0).map(step => ({...step, id: step.code}));
  const entry = process.steps.find(step => step.available);
  const entryHref = entry ? processStepHref(process, entry,projectId) : undefined;
  const handoffs = processHandoffs.filter(link => link.from === process.screen || link.to === process.screen);
  return <section className="space-y-6">
    <header className="flex flex-col gap-4 border-b border-slate-200 pb-5 sm:flex-row sm:items-end sm:justify-between">
      <div><h1 className="text-2xl font-bold">{process.label} · Process map</h1><p className="mt-2 text-sm">{stages.length} reference stages · Workflows will become available as we build them.</p></div>
      {entryHref && <Link className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 py-2 text-sm font-semibold text-white" href={entryHref}>Open {process.screen === "purchase-fms" ? "purchase workspace" : entry?.number === 0 ? "project register" : "document intake"}<ArrowRightIcon className="h-5 w-5" aria-hidden="true" /></Link>}
    </header>
    <section aria-label="Connected process maps" className="go-process-connections">
      <p className="go-eyebrow">Connected processes · Planned handoffs</p>
      <div>{["tender-fms", "projects", "purchase-fms"].map((screen,index) => {
        const related = processForScreen(screen)!;
        return <span key={screen}>{index > 0 && <ArrowRightIcon aria-hidden="true" />}{allowedScreens.includes(screen) ? <Link href={processMapHref(related,projectId)} aria-current={screen === process.screen ? "page" : undefined}>{related.label}</Link> : <span>{related.label}</span>}</span>;
      })}</div>
      <p className="mt-3 text-sm">Project delivery and purchasing progress in parallel. Finance approvals and communication support both.</p>
    </section>
    <section aria-labelledby="handoff-heading"><h2 id="handoff-heading">Inputs and handoffs</h2><ul className="go-map-handoffs">{handoffs.map(link => <li key={link.label}><strong>{link.label}</strong><p>{link.detail}</p></li>)}</ul></section>
    <p className="text-sm">Reference order does not enforce project dependencies or show actual progress. View a stage to inspect its expected evidence.</p>
    <DataGrid<ReferenceRow> key={process.screen} label={`${process.label} reference stages`} rows={stages} initialSort="number" status={step => step.available ? "Available workspace" : "Planned"} columns={[
      {key:"number",label:"Stage",value:step=>step.number},
      {key:"label",label:"Process / task",value:step=>step.label},
      {key:"group",label:"Phase",value:step=>step.group},
      {key:"availability",label:"Availability",value:step=>step.available ? "Available workspace" : "Planned", render:step=><GridStatus value={step.available ? "Available workspace" : "Planned"} />},
    ]} actions={step=><GridAction label={`View reference: ${step.label}`} onClick={()=>setSelected(step)} />} />
    <section className="go-map-support" aria-label="Cross-process controls"><h2>Finance and coordination</h2><p>Budget allocation, quotation decisions and payments belong to Finance, with Director oversight. Task doers/resolvers, blockers, notifications and evidence history connect the processes as their workflows are delivered.</p><p>Supplier verification may remain pending for quotation invitations. Purchase orders require Finance-cleared supplier documents; this gate will be connected when Purchase FMS is implemented.</p></section>
    {selected && <RecordDrawer title={`Stage ${selected.number} · ${selected.label}`} onClose={()=>setSelected(null)}>
      <p className="mb-5 text-sm">{selected.available ? "Available workspace" : "Planned workflow"} · Reference only; no task is assigned or completed here.</p>
      <dl className="space-y-4"><div><dt className="font-semibold">Phase</dt><dd>{selected.group}</dd></div><div><dt className="font-semibold">Purpose</dt><dd>{selected.purpose ?? selected.label}</dd></div>{selected.team && <div><dt className="font-semibold">Reference team</dt><dd>{selected.team} · Actual doer/resolver assignments will be recorded in the workflow.</dd></div>}</dl>
      <h2 className="mt-6">Expected evidence</h2>{selected.evidence?.length ? <ul className="list-disc space-y-2 pl-5">{selected.evidence.map(item=><li key={item}>{item}</li>)}</ul> : <p>The detailed evidence requirements will be defined with this workflow.</p>}
      {selected.available && <Link className="mt-6 inline-flex min-h-11 items-center text-blue-700 underline" href={processStepHref(process,selected,projectId)!}>Open available workspace</Link>}
    </RecordDrawer>}
  </section>;
}
