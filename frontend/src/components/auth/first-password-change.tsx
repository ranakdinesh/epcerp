"use client";
import { useState, type SubmitEvent } from "react";
import { useRouter } from "next/navigation";
import { ValidatedForm, ValidatedLabel, ValidatedInput } from "@/components/ui/validated-controls";
import { displayServerErrors } from "@/components/ui/server-field-errors";
const input="mt-2 w-full rounded-xl border border-slate-300 bg-white px-4 py-3 text-slate-900 focus:outline-none focus:ring-2 focus:ring-blue-700";
export default function FirstPasswordChange(){
 const router=useRouter();const[busy,setBusy]=useState(false);const[error,setError]=useState("");const[confirmError,setConfirmError]=useState("");
 async function submit(event:SubmitEvent<HTMLFormElement>){
  event.preventDefault();if(busy)return;const form=event.currentTarget;const data=new FormData(form);setError("");setConfirmError("");
  if(data.get("password")!==data.get("confirm_password")){setConfirmError("Passwords must match.");form.querySelector<HTMLInputElement>('[name="confirm_password"]')?.focus();return;}
  setBusy(true);
  try{
   const response=await fetch("/api/auth/me/password",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({current_password:data.get("current_password"),password:data.get("password")})});
   if(response.status===401){router.replace("/login?session=expired");return;}
   if(!response.ok){const result=await response.json().catch(()=>({detail:"Unable to update password."}));if(!displayServerErrors(form,result))setError(typeof result.detail==="string"?result.detail:"Unable to update password.");return;}
   form.reset();router.replace("/login?session=password-changed");router.refresh();
  }catch{setError("Unable to reach account security. Please try again.");}finally{setBusy(false);}
 }
 async function logout(){await fetch("/api/auth/logout",{method:"POST"});router.replace("/login?session=logged-out");router.refresh();}
 return <main className="min-h-screen bg-slate-50 px-6 py-8 text-slate-900"><div className="mx-auto max-w-[480px] rounded-3xl border border-slate-200 bg-white p-6 shadow-xl shadow-slate-200/30 sm:mt-12 sm:p-8"><p className="text-xs font-bold uppercase tracking-widest text-blue-800">Vishal Technopower · epcERP</p><h1 className="mt-3 text-2xl font-extrabold">Choose your personal password</h1><p className="mt-3 text-sm leading-6 text-slate-600">Your temporary password gives access to this security step. Change it to continue to your workspace.</p>
 <ValidatedForm onSubmit={submit} className="mt-6 space-y-5">
  <ValidatedLabel required className="block text-sm font-bold">Temporary password<ValidatedInput name="current_password" type="password" autoComplete="current-password" required maxLength={128} className={input}/></ValidatedLabel>
  <ValidatedLabel required className="block text-sm font-bold">New password<ValidatedInput name="password" type="password" autoComplete="new-password" required minLength={8} maxLength={128} pattern={"(?=.*\\p{Ll})(?=.*\\p{Lu})(?=.*\\p{Nd})(?=.*[^\\p{L}\\p{N}]).{8,128}"} title="Use 8 to 128 characters with uppercase, lowercase, a number and a symbol." className={input}/></ValidatedLabel>
  <p className="text-xs leading-5 text-slate-500">Use 8–128 characters with uppercase, lowercase, a number and a symbol. Choose a different password from the temporary one.</p>
  <ValidatedLabel required className="block text-sm font-bold">Confirm new password<ValidatedInput name="confirm_password" type="password" autoComplete="new-password" required maxLength={128} error={confirmError} onChange={()=>setConfirmError("")} className={input}/></ValidatedLabel>
  {error&&<p role="alert" className="rounded-xl bg-red-50 p-3 text-sm text-red-700">{error}</p>}
  <button disabled={busy} className="min-h-11 w-full rounded-xl bg-blue-800 px-4 py-3 text-sm font-bold text-white disabled:opacity-50">{busy?"Updating…":"Save password & return to login"}</button>
 </ValidatedForm><button type="button" disabled={busy} onClick={()=>void logout()} className="mt-4 min-h-11 text-sm font-bold text-blue-800">Sign out</button></div></main>;
}
