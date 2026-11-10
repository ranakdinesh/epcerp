"use client";

import { useCallback, useEffect, useRef, useState, type SubmitEvent } from "react";
import {useRouter} from "next/navigation";
import { EyeIcon, PencilSquareIcon, KeyIcon, PlusIcon, NoSymbolIcon, ArrowPathIcon } from "@heroicons/react/24/outline";
import { actionButtonClass } from "@/components/ui/action-button";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { UserDetails, type UserDetailRecord } from "@/components/users/user-details";
import { UserAvatar } from "@/components/personal-profile";
import { DataGrid, type GridQuery } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { RequiredSelection, ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedTextarea } from "@/components/ui/validated-controls";

type PlatformUser = UserDetailRecord;
type Role = { is_active?: boolean; id: string; name: string };
type DialogMode = "create" | "view" | "edit" | "reset" | "status" | null;
const button = actionButtonClass("secondary", true);
const input = "mt-2 min-h-11 w-full rounded-xl border border-slate-300 bg-white px-3 py-2 font-normal text-slate-900";
const deliveryLabel = (state: string) => ({pending:"Email queued",sending:"Email sending",delivered:"Email accepted",failed:"Email failed",cancelled:"Email cancelled"}[state] ?? "No invitation issued");

export function UserManagement() {
  const actor = useAuthenticatedUser();
 const router=useRouter();
  const canWrite = actor.permissions.includes("system:admin") || actor.permissions.includes("admin.users.write");
  const [users, setUsers] = useState<PlatformUser[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [total, setTotal] = useState(0);
  const [gridQuery, setGridQuery] = useState<GridQuery>({search:"",status:"",sort:"username",direction:"asc",page:1,size:20});
  const [isLoading, setIsLoading] = useState(true);
  const [notice, setNotice] = useState("");
  const [loadError, setLoadError] = useState("");
  const [formError, setFormError] = useState("");
 const[conflict,setConflict]=useState(false);
  const [rolesError, setRolesError] = useState("");
  const [busy, setBusy] = useState(false);
  const [dialogMode, setDialogMode] = useState<DialogMode>(null);
  const detailRequest = useRef<AbortController | null>(null);
  const [detailsLoading, setDetailsLoading] = useState(false);
  const [detailsError, setDetailsError] = useState("");
  const [selectedUser, setSelectedUser] = useState<PlatformUser | null>(null);
  const [selectedRoleNames, setSelectedRoleNames] = useState<string[]>([]);
  const [superadmin, setSuperadmin] = useState(false);
  const [sendTemporaryEmail, setSendTemporaryEmail] = useState(false);
  const [revision, setRevision] = useState(0);
  const rolesOptional = superadmin || (dialogMode === "edit" && selectedUser?.is_superuser === true);
  const canManage = (user: PlatformUser) => canWrite && (!user.is_superuser || actor.is_superuser === true);
  const changeQuery = useCallback((query: GridQuery) => setGridQuery(query), []);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      setIsLoading(true); setLoadError("");
      try {
        const params = new URLSearchParams({page:String(gridQuery.page),page_size:String(gridQuery.size),sort:`username_${gridQuery.direction}`});
        if(gridQuery.search.trim())params.set("search",gridQuery.search.trim());
        if (gridQuery.status) params.set("status", gridQuery.status.toLowerCase());
        const [usersResponse, rolesResponse, activityResponse] = await Promise.all([
          fetch(`/api/administration/users?${params}`, {cache:"no-store",signal:controller.signal}),
          fetch("/api/administration/roles", {cache:"no-store",signal:controller.signal}),
          fetch("/api/administration/users/online/activity", {cache:"no-store",signal:controller.signal}),
        ]);
        if (!usersResponse.ok || !rolesResponse.ok) throw new Error("Unable to load user administration");
        const userPage = await usersResponse.json() as { items: PlatformUser[]; total: number };
        const online = activityResponse.ok ? await activityResponse.json() as {items:{user_id:string;current_page:string}[]} : {items:[]};
        const pages = new Map(online.items.map(item => [item.user_id,item.current_page]));
        setUsers(userPage.items.map(user=>({...user,onlinePage:pages.get(user.id)}))); setTotal(userPage.total);
        setRoles(await rolesResponse.json() as Role[]);
      } catch { if(!controller.signal.aborted) setLoadError("Unable to load the user directory. Please refresh and try again."); }
      finally { if(!controller.signal.aborted) setIsLoading(false); }
    }, 0);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [gridQuery, revision]);

  useEffect(() => () => detailRequest.current?.abort(), []);

  function closeDialog() {
    if (busy) return;
    detailRequest.current?.abort();
    setDetailsLoading(false);
    setDialogMode(null);
  }

  async function openDialog(mode: DialogMode, user: PlatformUser | null = null) {
    detailRequest.current?.abort();
    setRolesError(""); setFormError(""); setDetailsError("");
    setSelectedUser(user); setSelectedRoleNames(user?.roles ?? []); setSuperadmin(false); setSendTemporaryEmail(false); setDialogMode(mode);
    const needsDetails = user !== null && (mode === "edit" || mode === "view");
    setDetailsLoading(needsDetails);
    if (!needsDetails) return;
    const controller = new AbortController();
    detailRequest.current = controller;
    try {
      const response = await fetch(`/api/administration/users/${user.id}`, {cache:"no-store", signal:controller.signal});
      if (!response.ok) throw new Error("Unable to load user");
      const fresh = await response.json() as PlatformUser;
      if (fresh.id !== user.id || typeof fresh.username !== "string" || typeof fresh.email !== "string"
        || typeof fresh.first_name !== "string" || typeof fresh.last_name !== "string" || typeof fresh.mobile !== "string"
        || !Array.isArray(fresh.roles) || !fresh.roles.every(role => typeof role === "string")) throw new Error("Incomplete user response");
      if (controller.signal.aborted) return;
      setSelectedUser({...user,...fresh});
      setSelectedRoleNames(fresh.roles);
    } catch {
      if (!controller.signal.aborted) setDetailsError("Unable to load this user’s saved details. Retry to continue.");
    } finally {
      if (!controller.signal.aborted) setDetailsLoading(false);
    }
  }
  async function saveUser(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault(); if(busy) return;
    const submittedForm = event.currentTarget;
    const form = new FormData(submittedForm);
    if (dialogMode !== "reset" && dialogMode !== "status" && !rolesOptional && !selectedRoleNames.length) {setRolesError("Select at least one role."); return;}
    setBusy(true); setFormError("");setConflict(false);
    const editing = dialogMode === "edit";
    const resetting = dialogMode === "reset";
    const changingStatus = dialogMode === "status";

    let payload: Record<string, unknown>;

    if (changingStatus) {
      payload = { is_active: !selectedUser?.is_active, reason: String(form.get("reason") ?? "").trim() };
    } else if (resetting) {
      const pwd = String(form.get("password") ?? "");
      const confirm = String(form.get("confirmation") ?? "");
      const newUsername = String(form.get("username") ?? "").trim();
      const customReason = String(form.get("reason") ?? "").trim();

      if (!sendTemporaryEmail) {
        if (!pwd) {
          setFormError("New password is required.");
          return;
        }
        if (pwd !== confirm) {
          setFormError("New password and confirm password do not match.");
          return;
        }
        const hasUpper = /[A-Z]/.test(pwd);
        const hasLower = /[a-z]/.test(pwd);
        const hasNumber = /[0-9]/.test(pwd);
        const hasSymbol = /[^A-Za-z0-9]/.test(pwd);
        if (pwd.length < 8 || pwd.length > 128 || !hasUpper || !hasLower || !hasNumber || !hasSymbol) {
          setFormError("Password must be 8 to 128 characters and include uppercase, lowercase, number, and symbol.");
          return;
        }
      }

      payload = {
        reason: customReason || (sendTemporaryEmail ? "Temporary password reset requested" : "Direct password reset by administrator"),
        ...(sendTemporaryEmail ? {} : { password: pwd }),
        ...(newUsername && newUsername !== selectedUser?.username ? { username: newUsername } : {}),
      };
    } else {
      payload = {
        ...(editing ? { expected_version: selectedUser?.edit_version } : { is_superuser: actor.is_superuser === true && superadmin }),
        username: String(form.get("username") ?? "").trim(),
        first_name: String(form.get("first_name") ?? "").trim(),
        last_name: String(form.get("last_name") ?? "").trim(),
        mobile: String(form.get("mobile") ?? "").trim(),
        email: String(form.get("email") ?? "").trim(),
        ...(editing && selectedUser?.is_superuser && selectedRoleNames.length === 0 ? {} : { role_names: selectedRoleNames }),
      };
    }

    const url = changingStatus ? `/api/administration/users/${selectedUser?.id}/status` : resetting ? `/api/administration/users/${selectedUser?.id}/password` : editing ? `/api/administration/users/${selectedUser?.id}` : "/api/administration/users";
    try {
      const response = await fetch(url, {method:editing?"PATCH":"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
      if(!response.ok){if(response.status===409)setConflict(true);const error=await response.json().catch(()=>({detail:"Unable to save user."}));if(!displayServerErrors(submittedForm,error))setFormError(typeof error.detail === "string" ? error.detail : "Unable to save user.");return;}
      setDialogMode(null);if(editing&&selectedUser?.id===actor.id)router.refresh();
      setNotice(
        changingStatus
          ? selectedUser?.is_active
            ? "User deactivated. Access revoked immediately; handover can be completed later."
            : "User reactivated. A new sign-in is required; revoked sessions remain invalid."
          : resetting
            ? sendTemporaryEmail
              ? "Temporary password email queued. The user must change it at next login."
              : `Password updated successfully for ${selectedUser?.username}. The user can now sign in with the new password.`
            : editing
              ? selectedUser?.email !== String(form.get("email") ?? "").trim()
                ? "User updated. Verification was sent to the new email; the current sign-in email remains unchanged until confirmation."
                : "User updated."
              : "User created. Welcome email queued with temporary credentials and the login link."
      );
      setRevision(value=>value+1);
    } catch {setFormError("Unable to reach user administration. Please try again.");}
    finally {setBusy(false);}
  }


  return <section className="mx-auto w-full max-w-[1500px] space-y-5">
    <div className="flex flex-col gap-4 border-b border-slate-200 pb-5 lg:flex-row lg:items-end lg:justify-between"><div><p className="text-xs font-bold uppercase tracking-[0.16em] text-blue-700">Administration</p><h2 className="mt-1 text-2xl font-extrabold tracking-tight text-slate-900">Users</h2></div>{canWrite&&<button type="button" onClick={()=>openDialog("create")} className={actionButtonClass("primary")}><PlusIcon className="h-4 w-4"/>Create user</button>}</div>
    <div className="grid gap-3 sm:grid-cols-3">{[["Matching users",total],["Active on this page",users.filter(u=>u.is_active).length],["Password change due on this page",users.filter(u=>u.must_change_password).length]].map(([label,value])=><div key={label} className="rounded-2xl border border-slate-200 bg-white p-4"><span className="block text-xs font-bold uppercase tracking-wide text-slate-500">{label}</span><span className="mt-1 block text-2xl font-extrabold text-slate-900">{value}</span></div>)}</div>
    {notice&&<p role="status" className="rounded-xl border border-blue-100 bg-blue-50 p-3 text-sm text-blue-900">{notice}</p>}
    {loadError&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{loadError}</p>}
    <DataGrid toolbar={<button type="button" className={button+" px-3 text-sm font-bold"} onClick={()=>setRevision(value=>value+1)}>Refresh delivery status</button>} label="User directory" rows={users} loading={isLoading} status={u=>u.is_active?"Active":"Inactive"} initialSort="username" stateKey="users" remote={{total,onChange:changeQuery}} columns={[
      {key:"username",label:"User",value:u=>u.username,render:u=><div className="flex items-center gap-3"><UserAvatar id={u.id} name={u.username} active={u.is_active}/><div><span className="block font-bold">{u.username}</span><span className="block text-xs text-slate-500">{u.email}</span></div></div>},
      {key:"roles",label:"Assigned roles",value:u=>u.roles.join(", "),sortable:false,render:u=><div className="flex flex-wrap gap-1">{u.roles.map(role=><span key={role} className="rounded-lg bg-blue-50 px-2 py-1 text-xs font-bold text-blue-800">{role}</span>)}</div>},
      {key:"status",label:"Status",value:u=>u.is_active?"Active":"Inactive",sortable:false},
      {key:"security",label:"Account access",value:u=>u.must_change_password?"Password change required":"Ready",sortable:false,render:u=><div><span className="block">{u.must_change_password?"Password change required":"Ready"}</span><span className={`block text-xs ${u.email_status==="failed"?"text-red-700":"text-slate-500"}`}>{deliveryLabel(u.email_status)}</span></div>},
      {key:"activity",label:"Activity",value:u=>u.onlinePage??"No recent activity",sortable:false},
    ]} actions={user=><div className="flex justify-end gap-1">
      <button className={button} aria-label={`View ${user.username}`} title="View user" onClick={()=>openDialog("view",user)}><EyeIcon className="h-4 w-4"/></button>
      {canManage(user)&&<><button className={button} aria-label={`Edit ${user.username}`} title="Edit user" onClick={()=>openDialog("edit",user)}><PencilSquareIcon className="h-4 w-4"/></button>{user.id!==actor.id&&<button className={actionButtonClass(user.is_active?"danger":"success",true)} aria-label={`${user.is_active?"Deactivate":"Reactivate"} ${user.username}`} title={user.is_active?"Deactivate user":"Reactivate user"} onClick={()=>openDialog("status",user)}>{user.is_active?<NoSymbolIcon className="h-4 w-4"/>:<ArrowPathIcon className="h-4 w-4"/>}</button>}{user.is_active&&user.id!==actor.id&&<button className={actionButtonClass("warning",true)} aria-label={`Reset password for ${user.username}`} title="Reset password" onClick={()=>openDialog("reset",user)}><KeyIcon className="h-4 w-4"/></button>}</>}
    </div>}/>
    {users.some(user=>user.onlinePage)&&<div className="rounded-2xl bg-slate-950 p-5 text-white"><p className="text-xs font-bold uppercase tracking-wider text-cyan-300">Live workspace insight</p><p className="mt-2 font-bold">Online users on this page</p><div className="mt-4 flex flex-wrap gap-3">{users.filter(user=>user.onlinePage).map(user=><div key={user.id} className="rounded-xl border border-slate-700 p-3"><span className="font-bold">{user.username}</span><p className="mt-1 text-xs text-slate-300">{user.onlinePage}</p></div>)}</div></div>}
    {dialogMode&&<RecordDrawer title={dialogMode==="status"?selectedUser?.is_active?"Deactivate user":"Reactivate user":dialogMode==="create"?"Create user":dialogMode==="reset"?"Reset user password":dialogMode==="edit"?"Edit user":"User details"} onClose={closeDialog}>
      {detailsLoading ? <p role="status" className="text-sm text-slate-600">Loading saved user details…</p> : detailsError ? <div className="space-y-3"><p role="alert" className="text-sm text-red-700">{detailsError}</p><button type="button" className={button+" px-3 font-bold"} onClick={()=>openDialog(dialogMode,selectedUser)}>Retry loading user</button></div> : dialogMode==="view"&&selectedUser ? <UserDetails user={selectedUser} deliveryStatus={deliveryLabel(selectedUser.email_status)} canManage={canManage(selectedUser)} onEdit={()=>openDialog("edit",selectedUser)} onReset={()=>openDialog("reset",selectedUser)} onStatus={()=>openDialog("status",selectedUser)}/> : <ValidatedForm key={`${dialogMode}-${selectedUser?.id??"new"}`} onSubmit={saveUser} className="space-y-5">
        {dialogMode==="status" ? <>
          <p className="text-sm leading-6 text-slate-600">{selectedUser?.is_active?<>Deactivate <strong>{selectedUser.username}</strong> now. Login and existing sessions will be blocked immediately. A pending handover will be opened or the existing case retained. Work can be reassigned later, even if no reporting manager is configured.</>:<>Reactivate <strong>{selectedUser?.username}</strong>. The user must sign in again. Previously revoked sessions stay invalid, and existing handover work remains recorded.</>}</p>
          <ValidatedLabel required className="block text-sm font-bold">Status change reason<ValidatedTextarea name="reason" required minLength={10} maxLength={1000} rows={3} className={input}/></ValidatedLabel>
        </> : dialogMode==="reset" ? <>
          <p className="text-sm leading-6 text-slate-600">
            Reset credentials for <strong>{selectedUser?.username}</strong> ({selectedUser?.email}). Existing active sessions will be terminated immediately.
          </p>
          <ValidatedLabel required={actor.is_superuser} className="block text-sm font-bold">
            Username {actor.is_superuser ? <span className="font-normal text-xs text-slate-500">(editable by Superadmin)</span> : null}
            <ValidatedInput
              name="username"
              required
              minLength={3}
              maxLength={64}
              pattern="[a-zA-Z0-9._-]+"
              defaultValue={selectedUser?.username}
              readOnly={!actor.is_superuser}
              className={input + (!actor.is_superuser ? " bg-slate-100 cursor-not-allowed" : "")}
            />
          </ValidatedLabel>
          {!sendTemporaryEmail ? (
            <>
              <div className="grid gap-3 sm:grid-cols-2">
                <ValidatedLabel required className="block text-sm font-bold">
                  New password
                  <ValidatedInput
                    name="password"
                    type="password"
                    autoComplete="new-password"
                    required
                    minLength={8}
                    maxLength={128}
                    placeholder="Enter new password"
                    className={input}
                  />
                </ValidatedLabel>
                <ValidatedLabel required className="block text-sm font-bold">
                  Confirm new password
                  <ValidatedInput
                    name="confirmation"
                    type="password"
                    autoComplete="new-password"
                    required
                    minLength={8}
                    maxLength={128}
                    placeholder="Confirm new password"
                    className={input}
                  />
                </ValidatedLabel>
              </div>
              <div className="rounded-xl border border-slate-200 bg-slate-50 p-3 text-xs text-slate-600">
                <p className="font-semibold text-slate-700">Password requirements:</p>
                <p className="mt-0.5">8 to 128 characters, with at least one uppercase letter, one lowercase letter, one number, and one symbol.</p>
              </div>
            </>
          ) : (
            <p className="rounded-xl bg-amber-50 p-3 text-sm leading-6 text-amber-900">
              A temporary password will be sent to <strong>{selectedUser?.email}</strong>. The user will be required to change it upon first login.
            </p>
          )}
          <ValidatedLabel className="flex min-h-11 items-center gap-3 rounded-xl border border-slate-200 p-3 text-sm font-medium text-slate-700 cursor-pointer">
            <ValidatedInput
              type="checkbox"
              checked={sendTemporaryEmail}
              onChange={(e) => setSendTemporaryEmail(e.target.checked)}
            />
            <span>Send temporary password via email instead of setting directly</span>
          </ValidatedLabel>
          <ValidatedLabel className="block text-sm font-bold">
            Reset reason <span className="font-normal text-xs text-slate-500">(optional)</span>
            <ValidatedTextarea
              name="reason"
              maxLength={1000}
              rows={2}
              placeholder="e.g. Administrative credential update"
              className={input}
            />
          </ValidatedLabel>
        </> : <>
          {dialogMode==="edit"&&selectedUser&&<>
            <ValidatedLabel className="block text-sm font-bold">Username{actor.is_superuser && <span className="font-normal text-xs text-slate-500"> (editable by Superadmin)</span>}<ValidatedInput name="username" readOnly={!actor.is_superuser} defaultValue={selectedUser.username} className={input+(!actor.is_superuser?" bg-slate-100":"")}/></ValidatedLabel>
            {(!selectedUser.first_name||!selectedUser.last_name||!selectedUser.mobile)&&<p role="status" className="ui-notice--warning rounded-xl p-3 text-sm">No saved value for {[!selectedUser.first_name&&"first name",!selectedUser.last_name&&"last name",!selectedUser.mobile&&"mobile number"].filter(Boolean).join(", ")}. Enter the missing details to complete this user’s profile.</p>}
          </>}
          {dialogMode==="create"&&<><p className="rounded-xl bg-blue-50 p-3 text-sm leading-6 text-blue-900">The user will receive a welcome email with a generated temporary password and a link to erp.vishaltechnopower.in. They must change the password at first login.</p></>}<div className="grid gap-3 sm:grid-cols-2"><ValidatedLabel required className="block text-sm font-bold">First name<ValidatedInput name="first_name" required maxLength={100} defaultValue={selectedUser?.first_name} className={input}/></ValidatedLabel><ValidatedLabel required className="block text-sm font-bold">Last name<ValidatedInput name="last_name" required maxLength={100} defaultValue={selectedUser?.last_name} className={input}/></ValidatedLabel></div><ValidatedLabel required className="block text-sm font-bold">Mobile number<ValidatedInput name="mobile" type="tel" required minLength={8} maxLength={24} defaultValue={selectedUser?.mobile} className={input}/></ValidatedLabel>{dialogMode==="create"&&<><ValidatedLabel required className="block text-sm font-bold">Username<ValidatedInput name="username" required minLength={3} maxLength={64} pattern="[a-zA-Z0-9._-]+" className={input}/></ValidatedLabel></>}
          <ValidatedLabel required className="block text-sm font-bold">Email<ValidatedInput name="email" type="email" required maxLength={255} defaultValue={selectedUser?.email} className={input}/></ValidatedLabel>
          {dialogMode==="edit"&&selectedUser&&<div className="user-access-card space-y-3 rounded-xl border p-4 text-sm"><p className="font-bold">Account status: {selectedUser.is_active?"Active":"Inactive"}</p><p className="text-slate-600">Deactivation blocks access immediately. Handovers are tracked separately and can be completed later.</p>{selectedUser.id!==actor.id&&<button type="button" className={actionButtonClass(selectedUser.is_active?"danger":"success")} onClick={()=>openDialog("status",selectedUser)}>{selectedUser.is_active?"Deactivate user":"Reactivate user"}</button>}</div>}
          {dialogMode==="create"&&actor.is_superuser===true&&<div className="space-y-2"><ValidatedLabel className="flex min-h-11 items-center gap-3 rounded-xl border border-slate-200 p-3 text-sm font-bold"><ValidatedInput type="checkbox" name="is_superuser" checked={superadmin} onChange={event=>{setSuperadmin(event.target.checked);setRolesError("");}}/>Superadmin</ValidatedLabel><p className="text-sm text-slate-600">Full platform access. Only an existing superadmin can create another. Maximum two accounts, including inactive accounts.</p></div>}
          <fieldset><legend className="text-sm font-bold">Assigned roles {!rolesOptional&&<span aria-hidden="true" className="text-red-600">*</span>}</legend><RequiredSelection selectionCount={rolesOptional?Math.max(1,selectedRoleNames.length):selectedRoleNames.length} onInvalid={()=>setRolesError("Select at least one role.")} className="mt-3 grid gap-2 sm:grid-cols-2">{roles.filter(role=>role.is_active!==false||selectedRoleNames.includes(role.name)).map(role=><ValidatedLabel key={role.id} className="user-role-option flex min-h-11 items-center gap-3 rounded-xl border border-slate-200 bg-white p-3 text-sm"><ValidatedInput type="checkbox" checked={selectedRoleNames.includes(role.name)} disabled={role.is_active===false&&!selectedRoleNames.includes(role.name)} onChange={()=>{setRolesError("");setSelectedRoleNames(names=>names.includes(role.name)?names.filter(name=>name!==role.name):[...names,role.name]);}}/>{role.name}{role.is_active===false?" (Inactive)":""}</ValidatedLabel>)}</RequiredSelection>{rolesError&&<p role="alert" className="mt-2 text-sm text-red-700">{rolesError}</p>}</fieldset>
        </>}
        {formError&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{formError}</p>}{conflict&&dialogMode==="edit"&&<button type="button" className={actionButtonClass("secondary")} onClick={()=>{setConflict(false);void openDialog("edit",selectedUser);}}>Reload saved user (discard unsaved changes)</button>}
        <div className="record-drawer-actions sticky bottom-0 flex flex-col-reverse gap-3 py-4 sm:flex-row sm:justify-end"><button type="button" disabled={busy} onClick={closeDialog} className={actionButtonClass("secondary")}>Cancel</button><button type="submit" disabled={busy} className={actionButtonClass(dialogMode==="status"?(selectedUser?.is_active?"danger":"success"):dialogMode==="reset"?(sendTemporaryEmail?"warning":"primary"):dialogMode==="create"?"primary":"primary")}>{busy?"Saving…":dialogMode==="status"?selectedUser?.is_active?"Deactivate user now":"Reactivate user":dialogMode==="reset"?(sendTemporaryEmail?"Send temporary password":"Reset password"):dialogMode==="create"?"Create user & email access":"Save changes"}</button></div>
      </ValidatedForm>}
    </RecordDrawer>}
  </section>;
}
