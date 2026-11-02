"use client";

import { useEffect, useState, type ReactNode } from "react";
import { SecurityHistory } from "@/components/users/security-history";
import { actionButtonClass } from "@/components/ui/action-button";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { UserIdentity } from "@/components/personal-profile";

export type UserDetailRecord = {
  id: string;
  username: string;
  first_name: string;
  last_name: string;
  mobile: string;
  email: string;
  roles: string[];
  is_active: boolean;
  is_superuser: boolean;
  must_change_password: boolean;
  email_status: string;
  edit_version?: number;
  created_at?: string;
  onlinePage?: string;
};
type Employee = { id: string; username: string; first_name: string; last_name: string; department_id: string; manager_id: string; is_company_root: boolean; version: number };
type Department = { id: string; name: string; is_active: boolean };
type OrganisationState = { phase: "loading" } | { phase: "error"; message: string } | { phase: "ready"; employee?: Employee; department?: Department; manager?: Employee };
const action = actionButtonClass("secondary");

function DetailList({ items }: { items: [string, ReactNode][] }) {
  return <dl className="grid gap-x-6 gap-y-4 sm:grid-cols-2">{items.map(([label, value]) => <div key={label} className="min-w-0"><dt className="text-xs font-bold uppercase tracking-wide text-slate-500">{label}</dt><dd className="mt-1 whitespace-pre-wrap break-words text-sm text-slate-900">{value || "Not provided"}</dd></div>)}</dl>;
}

function OrganisationDetails({ userID }: { userID: string }) {
  const [state, setState] = useState<OrganisationState>({ phase: "loading" });
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setState({ phase: "loading" });
      try {
        const [peopleResponse, departmentsResponse] = await Promise.all([
          fetch("/api/administration/organisation/employees", { cache: "no-store", signal: controller.signal }),
          fetch("/api/administration/organisation/departments", { cache: "no-store", signal: controller.signal }),
        ]);
        if (!peopleResponse.ok || !departmentsResponse.ok) throw new Error(peopleResponse.status === 403 || departmentsResponse.status === 403 ? "You do not have permission to view organisation details." : "Organisation details could not be loaded.");
        const [people, departments] = await Promise.all([peopleResponse.json() as Promise<Employee[]>, departmentsResponse.json() as Promise<Department[]>]);
        const employee = people.find(person => person.id === userID);
        if (!controller.signal.aborted) setState({ phase: "ready", employee, department: departments.find(department => department.id === employee?.department_id), manager: people.find(person => person.id === employee?.manager_id) });
      } catch (error) {
        if (!controller.signal.aborted) setState({ phase: "error", message: error instanceof Error && error.message === "You do not have permission to view organisation details." ? error.message : "Organisation details could not be loaded." });
      }
    }, 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [userID, revision]);

  if (state.phase === "loading") return <p role="status" className="text-sm text-slate-600">Loading organisation details…</p>;
  if (state.phase === "error") return <div className="space-y-3"><p role="alert" className="text-sm text-red-700">{state.message}</p><button type="button" className={action} onClick={() => setRevision(value => value + 1)}>Retry organisation details</button></div>;
  const { employee, department, manager } = state;
  const configured = !!employee && employee.version > 0;
  return <div className="space-y-4">
    <DetailList items={[
      ["Department", department ? `${department.name}${department.is_active ? "" : " (Inactive)"}` : "Not configured"],
      ["Reporting manager", employee?.is_company_root ? "Company root (no reporting manager)" : manager ? `${[manager.first_name, manager.last_name].filter(Boolean).join(" ") || manager.username} (@${manager.username})` : employee?.manager_id ? "Not available" : "Not assigned"],
      ["Organisation profile", configured ? "Configured" : "Not configured"],
    ]}/>
    <a className="inline-flex min-h-11 items-center text-sm font-bold text-blue-700 underline" href="/dashboard?screen=organisation">Open organisation profile</a>
  </div>;
}

export function UserDetails({ user, deliveryStatus, canManage, onEdit, onReset, onStatus }: {
  user: UserDetailRecord;
  deliveryStatus: string;
  canManage: boolean;
  onEdit: () => void;
  onReset: () => void;
  onStatus: () => void;
}) {
  const actor = useAuthenticatedUser();
  const canReadOrganisation = actor.is_superuser === true || actor.permissions.includes("system:admin") || actor.permissions.includes("admin.organisation.read");
  const canReadHandover = actor.is_superuser === true || actor.permissions.includes("system:admin") || actor.permissions.includes("admin.offboarding.read");
  const created = user.created_at ? new Date(user.created_at) : null;
  const createdLabel = created && !Number.isNaN(created.getTime()) ? <time dateTime={user.created_at}>{new Intl.DateTimeFormat("en-IN", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Kolkata" }).format(created)} IST</time> : "Not available";
  return <div className="space-y-6">
    <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 pb-4"><div><UserIdentity id={user.id} name={user.username} active={user.is_active}/><p className="mt-1 text-sm text-slate-600">@{user.username}</p></div><span className={`rounded-lg px-3 py-1 text-sm font-bold ${user.is_active ? "bg-blue-50 text-blue-800" : "bg-slate-100 text-slate-700"}`}>{user.is_active ? "Active" : "Inactive"}</span></div>
    <section aria-labelledby="user-contact-title" className="space-y-4"><h3 id="user-contact-title" className="text-base font-bold text-slate-900">Personal & contact details</h3><DetailList items={[
      ["Username", user.username], ["First name", user.first_name], ["Last name", user.last_name], ["Email", user.email], ["Mobile number", user.mobile],
    ]}/></section>
    <section aria-labelledby="user-access-title" className="space-y-4 border-t border-slate-200 pt-5"><h3 id="user-access-title" className="text-base font-bold text-slate-900">Account & access</h3><DetailList items={[
      ["Account status", user.is_active ? "Active" : "Inactive (access blocked)"],
      ["Access level", user.is_superuser ? "Superadmin (full platform access)" : "Role-based access"],
      ["Assigned roles", user.roles.length ? user.roles.join(", ") : "No roles assigned"],
      ["Password status", user.must_change_password ? "Password change required at next login" : "No password change required"],
      ["Invitation / password email", deliveryStatus], ["Created at", createdLabel], ["Recent activity", user.onlinePage || "No recent activity reported"], ["User ID", user.id],
    ]}/></section>
    <section aria-labelledby="user-organisation-title" className="space-y-4 border-t border-slate-200 pt-5"><h3 id="user-organisation-title" className="text-base font-bold text-slate-900">Organisation</h3>{canReadOrganisation ? <OrganisationDetails userID={user.id}/> : <p className="text-sm text-slate-600">Organisation details require organisation read access.</p>}</section>
    <SecurityHistory entityID={user.id}/>
    {(canManage || canReadHandover) && <div className="flex flex-wrap gap-3 border-t border-slate-200 pt-4">
      {canManage && <><button type="button" className={actionButtonClass("primary")} onClick={onEdit}>Edit user</button>{user.id !== actor.id && <button type="button" className={actionButtonClass(user.is_active?"danger":"success")} onClick={onStatus}>{user.is_active ? "Deactivate user" : "Reactivate user"}</button>}{user.is_active && user.id !== actor.id && <button type="button" className={actionButtonClass("warning")} onClick={onReset}>Reset password</button>}</>}
      {canReadHandover && <a className={action} href="/dashboard?screen=offboarding">View handovers</a>}
    </div>}
  </div>;
}
