"use client";

import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { actionButtonClass } from "@/components/ui/action-button";
import { DataGrid, GridAction, GridStatus } from "@/components/ui/data-grid";
import { serverErrorMessage } from "@/components/ui/server-field-errors";
import { ValidatedForm, ValidatedInput, ValidatedLabel } from "@/components/ui/validated-controls";
import { SecurityHistory } from "@/components/users/security-history";
import { PermissionCatalogue } from "./permission-catalogue";
import type { Permission } from "./permission-groups";

type Role = {
  id: string;
  name: string;
  description: string;
  permissions: string[];
  is_active: boolean;
  edit_version: number;
  user_count: number;
};

const roleStatus = (role: Role) => role.is_active ? "Active" : "Inactive";
const rolesPage = "/dashboard?screen=roles";
const endpoint = "/api/administration/roles";
const fieldClass = "mt-1 block min-h-11 w-full rounded-lg border border-slate-300 bg-white px-3 text-sm font-normal";

async function read<T>(path: string, fallback: string, options?: RequestInit): Promise<T> {
  const response = await fetch(path, { cache: "no-store", ...options });
  const data = await response.json().catch(() => null);
  if (!response.ok) throw new Error(serverErrorMessage(data, fallback));
  return data as T;
}

export function RoleManagement() {
  const actor = useAuthenticatedUser();
  const canWrite = actor.is_superuser === true || actor.permissions.includes("admin.roles.write");
  const [roles, setRoles] = useState<Role[]>([]);
  const router = useRouter();
  const params = useSearchParams();
  const roleId = params.get("role");
  const view = params.get("roleView");
  const role = roles.find(item => item.id === roleId);
  const mayEdit = (role: Role) => canWrite && (role.name !== "superadmin" || actor.is_superuser === true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  async function loadRoles() {
    setLoading(true);
    setError("");
    try {
      setRoles(await read<Role[]>(endpoint, "Unable to load roles."));
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to load roles.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    const timer = window.setTimeout(() => void loadRoles(), 0);
    return () => window.clearTimeout(timer);
  }, []);

  async function installFinanceRoles() {
    setLoading(true);
    setError("");
    setNotice("");
    try {
      await read(endpoint + "/finance-presets", "Unable to create finance roles.", { method: "POST" });
      setNotice("Finance Viewer, Budget Editor and Approver roles are ready. Assign staff in Users, then grant their project team capabilities.");
      await loadRoles();
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to create finance roles.");
      setLoading(false);
    }
  }

  function finish(message: string) {
    setNotice(message);
    void loadRoles();
    router.push(rolesPage);
  }

  if (view === "history") return <section className="space-y-4">
    <Link className={actionButtonClass("secondary")} href={rolesPage}>Back to roles</Link>
    <SecurityHistory />
  </section>;

  if (view === "presets" && canWrite) return <section className="space-y-4">
    <Link className={actionButtonClass("secondary")} href={rolesPage}>Back to roles</Link>
    <h2 className="text-2xl font-bold">Finance role presets</h2>
    <p className="text-sm">Viewer reads budgets and quote differences. Budget Editor allocates costs. Approver approves budgets and Accounts decisions. Each user also needs project team capabilities.</p>
    {notice && <p role="status">{notice}</p>}
    {error && <p role="alert">{error}</p>}
    <button type="button" disabled={loading} onClick={() => void installFinanceRoles()} className={actionButtonClass("primary")}>Create finance roles</button>
  </section>;

  if (view === "create" && canWrite) return <RoleEditor key="create" role={null} editable onSaved={() => finish("Role created.")} />;
  if (roleId) {
    if (error || loading || !role) return <section className="space-y-4">
      <Link className={actionButtonClass("secondary")} href={rolesPage}>Back to roles</Link>
      {loading ? <p role="status">Loading role…</p> : <p role="alert">{error || "Role not found."}</p>}
    </section>;
    if (view === "details" && mayEdit(role)) return <RoleDetails key={role.id} role={role} onSaved={() => finish("Role details saved.")} />;
    return <RoleEditor key={role.id} role={role} editable={mayEdit(role)} onSaved={() => finish("Permissions saved.")} />;
  }

  return (
    <section className="mx-auto w-full max-w-[1500px] space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-2xl font-bold">Roles</h2>
        <div className="flex flex-wrap gap-2">
          <Link className={actionButtonClass("secondary")} href={rolesPage + "&roleView=history"}>Security history</Link>
          {canWrite && <Link className={actionButtonClass("secondary")} href={rolesPage + "&roleView=presets"}>Finance role presets</Link>}
          {canWrite && <Link className={actionButtonClass("primary")} href={rolesPage + "&roleView=create"}>Create role</Link>}
        </div>
      </div>
      {error && <div role="alert" className="flex items-center gap-3 rounded-lg border border-red-200 p-3 text-sm text-red-800">
        {error}<button type="button" className={actionButtonClass("secondary")} onClick={() => void loadRoles()}>Reload roles</button>
      </div>}
      {notice && <p role="status" className="rounded-lg bg-green-50 p-3 text-sm text-green-800">{notice}</p>}
      <DataGrid label="Roles" rows={roles} loading={loading} initialSort="name" status={roleStatus} statusOptions={["Active", "Inactive"]} columns={[
        { key: "name", label: "Role", value: role => role.name, render: role => <strong>{role.name}</strong> },
        { key: "description", label: "Description", value: role => role.description },
        { key: "users", label: "Assigned users", value: role => role.user_count },
        { key: "permissions", label: "Permissions", value: role => role.permissions.length },
        { key: "status", label: "Status", value: roleStatus, render: role => <GridStatus value={roleStatus(role)} /> },
      ]} actions={role => {
        const editable = mayEdit(role);
        const href = rolesPage + "&role=" + encodeURIComponent(role.id);
        return <div className="flex justify-end">
          <GridAction kind={editable ? "edit" : "view"} label={`${editable ? "Edit" : "View"} ${role.name} permissions`} onClick={() => router.push(href)} />
        </div>;
      }} />
    </section>
  );
}

function RoleEditor({ role, editable, onSaved }: {
  role: Role | null;
  editable: boolean;
  onSaved: () => void;
}) {
  const [savedRole, setSavedRole] = useState(role);
  const [permissions, setPermissions] = useState<Permission[] | null>(null);
  const [selectedCodes, setSelectedCodes] = useState(role?.permissions ?? []);
  const [assignedOnly, setAssignedOnly] = useState(!editable);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [conflict, setConflict] = useState(false);
  const [catalogueAttempt, setCatalogueAttempt] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    read<Permission[]>(endpoint + "/permissions", "Unable to load the permission catalogue.", { signal: controller.signal })
      .then(permissions => { if (!controller.signal.aborted) setPermissions(permissions); })
      .catch(error => { if (!controller.signal.aborted) setError(error.message); });
    return () => controller.abort();
  }, [catalogueAttempt]);

  const availableCodes = new Set(permissions?.map(permission => permission.code));
  const selected = selectedCodes.filter(code => availableCodes.has(code));
  const legacy = selectedCodes.filter(code => !availableCodes.has(code));
  const visible = permissions?.filter(permission => !assignedOnly || selected.includes(permission.code)) ?? [];

  function toggle(code: string) {
    setSelectedCodes(current => current.includes(code) ? current.filter(item => item !== code) : [...current, code]);
  }

  async function reload() {
    if (!savedRole || busy) return;
    setBusy(true);
    try {
      const current = (await read<Role[]>(endpoint, "Unable to reload roles.")).find(item => item.id === savedRole.id);
      if (!current) throw new Error("This role is no longer available.");
      setSavedRole(current);
      setSelectedCodes(current.permissions);
      setError("");
      setConflict(false);
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to reload the saved role.");
    } finally {
      setBusy(false);
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || !editable || !permissions || conflict) return;
    if (!savedRole && !selected.length) {
      setError("Select at least one permission.");
      return;
    }
    const form = new FormData(event.currentTarget);
    const previous = savedRole?.permissions.filter(code => availableCodes.has(code)) ?? [];
    const changed = selected.length !== previous.length || selected.some(code => !previous.includes(code));
    const responseBody = {
      ...(savedRole ? { expected_version: savedRole.edit_version } : {
        name: String(form.get("name") ?? "").trim().toLowerCase().replace(/\s+/g, "-"),
        description: String(form.get("description") ?? "").trim(),
      }),
      ...(!savedRole || changed ? { permission_codes: selected } : {}),
    };
    setBusy(true);
    setError("");
    try {
      const response = await fetch(savedRole ? `${endpoint}/${savedRole.id}` : endpoint, {
        method: savedRole ? "PATCH" : "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(responseBody),
      });
      if (!response.ok) {
        setConflict(Boolean(savedRole) && response.status === 409);
        throw new Error(serverErrorMessage(await response.json().catch(() => null), "Unable to save the role."));
      }
      onSaved();
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to save the role.");
    } finally {
      setBusy(false);
    }
  }

  return <section className="mx-auto w-full max-w-[1500px] space-y-5">
    <Link className={actionButtonClass("secondary")} href={rolesPage}>Back to roles</Link>
    <div className="flex flex-wrap items-center justify-between gap-3">
      <h2 className="text-2xl font-bold">{savedRole ? `${savedRole.name} permissions` : "Create role"}</h2>
      {editable && savedRole && <Link className={actionButtonClass("secondary")} href={rolesPage + "&role=" + encodeURIComponent(savedRole.id) + "&roleView=details"}>Edit role details</Link>}
      {editable && <button type="submit" form="role-editor-form" className={actionButtonClass("primary")} disabled={busy || !permissions || conflict}>
        {busy ? "Saving…" : savedRole ? "Save permissions" : "Create role"}
      </button>}
    </div>
    <div className="space-y-2">
      {error && <p role="alert" className="text-sm text-red-700">{error}</p>}
      {conflict && <button type="button" className={actionButtonClass("secondary")} onClick={() => void reload()} disabled={busy}>Reload saved role and discard changes</button>}
      {!permissions && error && <button type="button" className={actionButtonClass("secondary")} onClick={() => { setError(""); setCatalogueAttempt(attempt => attempt + 1); }}>Retry loading permissions</button>}
    </div>
    <ValidatedForm id="role-editor-form" key={savedRole?.edit_version ?? "new"} className="space-y-4" onSubmit={submit}>
      {!savedRole && <fieldset disabled={busy} className="grid gap-4 sm:grid-cols-2">
        <ValidatedLabel className="text-sm font-semibold">Role name
          <ValidatedInput name="name" required minLength={3} maxLength={64} placeholder="Purchase Manager" className={fieldClass} />
        </ValidatedLabel>
        <ValidatedLabel className="text-sm font-semibold">Description
          <ValidatedInput name="description" maxLength={255} className={fieldClass} />
        </ValidatedLabel>
      </fieldset>}
      {permissions ? <>
        <PermissionCatalogue permissions={visible} selectedCodes={selected} legacyCodes={legacy} onToggle={editable ? toggle : undefined} disabled={busy}
          toolbar={<div className="flex flex-wrap items-center gap-4">
            {editable && <ValidatedLabel className="flex items-center gap-2 text-sm">
              <ValidatedInput type="checkbox" checked={assignedOnly} onChange={event => setAssignedOnly(event.target.checked)} /> Assigned only
            </ValidatedLabel>}
            <span role="status" className="text-sm text-slate-600">{selected.length + legacy.length} permissions {editable ? "selected" : "assigned"}</span>
          </div>} />
      </> : !error && <p role="status">Loading permissions…</p>}
    </ValidatedForm>
  </section>;
}


function RoleDetails({ role, onSaved }: { role: Role; onSaved: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      await read(`${endpoint}/${role.id}`, "Unable to save role details.", {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          expected_version: role.edit_version,
          description: String(form.get("description") ?? "").trim(),
          is_active: form.has("is_active"),
        }),
      });
      onSaved();
    } catch (error) {
      setError(error instanceof Error ? error.message : "Unable to save role details.");
    } finally {
      setBusy(false);
    }
  }

  return <section className="space-y-5">
    <Link className={actionButtonClass("secondary")} href={rolesPage}>Back to roles</Link>
    <h2 className="text-2xl font-bold">{role.name} details</h2>
    {error && <p role="alert" className="text-red-700">{error}</p>}
    <ValidatedForm onSubmit={submit} className="max-w-xl space-y-4">
      <ValidatedLabel className="text-sm font-semibold">Description
        <ValidatedInput name="description" maxLength={255} defaultValue={role.description} disabled={busy} className={fieldClass} />
      </ValidatedLabel>
      <ValidatedLabel className="flex items-center gap-2 text-sm">
        <ValidatedInput type="checkbox" name="is_active" defaultChecked={role.is_active} disabled={busy} /> Active role
      </ValidatedLabel>
      <button type="submit" className={actionButtonClass("primary")} disabled={busy}>{busy ? "Saving…" : "Save details"}</button>
    </ValidatedForm>
  </section>;
}
