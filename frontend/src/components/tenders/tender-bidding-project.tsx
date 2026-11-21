"use client";

import Link from "next/link";
import { useEffect, useRef } from "react";
import { ValidatedInput, ValidatedLabel, ValidatedSelect, ValidatedTextarea } from "@/components/ui/validated-controls";

export type BiddingProject = {
  id: string;
  number: string;
  name: string;
  purchase_enabled: boolean;
  quotations_approved?: boolean;
  tender_award_pending?: boolean;
  can_open_project: boolean;
  can_open_purchase: boolean;
  can_start_purchase: boolean;
};
export type BiddingProjectContext = {
  project: BiddingProject | null;
  setup: {
    defaults: {
      name: string; client_id: string; department: string; site_name: string; site_city: string; site_state: string;
      project_type: string; scope_summary: string; estimated_value: string; due_date: string;
    };
    clients: { id: string; name: string; code: string }[];
    managers: { id: string; name: string }[];
    actor_id: string;
  } | null;
};

const field = "mt-1 block min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm dark:border-slate-700 dark:bg-slate-800";
const action = "ui-action inline-flex items-center justify-center";

export function BiddingProjectFields({ setup, tenderManager }: { setup: NonNullable<BiddingProjectContext["setup"]>; tenderManager: string }) {
  const { defaults, clients, managers, actor_id } = setup;
  const defaultManager = managers.some(manager => manager.id === tenderManager) ? tenderManager : actor_id;
  return (
    <fieldset className="grid gap-4 rounded-xl border border-slate-200 p-4 sm:grid-cols-2 dark:border-slate-700">
      <legend className="px-2 text-sm font-bold">Project to create on authorization</legend>
      <p className="text-sm text-slate-600 sm:col-span-2 dark:text-slate-300">Review the details copied from the tender checklist. You can complete the project record after it is created.</p>
      <ValidatedLabel>Project name
        <ValidatedInput name="project_name" className={field} defaultValue={defaults.name} required minLength={3} maxLength={255} />
      </ValidatedLabel>
      <ValidatedLabel>Registered client
        <ValidatedSelect name="project_client_id" className={field} defaultValue={defaults.client_id} required>
          <option value="">Select an active client</option>
          {clients.map(client => <option key={client.id} value={client.id}>{client.name} · {client.code}</option>)}
        </ValidatedSelect>
        {defaults.department && !defaults.client_id && <span className="mt-1 block text-xs text-slate-500">Checklist department: {defaults.department}</span>}
      </ValidatedLabel>
      <ValidatedLabel>Project manager
        <ValidatedSelect name="project_manager_id" className={field} defaultValue={defaultManager} required>
          <option value="">Select a project manager</option>
          {managers.map(manager => <option key={manager.id} value={manager.id}>{manager.name}</option>)}
        </ValidatedSelect>
      </ValidatedLabel>
      <ValidatedLabel>Site / location
        <ValidatedInput name="project_site_name" className={field} defaultValue={defaults.site_name} maxLength={255} />
      </ValidatedLabel>
      <ValidatedLabel>City
        <ValidatedInput name="project_site_city" className={field} defaultValue={defaults.site_city} maxLength={128} />
      </ValidatedLabel>
      <ValidatedLabel>State
        <ValidatedInput name="project_site_state" className={field} defaultValue={defaults.site_state} maxLength={128} />
      </ValidatedLabel>
      <ValidatedLabel>Business type
        <ValidatedSelect name="project_type" className={field} defaultValue={defaults.project_type}>
          <option value="">Select a business type</option>
          <option value="electrical">Electrical</option>
          <option value="solar">Solar</option>
          <option value="ev">EV</option>
          <option value="mixed">Mixed</option>
        </ValidatedSelect>
      </ValidatedLabel>
      <ValidatedLabel className="sm:col-span-2">Scope summary
        <ValidatedTextarea name="project_scope_summary" className={field} defaultValue={defaults.scope_summary} rows={2} maxLength={10000} />
      </ValidatedLabel>
    </fieldset>
  );
}

export function BiddingProjectHandoff({ project, authorized, canWrite, focusOnSave, saving, onStart, onContinue }: {
  project: BiddingProject;
  authorized: boolean;
  canWrite: boolean;
  focusOnSave: boolean;
  saving: boolean;
  onStart: () => void;
  onContinue: () => void;
}) {
  const panel = useRef<HTMLElement>(null);
  useEffect(() => {
    if (!focusOnSave) return;
    panel.current?.focus({ preventScroll: true });
    panel.current?.scrollIntoView({ block: "center" });
  }, [focusOnSave]);
  return (
    <section ref={panel} tabIndex={-1} className="space-y-3 rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-900 dark:bg-blue-950/40" aria-label="Project and Purchase FMS">
      <h3 className="font-bold">{project.number} · {project.name}</h3>
      {project.purchase_enabled
        ? <p className="text-sm">Purchase FMS has started for this project.</p>
        : authorized && <p className="text-sm">{canWrite && project.can_start_purchase ? "Would you like to start Purchase FMS for this project now?" : "A user with Purchase FMS setup access can start procurement for this project."}</p>}
      {project.quotations_approved&&<p role="status" className="rounded-lg border-l-4 border-[#588368] bg-green-50 p-3 text-green-950">
        Finance approved all quotation groups.{project.tender_award_pending&&" Awaiting company award in Tender FMS."}
      </p>}
      <div className="flex flex-wrap gap-2">
        {project.purchase_enabled && project.can_open_purchase && <Link href={`/dashboard?screen=purchase-fms&project=${encodeURIComponent(project.id)}&step=PUR-01`} className={`${action} ui-action--primary`}>Open Purchase FMS</Link>}
        {!project.purchase_enabled && authorized && canWrite && project.can_start_purchase && <button type="button" disabled={saving} className={`${action} ui-action--primary`} onClick={onStart}>{saving ? "Starting…" : "Start Purchase FMS"}</button>}
        {project.can_open_project && <Link href={`/dashboard?screen=projects&project=${encodeURIComponent(project.id)}`} className={action}>Open project</Link>}
        {authorized && !project.purchase_enabled && <button type="button" className={action} onClick={onContinue} disabled={saving}>Continue tender without starting Purchase FMS</button>}
      </div>
    </section>
  );
}
