"use client";

import {useEffect, useState} from "react";
import Link from "next/link";
import {useAuthenticatedUser} from "@/components/auth-user-provider";
import {ValidatedForm, ValidatedLabel, ValidatedSelect} from "@/components/ui/validated-controls";
import type {ItemListMode} from "@/components/purchase-fms/purchase-items-model";
import {purchaseIndentReadPermissions, purchaseIndentSettingsWrite} from "@/lib/purchase-indent-settings";
import {PurchaseIndentSettingsForm} from "./purchase-indent-settings";

const endpoint = "/api/purchase-fms/application-settings";
const action = "app-action inline-flex min-h-11 items-center rounded-lg px-4 font-semibold disabled:opacity-50";

export function ApplicationSettings({openAccount}: {openAccount: (mode: "profile" | "password") => void}) {
  const {permissions} = useAuthenticatedUser();
  const admin = permissions.includes("system:admin");
  const indent = admin || permissions.includes(purchaseIndentSettingsWrite) ||
    purchaseIndentReadPermissions.every(permission => permissions.includes(permission));
  const tender = admin || permissions.includes("settings.tsc_formats.view");
  return <section className="space-y-6">
    <h1 className="text-2xl font-bold">Settings</h1>
    <section className="space-y-3">
      <h2 className="text-lg font-semibold">My account</h2>
      <div className="flex flex-wrap gap-2">
        <button type="button" className={action} onClick={() => openAccount("profile")}>Edit profile</button>
        <button type="button" className={action} onClick={() => openAccount("password")}>Change password</button>
      </div>
    </section>
    {(indent || admin) && <section className="space-y-3">
      <h2 className="text-lg font-semibold">Purchase FMS</h2>
      {indent && <PurchaseIndentSettingsForm />}
      {admin && <PurchaseItemListSettings />}
    </section>}
    {tender && <section className="space-y-3">
      <h2 className="text-lg font-semibold">Tender FMS</h2>
      <Link href="/dashboard?screen=tsc-formats" className={action}>TSC formats</Link>
    </section>}
  </section>;
}

function PurchaseItemListSettings() {
 const [saved,setSaved]=useState<ItemListMode|null>(null);
 const [mode,setMode]=useState<ItemListMode>("single");
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 const [notice,setNotice]=useState("");
 const [reload,setReload]=useState(0);

 useEffect(()=>{
  const controller=new AbortController();
  fetch(endpoint,{cache:"no-store",signal:controller.signal}).then(async response=>{
   const data=await response.json();
   if(!response.ok)throw Error(typeof data.detail==="string"?data.detail:"Unable to load application settings.");
   if(!controller.signal.aborted){setSaved(data.item_list_mode);setMode(data.item_list_mode);setError("");}
  }).catch(failure=>{if(!controller.signal.aborted)setError(failure.message);});
  return ()=>controller.abort();
 },[reload]);

 async function save() {
  setBusy(true);
  setError("");
  setNotice("");
  try {
   const response=await fetch(endpoint,{method:"PATCH",headers:{"Content-Type":"application/json"},body:JSON.stringify({item_list_mode:mode})});
   const data=await response.json();
   if(!response.ok)throw Error(typeof data.detail==="string"?data.detail:"Unable to save application settings.");
   setSaved(data.item_list_mode);
   setMode(data.item_list_mode);
   setNotice("Purchase settings saved.");
  } catch(failure) {
   setError(failure instanceof Error?failure.message:"Unable to save application settings.");
  } finally {
   setBusy(false);
  }
 }

 return <section className="space-y-4">
  {error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-red-800">{error}</p>}
  {notice&&<p role="status" className="rounded-lg bg-green-50 p-3 text-green-800">{notice}</p>}
  {saved===null?error?<button type="button" className="min-h-11 rounded-lg border px-4" onClick={()=>setReload(value=>value+1)}>Retry</button>:<p role="status">Loading application settings…</p>:
   <ValidatedForm className="max-w-2xl space-y-4 rounded-xl border bg-white p-5" onSubmit={event=>{event.preventDefault();if(!busy&&mode!==saved)void save();}}>
    <ValidatedLabel className="block font-semibold">Item lists per project
     <ValidatedSelect className="mt-2 block min-h-11 w-full max-w-sm rounded-lg border px-3" value={mode} disabled={busy} onChange={event=>{setMode(event.target.value==="multiple"?"multiple":"single");setNotice("");}}>
      <option value="single">Single list</option>
      <option value="multiple">Multiple lists</option>
     </ValidatedSelect>
    </ValidatedLabel>
    <p className="text-sm text-slate-600">{mode==="single"?"Show all items together and use one list for new projects.":"Show the prepared list selector and allow multiple lists per project."}</p>
    <button type="submit" className={action} disabled={busy||mode===saved}>{busy?"Saving…":"Save item list settings"}</button>
   </ValidatedForm>}
 </section>;
}
