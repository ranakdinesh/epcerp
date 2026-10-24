"use client";

import { useCallback, useEffect, useState, type SubmitEvent } from "react";
import {useRouter} from "next/navigation";
import { EyeIcon, PencilSquareIcon, KeyIcon, PlusIcon } from "@heroicons/react/24/outline";
import { useAuthenticatedUser } from "@/components/auth-user-provider";
import { UserAvatar, UserIdentity } from "@/components/personal-profile";
import { DataGrid, type GridQuery } from "@/components/ui/data-grid";
import { RecordDrawer } from "@/components/ui/record-drawer";
import { displayServerErrors } from "@/components/ui/server-field-errors";
import { RequiredSelection, ValidatedForm, ValidatedInput, ValidatedLabel, ValidatedTextarea } from "@/components/ui/validated-controls";

type PlatformUser = { id: string; username: string; first_name:string;last_name:string;mobile:string; email: string; roles: string[]; is_active: boolean; is_superuser: boolean; must_change_password: boolean; email_status: string; onlinePage?: string };
type Role = { id: string; name: string };
type DialogMode = "create" | "view" | "edit" | "reset" | null;
const button = "inline-flex min-h-11 min-w-11 items-center justify-center rounded-xl border border-slate-200 text-slate-600 hover:bg-blue-50 hover:text-blue-800 focus:outline-none focus:ring-2 focus:ring-blue-600";
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
  const [rolesError, setRolesError] = useState("");
  const [busy, setBusy] = useState(false);
  const [dialogMode, setDialogMode] = useState<DialogMode>(null);
  const [selectedUser, setSelectedUser] = useState<PlatformUser | null>(null);
  const [selectedRoleNames, setSelectedRoleNames] = useState<string[]>([]);
  const [superadmin, setSuperadmin] = useState(false);
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

  function openDialog(mode: DialogMode, user: PlatformUser | null = null) {
    setRolesError(""); setFormError(""); setSelectedUser(user); setSelectedRoleNames(user?.roles ?? []); setSuperadmin(false); setDialogMode(mode);
  }
  async function saveUser(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault(); if(busy) return;
    const submittedForm = event.currentTarget;
    const form = new FormData(submittedForm);
    if (dialogMode !== "reset" && !rolesOptional && !selectedRoleNames.length) {setRolesError("Select at least one role."); return;}
    setBusy(true); setFormError("");
    const editing = dialogMode === "edit";
    const resetting = dialogMode === "reset";
    const payload = resetting ? {reason:String(form.get("reason") ?? "").trim()} : { ...(editing ? {} : {is_superuser:actor.is_superuser === true && superadmin,username:String(form.get("username") ?? "").trim(),first_name:String(form.get("first_name")??"").trim(),last_name:String(form.get("last_name")??"").trim(),mobile:String(form.get("mobile")??"").trim()}),email:String(form.get("email") ?? "").trim(),...(editing && selectedUser?.is_superuser && selectedRoleNames.length===0 ? {} : {role_names:selectedRoleNames})};
    const url = resetting ? `/api/administration/users/${selectedUser?.id}/password` : editing ? `/api/administration/users/${selectedUser?.id}` : "/api/administration/users";
    try {
      const response = await fetch(url, {method:editing?"PATCH":"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
      if(!response.ok){const error=await response.json().catch(()=>({detail:"Unable to save user."}));if(!displayServerErrors(submittedForm,error))setFormError(typeof error.detail === "string" ? error.detail : "Unable to save user.");return;}
      setDialogMode(null);if(editing&&selectedUser?.id===actor.id)router.refresh();setNotice(resetting ? "Temporary password email queued. The user must change it at next login." : editing ? "User updated." : "User created. Welcome email queued with temporary credentials and the login link.");setRevision(value=>value+1);
    } catch {setFormError("Unable to reach user administration. Please try again.");}
    finally {setBusy(false);}
  }


  return <section className="mx-auto w-full max-w-[1500px] space-y-5">
    <div className="flex flex-col gap-4 border-b border-slate-200 pb-5 lg:flex-row lg:items-end lg:justify-between"><div><p className="text-xs font-bold uppercase tracking-[0.16em] text-blue-700">Administration</p><h2 className="mt-1 text-2xl font-extrabold tracking-tight text-slate-900">Users</h2></div>{canWrite&&<button type="button" onClick={()=>openDialog("create")} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white"><PlusIcon className="h-4 w-4"/>Create user</button>}</div>
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
      {canManage(user)&&<><button className={button} aria-label={`Edit ${user.username}`} title="Edit user" onClick={()=>openDialog("edit",user)}><PencilSquareIcon className="h-4 w-4"/></button>{user.is_active&&user.id!==actor.id&&<button className={button} aria-label={`Reset password for ${user.username}`} title="Reset password" onClick={()=>openDialog("reset",user)}><KeyIcon className="h-4 w-4"/></button>}</>}
    </div>}/>
    {users.some(user=>user.onlinePage)&&<div className="rounded-2xl bg-slate-950 p-5 text-white"><p className="text-xs font-bold uppercase tracking-wider text-cyan-300">Live workspace insight</p><p className="mt-2 font-bold">Online users on this page</p><div className="mt-4 flex flex-wrap gap-3">{users.filter(user=>user.onlinePage).map(user=><div key={user.id} className="rounded-xl border border-slate-700 p-3"><span className="font-bold">{user.username}</span><p className="mt-1 text-xs text-slate-300">{user.onlinePage}</p></div>)}</div></div>}
    {dialogMode&&<RecordDrawer title={dialogMode==="create"?"Create user":dialogMode==="reset"?"Reset user password":dialogMode==="edit"?"Edit user":"User details"} onClose={()=>{if(!busy)setDialogMode(null);}}>
      {dialogMode==="view"&&selectedUser ? <div className="space-y-5"><UserIdentity id={selectedUser.id} name={selectedUser.username} active={selectedUser.is_active}/><p>{selectedUser.first_name} {selectedUser.last_name}</p><p>{selectedUser.email}</p><p>{selectedUser.mobile||"Mobile setup required"}</p><a className="text-sm font-bold text-blue-700 underline" href="/dashboard?screen=organisation">Open organisation profile</a><p className="text-sm">Access: {selectedUser.is_superuser?"Superadmin (full platform access)":selectedUser.roles.join(", ")}</p><p className="text-sm">{deliveryLabel(selectedUser.email_status)}</p>{canManage(selectedUser)&&selectedUser.is_active&&selectedUser.id!==actor.id&&<div className="flex flex-wrap gap-3"><button className={button+" gap-2 px-3"} onClick={()=>openDialog("reset",selectedUser)}><KeyIcon className="h-4 w-4"/>Reset password</button><a className={button+" gap-2 px-3"} href="/dashboard?screen=offboarding">Start handover</a></div>}{formError&&<p role="alert" className="text-sm text-red-700">{formError}</p>}</div> : <ValidatedForm key={`${dialogMode}-${selectedUser?.id??"new"}`} onSubmit={saveUser} className="space-y-5">
        {dialogMode==="reset" ? <><p className="text-sm leading-6 text-slate-600">Send a new temporary password to <strong>{selectedUser?.email}</strong>. Existing credentials and sessions will be revoked. The temporary password expires in 24 hours and must be changed at first login.</p><ValidatedLabel required className="block text-sm font-bold">Reset reason<ValidatedTextarea name="reason" required minLength={10} maxLength={1000} className={input} rows={3}/></ValidatedLabel></> : <>
          {dialogMode==="create"&&<><p className="rounded-xl bg-blue-50 p-3 text-sm leading-6 text-blue-900">The user will receive a welcome email with a generated temporary password and a link to erp.vishaltechnopower.in. They must change the password at first login.</p><div className="grid gap-3 sm:grid-cols-2"><ValidatedLabel required className="block text-sm font-bold">First name<ValidatedInput name="first_name" required maxLength={100} className={input}/></ValidatedLabel><ValidatedLabel required className="block text-sm font-bold">Last name<ValidatedInput name="last_name" required maxLength={100} className={input}/></ValidatedLabel></div><ValidatedLabel required className="block text-sm font-bold">Mobile number<ValidatedInput name="mobile" type="tel" required minLength={8} maxLength={24} className={input}/></ValidatedLabel><ValidatedLabel required className="block text-sm font-bold">Username<ValidatedInput name="username" required minLength={3} maxLength={64} pattern="[a-zA-Z0-9._-]+" className={input}/></ValidatedLabel></>}
          <ValidatedLabel required className="block text-sm font-bold">Email<ValidatedInput name="email" type="email" required maxLength={255} defaultValue={selectedUser?.email} className={input}/></ValidatedLabel>
          {dialogMode==="create"&&actor.is_superuser===true&&<div className="space-y-2"><ValidatedLabel className="flex min-h-11 items-center gap-3 rounded-xl border border-slate-200 p-3 text-sm font-bold"><ValidatedInput type="checkbox" name="is_superuser" checked={superadmin} onChange={event=>{setSuperadmin(event.target.checked);setRolesError("");}}/>Superadmin</ValidatedLabel><p className="text-sm text-slate-600">Full platform access. Only an existing superadmin can create another. Maximum two accounts, including inactive accounts.</p></div>}
          <fieldset><legend className="text-sm font-bold">Assigned roles {!rolesOptional&&<span aria-hidden="true" className="text-red-600">*</span>}</legend><RequiredSelection selectionCount={rolesOptional?Math.max(1,selectedRoleNames.length):selectedRoleNames.length} onInvalid={()=>setRolesError("Select at least one role.")} className="mt-3 grid gap-2 sm:grid-cols-2">{roles.map(role=><ValidatedLabel key={role.id} className="flex min-h-11 items-center gap-3 rounded-xl border border-slate-200 bg-white p-3 text-sm"><ValidatedInput type="checkbox" checked={selectedRoleNames.includes(role.name)} onChange={()=>{setRolesError("");setSelectedRoleNames(names=>names.includes(role.name)?names.filter(name=>name!==role.name):[...names,role.name]);}}/>{role.name}</ValidatedLabel>)}</RequiredSelection>{rolesError&&<p role="alert" className="mt-2 text-sm text-red-700">{rolesError}</p>}</fieldset>
        </>}
        {formError&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-800">{formError}</p>}
        <div className="sticky bottom-0 flex flex-col-reverse gap-3 border-t border-slate-200 bg-slate-50 py-4 sm:flex-row sm:justify-end"><button type="button" disabled={busy} onClick={()=>setDialogMode(null)} className={button+" px-4"}>Cancel</button><button type="submit" disabled={busy} className="min-h-11 rounded-xl bg-blue-700 px-4 text-sm font-bold text-white disabled:opacity-50">{busy?"Saving…":dialogMode==="reset"?"Send temporary password":dialogMode==="create"?"Create user & email access":"Save changes"}</button></div>
      </ValidatedForm>}
    </RecordDrawer>}
  </section>;
}
