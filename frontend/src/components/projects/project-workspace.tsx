"use client";
import {ProjectQuotationStatus} from "./project-quotation-status";
import { ProjectPermissionProvider } from "./project-permissions";
import { permitted, canOpen, projectTabAllowed } from "./workspace-permissions";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { RequiredSelection, ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedSelect } from "@/components/ui/validated-controls";

import { FormEvent, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";

import { ProjectFreezePanel } from "./project-freeze-panel";
import { ProjectDateStatus } from "./project-date-status";
import { ProjectInitiationPanel } from "@/components/projects/project-initiation-panel";
import { ProjectBudgetPanel } from "@/components/projects/project-budget-panel";
import { ProjectPlanningPanel } from "@/components/projects/project-planning-panel";
import { ProjectDeliveryControl } from "@/components/projects/project-delivery-control";
import { TaskTemplateLibrary } from "@/components/projects/task-template-library";
import { ProjectProcessesPanel } from "@/components/projects/project-processes-panel";
import { ProjectSchedulePanel } from "@/components/projects/project-schedule-panel";

type Project = {
  id: string;
  project_number: string;
  name: string;
  short_name: string;
  lifecycle_status: string;
  site_name: string;
  site_city: string;
  site_state: string;
  planned_start_date: string | null;
  planned_completion_date: string | null;
  actual_start_date: string | null;
  is_active: boolean;
  scope_summary: string;
  contract_value: string | null;
  currency: string;
};
type Workspace = { permissions: string[]; project: Project; o2te_total: number; o2te_completed: number; o2te_attention: number };
type Member = { id: string; user_id: string; username: string; email: string; responsibility: string; capability_codes: string[]; is_active: boolean };
type Assignee = { id: string; username: string; email: string; is_superuser: boolean; permitted_capability_codes: string[] };
import { ProjectPurchasing } from "@/components/purchasing/project-purchasing";
import { PurchaseProgressPanel } from "@/components/purchasing/purchase-progress";
import { PurchaseStatus } from "@/components/purchasing/purchase-status";
import { ProjectInsights } from "@/components/dashboards/project-insights";
import { ProjectItems } from "@/components/items/project-items";
import { ContractorWorkWorkspace } from "@/components/projects/contractor-work-workspace";

type Tab = "purchasing" | "items" | "templates" | "control" | "processes" | "overview" | "planning" | "budget" | "schedule" | "delivery" | "team" | "contractor_work";

const CAPABILITIES: ReadonlyArray<{ code: string; label: string }> = [
  {code:"purchase_inspection.view",label:"purchase inspection view"},
  {code:"purchase_inspection.read",label:"purchase inspection read"},
  {code:"purchase_inspection.write",label:"purchase inspection write"},
  {code:"purchase_delivery.view",label:"purchase delivery view"},
  {code:"purchase_delivery.read",label:"purchase delivery read"},
  {code:"purchase_delivery.write",label:"purchase delivery write"},
  {code:"freeze.read",label:"freeze read"},
  {code:"freeze.write",label:"freeze write"},
  {code:"freeze.unfreeze",label:"freeze unfreeze"},
  {code:"purchase_bo.view",label:"Open BO item lists"},
  {code:"purchase_indent_groups.view",label:"Open item groups"},
  {code:"purchase_group_vendors.view",label:"Open group vendors"},
  {code:"purchase_group_vendors.write",label:"Choose and add group vendors"},
  {code:"purchase_item_vendor_report.view",label:"Open item vendor report"},
  {code:"purchase_indent_register.view",label:"Open vendor indent register"},
  {code:"purchase_indent_documents.view",label:"Open indent documents"},
  {code:"purchase_indent_documents.generate",label:"Generate vendor indents"},
  {code:"purchase_indent_documents.print",label:"Print vendor indents"},
  {code:"purchase_indent_documents.dispatch",label:"Record vendor dispatch"},
  {code:"group_comparisons.view",label:"Open price comparison"},
  {code:"group_comparisons.read",label:"Read price comparisons"},
  {code:"group_comparisons.write",label:"Finalise quotations for Accounts"},
  {code:"vendor_verification.view",label:"Open Finance vendor review"},
  {code:"vendor_verification.read",label:"Read Finance vendor evidence"},
  {code:"vendor_verification.write",label:"Verify vendor documents as Finance"},
  {code:"quotation_approvals.view",label:"Open Accounts quotation approval"},
  {code:"quotation_approvals.read",label:"Compare quotations against approved budgets"},
  {code:"quotation_approvals.write",label:"Accept or reject quotation items as Accounts"},
  {code:"group_awards.view",label:"Open award approval placeholder"},
  {code:"group_awards.approve",label:"Group award approval (workflow pending)"},
  {code:"team.view",label:"View team"},
  {code:"planning.view",label:"View planning"},
  {code:"schedule.view",label:"View schedule"},
  {code:"processes.view",label:"View processes"},
  {code:"delivery.view",label:"View delivery"},
  {code:"contracts.view",label:"View contracts"},
  {code:"o2te.view",label:"View o2te"},
  {code:"items.view",label:"View items"},
  {code:"contractor_work.view",label:"View contractor work"},
  {code:"purchase_workflow.view",label:"View purchase workflow"},
  {code:"purchase_holds.view",label:"View purchase holds"},
  {code:"purchase_indents.view",label:"View purchase indents"},
  {code:"budgets.view",label:"View project budgets"},
  {code:"budgets.read",label:"Read project budgets"},
  {code:"budgets.write",label:"Write project budgets"},
  {code:"budgets.approve",label:"Approve project budgets"},
  {code:"group_quotations.view",label:"View group quotations"},
  {code:"group_quotations.read",label:"Read group quotations"},
  {code:"group_quotations.write",label:"Write group quotations"},

  { code: "contractor_work.read", label: "Read contractor work" },
  { code: "contractor_work.write", label: "Prepare contractor work" },
  { code: "contractor_work.approve", label: "Issue contractor work orders" },
  {code:"purchase_progress.read",label:"Read purchase progress"},
  {code:"purchase_indent_documents.read",label:"Read vendor indent documents and logs"},
  {code:"purchase_indent_documents.write",label:"Legacy indent write (grant specific document actions)"},
  {code:"purchase_indent_groups.read",label:"Read indent item groups"},
  {code:"purchase_indent_groups.write",label:"Manage item grouping"},
  {code:"purchase_bo.read",label:"Read BO lists"},
  {code:"purchase_bo.write",label:"Prepare BO lists"},
  {code:"purchase_bo.complete",label:"Complete BO lists"},
  {code:"purchase_sourcing.read",label:"Purchase sourcing read"},
  {code:"purchase_sourcing.write",label:"Purchase sourcing write"},
  {code:"purchase_sourcing.approve",label:"Purchase sourcing approve"},
  {code:"purchase_workflow.read",label:"Purchase workflow read"},
  {code:"purchase_workflow.write",label:"Purchase workflow write"},
  {code:"purchase_workflow.execute",label:"Purchase workflow execute"},
  {code:"purchase_workflow.resolve",label:"Purchase workflow resolve"},
  {code:"purchase_holds.read",label:"Read purchase holds"},
  {code:"purchase_holds.write",label:"Write purchase holds"},
  {code:"purchase_holds.release",label:"Release purchase holds"},
  {code:"purchase_indents.read",label:"Read purchase indents"},
  {code:"purchase_indents.write",label:"Write purchase indents"},
  {code:"purchase_indents.review",label:"Review purchase indents"},
  {code:"purchase_indents.release",label:"Release purchase indents"},
  { code: "items.read", label: "Read project items" },
  { code: "items.write", label: "Maintain project items" },
  {code:"delivery.read",label:"Read weighted task plans"},
  {code:"delivery.write",label:"Manage task plans and assignments"},
  {code:"delivery.execute",label:"Execute assigned subtasks"},
  {code:"escalations.read",label:"Read task escalations"},
  {code:"escalations.write",label:"Raise / resolve assigned task escalations"},
  { code: "dashboard.view", label: "Workspace overview" },
  { code: "contracts.read", label: "Read contract records" },
  { code: "contracts.write", label: "Update contract records" },
  { code: "o2te.read", label: "Read O2TE checklist" },
  { code: "o2te.write", label: "Update O2TE checklist" },
  { code: "team.read", label: "Read project team" },
  { code: "team.write", label: "Manage project team" },
  { code: "planning.read", label: "Read project plan" },
  { code: "planning.write", label: "Maintain work packages" },
  { code: "processes.read", label: "Read project delivery processes" },
  { code: "processes.write", label: "Manage project process dependencies and progress" },
  { code: "schedule.read", label: "Read project schedule" },
  { code: "schedule.write", label: "Maintain project schedule" },
];

function apiMessage(payload: unknown, fallback: string): string {
  return typeof payload === "object" && payload !== null && "detail" in payload && typeof payload.detail === "string" ? payload.detail : fallback;
}

function Icon({ name }: { name: "back" | "people" | "file" | "plus" | "edit" | "close" }) {
  const paths = {
    back: <path d="M19 12H5m6-6-6 6 6 6" />,
    people: <><circle cx="9" cy="8" r="3" /><path d="M3 20c.8-4 3-6 6-6s5.2 2 6 6M17 8h4m-2-2v4M16 15c2.7.2 4.3 1.8 5 5" /></>,
    file: <><path d="M6 3h8l4 4v14H6z" /><path d="M14 3v5h5M9 13h6M9 17h6" /></>,
    plus: <path d="M12 5v14M5 12h14" />,
    edit: <><path d="m14.5 5.5 4 4M4 20l4.2-1 10.6-10.6a2.8 2.8 0 0 0-4-4L4.2 15 4 20Z" /></>,
    close: <path d="m6 6 12 12M18 6 6 18" />,
  };
  return <svg aria-hidden="true" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth="1.9">{paths[name]}</svg>;
}

function WorkspaceSkeleton() {
  return <div aria-busy="true" className="mx-auto w-full max-w-[1500px] animate-pulse space-y-5"><div className="h-28 rounded-3xl bg-slate-200" /><div className="grid gap-4 md:grid-cols-3">{[1, 2, 3].map((item) => <div key={item} className="h-32 rounded-2xl bg-slate-200" />)}</div></div>;
}

export function ProjectWorkspace({ projectId, onBack }: { projectId: string; onBack: () => void }) {
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedTab = searchParams.get("tab");
  const tab: Tab = (["purchasing", "items", "templates", "control", "processes", "overview", "planning", "budget", "schedule", "delivery", "team", "contractor_work"] as const)
    .find(value => value === requestedTab) ?? "overview";
  function setTab(value: Tab) {
    const params = new URLSearchParams({ screen: "projects", project: projectId, tab: value });
    router.push(`/dashboard?${params}`, { scroll: false });
  }
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [showInitiation, setShowInitiation] = useState(false);
  const [showMemberDialog, setShowMemberDialog] = useState(false);
  const [editing, setEditing] = useState<Member | null>(null);
  const [assignees, setAssignees] = useState<Assignee[]>([]);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState(""); const [capabilityError, setCapabilityError] = useState("");
  const [userId, setUserId] = useState("");
  const [responsibility, setResponsibility] = useState("Project team member");
  const [capabilityCodes, setCapabilityCodes] = useState<string[]>(["dashboard.view"]);

  useEffect(() => {
    const controller = new AbortController();
    void fetch(`/api/projects/${projectId}/workspace`, { cache: "no-store", signal: controller.signal })
      .then(async (response) => { const payload = await response.json(); if (!response.ok) throw new Error(apiMessage(payload, "Unable to load this project workspace.")); return payload as Workspace; })
      .then((payload) => { setWorkspace(payload); setError(""); })
      .catch((reason: unknown) => { if (!(reason instanceof DOMException && reason.name === "AbortError")) setError(reason instanceof Error ? reason.message : "Unable to load this project workspace."); });
    return () => controller.abort();
  }, [projectId, reload]);

  useEffect(() => {
    if (tab !== "team" || !workspace || !projectTabAllowed(workspace.permissions,"team")) return;
    const controller = new AbortController();
    void fetch(`/api/projects/${projectId}/team`, { cache: "no-store", signal: controller.signal })
      .then(async (response) => { const payload = await response.json(); if (!response.ok) throw new Error(apiMessage(payload, "Unable to load project team.")); return payload as Member[]; })
      .then((payload) => { setMembers(payload); setError(""); })
      .catch((reason: unknown) => { if (!(reason instanceof DOMException && reason.name === "AbortError")) setError(reason instanceof Error ? reason.message : "Unable to load project team."); });
    return () => controller.abort();
  }, [projectId, reload, tab, workspace]);

  function openMemberDialog(member: Member | null) {
    setEditing(member); setUserId(member?.user_id ?? ""); setResponsibility(member?.responsibility ?? "Project team member"); setCapabilityCodes(member?.capability_codes ?? ["dashboard.view"]); setFormError(""); setShowMemberDialog(true);
    void fetch(`/api/projects/${projectId}/team/assignable-users`, { cache: "no-store" })
      .then(async (response) => { const payload = await response.json(); if (!response.ok) throw new Error(apiMessage(payload, "Unable to load assignable users.")); return payload as Assignee[]; })
      .then(setAssignees)
      .catch((reason: unknown) => setFormError(reason instanceof Error ? reason.message : "Unable to load assignable users."));
  }

  const selectedAssignee = assignees.find((user) => user.id === userId);
  const allowedCapabilities = selectedAssignee?.permitted_capability_codes ?? capabilityCodes;
  function toggleCapability(code: string) { setCapabilityError(""); setCapabilityCodes((current) => current.includes(code) ? current.filter((item) => item !== code) : [...current, code]); }
  async function saveMember(event: FormEvent<HTMLFormElement>) {
    const submittedForm = event.currentTarget instanceof HTMLFormElement ? event.currentTarget : undefined;
    event.preventDefault(); if (!userId) { setFormError("Select an active user."); return; } if (!capabilityCodes.length) { setCapabilityError("Select at least one project capability."); return; }
    setSaving(true); setFormError("");
    const payload = { user_id: userId, responsibility, capability_codes: capabilityCodes, is_active: editing?.is_active ?? true };
    const url = editing ? `/api/projects/${projectId}/team/${editing.id}` : `/api/projects/${projectId}/team`;
    try { const response = await fetch(url, { method: editing ? "PATCH" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) }); const result = await response.json().catch(() => ({})); if (!response.ok) { if (displayServerErrors(submittedForm, result)) return;  setFormError(apiMessage(result, "Unable to save the project assignment.")); return; } setShowMemberDialog(false); setReload((value) => value + 1); } finally { setSaving(false); }
  }
  async function toggleMember(member: Member) {
    const response = await fetch(`/api/projects/${projectId}/team/${member.id}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ user_id: member.user_id, responsibility: member.responsibility, capability_codes: member.capability_codes, is_active: !member.is_active }) });
    const payload = await response.json().catch(() => ({})); if (!response.ok) { setError(apiMessage(payload, "Unable to update this project assignment.")); return; } setReload((value) => value + 1);
  }

  if (!workspace && !error) return <WorkspaceSkeleton />;
  if (!workspace) return <section className="mx-auto max-w-2xl rounded-2xl border border-red-200 bg-red-50 p-6"><button type="button" onClick={onBack} className="mb-4 inline-flex min-h-11 items-center gap-2 font-bold text-blue-800"><Icon name="back" />Back to projects</button><p role="alert" className="font-semibold text-red-800">{error}</p></section>;
  const { project } = workspace;
  const access = workspace.permissions;
  if (!projectTabAllowed(access,tab)) return <section className="p-6"><button type="button" onClick={onBack}>Back to projects</button><p role="status" className="mt-4">You do not have permission to open this project section.</p></section>;
  if (tab === "purchasing" && searchParams.get("purchaseView") === "bo") return <ProjectPermissionProvider permissions={access}><ProjectPurchasing project={projectId} projectName={project.name} scopeSummary={project.scope_summary} /></ProjectPermissionProvider>;
  const location = [project.site_name, project.site_city, project.site_state].filter(Boolean).join(", ") || "Site to be confirmed";
  return <ProjectPermissionProvider permissions={access}><section className="mx-auto w-full max-w-[1500px] space-y-5">{tab === "control" || tab === "overview" ? <header className="flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-slate-950 px-5 py-3 text-white"><div><button onClick={onBack} className="min-h-9 text-xs font-bold text-blue-200">← All projects</button><h2 className="text-xl font-extrabold">{project.name}</h2><p className="mt-1 text-xs text-slate-300">{project.project_number} · {location}</p></div><button disabled={!canOpen(access,"projects.contracts")} onClick={()=>setShowInitiation(true)} className="min-h-11 rounded-xl bg-blue-700 px-4 text-sm font-bold">Contract control</button></header> : <div className="overflow-hidden rounded-3xl border border-slate-800 bg-slate-950 text-white shadow-[0_24px_60px_-30px_rgba(15,23,42,0.8)]"><div className="border-b border-white/10 px-5 py-4 sm:px-7"><button type="button" onClick={onBack} className="inline-flex min-h-10 items-center gap-2 text-sm font-bold text-blue-200 hover:text-white focus:outline-none focus:ring-2 focus:ring-blue-300"><Icon name="back" />All projects</button></div><div className="grid gap-6 px-5 py-6 sm:px-7 lg:grid-cols-[1fr_auto]"><div><p className="font-mono text-xs font-bold tracking-[0.15em] text-blue-300">{project.project_number}</p><h2 className="mt-2 text-2xl font-extrabold tracking-tight sm:text-3xl">{project.name}</h2><p className="mt-2 max-w-3xl text-sm leading-6 text-slate-300">{project.scope_summary || "Project scope has not been recorded yet."}</p><p className="mt-4 text-sm font-medium text-slate-400">{location}</p></div><div className="flex flex-wrap content-start gap-2"><span className="rounded-full bg-white/10 px-3 py-1.5 text-xs font-bold capitalize">{project.lifecycle_status.replace("_", " ")}</span><button type="button" disabled={!canOpen(access,"projects.contracts")} onClick={() => setShowInitiation(true)} className="inline-flex min-h-11 items-center gap-2 rounded-xl bg-blue-500 px-4 text-sm font-bold text-white hover:bg-blue-400 focus:outline-none focus:ring-2 focus:ring-white"><Icon name="file" />Contract control</button></div></div></div>}
    {permitted(access,"projects.freeze.read") && <ProjectFreezePanel key={project.id} projectId={project.id} />}<ProjectDateStatus project={project} />
    {error ? <p role="alert" className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-semibold text-red-800">{error}</p> : null}
    <div className="flex gap-2 overflow-x-auto border-b border-slate-200" role="tablist" aria-label="Project workspace sections">{([['purchasing', 'Purchasing'], ['contractor_work', 'Contractor Work & Manpower'], ['control', 'Project Control'], ['templates', 'Task templates'], ['processes', 'Processes'], ['overview', 'Dashboard'], ['items', 'Items'], ['budget', 'Budget'], ['planning', 'Work packages'], ['schedule', 'Schedule'], ['delivery', 'Delivery readiness'], ['team', 'Project team']] as const).filter(([value]) => projectTabAllowed(access,value)).map(([value, label]) => <button key={value} role="tab" aria-selected={tab === value} onClick={() => setTab(value)} className={`min-h-11 shrink-0 border-b-2 px-4 text-sm font-bold transition focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-inset ${tab === value ? "border-blue-700 text-blue-800" : "border-transparent text-slate-500 hover:text-slate-900"}`}>{label}</button>)}</div>
    {tab === "templates" ? <TaskTemplateLibrary /> : null}
    {tab !== "purchasing" && tab !== "contractor_work" && permitted(access,"projects.purchase_holds.read") && <PurchaseStatus key={projectId + tab} project={projectId} />}
    {tab === "contractor_work" ? <ContractorWorkWorkspace key={projectId} projectId={projectId} projectName={workspace.project.name} /> : null}
    {tab === "purchasing" && <ProjectPurchasing key={projectId} project={projectId} scopeSummary={workspace.project.scope_summary} />}
    {tab === "control" ? <ProjectDeliveryControl key={projectId} projectId={projectId} startDate={project.planned_start_date} finishDate={project.planned_completion_date} /> : null}
    {tab === "processes" ? <ProjectProcessesPanel key={projectId} projectId={projectId} onOpenWorkspace={workspace => { if (workspace === "contracts") setShowInitiation(true); else if (workspace === "contractor_work") setTab("contractor_work"); else setTab(workspace === "team" ? "team" : "planning"); }} /> : null}
    {tab === "overview" && permitted(access,"projects.purchase_progress.read") ? <PurchaseProgressPanel key={projectId + "progress"} project={projectId} /> : null}
    {tab === "overview" && <ProjectQuotationStatus project={projectId} finance={permitted(access,"accounts.quotation_approvals.read")&&permitted(access,"accounts.quotation_approvals.view")}/>}
    {tab === "overview" ? <ProjectInsights key={projectId} project={project} readiness={{total:workspace.o2te_total,completed:workspace.o2te_completed,attention:workspace.o2te_attention}} /> : null}
    {tab === "delivery" ? <div className="grid gap-4 sm:grid-cols-3"><div className="rounded-2xl border border-slate-200 bg-white p-5"><p className="text-xs font-bold uppercase tracking-[0.12em] text-slate-500">O2TE obligations</p><p className="mt-2 text-3xl font-extrabold text-slate-900">{workspace.o2te_total}</p></div><div className="rounded-2xl border border-emerald-100 bg-emerald-50 p-5"><p className="text-xs font-bold uppercase tracking-[0.12em] text-emerald-700">Completed</p><p className="mt-2 text-3xl font-extrabold text-emerald-800">{workspace.o2te_completed}</p></div><div className="rounded-2xl border border-amber-100 bg-amber-50 p-5"><p className="text-xs font-bold uppercase tracking-[0.12em] text-amber-700">Needs action</p><p className="mt-2 text-3xl font-extrabold text-amber-800">{workspace.o2te_attention}</p></div><div className="sm:col-span-3 rounded-2xl border border-slate-200 bg-white p-5"><h3 className="font-extrabold text-slate-900">Delivery readiness</h3><p className="mt-2 text-sm leading-6 text-slate-600">The O2TE checklist is the implemented readiness control. WBS, programme, procurement, workforce, quality, and commercial records will be attached to this same workspace in later vertical slices.</p><button type="button" disabled={!canOpen(access,"projects.contracts")} onClick={() => setShowInitiation(true)} className="mt-4 inline-flex min-h-11 items-center gap-2 rounded-xl border border-blue-200 px-4 text-sm font-bold text-blue-800 hover:bg-blue-50"><Icon name="file" />Open O2TE checklist</button></div></div> : null}
    {tab === "team" ? <div className="rounded-2xl border border-slate-200 bg-white shadow-[0_14px_34px_-26px_rgba(15,23,42,0.35)]"><div className="flex flex-col gap-3 border-b border-slate-100 p-5 sm:flex-row sm:items-center sm:justify-between"><div><h3 className="text-lg font-extrabold text-slate-900">Project team</h3><p className="mt-1 text-sm text-slate-500">Assignments narrow a user&apos;s global role permissions to this project.</p></div><button type="button" disabled={!permitted(access,"projects.team.write")} onClick={() => openMemberDialog(null)} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white hover:bg-blue-800 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="plus" />Assign employee</button></div><div className="divide-y divide-slate-100">{members.map((member) => <article key={member.id} className="flex flex-col gap-3 p-5 sm:flex-row sm:items-center sm:justify-between"><div><div className="flex flex-wrap items-center gap-2"><p className="font-bold text-slate-900">{member.username}</p>{!member.is_active ? <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-bold text-slate-600">Inactive</span> : null}</div><p className="mt-1 text-sm text-slate-500">{member.email}</p><p className="mt-2 text-sm font-semibold text-blue-800">{member.responsibility}</p><div className="mt-2 flex flex-wrap gap-1.5">{member.capability_codes.map((code) => <span key={code} className="rounded-full bg-slate-100 px-2 py-1 text-[11px] font-bold text-slate-600">{code}</span>)}</div></div><div className="flex gap-2"><button aria-label={`Edit ${member.username} assignment`} type="button" disabled={!permitted(access,"projects.team.write")} onClick={() => openMemberDialog(member)} className="grid min-h-11 min-w-11 place-items-center rounded-xl border border-slate-200 text-slate-600 hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="edit" /></button><button type="button" disabled={!permitted(access,"projects.team.write")} onClick={() => void toggleMember(member)} className={`min-h-11 rounded-xl border px-3 text-sm font-bold focus:outline-none focus:ring-2 focus:ring-blue-500 ${member.is_active ? "border-red-200 text-red-700 hover:bg-red-50" : "border-emerald-200 text-emerald-700 hover:bg-emerald-50"}`}>{member.is_active ? "Deactivate" : "Reactivate"}</button></div></article>)}{!members.length ? <div className="px-5 py-12 text-center"><span className="mx-auto grid h-11 w-11 place-items-center rounded-full bg-blue-50 text-blue-700"><Icon name="people" /></span><p className="mt-3 font-bold text-slate-800">No employees assigned yet</p><p className="mt-1 text-sm text-slate-500">Assign accountable people before work begins.</p></div> : null}</div></div> : null}
    {tab === "budget" ? <ProjectBudgetPanel projectId={projectId} /> : null}
    {tab === "planning" ? <ProjectPlanningPanel projectId={projectId} /> : null}
    {tab === "items" ? <ProjectItems projectId={projectId} /> : null}
    {tab === "schedule" ? <ProjectSchedulePanel projectId={projectId} /> : null}
    {showInitiation && canOpen(access,"projects.contracts") ? <ProjectInitiationPanel projectId={projectId} onClose={() => setShowInitiation(false)} onChanged={() => setReload((value) => value + 1)} /> : null}
    {showMemberDialog ? <div role="dialog" aria-modal="true" aria-labelledby="member-dialog-title" className="fixed inset-0 z-50 flex items-end bg-slate-950/50 p-0 backdrop-blur-sm sm:items-center sm:justify-center sm:p-5"><ValidatedForm onSubmit={(event) => void saveMember(event)} className="vtpl-modal max-h-[94dvh] w-full overflow-y-auto rounded-t-3xl bg-white p-5 shadow-2xl sm:max-w-2xl sm:rounded-3xl sm:p-7"><div className="flex items-start justify-between border-b border-slate-100 pb-5"><div><p className="text-xs font-bold uppercase tracking-[0.14em] text-blue-700">Project access</p><h3 id="member-dialog-title" className="mt-1 text-xl font-extrabold">{editing ? `Edit ${editing.username}` : "Assign employee"}</h3></div><button type="button" aria-label="Close assignment dialog" onClick={() => setShowMemberDialog(false)} className="grid min-h-11 min-w-11 place-items-center rounded-xl text-slate-500 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"><Icon name="close" /></button></div>{formError ? <p role="alert" className="mt-5 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm font-semibold text-red-800">{formError}</p> : null}<fieldset disabled={saving} className="mt-5 space-y-4"><ValidatedLabel className="block text-sm font-bold text-slate-700">Employee<ValidatedSelect name="user_id" required disabled={Boolean(editing)} value={userId} onChange={(event) => { setUserId(event.target.value); setCapabilityCodes(["dashboard.view"]); }} className="mt-1 min-h-11 w-full rounded-xl border border-slate-300 bg-white px-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"><option value="">Select an active user</option>{assignees.map((user) => <option key={user.id} value={user.id}>{user.username} - {user.email}</option>)}</ValidatedSelect></ValidatedLabel><ValidatedLabel className="block text-sm font-bold text-slate-700">Responsibility<ValidatedInput name="responsibility" required value={responsibility} onChange={(event) => setResponsibility(event.target.value)} className="mt-1 min-h-11 w-full rounded-xl border border-slate-300 px-3 text-sm outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100" placeholder="Example: Project manager" /></ValidatedLabel><RequiredSelection  onInvalid={() => setCapabilityError("Select at least one project capability.")}><legend className="text-sm font-bold text-slate-700">Project capabilities<span aria-hidden="true" className="text-red-600"> *</span></legend><p className="mt-1 text-xs leading-5 text-slate-500">Only capabilities already granted by the employee&apos;s global role can be selected.</p><div className="mt-3 grid gap-2 sm:grid-cols-2">{CAPABILITIES.map((capability) => { const allowed = allowedCapabilities.includes(capability.code); return <ValidatedLabel key={capability.code} className={`flex min-h-11 items-center gap-3 rounded-xl border px-3 text-sm ${allowed ? "border-slate-200 text-slate-700" : "border-slate-100 bg-slate-50 text-slate-400"}`}><ValidatedInput type="checkbox" disabled={!allowed} checked={capabilityCodes.includes(capability.code)} onChange={() => toggleCapability(capability.code)} className="h-4 w-4 rounded border-slate-300 text-blue-700 focus:ring-blue-500" />{capability.label}</ValidatedLabel>; })}</div>{capabilityError ? <p role="alert" className="mt-1 text-sm text-red-600">{capabilityError}</p> : null}</RequiredSelection></fieldset><div className="mt-6 flex justify-end gap-2"><button type="button" onClick={() => setShowMemberDialog(false)} className="min-h-11 rounded-xl border border-slate-200 px-4 text-sm font-bold text-slate-700">Cancel</button><button disabled={saving} className="min-h-11 rounded-xl bg-blue-700 px-5 text-sm font-bold text-white hover:bg-blue-800 disabled:opacity-60">{saving ? "Saving..." : editing ? "Save assignment" : "Assign employee"}</button></div></ValidatedForm></div> : null}
  </section></ProjectPermissionProvider>;
}
