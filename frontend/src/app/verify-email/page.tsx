"use client";
import {useEffect,useState} from "react";
import {actionButtonClass} from "@/components/ui/action-button";
export default function VerifyEmail(){
 const[token,setToken]=useState("");const[busy,setBusy]=useState(false);const[done,setDone]=useState(false);const[error,setError]=useState("");
 useEffect(()=>{const raw=new URLSearchParams(window.location.hash.slice(1)).get("token")??"";const timer=setTimeout(()=>{setToken(raw);window.history.replaceState(null,"","/verify-email");},0);return()=>clearTimeout(timer);},[]);
 async function verify(){setBusy(true);setError("");try{const r=await fetch("/api/auth/verify-email",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({token})});if(!r.ok){const e=await r.json();throw Error(e.detail||"Unable to verify email");}setDone(true);setToken("");}catch(e){setError(e instanceof Error?e.message:"Unable to verify email");}finally{setBusy(false);}}
 return <main className="mx-auto mt-16 max-w-lg space-y-5 rounded-xl border border-slate-300 bg-white p-6 text-slate-900"><h1 className="text-2xl font-bold">Verify your email address</h1>{done?<><p role="status">Your new email address is verified. Sign in again using your new email or your username.</p><a className={actionButtonClass("primary")} href="/login?session=email-verified">Sign in</a></>:<><p>Confirm this address for your epcERP account. Your existing sessions will be signed out.</p>{error&&<p role="alert">{error}</p>}<button className={actionButtonClass("primary")} disabled={busy||token.length!==64} onClick={()=>void verify()}>{busy?"Verifying…":"Verify email address"}</button>{!token&&<p>Open the complete verification link from your email.</p>}</>}</main>;
}
